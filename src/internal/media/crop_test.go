package media

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func containsLibwebp(out []byte) bool {
	return bytes.Contains(out, []byte("libwebp"))
}

// buildOrientedWebPFixture creates a raw (stored) WebP image of size rawW x
// rawH, black except for a red "marker" square in its raw top-left corner,
// tags it with EXIF Orientation=6 (rotate 90 CW for display), and returns
// its path. Requires ffmpeg and exiftool.
func buildOrientedWebPFixture(t *testing.T, dir string, rawW, rawH, marker int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, rawW, rawH))
	for y := 0; y < rawH; y++ {
		for x := 0; x < rawW; x++ {
			c := color.RGBA{0, 0, 0, 255}
			if x < marker && y < marker {
				c = color.RGBA{255, 0, 0, 255} // marker: raw top-left corner
			}
			img.Set(x, y, c)
		}
	}

	pngPath := filepath.Join(dir, "base.png")
	f, err := os.Create(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatal(err)
	}
	f.Close()

	webpPath := filepath.Join(dir, "fixture.webp")
	cmd := exec.Command("ffmpeg", "-y", "-i", pngPath, "-lossless", "1", "-c:v", "libwebp", webpPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg build fixture: %v: %s", err, out)
	}

	cmd = exec.Command("exiftool", "-overwrite_original", "-Orientation#=6", webpPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("exiftool set orientation: %v: %s", err, out)
	}

	return webpPath
}

// TestCropWebPAppliesOrientation guards against cropWebP computing the crop
// rectangle against the raw (un-rotated) pixel buffer instead of the
// display-oriented one, unlike cropStandard/cropHEIF. A raw image with a
// marker in its top-left corner and EXIF Orientation=6 (rotate 90 CW for
// display) puts that marker in the display image's top-right corner; a crop
// of that display-space region must contain the marker color.
func TestCropWebPAppliesOrientation(t *testing.T) {
	if !CheckExiftool() {
		t.Skip("exiftool not available")
	}
	if !CheckFFmpeg().Available {
		t.Skip("ffmpeg not available")
	}
	if out, err := exec.Command("ffmpeg", "-encoders").Output(); err != nil || !containsLibwebp(out) {
		t.Skip("ffmpeg build has no libwebp encoder")
	}

	dir := t.TempDir()
	const rawW, rawH, marker = 100, 60, 20
	webpPath := buildOrientedWebPFixture(t, dir, rawW, rawH, marker)

	// Ground truth: apply orientation 6 in Go to the same raw image and find
	// where the red marker landed in display space, exactly like production
	// code does before cropping.
	f, err := os.Open(webpPath)
	if err != nil {
		t.Fatal(err)
	}
	raw, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	oriented := applyOrientation(raw, 6)
	db := oriented.Bounds()

	var markerMinX, markerMinY, markerMaxX, markerMaxY int
	found := false
	for y := db.Min.Y; y < db.Max.Y; y++ {
		for x := db.Min.X; x < db.Max.X; x++ {
			r, g, b, _ := oriented.At(x, y).RGBA()
			if r > 0x8000 && g < 0x2000 && b < 0x2000 {
				if !found {
					markerMinX, markerMinY, markerMaxX, markerMaxY = x, y, x, y
					found = true
					continue
				}
				if x < markerMinX {
					markerMinX = x
				}
				if x > markerMaxX {
					markerMaxX = x
				}
				if y < markerMinY {
					markerMinY = y
				}
				if y > markerMaxY {
					markerMaxY = y
				}
			}
		}
	}
	if !found {
		t.Fatal("marker not found in ground-truth oriented image")
	}

	dispW, dispH := float64(db.Dx()), float64(db.Dy())
	x := float64(markerMinX) / dispW
	y := float64(markerMinY) / dispH
	w := float64(markerMaxX-markerMinX+1) / dispW
	h := float64(markerMaxY-markerMinY+1) / dispH

	if err := CropImage(webpPath, x, y, w, h); err != nil {
		t.Fatalf("CropImage: %v", err)
	}

	cf, err := os.Open(webpPath)
	if err != nil {
		t.Fatal(err)
	}
	defer cf.Close()
	cropped, _, err := image.Decode(cf)
	if err != nil {
		t.Fatalf("decode cropped result: %v", err)
	}

	cb := cropped.Bounds()
	cx, cy := cb.Min.X+cb.Dx()/2, cb.Min.Y+cb.Dy()/2
	r, g, b, _ := cropped.At(cx, cy).RGBA()
	if !(r > 0x8000 && g < 0x2000 && b < 0x2000) {
		t.Errorf("crop of the marker's display-space region is not red at center (got r=%d g=%d b=%d) — orientation was not applied before cropping", r, g, b)
	}
}
