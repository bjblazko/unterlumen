package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"

	// Register image decoders
	_ "golang.org/x/image/webp"
	_ "image/gif"
)

// ExifData holds all extracted EXIF metadata from an image file.
type ExifData struct {
	Tags          map[string]string `json:"tags,omitempty"`
	Width         int               `json:"width,omitempty"`
	Height        int               `json:"height,omitempty"`
	Latitude      *float64          `json:"latitude,omitempty"`
	Longitude     *float64          `json:"longitude,omitempty"`
	DateTaken     *string           `json:"dateTaken,omitempty"`
	DateDigitized *string           `json:"dateDigitized,omitempty"`
	DateModified  *string           `json:"dateModified,omitempty"`
}

// exifWalker collects EXIF tags into a map.
type exifWalker struct {
	tags map[string]string
}

func (w *exifWalker) Walk(name exif.FieldName, tag *tiff.Tag) error {
	switch string(name) {
	case "ExifIFDPointer", "GPSInfoIFDPointer", "InteroperabilityIFDPointer",
		"ThumbJPEGInterchangeFormat", "ThumbJPEGInterchangeFormatLength",
		"MakerNote":
		return nil
	}
	w.tags[string(name)] = tag.String()
	return nil
}

// ExtractAllEXIF reads all EXIF metadata from an image file.
// For JPEG/TIFF files, decodes EXIF directly. For HEIF/HEIC/HIF files,
// scans the ISOBMFF container for embedded EXIF data.
// Returns nil with no error for files that have no EXIF data.
func ExtractAllEXIF(path string) (*ExifData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	x, err := decodeFileExif(f, path)
	if err != nil {
		return nil, err
	}

	data := &ExifData{Tags: make(map[string]string)}
	x.Walk(&exifWalker{tags: data.Tags})

	if sim := extractFujiFilmSimulation(x); sim != "" {
		data.Tags["FilmSimulation"] = sim
	}

	data.DateTaken = parseExifDateTag(data.Tags, "DateTimeOriginal", "OffsetTimeOriginal")
	data.DateDigitized = parseExifDateTag(data.Tags, "DateTimeDigitized", "OffsetTimeDigitized")
	data.DateModified = parseExifDateTag(data.Tags, "DateTime", "OffsetTime")

	lat, lon, err := x.LatLong()
	if err == nil {
		data.Latitude = &lat
		data.Longitude = &lon
	}

	data.Width, data.Height = exifImageDimensions(x)
	return data, nil
}

// ExtractDateTaken returns the EXIF DateTimeOriginal from an image file.
func ExtractDateTaken(path string) (time.Time, error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return time.Time{}, err
	}
	return x.DateTime()
}

// ExtractDateAndMeta performs a single EXIF decode pass to extract the date taken,
// GPS presence, and Fujifilm film simulation. Falls back to decodeEmbeddedExif
// for HEIF/HEIC/HIF files, fixing HEIF date extraction that ExtractDateTaken misses.
func ExtractDateAndMeta(path string) (time.Time, *EntryMeta, error) {
	if IsVideo(path) {
		return videoDateAndMeta(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, nil, err
	}
	defer f.Close()

	x, err := decodeFileExif(f, path)
	if err != nil {
		return time.Time{}, nil, err
	}

	dt, dtErr := x.DateTime()
	meta := buildEntryMeta(x)

	if dtErr != nil {
		return time.Time{}, meta, dtErr
	}
	return dt, meta, nil
}

func buildEntryMeta(x *exif.Exif) *EntryMeta {
	meta := &EntryMeta{}
	if _, _, err := x.LatLong(); err == nil {
		meta.HasGPS = true
	}
	if sim := extractFujiFilmSimulation(x); sim != "" {
		meta.FilmSimulation = sim
	}
	w, h := exifImageDimensions(x)
	if ar := AspectRatioLabel(w, h); ar != "" {
		meta.AspectRatio = ar
	}
	return meta
}

// extractJPEGOrientation reads the EXIF orientation tag from raw JPEG bytes.
// Returns 1 (normal) on any error or missing tag.
func extractJPEGOrientation(data []byte) int {
	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	v, err := tag.Int(0)
	if err != nil || v < 1 || v > 8 {
		return 1
	}
	return v
}

// ExtractOrientation reads the EXIF orientation tag (1–8) from an image file.
// Returns 1 (normal) on any error or missing tag.
func ExtractOrientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return 1
	}

	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}

	v, err := tag.Int(0)
	if err != nil || v < 1 || v > 8 {
		return 1
	}
	return v
}

// ExtractHEIFOrientation reads the irot (image rotation) box from a HEIF file.
// HEIF stores rotation in the ISOBMFF container, not in EXIF.
// Returns an EXIF-compatible orientation value (1–8), defaults to 1.
func ExtractHEIFOrientation(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return 1
	}
	defer f.Close()

	buf := make([]byte, 16*1024)
	n, err := f.Read(buf)
	if err != nil || n < 9 {
		return 1
	}
	buf = buf[:n]

	for i := 0; i <= len(buf)-9; i++ {
		if string(buf[i+4:i+8]) == "irot" {
			boxSize := binary.BigEndian.Uint32(buf[i : i+4])
			if boxSize == 9 {
				switch buf[i+8] & 0x03 {
				case 0:
					return 1
				case 1:
					return 8
				case 2:
					return 3
				case 3:
					return 6
				}
			}
		}
	}
	return 1
}

// heifExifOrientation reads the EXIF orientation tag from a HEIF file's
// embedded EXIF metadata block. Returns 1 if not found.
func heifExifOrientation(path string) int {
	x, err := decodeEmbeddedExif(path)
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	v, err := tag.Int(0)
	if err != nil || v < 1 || v > 8 {
		return 1
	}
	return v
}

// heifOrientation returns the display orientation for a HEIF file.
// It prefers the container irot box; if irot is not set it falls back
// to the embedded EXIF orientation (used by Fujifilm and similar cameras
// that store rotation in EXIF rather than the irot box).
func heifOrientation(path string) int {
	if ori := ExtractHEIFOrientation(path); ori != 1 {
		return ori
	}
	return heifExifOrientation(path)
}

// decodeEmbeddedExif scans a file for an embedded EXIF block.
// HEIF/HEIC/HIF files store EXIF data inside their ISOBMFF container.
// The string "Exif" also appears in iinf/infe boxes as an item type name,
// so we validate each match by checking for a valid TIFF header ("II*\0"
// or "MM\0*") before attempting to decode.
// decodeFileExif reads the EXIF of an open file. A JPEG or TIFF goes to the
// decoder; anything else — HEIF above all — has its EXIF block found in its
// first megabyte. Handing a HEIF to the decoder first made it read the whole
// file looking for a JPEG marker before failing: on a network share, about
// half a second per photo before the right reader even started.
func decodeFileExif(f *os.File, path string) (*exif.Exif, error) {
	head := make([]byte, 4)
	if _, err := io.ReadFull(f, head); err != nil {
		return nil, err
	}
	if !isJPEGOrTIFF(head) {
		return decodeEmbeddedExif(path)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	x, err := exif.Decode(f)
	if err != nil {
		return decodeEmbeddedExif(path)
	}
	return x, nil
}

func isJPEGOrTIFF(head []byte) bool {
	return bytes.HasPrefix(head, []byte{0xFF, 0xD8}) ||
		bytes.Equal(head, []byte("II*\x00")) || bytes.Equal(head, []byte("MM\x00*"))
}

// decodeEmbeddedExif finds the EXIF block in a container such as HEIF,
// where it sits in the meta box near the start: in the first 64 KB for the
// cameras seen so far (byte 357 for Fujifilm), within the first megabyte for
// any. The small read comes first; over a network share each megabyte read
// costs about a tenth of a second.
func decodeEmbeddedExif(path string) (*exif.Exif, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	for _, size := range []int{64 << 10, 1 << 20} {
		buf := make([]byte, size)
		n, err := f.ReadAt(buf, 0)
		if n == 0 && err != nil {
			return nil, err
		}
		if x := findEmbeddedExif(buf[:n]); x != nil {
			return x, nil
		}
		if n < size {
			break // the whole file has been looked at
		}
	}
	return nil, errors.New("no embedded EXIF data found")
}

// findEmbeddedExif looks for an "Exif" block, then for a bare TIFF header.
func findEmbeddedExif(buf []byte) *exif.Exif {
	marker := []byte("Exif\x00\x00")
	for offset := 0; ; {
		idx := bytes.Index(buf[offset:], marker)
		if idx < 0 {
			break
		}
		tiffStart := offset + idx + len(marker)
		if tiffStart+8 <= len(buf) && isTIFFHeader(buf[tiffStart:]) {
			if x, err := exif.Decode(bytes.NewReader(buf[tiffStart:])); err == nil {
				return x
			}
		}
		offset += idx + 1
	}
	for _, hdr := range [][]byte{{0x49, 0x49, 0x2a, 0x00}, {0x4d, 0x4d, 0x00, 0x2a}} {
		for offset := 0; ; {
			idx := bytes.Index(buf[offset:], hdr)
			if idx < 0 {
				break
			}
			if tiffStart := offset + idx; tiffStart+8 <= len(buf) {
				if x, err := exif.Decode(bytes.NewReader(buf[tiffStart:])); err == nil {
					return x
				}
			}
			offset += idx + 1
		}
	}
	return nil
}

// isTIFFHeader checks whether data starts with a valid TIFF byte order mark.
func isTIFFHeader(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return (data[0] == 0x49 && data[1] == 0x49 && data[2] == 0x2a && data[3] == 0x00) ||
		(data[0] == 0x4d && data[1] == 0x4d && data[2] == 0x00 && data[3] == 0x2a)
}

// parseExifDateTag parses an EXIF date string and optional offset from the Tags map
// into a normalized ISO 8601 string. Returns nil if the date tag is absent or unparseable.
func parseExifDateTag(tags map[string]string, dateKey, offsetKey string) *string {
	raw, ok := tags[dateKey]
	if !ok {
		return nil
	}
	raw = strings.Trim(raw, `"`)
	t, err := time.Parse("2006:01:02 15:04:05", raw)
	if err != nil {
		return nil
	}
	iso := t.Format("2006-01-02T15:04:05")
	if off, ok := tags[offsetKey]; ok {
		if off = strings.Trim(off, `"`); off != "" {
			iso += off
		}
	}
	return &iso
}

// exifImageDimensions returns the actual image dimensions from EXIF tags.
func exifImageDimensions(x *exif.Exif) (int, int) {
	if tag, err := x.Get(exif.PixelXDimension); err == nil {
		if w, err := tag.Int(0); err == nil {
			if tag2, err := x.Get(exif.PixelYDimension); err == nil {
				if h, err := tag2.Int(0); err == nil {
					return w, h
				}
			}
		}
	}
	if tag, err := x.Get(exif.ImageWidth); err == nil {
		if w, err := tag.Int(0); err == nil {
			if tag2, err := x.Get(exif.ImageLength); err == nil {
				if h, err := tag2.Int(0); err == nil {
					return w, h
				}
			}
		}
	}
	return 0, 0
}
