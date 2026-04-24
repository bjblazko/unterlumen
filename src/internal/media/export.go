package media

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
)

// ScaleMode defines how the output image should be scaled.
type ScaleMode string

const (
	ScaleModeNone    ScaleMode = "none"
	ScaleModePercent ScaleMode = "percent"
	ScaleModePixels  ScaleMode = "pixels"
	ScaleModeMaxDim  ScaleMode = "max_dim"
)

// ScaleOptions defines the scaling parameters.
type ScaleOptions struct {
	Mode         ScaleMode `json:"mode"`
	Percent      float64   `json:"percent,omitempty"`      // ScaleModePercent
	Width        int       `json:"width,omitempty"`        // ScaleModePixels
	Height       int       `json:"height,omitempty"`       // ScaleModePixels
	MaintainAR   bool      `json:"maintainAR,omitempty"`   // ScaleModePixels: fit within box
	MaxDimension string    `json:"maxDimension,omitempty"` // "width" or "height", ScaleModeMaxDim
	MaxValue     int       `json:"maxValue,omitempty"`     // ScaleModeMaxDim
}

// ExportOptions controls how an image should be exported.
type ExportOptions struct {
	Format   string      `json:"format"`   // "jpeg", "png", "webp"
	Quality  int         `json:"quality"`  // 1–100, ignored for PNG
	Scale    ScaleOptions `json:"scale"`
	ExifMode string      `json:"exifMode"` // "strip", "keep", "keep_no_gps"
}

// ExportedName returns the output filename for srcName with the given format.
func ExportedName(srcName, format string) string {
	ext := filepath.Ext(srcName)
	base := strings.TrimSuffix(srcName, ext)
	switch format {
	case "jpeg":
		return base + ".jpg"
	case "png":
		return base + ".png"
	case "webp":
		return base + ".webp"
	default:
		return base + ".jpg"
	}
}

// ExportMIMEType returns the MIME type for the given export format.
func ExportMIMEType(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

// computeTargetDims calculates the output dimensions given input dimensions and scale options.
func computeTargetDims(origW, origH int, scale ScaleOptions) (int, int) {
	switch scale.Mode {
	case ScaleModePercent:
		pct := scale.Percent
		if pct <= 0 {
			pct = 100
		}
		return max(1, int(float64(origW)*pct/100)), max(1, int(float64(origH)*pct/100))

	case ScaleModePixels:
		if scale.Width <= 0 && scale.Height <= 0 {
			return origW, origH
		}
		w, h := scale.Width, scale.Height
		if w <= 0 {
			w = origW
		}
		if h <= 0 {
			h = origH
		}
		if !scale.MaintainAR {
			return w, h
		}
		// Fit within w×h box, maintain aspect ratio
		rw := float64(w) / float64(origW)
		rh := float64(h) / float64(origH)
		r := rw
		if rh < rw {
			r = rh
		}
		return max(1, int(float64(origW)*r)), max(1, int(float64(origH)*r))

	case ScaleModeMaxDim:
		if scale.MaxValue <= 0 {
			return origW, origH
		}
		if scale.MaxDimension == "height" {
			ratio := float64(scale.MaxValue) / float64(origH)
			return max(1, int(float64(origW)*ratio)), scale.MaxValue
		}
		// Default: constrain by width
		ratio := float64(scale.MaxValue) / float64(origW)
		return scale.MaxValue, max(1, int(float64(origH)*ratio))
	}

	// ScaleModeNone or unknown
	return origW, origH
}

// getImageDims returns image width and height using the cheapest available method.
// For HEIF it uses EXIF metadata; for others it reads the image header only.
func getImageDims(srcPath string) (int, int) {
	if IsHEIF(srcPath) {
		exifData, err := ExtractAllEXIF(srcPath)
		if err == nil && exifData != nil && exifData.Width > 0 {
			return exifData.Width, exifData.Height
		}
		return 0, 0
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// ExportImage converts an image file to the specified format with the given options.
// This is the reusable core function — safe to call from any context.
func ExportImage(srcPath string, opts ExportOptions) ([]byte, error) {
	if opts.Quality <= 0 || opts.Quality > 100 {
		opts.Quality = 85
	}
	if opts.Format == "" {
		opts.Format = "jpeg"
	}

	if opts.Format == "webp" {
		return exportWebP(srcPath, opts)
	}

	img, err := decodeSourceImage(srcPath)
	if err != nil {
		return nil, err
	}

	scaled := scaleImage(img, opts.Scale)

	encoded, err := encodeToFormat(scaled, opts)
	if err != nil {
		return nil, err
	}

	if opts.ExifMode == "keep" || opts.ExifMode == "keep_no_gps" {
		if patched, err := injectExif(srcPath, encoded, opts.Format, opts.ExifMode); err == nil {
			encoded = patched
		}
	}

	return encoded, nil
}

// decodeSourceImage opens and decodes a source image, applying EXIF orientation.
// For HEIF, uses full-resolution decode to avoid low-res embedded previews.
func decodeSourceImage(srcPath string) (image.Image, error) {
	if IsHEIF(srcPath) {
		// Use full-resolution decode (not the embedded preview used by the viewer).
		// ConvertHEIFToJPEG prefers the embedded JPEG stream which may be a low-res
		// preview (e.g. 1920×1280 inside a 7728×5152 HEIF), causing percentage and
		// max-dimension scaling to operate on the wrong base dimensions.
		jpegBytes, err := convertHEIFExport(srcPath)
		if err != nil {
			return nil, fmt.Errorf("HEIF full-res decode: %w", err)
		}
		img, _, err := image.Decode(bytes.NewReader(jpegBytes))
		if err != nil {
			return nil, fmt.Errorf("decode converted HEIF: %w", err)
		}
		// Orientation already applied by convertHEIFExport.
		return img, nil
	}

	f, err := os.Open(srcPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if orientation := ExtractOrientation(srcPath); orientation > 1 {
		img = applyOrientation(img, orientation)
	}
	return img, nil
}

func scaleImage(img image.Image, scale ScaleOptions) image.Image {
	origW := img.Bounds().Dx()
	origH := img.Bounds().Dy()
	targetW, targetH := computeTargetDims(origW, origH, scale)
	if targetW == origW && targetH == origH {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

func encodeToFormat(img image.Image, opts ExportOptions) ([]byte, error) {
	var buf bytes.Buffer
	switch opts.Format {
	case "jpeg":
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: opts.Quality}); err != nil {
			return nil, fmt.Errorf("JPEG encode: %w", err)
		}
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("PNG encode: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported format: %s", opts.Format)
	}
	return buf.Bytes(), nil
}

// exportWebP converts an image to WebP using ffmpeg with lanczos scaling.
func exportWebP(srcPath string, opts ExportOptions) ([]byte, error) {
	inputPath := srcPath

	var ffArgs []string

	scaleFilter := buildFFmpegScaleFilter(srcPath, opts.Scale)
	if scaleFilter != "" {
		ffArgs = append(ffArgs, "-vf", scaleFilter)
	}

	ffArgs = append(ffArgs,
		"-quality", fmt.Sprintf("%d", opts.Quality),
		"-map_metadata", "-1",
		"-f", "webp",
		"pipe:1",
	)

	encoded, err := ffmpegRun(inputPath, ffArgs...)
	if err != nil {
		return nil, fmt.Errorf("WebP encode via ffmpeg: %w", err)
	}

	// EXIF injection (non-fatal).
	if opts.ExifMode == "keep" || opts.ExifMode == "keep_no_gps" {
		if patched, err := injectExif(srcPath, encoded, "webp", opts.ExifMode); err == nil {
			encoded = patched
		}
	}

	return encoded, nil
}

// buildFFmpegScaleFilter returns the ffmpeg -vf scale filter string.
func buildFFmpegScaleFilter(srcPath string, scale ScaleOptions) string {
	if scale.Mode == ScaleModeNone || scale.Mode == "" {
		return ""
	}

	origW, origH := getImageDims(srcPath)
	if origW <= 0 || origH <= 0 {
		return ""
	}

	targetW, targetH := computeTargetDims(origW, origH, scale)
	if targetW == origW && targetH == origH {
		return ""
	}

	return fmt.Sprintf("scale=%d:%d:flags=lanczos", targetW, targetH)
}

// injectExif copies EXIF from srcPath into data using exiftool.
// mode: "keep" copies all metadata; "keep_no_gps" copies all except GPS.
func injectExif(srcPath string, data []byte, format, mode string) ([]byte, error) {
	if !CheckExiftool() {
		return nil, fmt.Errorf("exiftool not available")
	}

	ext := "." + format
	if format == "jpeg" {
		ext = ".jpg"
	}

	tmp, err := os.CreateTemp("", "unterlumen-export-*"+ext)
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return nil, err
	}
	tmp.Close()

	var args []string
	if mode == "keep_no_gps" {
		args = []string{"-TagsFromFile", srcPath, "-GPS:All=", "-overwrite_original", tmpPath}
	} else {
		args = []string{"-TagsFromFile", srcPath, "-overwrite_original", tmpPath}
	}

	cmd := exec.Command("exiftool", args...)
	cmd.Stderr = &bytes.Buffer{}
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("exiftool: %w", err)
	}

	return os.ReadFile(tmpPath)
}

// GetSourceDims returns the pixel dimensions of an image file without fully decoding it.
func GetSourceDims(srcPath string) (int, int) {
	return getImageDims(srcPath)
}

// EstimateSize returns the input file size, estimated output size, source and output dimensions.
// Uses heuristic formulas without encoding the image — fast and cheap.
func EstimateSize(srcPath string, opts ExportOptions) (inputBytes, outputBytes int64, origW, origH, outW, outH int, err error) {
	info, statErr := os.Stat(srcPath)
	if statErr != nil {
		return 0, 0, 0, 0, 0, 0, statErr
	}
	inputBytes = info.Size()

	origW, origH = getImageDims(srcPath)
	if origW <= 0 {
		// Fallback dimensions for estimation when decode fails
		origW, origH = 3000, 2000
	}

	outW, outH = computeTargetDims(origW, origH, opts.Scale)

	pixels := int64(outW) * int64(outH)
	q := int64(opts.Quality)
	if q <= 0 || q > 100 {
		q = 85
	}

	switch opts.Format {
	case "jpeg":
		// Empirically: ~0.1 bytes/pixel at quality=85 for typical photos
		outputBytes = pixels * 3 * q / 100 / 8
	case "webp":
		// WebP is roughly 25–35% smaller than JPEG at equivalent quality
		outputBytes = pixels * 3 * q / 100 / 11
	case "png":
		// Lossless PNG: roughly 25% of raw pixel data for photos
		outputBytes = pixels * 3 / 4
	default:
		outputBytes = pixels * 3 * q / 100 / 8
	}

	return inputBytes, outputBytes, origW, origH, outW, outH, nil
}
