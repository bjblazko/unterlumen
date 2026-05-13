package media

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// CropImage crops a photo in-place.
// x, y, w, h are fractions [0,1] of the visually-rendered (orientation-applied) image.
// The original file is not touched until the final atomic rename.
func CropImage(srcPath string, x, y, w, h float64) error {
	if IsHEIF(srcPath) {
		return cropHEIF(srcPath, x, y, w, h)
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	if ext == ".webp" {
		return cropWebP(srcPath, x, y, w, h)
	}
	return cropStandard(srcPath, ext, x, y, w, h)
}

func cropStandard(srcPath, ext string, x, y, w, h float64) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}

	if orientation := ExtractOrientation(srcPath); orientation > 1 {
		img = applyOrientation(img, orientation)
	}

	cropped, err := cropRect(img, x, y, w, h)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(srcPath), ".crop_tmp_*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	if err := encodeForCrop(tmp, cropped, ext); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("encode: %w", err)
	}
	tmp.Close()

	if err := cropCopyMetadata(srcPath, tmpPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, srcPath)
}

// cropRect cuts the given fraction rect out of img and returns a new *image.RGBA.
func cropRect(img image.Image, x, y, w, h float64) (image.Image, error) {
	b := img.Bounds()
	vW, vH := b.Dx(), b.Dy()
	cX := clampInt(int(x*float64(vW)), 0, vW)
	cY := clampInt(int(y*float64(vH)), 0, vH)
	cW := clampInt(int(w*float64(vW)), 0, vW-cX)
	cH := clampInt(int(h*float64(vH)), 0, vH-cY)
	if cW <= 0 || cH <= 0 {
		return nil, fmt.Errorf("crop region is empty")
	}
	dst := image.NewRGBA(image.Rect(0, 0, cW, cH))
	draw.Draw(dst, dst.Bounds(), img, image.Point{X: b.Min.X + cX, Y: b.Min.Y + cY}, draw.Src)
	return dst, nil
}

func encodeForCrop(f *os.File, img image.Image, ext string) error {
	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
	case ".png":
		return png.Encode(f, img)
	case ".gif":
		return gif.Encode(f, img, nil)
	}
	return fmt.Errorf("unsupported format: %s", ext)
}

func cropWebP(srcPath string, x, y, w, h float64) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	img, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		return fmt.Errorf("decode WebP: %w", err)
	}

	b := img.Bounds()
	vW, vH := b.Dx(), b.Dy()
	cX := clampInt(int(x*float64(vW)), 0, vW)
	cY := clampInt(int(y*float64(vH)), 0, vH)
	cW := clampInt(int(w*float64(vW)), 0, vW-cX)
	cH := clampInt(int(h*float64(vH)), 0, vH-cY)
	if cW <= 0 || cH <= 0 {
		return fmt.Errorf("crop region is empty")
	}

	tmp, err := os.CreateTemp(filepath.Dir(srcPath), ".crop_tmp_*.webp")
	if err != nil {
		return err
	}
	tmp.Close()
	tmpPath := tmp.Name()

	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg",
		"-i", srcPath,
		"-vf", fmt.Sprintf("crop=%d:%d:%d:%d", cW, cH, cX, cY),
		"-c:v", "libwebp",
		"-quality", "90",
		"-y", tmpPath,
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg WebP crop: %v: %s", err, stderr.String())
	}

	if err := cropCopyMetadata(srcPath, tmpPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, srcPath)
}

func cropHEIF(srcPath string, x, y, w, h float64) error {
	if !CheckSips() {
		return fmt.Errorf("HEIF crop requires sips (macOS only)")
	}

	// sips reports visual (orientation-applied) pixel dimensions.
	vW, vH, err := sipsGetDimensions(srcPath)
	if err != nil {
		return fmt.Errorf("get dimensions: %w", err)
	}

	cX := clampInt(int(x*float64(vW)), 0, vW)
	cY := clampInt(int(y*float64(vH)), 0, vH)
	cW := clampInt(int(w*float64(vW)), 0, vW-cX)
	cH := clampInt(int(h*float64(vH)), 0, vH-cY)
	if cW <= 0 || cH <= 0 {
		return fmt.Errorf("crop region is empty")
	}

	// Create temp file with .heic extension so sips outputs HEIF.
	tmp, err := os.CreateTemp(filepath.Dir(srcPath), ".crop_tmp_*.heic")
	if err != nil {
		return err
	}
	tmp.Close()
	tmpPath := tmp.Name()
	os.Remove(tmpPath) // sips creates the output file itself

	var stderr bytes.Buffer
	cmd := exec.Command("sips",
		"--cropToHeightWidth", strconv.Itoa(cH), strconv.Itoa(cW),
		"--cropOffset", strconv.Itoa(cY), strconv.Itoa(cX),
		srcPath, "--out", tmpPath,
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("sips crop: %v: %s", err, stderr.String())
	}

	if err := cropCopyMetadata(srcPath, tmpPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return os.Rename(tmpPath, srcPath)
}

// cropCopyMetadata copies all metadata from srcPath to dstPath using exiftool,
// excluding dimension tags (which the encoder has already set correctly) and
// clearing the Orientation tag since the image is now stored correctly oriented.
// Non-fatal: if exiftool is unavailable the error is returned but callers may
// choose to surface it as a warning.
func cropCopyMetadata(srcPath, dstPath string) error {
	if !CheckExiftool() {
		return fmt.Errorf("exiftool is not available; metadata not preserved")
	}

	var stderr bytes.Buffer
	cmd := exec.Command("exiftool",
		"-TagsFromFile", srcPath,
		"-all:all",
		"--ImageWidth", "--ImageHeight",
		"--ExifImageWidth", "--ExifImageHeight",
		"--PixelXDimension", "--PixelYDimension",
		"-Orientation=",
		"-overwrite_original",
		dstPath,
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exiftool metadata copy: %v: %s", err, stderr.String())
	}
	return nil
}

// sipsGetDimensions returns the visual (orientation-applied) pixel dimensions
// of an image file as reported by sips.
func sipsGetDimensions(path string) (int, int, error) {
	var out, stderr bytes.Buffer
	cmd := exec.Command("sips", "-g", "pixelWidth", "-g", "pixelHeight", path)
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, 0, fmt.Errorf("sips -g: %v: %s", err, stderr.String())
	}
	return parseSipsDimensions(out.String())
}

// parseSipsDimensions parses output like:
//
//	/path/to/file.heic
//	  pixelWidth: 4032
//	  pixelHeight: 3024
func parseSipsDimensions(output string) (int, int, error) {
	var w, h int
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "pixelWidth: "); ok {
			v, err := strconv.Atoi(strings.TrimSpace(after))
			if err == nil {
				w = v
			}
		}
		if after, ok := strings.CutPrefix(line, "pixelHeight: "); ok {
			v, err := strconv.Atoi(strings.TrimSpace(after))
			if err == nil {
				h = v
			}
		}
	}
	if w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("could not parse sips dimensions from output: %q", output)
	}
	return w, h, nil
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
