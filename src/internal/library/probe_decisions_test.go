//go:build probe

package library

import (
	"os"
	"testing"
	"time"

	fd "huepattl.de/unterlumen/internal/facedecisions"
)

// The first sharing of a real library's decisions, on a copy:
//
//	PROBE_DB=… PROBE_SHARED=<empty dir> go test -tags probe -run ProbeDecisions -v ./internal/library/
func TestProbeDecisions(t *testing.T) {
	path, dir := os.Getenv("PROBE_DB"), os.Getenv("PROBE_SHARED")
	if path == "" || dir == "" {
		t.Skip("set PROBE_DB and PROBE_SHARED")
	}
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	s := newStore(db, "")
	s.decisions = fd.New(dir)
	before, _ := s.HiddenCount()
	var nos int
	s.db.QueryRow(`SELECT COUNT(*) FROM face_not`).Scan(&nos)
	people, _ := s.People()
	start := time.Now()
	regroup, err := s.ApplyDecisions()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("first apply %d ms, regroup %v", time.Since(start).Milliseconds(), regroup)
	start = time.Now()
	if _, err := s.ApplyDecisions(); err != nil {
		t.Fatal(err)
	}
	t.Logf("second apply %d ms", time.Since(start).Milliseconds())
	after, _ := s.HiddenCount()
	d, _ := s.decisions.Read()
	peopleAfter, _ := s.People()
	changed := 0
	for i := range people {
		if people[i].Cover != peopleAfter[i].Cover {
			changed++
		}
	}
	t.Logf("hidden %d → %d, in the file %d; No's %d, in the file %d; covers in the file %d, changed %d", before, after, len(d.Hidden), nos, len(d.Not), len(d.Covers), changed)
	if after != before || len(d.Hidden) != before || changed != 0 {
		t.Error("the first sharing changed the library")
	}
}
