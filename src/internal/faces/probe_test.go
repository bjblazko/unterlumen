package faces

import (
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/image/draw"

	"huepattl.de/unterlumen/internal/onnx"
)

// TestProbeFolder runs the engine over the JPEGs in UNTERLUMEN_FACES_PHOTOS
// with the recognition folder UNTERLUMEN_FACES_DIR and ONNX Runtime
// UNTERLUMEN_ONNX_LIB (or the one in that folder), logs timings and writes
// every face to UNTERLUMEN_FACES_OUT, when set, for a person to look at.
func TestProbeFolder(t *testing.T) {
	dir, photos, out := os.Getenv("UNTERLUMEN_FACES_DIR"), os.Getenv("UNTERLUMEN_FACES_PHOTOS"), os.Getenv("UNTERLUMEN_FACES_OUT")
	if dir == "" || photos == "" {
		t.Skip("UNTERLUMEN_FACES_DIR and UNTERLUMEN_FACES_PHOTOS are not set")
	}
	lib := os.Getenv("UNTERLUMEN_ONNX_LIB")
	if lib == "" {
		lib = filepath.Join(dir, onnx.LibraryFile())
	}
	e, err := Open(lib, dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	files, _ := filepath.Glob(filepath.Join(photos, "*.jp*g"))
	var all []Face
	var labels []string
	start := time.Now()
	for _, f := range files {
		img := decodeFile(t, f)
		t0 := time.Now()
		found, err := e.Find(img)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		t.Logf("%-60s %d faces in %v", filepath.Base(f), len(found), time.Since(t0).Round(time.Millisecond))
		for i, fc := range found {
			name := fmt.Sprintf("%s-%d.jpg", filepath.Base(f), i)
			if out != "" {
				writeCrop(t, img, fc, filepath.Join(out, name))
			}
			all = append(all, fc)
			labels = append(labels, name)
		}
	}
	t.Logf("%d photos, %d faces, %v", len(files), len(all), time.Since(start).Round(time.Millisecond))
	var hist [10]int
	for i := range all {
		for j := i + 1; j < len(all); j++ {
			s := Similarity(all[i].Embedding, all[j].Embedding)
			hist[min(9, max(0, int(s*10)))]++
		}
	}
	t.Logf("similarity of every pair, by tenths: %v", hist)
	embs := make([][]float32, len(all))
	for i, f := range all {
		embs[i] = f.Embedding
	}
	sizes := map[int]int{}
	for _, g := range Group(embs) {
		sizes[g]++
	}
	bySize := map[int]int{}
	for _, n := range sizes {
		bySize[n]++
	}
	t.Logf("groups by size (size: count): %v", bySize)
}

func decodeFile(t *testing.T, path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func writeCrop(t *testing.T, img image.Image, fc Face, path string) {
	b := img.Bounds()
	r := image.Rect(int(fc.X*float64(b.Dx())), int(fc.Y*float64(b.Dy())),
		int((fc.X+fc.W)*float64(b.Dx())), int((fc.Y+fc.H)*float64(b.Dy())))
	crop := image.NewRGBA(image.Rect(0, 0, 128, 128))
	draw.BiLinear.Scale(crop, crop.Bounds(), img, r, draw.Src, nil)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	jpeg.Encode(f, crop, nil) //nolint:errcheck
}
