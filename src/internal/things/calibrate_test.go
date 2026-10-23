package things

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// TestCalibrate scores the vectors TestProbeFolder wrote to
// UNTERLUMEN_THINGS_DUMP with the current vocabulary's floors, so a person
// can compare the things with the photos and set the floors. The text
// vectors in the dump must be of the current vocabulary.
func TestCalibrate(t *testing.T) {
	path := os.Getenv("UNTERLUMEN_THINGS_DUMP")
	if path == "" {
		t.Skip("UNTERLUMEN_THINGS_DUMP is not set")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var d probeDump
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	if len(d.Words) != len(Vocabulary) {
		t.Fatalf("the dump has %d words, the vocabulary %d: probe again", len(d.Words), len(Vocabulary))
	}
	counts := map[string]int{}
	for i, p := range d.Photos {
		var names []string
		for _, f := range Score(p.Seen.Vector, d.Words, p.Seen.Boxes) {
			names = append(names, fmt.Sprintf("%s %.3f%s", f.Name, f.Score, map[string]string{"detector": "d"}[f.Source]))
			counts[f.Name]++
		}
		t.Logf("%03d %s", i, strings.Join(names, ", "))
	}
	t.Logf("things per name: %v", counts)
}

// TestProbePhrases compares phrases with the vectors in UNTERLUMEN_THINGS_DUMP
// and logs the best photos of each, by their place in the dump, for a person
// to compare with the photos: where a search by words should stop.
func TestProbePhrases(t *testing.T) {
	lib, dir, path := os.Getenv("UNTERLUMEN_ONNX_LIB"), os.Getenv("UNTERLUMEN_THINGS_DIR"), os.Getenv("UNTERLUMEN_THINGS_DUMP")
	if lib == "" || dir == "" || path == "" {
		t.Skip("UNTERLUMEN_ONNX_LIB, UNTERLUMEN_THINGS_DIR and UNTERLUMEN_THINGS_DUMP are not set")
	}
	e, err := Open(lib, dir, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	b, _ := os.ReadFile(path)
	var d probeDump
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatal(err)
	}
	for _, phrase := range strings.Split(os.Getenv("UNTERLUMEN_THINGS_PHRASES"), ";") {
		q, err := e.Describe(phrase)
		if err != nil {
			t.Fatal(err)
		}
		type hit struct {
			i   int
			sim float32
		}
		hits := make([]hit, len(d.Photos))
		for i, p := range d.Photos {
			hits[i] = hit{i, Similarity(q, p.Seen.Vector)}
		}
		sort.Slice(hits, func(a, b int) bool { return hits[a].sim > hits[b].sim })
		var parts []string
		for _, h := range hits[:12] {
			parts = append(parts, fmt.Sprintf("%03d %.3f", h.i, h.sim))
		}
		t.Logf("%-28s %s", phrase, strings.Join(parts, ", "))
	}
}
