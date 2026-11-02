package metadata

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"huepattl.de/unterlumen/internal/media"
)

func TestSidecarWinsAndSaysSo(t *testing.T) {
	file := fileValues{fields: map[Field]string{DateTaken: "2020-01-01T10:00:00", Creator: "Camera"}, keywords: []string{"a"}}
	beside := map[Field]string{DateTaken: "2021-02-02T10:00:00"}
	c := merge("/p/x.jpg", file, beside, nil, FromSidecar)
	if c.Fields[DateTaken] != (Value{"2021-02-02T10:00:00", FromSidecar}) || c.Fields[Creator] != (Value{"Camera", FromFile}) {
		t.Errorf("fields %v", c.Fields)
	}
	if c.KeywordsFrom != FromFile || !slices.Equal(c.Keywords, []string{"a"}) {
		t.Errorf("keywords %v from %s", c.Keywords, c.KeywordsFrom)
	}
}

func TestSidecarWriteReadAndOverlay(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "2019-03-30_11-40-49_x.jpg")
	os.WriteFile(photo, []byte("not really a jpeg"), 0o644) //nolint:errcheck
	before := []byte("not really a jpeg")
	cur := merge(photo, fileValues{fields: map[Field]string{}}, nil, nil, FromSidecar)
	if cur.Fields[DateTaken].From != FromFileName {
		t.Fatalf("date from the name expected, got %v", cur.Fields[DateTaken])
	}
	change := Change{Set: map[Field]string{Title: "Harbour", Rating: "3"}, Date: &DateFix{Mode: "day", Day: "2024-05-01"}, KeywordsAdd: []string{"sea"}}
	plan := PlanFor(change, cur)
	if plan.Set[DateTaken] != "2024-05-01T11:40:49" {
		t.Fatalf("plan %v", plan)
	}
	if err := Write(photo, TargetSidecar, plan); err != nil {
		t.Fatal(err)
	}
	if now, _ := os.ReadFile(photo); string(now) != string(before) {
		t.Error("the photo itself changed")
	}
	fields, kw, from, err := readBeside(photo)
	if err != nil || from != FromSidecar || fields[Title] != "Harbour" || fields[Rating] != "3" || fields[DateTaken] != "2024-05-01T11:40:49" || !slices.Equal(kw, []string{"sea"}) {
		t.Errorf("read %v %v %s %v", fields, kw, from, err)
	}
	data := Overlay(photo, &media.ExifData{Tags: map[string]string{"Artist": "x"}})
	if data.DateTaken == nil || *data.DateTaken != "2024-05-01T11:40:49" || data.Tags["DateTakenFrom"] != "sidecar" || data.Tags["Rating"] != "3" {
		t.Errorf("overlay %+v", data)
	}
}

func TestShiftSkipsAFileWithoutDate(t *testing.T) {
	plan := PlanFor(Change{Date: &DateFix{Mode: "shift", ShiftSeconds: 60}}, Current{Fields: map[Field]Value{}})
	if plan.Skipped == "" {
		t.Error("a file without a date was not skipped")
	}
}

func TestFileArgs(t *testing.T) {
	args := fileArgs(map[Field]string{DateTaken: "2024-05-01T12:30:00+02:00", Creator: ""}, []string{"sea"}, []string{"old"})
	for _, want := range []string{"-EXIF:DateTimeOriginal=2024:05:01 12:30:00", "-EXIF:OffsetTimeOriginal=+02:00",
		"-EXIF:Artist=", "-XMP-dc:Subject-=old", "-XMP-dc:Subject+=sea", "-IPTC:Keywords+=sea", "-overwrite_original"} {
		if !slices.Contains(args, want) {
			t.Errorf("missing %s in %v", want, args)
		}
	}
}
