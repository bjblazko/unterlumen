package things

import (
	"encoding/json"
	"fmt"
	"image/jpeg"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestProbeFolder runs the engine over the JPEGs in UNTERLUMEN_THINGS_PHOTOS
// with the things folder UNTERLUMEN_THINGS_DIR and ONNX Runtime
// UNTERLUMEN_ONNX_LIB, and logs timings, the best-scoring things and the
// boxes of every photo, for a person to look at.
func TestProbeFolder(t *testing.T) {
	lib, dir, photos := os.Getenv("UNTERLUMEN_ONNX_LIB"), os.Getenv("UNTERLUMEN_THINGS_DIR"), os.Getenv("UNTERLUMEN_THINGS_PHOTOS")
	if lib == "" || dir == "" || photos == "" {
		t.Skip("UNTERLUMEN_ONNX_LIB, UNTERLUMEN_THINGS_DIR and UNTERLUMEN_THINGS_PHOTOS are not set")
	}
	e, err := Open(lib, dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	t0 := time.Now()
	words, err := e.VocabularyVectors()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d things described in %v", len(words), time.Since(t0).Round(time.Millisecond))
	files, _ := filepath.Glob(filepath.Join(photos, "*.jp*g"))
	dump := probeDump{Words: words}
	var look time.Duration
	for _, f := range files {
		r, _ := os.Open(f)
		img, err := jpeg.Decode(r)
		r.Close()
		if err != nil {
			continue
		}
		t1 := time.Now()
		seen, err := e.Look(img)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		look += time.Since(t1)
		dump.Photos = append(dump.Photos, probePhoto{File: filepath.Base(f), Seen: seen})
		t.Logf("%-40s %s | %s", filepath.Base(f), bestWords(seen.Vector, words, 6), boxNames(seen.Boxes))
	}
	if out := os.Getenv("UNTERLUMEN_THINGS_OUT"); out != "" {
		b, _ := json.Marshal(dump)
		if err := os.WriteFile(out, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if len(files) > 0 {
		t.Logf("%d photos, %v each", len(files), (look / time.Duration(len(files))).Round(time.Millisecond))
	}
}

func bestWords(v []float32, words [][]float32, n int) string {
	idx := make([]int, len(words))
	sims := make([]float32, len(words))
	for i, w := range words {
		idx[i], sims[i] = i, Similarity(v, w)
	}
	sort.Slice(idx, func(a, b int) bool { return sims[idx[a]] > sims[idx[b]] })
	var parts []string
	for _, i := range idx[:n] {
		parts = append(parts, Vocabulary[i].Name+" "+fmt.Sprintf("%.3f", sims[i]))
	}
	return strings.Join(parts, ", ")
}

func boxNames(boxes []Box) string {
	var parts []string
	for _, b := range boxes {
		parts = append(parts, b.Class+" "+fmt.Sprintf("%.2f", b.Score))
	}
	return strings.Join(parts, ", ")
}

// probeDump is what TestProbeFolder writes to UNTERLUMEN_THINGS_OUT, for
// trying ways of scoring without running the models again.
type probeDump struct {
	Words  [][]float32
	Photos []probePhoto
}

type probePhoto struct {
	File string
	Seen Seen
}
