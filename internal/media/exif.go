package media

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"strings"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"

	// Register image decoders
	_ "image/gif"
	_ "golang.org/x/image/webp"
)

// ExifData holds all extracted EXIF metadata from an image file.
type ExifData struct {
	Tags      map[string]string `json:"tags,omitempty"`
	Width     int               `json:"width,omitempty"`
	Height    int               `json:"height,omitempty"`
	Latitude  *float64          `json:"latitude,omitempty"`
	Longitude *float64          `json:"longitude,omitempty"`
}

// exifWalker collects EXIF tags into a map.
type exifWalker struct {
	tags map[string]string
}

func (w *exifWalker) Walk(name exif.FieldName, tag *tiff.Tag) error {
	// Skip internal pointer tags and binary blobs
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

	x, err := exif.Decode(f)
	if err != nil {
		// Standard decode failed — try scanning for embedded EXIF
		// (handles HEIF/HEIC/HIF where EXIF is inside the ISOBMFF container)
		x, err = decodeEmbeddedExif(path)
		if err != nil {
			return nil, err
		}
	}

	data := &ExifData{
		Tags: make(map[string]string),
	}

	// Walk all tags
	walker := &exifWalker{tags: data.Tags}
	x.Walk(walker)

	// Extract GPS coordinates
	lat, lon, err := x.LatLong()
	if err == nil {
		data.Latitude = &lat
		data.Longitude = &lon
	}

	// Extract dimensions
	data.Width, data.Height = exifImageDimensions(x)

	return data, nil
}

// decodeEmbeddedExif scans a file for an embedded EXIF block.
// HEIF/HEIC/HIF files store EXIF data inside their ISOBMFF container.
// The string "Exif" also appears in iinf/infe boxes as an item type name,
// so we validate each match by checking for a valid TIFF header ("II*\0"
// or "MM\0*") before attempting to decode.
func decodeEmbeddedExif(path string) (*exif.Exif, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read up to 1MB — EXIF metadata is in the container's meta box,
	// typically near the start of the file.
	buf := make([]byte, 1024*1024)
	n, err := f.Read(buf)
	if err != nil {
		return nil, err
	}
	buf = buf[:n]

	// Search for "Exif\0\0" markers followed by a valid TIFF header.
	// HEIF files may contain "Exif\0" as an item type name in infe boxes,
	// so we skip matches that aren't followed by a TIFF header.
	marker := []byte("Exif\x00\x00")
	offset := 0
	for {
		idx := bytes.Index(buf[offset:], marker)
		if idx < 0 {
			break
		}
		tiffStart := offset + idx + len(marker)
		if tiffStart+8 <= len(buf) && isTIFFHeader(buf[tiffStart:]) {
			x, err := exif.Decode(bytes.NewReader(buf[tiffStart:]))
			if err == nil {
				return x, nil
			}
		}
		offset += idx + 1
	}

	// Fallback: search for bare TIFF headers ("II*\0" little-endian,
	// "MM\0*" big-endian). Some HEIF variants store EXIF with a 4-byte
	// offset prefix and no "Exif\0\0" marker.
	for _, hdr := range [][]byte{{0x49, 0x49, 0x2a, 0x00}, {0x4d, 0x4d, 0x00, 0x2a}} {
		offset = 0
		for {
			idx := bytes.Index(buf[offset:], hdr)
			if idx < 0 {
				break
			}
			tiffStart := offset + idx
			if tiffStart+8 <= len(buf) {
				x, err := exif.Decode(bytes.NewReader(buf[tiffStart:]))
				if err == nil {
					return x, nil
				}
			}
			offset += idx + 1
		}
	}

	return nil, errors.New("no embedded EXIF data found")
}

// isTIFFHeader checks whether data starts with a valid TIFF byte order mark.
func isTIFFHeader(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	// Little-endian: "II*\0"
	if data[0] == 0x49 && data[1] == 0x49 && data[2] == 0x2a && data[3] == 0x00 {
		return true
	}
	// Big-endian: "MM\0*"
	if data[0] == 0x4d && data[1] == 0x4d && data[2] == 0x00 && data[3] == 0x2a {
		return true
	}
	return false
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
	if err != nil {
		return 1
	}

	if v < 1 || v > 8 {
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

	// Read enough of the file to find the irot box (typically in the first few KB)
	buf := make([]byte, 16*1024)
	n, err := f.Read(buf)
	if err != nil || n < 9 {
		return 1
	}
	buf = buf[:n]

	// Search for irot box: 4-byte size + "irot" + 1-byte angle
	for i := 0; i <= len(buf)-9; i++ {
		if string(buf[i+4:i+8]) == "irot" {
			boxSize := binary.BigEndian.Uint32(buf[i : i+4])
			if boxSize == 9 {
				angle := buf[i+8] & 0x03
				// Map irot angle (CCW in 90° units) to EXIF orientation
				switch angle {
				case 0:
					return 1 // no rotation
				case 1:
					return 8 // 90° CCW = EXIF rotate 270° CW
				case 2:
					return 3 // 180°
				case 3:
					return 6 // 270° CCW = EXIF rotate 90° CW
				}
			}
		}
	}

	return 1
}

// applyOrientationJPEG decodes JPEG data, applies orientation, and re-encodes.
// Returns the original data unchanged if orientation <= 1.
func applyOrientationJPEG(data []byte, orientation int, quality int) ([]byte, error) {
	if orientation <= 1 {
		return data, nil
	}

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return data, err
	}

	rotated := applyOrientation(img, orientation)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, rotated, &jpeg.Options{Quality: quality}); err != nil {
		return data, err
	}

	return buf.Bytes(), nil
}

// applyOrientation transforms an image according to the EXIF orientation tag.
// Orientations 2–8 are mapped to the corresponding pixel coordinate remapping.
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	minX := bounds.Min.X
	minY := bounds.Min.Y

	// Orientations 5–8 swap width and height (90°/270° rotations + transposes)
	var dstW, dstH int
	if orientation >= 5 {
		dstW, dstH = h, w
	} else {
		dstW, dstH = w, h
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			var srcX, srcY int
			switch orientation {
			case 2: // flip horizontal
				srcX, srcY = w-1-x, y
			case 3: // rotate 180
				srcX, srcY = w-1-x, h-1-y
			case 4: // flip vertical
				srcX, srcY = x, h-1-y
			case 5: // transpose
				srcX, srcY = y, x
			case 6: // rotate 90 CW
				srcX, srcY = y, h-1-x
			case 7: // transverse
				srcX, srcY = w-1-y, h-1-x
			case 8: // rotate 270 CW
				srcX, srcY = w-1-y, x
			}
			dst.Set(x, y, img.At(minX+srcX, minY+srcY))
		}
	}

	return dst
}

// ExtractThumbnail tries to extract the embedded EXIF thumbnail from a JPEG file.
// When orientation > 1, the thumbnail is decoded, rotated, and re-encoded.
// Returns an error if the thumbnail's aspect ratio doesn't match the actual image
// (e.g., camera stores full-sensor thumbnail for an in-camera 1:1 crop).
func ExtractThumbnail(path string, orientation int) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		return nil, "", err
	}

	thumb, err := x.JpegThumbnail()
	if err != nil {
		return nil, "", err
	}

	// Validate thumbnail aspect ratio against actual image dimensions.
	// Some cameras store a full-sensor thumbnail even for in-camera crops,
	// producing a thumbnail with the wrong aspect ratio.
	thumbCfg, cfgErr := jpeg.DecodeConfig(bytes.NewReader(thumb))
	if cfgErr == nil {
		imgW, imgH := exifImageDimensions(x)
		if imgW > 0 && imgH > 0 && thumbCfg.Width > 0 && thumbCfg.Height > 0 {
			imgAR := float64(imgW) / float64(imgH)
			thumbAR := float64(thumbCfg.Width) / float64(thumbCfg.Height)
			if math.Abs(imgAR-thumbAR)/imgAR > 0.1 {
				return nil, "", errors.New("thumbnail aspect ratio mismatch")
			}
		}
	}

	if orientation > 1 {
		img, decErr := jpeg.Decode(bytes.NewReader(thumb))
		if decErr == nil {
			rotated := applyOrientation(img, orientation)
			var buf bytes.Buffer
			if encErr := jpeg.Encode(&buf, rotated, &jpeg.Options{Quality: 80}); encErr == nil {
				return buf.Bytes(), "image/jpeg", nil
			}
		}
	}

	return thumb, "image/jpeg", nil
}

// exifImageDimensions returns the actual image dimensions from EXIF tags.
// Tries PixelXDimension/PixelYDimension first, then ImageWidth/ImageLength.
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

// GenerateThumbnail creates a thumbnail by decoding and resizing the image.
// Orientation is applied after decoding and before resizing.
// Used as fallback when no EXIF thumbnail is available.
func GenerateThumbnail(path string, maxDim int, orientation int) ([]byte, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", err
	}

	if orientation > 1 {
		img = applyOrientation(img, orientation)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if w <= maxDim && h <= maxDim {
		if orientation <= 1 {
			// No rotation needed, serve raw file
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, "", err
			}
			ct := "image/jpeg"
			if format == "png" {
				ct = "image/png"
			}
			return data, ct, nil
		}
		// Rotation was applied, must re-encode
		var buf bytes.Buffer
		ct := "image/jpeg"
		if strings.HasSuffix(strings.ToLower(path), ".png") {
			ct = "image/png"
			err = png.Encode(&buf, img)
		} else {
			err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80})
		}
		if err != nil {
			return nil, "", err
		}
		return buf.Bytes(), ct, nil
	}

	// Calculate new dimensions maintaining aspect ratio
	var newW, newH int
	if w > h {
		newW = maxDim
		newH = h * maxDim / w
	} else {
		newH = maxDim
		newW = w * maxDim / h
	}

	// Simple nearest-neighbor resize
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := x * w / newW
			srcY := y * h / newH
			dst.Set(x, y, img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	var buf bytes.Buffer
	ct := "image/jpeg"
	if strings.HasSuffix(strings.ToLower(path), ".png") {
		ct = "image/png"
		err = png.Encode(&buf, dst)
	} else {
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 80})
	}
	if err != nil {
		return nil, "", err
	}

	return buf.Bytes(), ct, nil
}

// ResizeJPEGBytes takes JPEG image data and resizes it to fit within maxDim.
// Returns the resized JPEG bytes.
func ResizeJPEGBytes(data []byte, maxDim int) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if w <= maxDim && h <= maxDim {
		return data, nil
	}

	var newW, newH int
	if w > h {
		newW = maxDim
		newH = h * maxDim / w
	} else {
		newH = maxDim
		newW = w * maxDim / h
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		for x := 0; x < newW; x++ {
			srcX := x * w / newW
			srcY := y * h / newH
			dst.Set(x, y, img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY))
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
