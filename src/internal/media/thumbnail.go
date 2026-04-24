package media

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"strings"

	"golang.org/x/image/draw"
)

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
	w, h := bounds.Dx(), bounds.Dy()

	if w <= maxDim && h <= maxDim {
		return serveSmallImage(path, img, format, orientation)
	}

	return resizeImage(img, path, maxDim, w, h)
}

func serveSmallImage(path string, img image.Image, format string, orientation int) ([]byte, string, error) {
	if orientation <= 1 {
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
	return encodeImage(img, path)
}

func resizeImage(img image.Image, path string, maxDim, w, h int) ([]byte, string, error) {
	var newW, newH int
	if w > h {
		newW = maxDim
		newH = h * maxDim / w
	} else {
		newH = maxDim
		newW = w * maxDim / h
	}
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return encodeImage(dst, path)
}

func encodeImage(img image.Image, path string) ([]byte, string, error) {
	var buf bytes.Buffer
	if strings.HasSuffix(strings.ToLower(path), ".png") {
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/png", nil
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/jpeg", nil
}

// ResizeJPEGBytes takes JPEG image data and resizes it to fit within maxDim.
func ResizeJPEGBytes(data []byte, maxDim int) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

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
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
