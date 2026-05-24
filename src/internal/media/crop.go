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

	// sips --cropOffset uses an ambiguous coordinate space that varies with how the
	// HEIC stores rotation metadata (irot box vs. embedded-JPEG EXIF). To avoid this,
	// decode to a visual JPEG first (sips applies all orientation sources, baking
	// rotation into pixel data), crop in Go image space (unambiguous), then re-encode
	// back to HEIC. This two-pass approach is slower but guarantees correct coordinates.
	displayJPEG, err := sipsConvert(srcPath)
	if err != nil {
		return fmt.Errorf("HEIF decode: %w", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(displayJPEG))
	if err != nil {
		return fmt.Errorf("JPEG decode: %w", err)
	}

	// sipsConvert may return the stored (encoded) pixels with EXIF orientation
	// preserved rather than baked in. Apply orientation explicitly so the crop
	// is computed in the visual (display) coordinate space.
	if ori := extractJPEGOrientation(displayJPEG); ori > 1 {
		img = applyOrientation(img, ori)
	}

	cropped, err := cropRect(img, x, y, w, h)
	if err != nil {
		return err
	}

	var jpegBuf bytes.Buffer
	if err := jpeg.Encode(&jpegBuf, cropped, &jpeg.Options{Quality: 92}); err != nil {
		return fmt.Errorf("JPEG encode: %w", err)
	}

	dir := filepath.Dir(srcPath)

	// Write cropped JPEG to a temp file.
	tmpJPG, err := os.CreateTemp(dir, ".crop_tmp_*.jpg")
	if err != nil {
		return err
	}
	tmpJPGPath := tmpJPG.Name()
	_, writeErr := tmpJPG.Write(jpegBuf.Bytes())
	tmpJPG.Close()
	if writeErr != nil {
		os.Remove(tmpJPGPath)
		return writeErr
	}
	defer os.Remove(tmpJPGPath)

	// Convert cropped JPEG to HEIC.
	tmpHEIC, err := os.CreateTemp(dir, ".crop_tmp_*.heic")
	if err != nil {
		return err
	}
	tmpHEIC.Close()
	tmpHEICPath := tmpHEIC.Name()
	os.Remove(tmpHEICPath) // sips creates the output file itself

	var stderr bytes.Buffer
	cmd := exec.Command("sips", "-s", "format", "heic", tmpJPGPath, "--out", tmpHEICPath)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmpHEICPath)
		return fmt.Errorf("sips HEIC encode: %v: %s", err, stderr.String())
	}

	if err := cropCopyMetadata(srcPath, tmpHEICPath); err != nil {
		os.Remove(tmpHEICPath)
		return err
	}

	return os.Rename(tmpHEICPath, srcPath)
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

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
