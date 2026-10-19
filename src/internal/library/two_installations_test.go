package library

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/media"
)

// Two installations share a library: each has its own index of the same
// folder. Whatever each changes, and in whichever order they scan
// afterwards, nothing either did is lost and both end up knowing the same.
// That holds because a scan only reads the folder, and every change writes
// only itself into the file or sidecar as it is on disk.
func TestTwoInstallationsLoseNothingWhateverTheOrder(t *testing.T) {
	for _, order := range []string{"mac first", "nas first"} {
		t.Run(order, func(t *testing.T) {
			src := t.TempDir()
			photo := filepath.Join(src, "IMG_9019.jpeg")
			writePhoto(t, photo, "taken in Hagen, located in Austin", day)
			mac, nas := installation(t, src), installation(t, src)

			// On the Mac: a title, and the location fixed — the file changes.
			id := onlyID(t, mac)
			if err := mac.WriteNote(id, "title", "Hagen"); err != nil {
				t.Fatal(err)
			}
			writePhoto(t, photo, "taken in Hagen, located in Hagen", day.Add(time.Hour))
			// On the NAS, still knowing the old file: a field, and a
			// gallery published and another one taken back.
			if err := nas.WriteNote(id, "people", "Paula"); err != nil {
				t.Fatal(err)
			}
			at := day.Add(2 * time.Hour)
			media.AppendPublication(photo, media.Publication{Channel: "blog", PostID: "kept", PublishedAt: at})       //nolint:errcheck
			media.AppendPublication(photo, media.Publication{Channel: "blog", PostID: "taken-back", PublishedAt: at}) //nolint:errcheck
			media.RemovePublication(photo, "blog", "taken-back")                                                      //nolint:errcheck

			first, second := mac, nas
			if order == "nas first" {
				first, second = nas, mac
			}
			scan(first, src)
			scan(second, src)
			scan(first, src)

			for name, s := range map[string]*Store{"mac": mac, "nas": nas} {
				got := metaOf(t, s, onlyID(t, s))
				if got["title"] != "Hagen" || got["people"] != "Paula" || got["built:blog:kept"] == "" {
					t.Errorf("%s lost something: %v", name, got)
				}
				for k := range got {
					if strings.Contains(k, "taken-back") {
						t.Errorf("%s still claims the gallery taken back: %s", name, k)
					}
				}
			}
			if onlyID(t, mac) != onlyID(t, nas) {
				t.Error("the two indexes name the photo differently")
			}
		})
	}
}

// installation is one installation's index of src after its first scan.
func installation(t *testing.T, src string) *Store {
	t.Helper()
	s := newTestStore(t)
	s.SetProp(notesInSidecarProp, "1") //nolint:errcheck
	scan(s, src)
	return s
}

func scan(s *Store, src string) { drain(NewIndexer(s, s.dir, src).Run) }
