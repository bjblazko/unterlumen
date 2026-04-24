package media

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"math"
	"os"

	"github.com/rwcarlsen/goexif/exif"
)

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
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, applyOrientation(img, orientation), &jpeg.Options{Quality: quality}); err != nil {
		return data, err
	}
	return buf.Bytes(), nil
}

// applyOrientation transforms an image according to the EXIF orientation tag.
func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	minX := bounds.Min.X
	minY := bounds.Min.Y

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
			case 2:
				srcX, srcY = w-1-x, y
			case 3:
				srcX, srcY = w-1-x, h-1-y
			case 4:
				srcX, srcY = x, h-1-y
			case 5:
				srcX, srcY = y, x
			case 6:
				srcX, srcY = y, h-1-x
			case 7:
				srcX, srcY = w-1-y, h-1-x
			case 8:
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

	if err := validateThumbnailAspect(x, thumb); err != nil {
		return nil, "", err
	}

	if orientation > 1 {
		if rotated, err := rotateThumbnail(thumb, orientation); err == nil {
			return rotated, "image/jpeg", nil
		}
	}
	return thumb, "image/jpeg", nil
}

func validateThumbnailAspect(x *exif.Exif, thumb []byte) error {
	thumbCfg, err := jpeg.DecodeConfig(bytes.NewReader(thumb))
	if err != nil {
		return nil // can't validate, assume OK
	}
	imgW, imgH := exifImageDimensions(x)
	if imgW <= 0 || imgH <= 0 || thumbCfg.Width <= 0 || thumbCfg.Height <= 0 {
		return nil
	}
	imgAR := float64(imgW) / float64(imgH)
	thumbAR := float64(thumbCfg.Width) / float64(thumbCfg.Height)
	if math.Abs(imgAR-thumbAR)/imgAR > 0.1 {
		return errors.New("thumbnail aspect ratio mismatch")
	}
	return nil
}

func rotateThumbnail(thumb []byte, orientation int) ([]byte, error) {
	img, err := jpeg.Decode(bytes.NewReader(thumb))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, applyOrientation(img, orientation), &jpeg.Options{Quality: 80}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
