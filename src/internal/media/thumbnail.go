package media

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"strings"

	"golang.org/x/image/draw"
)

const thumbnailCacheVersion = "v4"

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

// ExtractThumbnailCached extracts the embedded EXIF thumbnail (orientation-corrected)
// and caches the result to disk. Concurrency is limited by the shared worker pool.
// Returns an error if the file has no usable embedded thumbnail.
func ExtractThumbnailCached(ctx context.Context, path string) ([]byte, string, error) {
	key := cacheKey(path, "thumb-exif-v2")
	if cached := readCache(key); cached != nil {
		return cached, "image/jpeg", nil
	}

	result := thumbnailWork.run(ctx, key, func() thumbnailWorkResult {
		if cached := readCache(key); cached != nil {
			return thumbnailWorkResult{data: cached, contentType: "image/jpeg"}
		}

		data, err := extractOrientedEXIFThumbnail(path)
		if err != nil {
			return thumbnailWorkResult{err: err}
		}
		writeCache(key, data)
		return thumbnailWorkResult{data: data, contentType: "image/jpeg"}
	})
	return result.data, result.contentType, result.err
}

// GenerateThumbnailCached caches source-based thumbnails by file mtime and size.
func GenerateThumbnailCached(ctx context.Context, path string, maxDim int, orientation int) ([]byte, string, error) {
	key := cacheKey(path, fmt.Sprintf("thumb-source-%s-%d-%d", thumbnailCacheVersion, orientation, maxDim))
	if cached := readCache(key); cached != nil {
		return cached, detectThumbnailContentType(cached), nil
	}

	result := thumbnailWork.run(ctx, key, func() thumbnailWorkResult {
		if cached := readCache(key); cached != nil {
			return thumbnailWorkResult{data: cached, contentType: detectThumbnailContentType(cached)}
		}

		data, ct, err := GenerateThumbnail(path, maxDim, orientation)
		if err != nil {
			return thumbnailWorkResult{err: err}
		}
		writeCache(key, data)
		return thumbnailWorkResult{data: data, contentType: ct}
	})
	return result.data, result.contentType, result.err
}

// ExtractHEIFPreviewThumbnail returns a standard-quality HEIF thumbnail.
// It starts from the cached preview JPEG and only resizes when the preview is
// still larger than the requested thumbnail.
func ExtractHEIFPreviewThumbnail(ctx context.Context, path string, maxDim int) ([]byte, error) {
	key := cacheKey(path, fmt.Sprintf("thumb-heif-preview-%s-%d", thumbnailCacheVersion, maxDim))
	if cached := readCache(key); cached != nil {
		return cached, nil
	}

	result := thumbnailWork.run(ctx, key, func() thumbnailWorkResult {
		if cached := readCache(key); cached != nil {
			return thumbnailWorkResult{data: cached}
		}

		jpegData, err := extractHEIFPreview(path, maxDim)
		if err != nil {
			return thumbnailWorkResult{err: err}
		}
		thumb, err := ResizeJPEGBytes(jpegData, maxDim)
		if err != nil {
			return thumbnailWorkResult{err: err}
		}
		writeCache(key, thumb)
		return thumbnailWorkResult{data: thumb}
	})
	return result.data, result.err
}

// GenerateHEIFThumbnail returns a high-quality HEIF thumbnail generated from
// the full decoded source image rather than the embedded preview JPEG.
func GenerateHEIFThumbnail(ctx context.Context, path string, maxDim int) ([]byte, error) {
	key := cacheKey(path, fmt.Sprintf("thumb-heif-source-%s-%d", thumbnailCacheVersion, maxDim))
	if cached := readCache(key); cached != nil {
		return cached, nil
	}

	result := thumbnailWork.run(ctx, key, func() thumbnailWorkResult {
		if cached := readCache(key); cached != nil {
			return thumbnailWorkResult{data: cached}
		}

		jpegData, err := convertHEIFExport(path)
		if err != nil {
			return thumbnailWorkResult{err: err}
		}
		thumb, err := ResizeJPEGBytes(jpegData, maxDim)
		if err != nil {
			return thumbnailWorkResult{err: err}
		}
		writeCache(key, thumb)
		return thumbnailWorkResult{data: thumb}
	})
	return result.data, result.err
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
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width <= maxDim && cfg.Height <= maxDim {
		return data, nil
	}

	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	w, h := cfg.Width, cfg.Height

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

func cachedResizeJPEGBytes(path, purpose string, data []byte, maxDim int) ([]byte, error) {
	key := cacheKey(path, fmt.Sprintf("%s-%d", purpose, maxDim))
	if cached := readCache(key); cached != nil {
		return cached, nil
	}

	resized, err := ResizeJPEGBytes(data, maxDim)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(resized, data) {
		writeCache(key, resized)
	}
	return resized, nil
}

func detectThumbnailContentType(data []byte) string {
	contentType := http.DetectContentType(data)
	switch contentType {
	case "image/png", "image/gif", "image/webp", "image/jpeg":
		return contentType
	default:
		return "image/jpeg"
	}
}
