package metadata

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"huepattl.de/unterlumen/internal/media"
)

func tinyJPEG(t *testing.T, dir string) string {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 4, 4)), nil); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// Into the file and out again, with exiftool itself.
func TestFileWriteAndReadWithExiftool(t *testing.T) {
	if !media.CheckExiftool() {
		t.Skip("exiftool is not installed")
	}
	p := tinyJPEG(t, t.TempDir())
	before, _ := os.ReadFile(p)
	change := Change{Set: map[Field]string{Title: "Hafen", Creator: "Ann", Rating: "4", Description: "Möwen"}, KeywordsAdd: []string{"sea", "sea"}}
	cur := Current{Fields: map[Field]Value{DateTaken: {"2024-05-01T12:30:00+02:00", FromFile}}}
	change.Date = &DateFix{Mode: "shift", ShiftSeconds: 3600}
	if err := Write(p, TargetFile, PlanFor(change, cur)); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(p)
	if bytes.Equal(before, after) {
		t.Fatal("the file did not change")
	}
	if _, err := os.Stat(media.SidecarPath(p)); err == nil {
		t.Error("a sidecar was written for a change into the file")
	}
	got, err := Read([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	c := got[p]
	for f, want := range map[Field]string{Title: "Hafen", Creator: "Ann", Rating: "4", Description: "Möwen", DateTaken: "2024-05-01T13:30:00+02:00"} {
		if c.Fields[f] != (Value{want, FromFile}) {
			t.Errorf("%s = %v, want %s from the file", f, c.Fields[f], want)
		}
	}
	if !slices.Equal(c.Keywords, []string{"sea"}) {
		t.Errorf("keywords %v", c.Keywords)
	}
}
