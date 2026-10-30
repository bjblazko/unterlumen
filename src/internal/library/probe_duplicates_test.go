//go:build probe

package library

// Times the search for duplicates over copies of real libraries, and shows
// what it finds of each kind:
//
//	PROBE_ROOT=/path/to/copy go test -tags probe -run ProbeDuplicates -v ./internal/library/
//
// PROBE_ROOT holds libraries/<id>/library.db, as the -lib-dir does.

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestProbeDuplicates(t *testing.T) {
	root := os.Getenv("PROBE_ROOT")
	if root == "" {
		t.Skip("set PROBE_ROOT to a copy of a library folder")
	}
	m, err := NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	r, err := m.Duplicates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Duplicates %d ms, %d groups, %d unanalysed", time.Since(start).Milliseconds(), len(r.Groups), r.Unanalysed)
	shown := map[string]int{}
	for _, g := range r.Groups {
		shown[g.Kind]++
		if shown[g.Kind] > 8 {
			continue
		}
		t.Logf("%s", g.Kind)
		for _, f := range g.Files {
			t.Logf("    %-10s %5d×%-5d %9d  %s", f.Relation, f.Width, f.Height, f.Size, f.Path)
		}
	}
	t.Logf("groups by kind: %v", shown)
}
