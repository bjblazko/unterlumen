package facedecisions

import (
	"errors"
	"testing"
)

var anna = Face{Photo: "p1", Box: [4]float64{0.1, 0.2, 0.1, 0.15}}

// The other installation found the same face a little differently.
var annaThere = Face{Photo: "p1", Box: [4]float64{0.11, 0.21, 0.1, 0.14}}

func TestDecisionsAreKeptInTheFileAndMatchByOverlap(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.Read(); !errors.Is(err, ErrMissing) {
		t.Fatalf("no file read as %v, want ErrMissing", err)
	}
	err := s.Change(func(d *Decisions) {
		d.AddHidden(anna)
		d.AddHidden(annaThere) // the same face: kept once
		d.AddNot(anna, "Ben")
		d.SetCover("Anna", anna, "2026-10-23T10:00:00")
		d.SetCover("anna", annaThere, "2026-10-22T10:00:00") // older: stays out
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Hidden) != 1 || len(d.Not) != 1 || d.Covers["anna"].Chosen != "2026-10-23T10:00:00" {
		t.Fatalf("decisions %+v", d)
	}
	s.Change(func(d *Decisions) { d.DropHidden(annaThere); d.DropNot(annaThere, "BEN") }) //nolint:errcheck
	if d, _ := s.Read(); len(d.Hidden) != 0 || len(d.Not) != 0 {
		t.Fatalf("after taking them back: %+v", d)
	}
	if Same(anna, Face{Photo: "p2", Box: anna.Box}) {
		t.Error("faces of two photos are one")
	}
}
