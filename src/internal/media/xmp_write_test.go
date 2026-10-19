package media

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// The other installation writes a name into the sidecar while this one is
// making its change: the change starts again from the new sidecar, and both
// names stay.
func TestChangeSidecarKeepsWhatWasWrittenMeanwhile(t *testing.T) {
	photo := filepath.Join(t.TempDir(), "p.jpg")
	if err := WriteFaceRegions(photo, []FaceRegion{{Name: "Anna", X: 0.1, Y: 0.1, W: 0.2, H: 0.2}}, 1, 0, 0); err != nil {
		t.Fatal(err)
	}
	meanwhile := true
	err := changeSidecar(photo, func(data []byte) ([]byte, error) {
		if meanwhile {
			meanwhile = false
			if err := ChangeFaceRegions(photo, []FaceRegion{{Name: "Ben", X: 0.6, Y: 0.1, W: 0.2, H: 0.2}}, 1, 0, 0); err != nil {
				t.Fatal(err)
			}
		}
		regions := upright(parseFaceRegions(data), 1)
		regions = append(regions, FaceRegion{Name: "Cleo", X: 0.4, Y: 0.6, W: 0.2, H: 0.2})
		return withFaceRegions(data, regions, 1, 0, 0)
	})
	if err != nil {
		t.Fatal(err)
	}
	regions, _, err := ReadFaceRegions(photo, 1)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range regions {
		names = append(names, r.Name)
	}
	if got := strings.Join(names, ","); got != "Anna,Ben,Cleo" {
		t.Errorf("names = %s, want Anna,Ben,Cleo", got)
	}
}

// A scan reading a sidecar while it is written sees the whole old or the
// whole new file, never an empty or half one.
func TestSidecarReaderNeverSeesHalfAFile(t *testing.T) {
	photo := filepath.Join(t.TempDir(), "p.jpg")
	if err := WriteField(photo, "note", strings.Repeat("a", 20000)); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for i := range 200 {
			if err := WriteField(photo, "note", strings.Repeat(string(rune('a'+i%26)), 20000)); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	short := -1
	for running := true; running; {
		select {
		case <-done:
			running = false
		default:
		}
		notes, err := ReadNotes(photo)
		if err != nil {
			t.Error(err)
		}
		if n := len(notes.Fields["note"]); n != 20000 && short < 0 {
			short = n
		}
	}
	wg.Wait()
	if short >= 0 {
		t.Errorf("read a note of %d characters while the sidecar was written", short)
	}
}

// Writing leaves nothing beside the sidecar and keeps its permissions.
func TestChangeSidecarLeavesNoTraceAndKeepsMode(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "p.jpg")
	if err := WriteTitle(photo, "First"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(SidecarPath(photo), 0o664); err != nil {
		t.Fatal(err)
	}
	if err := WriteTitle(photo, "Second"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("folder holds %d files, want only the sidecar", len(entries))
	}
	fi, err := os.Stat(SidecarPath(photo))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o664 {
		t.Errorf("mode = %o, want 664", fi.Mode().Perm())
	}
	if title, _ := ReadTitle(photo); title != "Second" {
		t.Errorf("title = %q, want Second", title)
	}
}
