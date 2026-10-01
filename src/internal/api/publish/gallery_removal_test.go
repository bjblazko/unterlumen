package publish

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/site"
)

// A photo has the same id in every library that holds it, and each library
// keeps its own keys. Unpublishing used to clear the keys in the first
// library that held the photo, in the order of the libraries' random ids, so
// the library that published it kept them half of the time (the flaky
// gallery-channel-multi-album e2e spec).
func TestForgetAlbumInPhotosClearsEveryLibraryHoldingThePhoto(t *testing.T) {
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	photoPath := filepath.Join(source, "p1.jpg")
	writeTestJPEG(t, photoPath, 40, 30)
	if err := media.AppendPublication(photoPath, media.Publication{Channel: "site", PostID: "gone", GalleryTitle: "Gone", PublishedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	var libIDs []string
	for _, name := range []string{"Whole folder", "Same folder again"} {
		l, err := mgr.CreateLibrary(name, "", source)
		if err != nil {
			t.Fatal(err)
		}
		store, err := mgr.OpenStore(l.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.UpsertPhoto("p1", photoPath, "p1.jpg", 4, time.Now(), "{}", "", "", "jpeg"); err != nil {
			t.Fatal(err)
		}
		for _, k := range []string{"built:site", "built:site:gone", "built:site:gone:title"} {
			if err := store.UpsertMeta("p1", k, "x"); err != nil {
				t.Fatal(err)
			}
		}
		store.Close()
		libIDs = append(libIDs, l.ID)
	}

	if n := forgetAlbumInPhotos(mgr, "site", "gone", []site.Photo{{PhotoID: "p1"}}); n != 0 {
		t.Errorf("photos not cleared: %d", n)
	}
	for _, id := range libIDs {
		store, err := mgr.OpenStore(id)
		if err != nil {
			t.Fatal(err)
		}
		entries, _ := store.GetMeta("p1")
		store.Close()
		for _, e := range entries {
			if strings.HasPrefix(e.Key, "built:site") {
				t.Errorf("library %s still has %s", id, e.Key)
			}
		}
	}
	if pubs, _ := media.ReadSidecar(photoPath); len(pubs) != 0 {
		t.Errorf("sidecar still records %+v", pubs)
	}
}
