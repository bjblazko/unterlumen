package library

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// dupPhoto is a file to seed: its content ID, size in pixels, dHash (0 for
// none measured) and time taken, with its fraction of a second.
type dupPhoto struct {
	name, id, taken, subsec string
	w, h                    int
	hash                    int64
}

func seedDuplicates(t *testing.T, s *Store, dir string, photos []dupPhoto) {
	t.Helper()
	for _, p := range photos {
		path := filepath.Join(dir, p.name)
		if err := os.WriteFile(path, []byte(p.id), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := s.UpsertPhoto(p.id, path, p.name, int64(p.w*p.h), time.Now(), "{}", "", p.taken, "jpg"); err != nil {
			t.Fatal(err)
		}
		fields := map[string]string{"PixelXDimension": fmt.Sprint(p.w), "PixelYDimension": fmt.Sprint(p.h)}
		if err := s.UpsertPathCache(path, p.id, 1, int64(p.w*p.h)); err != nil {
			t.Fatal(err)
		}
		if p.subsec != "" {
			fields["SubSecTimeOriginal"] = `"` + p.subsec + `"`
		}
		if err := s.UpsertExifIndex(p.id, fields, nil); err != nil {
			t.Fatal(err)
		}
		if p.hash != 0 {
			_, err := s.db.Exec(`INSERT OR REPLACE INTO photo_appearance(photo_id, version, analysed_at, mono_class, tone_key, dhash)
				VALUES(?, 1, ?, 'colour', 'mid', ?)`, p.id, time.Now(), p.hash)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func dupLibrary(t *testing.T, m *Manager, name string, photos []dupPhoto) (id, dir string) {
	t.Helper()
	dir = t.TempDir()
	l, err := m.CreateLibrary(name, "", dir)
	if err != nil {
		t.Fatal(err)
	}
	s, err := m.OpenStore(l.ID)
	if err != nil {
		t.Fatal(err)
	}
	seedDuplicates(t, s, dir, photos)
	return l.ID, dir
}

func groupNames(r *DupReport) []string {
	var out []string
	for _, g := range r.Groups {
		var names []string
		for _, f := range g.Files {
			names = append(names, f.Name+":"+f.Relation)
		}
		out = append(out, g.Kind+" "+strings.Join(names, ","))
	}
	return out
}

func duplicatesOf(t *testing.T, m *Manager) *DupReport {
	t.Helper()
	r, err := m.Duplicates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDuplicatesIdenticalAcrossLibraries(t *testing.T) {
	m := newTestManager(t)
	dupLibrary(t, m, "A", []dupPhoto{{name: "a.jpg", id: "same", w: 40, h: 30, hash: 0x0f0f}})
	dupLibrary(t, m, "B", []dupPhoto{{name: "b.jpg", id: "same", w: 40, h: 30, hash: 0x0f0f}})
	got := groupNames(duplicatesOf(t, m))
	if len(got) != 1 || !strings.HasPrefix(got[0], "identical ") {
		t.Errorf("groups = %v, want one identical group", got)
	}
}

func TestDuplicatesIdenticalWithinOneLibrary(t *testing.T) {
	m := newTestManager(t)
	id, dir := dupLibrary(t, m, "A", []dupPhoto{{name: "a.jpg", id: "same", w: 40, h: 30}})
	s, _ := m.OpenStore(id)
	seedDuplicates(t, s, filepath.Join(dir), []dupPhoto{{name: "copy.jpg", id: "same", w: 40, h: 30}})
	r := duplicatesOf(t, m)
	if got := groupNames(r); len(got) != 1 || got[0] != "identical a.jpg:,copy.jpg:identical" {
		t.Errorf("groups = %v", got)
	}
	if r.Unanalysed != 1 {
		t.Errorf("unanalysed = %d, want the one unmeasured picture", r.Unanalysed)
	}
}

func TestDuplicatesOverlappingLibrariesAreOneFile(t *testing.T) {
	m := newTestManager(t)
	_, dir := dupLibrary(t, m, "Outer", []dupPhoto{{name: "a.jpg", id: "one", w: 40, h: 30, hash: 1}})
	inner, err := m.CreateLibrary("Inner", "", dir)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := m.OpenStore(inner.ID)
	seedDuplicates(t, s, dir, []dupPhoto{{name: "a.jpg", id: "one", w: 40, h: 30, hash: 1}})
	if got := groupNames(duplicatesOf(t, m)); len(got) != 0 {
		t.Errorf("groups = %v, want none: one path in two libraries is one file", got)
	}
}

func TestDuplicatesScaledAndSimilar(t *testing.T) {
	m := newTestManager(t)
	const base = int64(0x5a5a_1234_00ff_f00f)
	dupLibrary(t, m, "A", []dupPhoto{
		{name: "big.jpg", id: "big", w: 6000, h: 4000, hash: base, taken: "2024-05-01T10:00:00", subsec: "640"},
		// the same picture at another size, its hash two bits off
		{name: "small.jpg", id: "small", w: 1500, h: 1000, hash: base ^ 0b11},
		// an edit of the same second, seven bits off
		{name: "edit.jpg", id: "edit", w: 5000, h: 4000, hash: base ^ 0x7f, taken: "2024-05-01T10:00:00", subsec: "64"},
		// another shot of the same second: its fraction differs
		{name: "same-second.jpg", id: "same-second", w: 6000, h: 4000, hash: base ^ 0x30000, taken: "2024-05-01T10:00:00", subsec: "910"},
		// bursts a second later, close but their own moments
		{name: "burst.jpg", id: "burst", w: 6000, h: 4000, hash: base ^ 0x1f00, taken: "2024-05-01T10:00:01"},
		{name: "burst2.jpg", id: "burst2", w: 6000, h: 4000, hash: base ^ 0x300, taken: "2024-05-01T10:00:02"},
	})
	got := groupNames(duplicatesOf(t, m))
	want := "similar big.jpg:,edit.jpg:similar,small.jpg:scaled"
	if len(got) != 1 || got[0] != want {
		t.Errorf("groups = %v, want [%s]", got, want)
	}
}

func TestDuplicatesShapeKeepsCropsApart(t *testing.T) {
	m := newTestManager(t)
	dupLibrary(t, m, "A", []dupPhoto{
		{name: "wide.jpg", id: "wide", w: 6000, h: 4000, hash: 0x1234},
		{name: "square.jpg", id: "square", w: 4000, h: 4000, hash: 0x1234},
		{name: "turned.jpg", id: "turned", w: 2000, h: 3000, hash: 0x1235},
	})
	want := "same wide.jpg:,turned.jpg:scaled"
	if got := groupNames(duplicatesOf(t, m)); len(got) != 1 || got[0] != want {
		t.Errorf("groups = %v, want [%s]", got, want)
	}
}

func TestDuplicatesLeaveOutFilesGoneFromDisk(t *testing.T) {
	m := newTestManager(t)
	_, dir := dupLibrary(t, m, "A", []dupPhoto{
		{name: "a.jpg", id: "same", w: 40, h: 30},
		{name: "b.jpg", id: "same", w: 40, h: 30},
	})
	if err := os.Remove(filepath.Join(dir, "b.jpg")); err != nil {
		t.Fatal(err)
	}
	if got := groupNames(duplicatesOf(t, m)); len(got) != 0 {
		t.Errorf("groups = %v, want none once a copy is gone", got)
	}
}

func TestDuplicateCandidatesAreNotCorrelated(t *testing.T) {
	m := newTestManager(t)
	id, _ := dupLibrary(t, m, "A", nil)
	s, _ := m.OpenStore(id)
	rows, err := s.db.Query(`EXPLAIN QUERY PLAN SELECT pc.abs_path FROM path_cache pc
		JOIN photos p ON p.id = pc.photo_id LEFT JOIN photo_appearance a ON a.photo_id = p.id
		WHERE p.status = 'ok' AND ` + notVideo)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var a, b, c int
		var detail string
		if err := rows.Scan(&a, &b, &c, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if joined := strings.Join(plan, "; "); strings.Contains(joined, "CORRELATED") {
		t.Fatalf("the duplicates query runs once per photo: %s", joined)
	}
}

func TestDuplicatesNameTheLibrariesNotReached(t *testing.T) {
	m := newTestManager(t)
	dupLibrary(t, m, "Here", []dupPhoto{{name: "a.jpg", id: "same", w: 40, h: 30}})
	_, dir := dupLibrary(t, m, "On the NAS", []dupPhoto{{name: "b.jpg", id: "same", w: 40, h: 30}})
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	r := duplicatesOf(t, m)
	if len(r.Unreachable) != 1 || r.Unreachable[0] != "On the NAS" {
		t.Errorf("unreachable = %v, want the library whose folder is gone", r.Unreachable)
	}
	if got := groupNames(r); len(got) != 0 {
		t.Errorf("groups = %v, want none", got)
	}
}
