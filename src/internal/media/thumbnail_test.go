package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCachedResizeJPEGBytesUsesCache(t *testing.T) {
	useTempThumbnailCache(t)

	sourcePath := filepath.Join(t.TempDir(), "image.heic")
	if err := os.WriteFile(sourcePath, []byte("heif"), 0o600); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	original := testJPEGBytes(t, 400, 200)
	resized, err := cachedResizeJPEGBytes(sourcePath, "test-preview", original, 100)
	if err != nil {
		t.Fatalf("cachedResizeJPEGBytes first call: %v", err)
	}
	assertJPEGSize(t, resized, 100, 50)

	cached, err := cachedResizeJPEGBytes(sourcePath, "test-preview", []byte("not a jpeg"), 100)
	if err != nil {
		t.Fatalf("cachedResizeJPEGBytes second call: %v", err)
	}
	if !bytes.Equal(resized, cached) {
		t.Fatalf("cached result mismatch")
	}
}

func TestGenerateThumbnailCachedUsesCache(t *testing.T) {
	useTempThumbnailCache(t)

	sourcePath := filepath.Join(t.TempDir(), "image.jpg")
	original := testJPEGBytes(t, 320, 160)
	if err := os.WriteFile(sourcePath, original, 0o600); err != nil {
		t.Fatalf("write source image: %v", err)
	}

	thumb, ct, err := GenerateThumbnailCached(context.Background(), sourcePath, 100, 1)
	if err != nil {
		t.Fatalf("GenerateThumbnailCached first call: %v", err)
	}
	if ct != "image/jpeg" {
		t.Fatalf("content type = %q, want image/jpeg", ct)
	}
	assertJPEGSize(t, thumb, 100, 50)

	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatalf("stat source image: %v", err)
	}
	if err := os.WriteFile(sourcePath, []byte("broken"), 0o600); err != nil {
		t.Fatalf("overwrite source image: %v", err)
	}
	if err := os.Chtimes(sourcePath, info.ModTime(), info.ModTime()); err != nil {
		t.Fatalf("restore mtime: %v", err)
	}

	cached, cachedCT, err := GenerateThumbnailCached(context.Background(), sourcePath, 100, 1)
	if err != nil {
		t.Fatalf("GenerateThumbnailCached second call: %v", err)
	}
	if cachedCT != "image/jpeg" {
		t.Fatalf("cached content type = %q, want image/jpeg", cachedCT)
	}
	if !bytes.Equal(thumb, cached) {
		t.Fatalf("cached thumbnail mismatch")
	}
}

func useTempThumbnailCache(t *testing.T) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	cacheDir = ""
	cacheDirOnce = sync.Once{}
}

func testJPEGBytes(t *testing.T, w, h int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 180, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode test jpeg: %v", err)
	}
	return buf.Bytes()
}

func assertJPEGSize(t *testing.T, data []byte, wantW, wantH int) {
	t.Helper()

	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode jpeg config: %v", err)
	}
	if cfg.Width != wantW || cfg.Height != wantH {
		t.Fatalf("jpeg size = %dx%d, want %dx%d", cfg.Width, cfg.Height, wantW, wantH)
	}
}
