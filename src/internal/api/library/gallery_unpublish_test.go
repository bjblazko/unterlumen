package apilibrary

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/site"
)

// galleryFixture: a share-links destination with one published gallery "g1"
// whose photo is also in a second gallery "g2" and was published to another
// destination as well.
func galleryFixture(t *testing.T) (mgr *lib.Manager, chStore *channels.Store, libID, hint string) {
	t.Helper()
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	chStore = channels.NewStore(dir, dir)
	for _, ch := range []*channels.Channel{
		{Slug: "shares", Name: "Shares", Format: "jpeg", Quality: 85, GalleryExport: true},
		{Slug: "instagram", Name: "Instagram", Format: "jpeg", Quality: 85},
	} {
		if err := chStore.Save(ch); err != nil {
			t.Fatal(err)
		}
	}
	libID = seedLibraryPhoto(t, mgr, "photoA")
	store, _ := mgr.OpenStore(libID)
	defer store.Close()
	hint, _ = store.GetPhotoPathHint("photoA")
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	for _, p := range []media.Publication{
		{Channel: "shares", PostID: "g1", GalleryTitle: "First", PublishedAt: at},
		{Channel: "shares", PostID: "g2", GalleryTitle: "Second", PublishedAt: at},
		{Channel: "instagram", PostID: "ig", PublishedAt: at},
	} {
		if err := media.AppendPublication(hint, p); err != nil {
			t.Fatal(err)
		}
		ts := at.Format(time.RFC3339)
		store.UpsertMeta("photoA", "built:"+p.Channel, ts)
		store.UpsertMeta("photoA", "built:"+p.Channel+":"+p.PostID, ts)
		if p.GalleryTitle != "" {
			store.UpsertMeta("photoA", "built:"+p.Channel+":"+p.PostID+":title", p.GalleryTitle)
		}
	}
	// Gallery g1 on disk, with the photo in its state.
	g1 := filepath.Join(chStore.OutputDir("shares"), "g1")
	if err := os.MkdirAll(g1, 0o755); err != nil {
		t.Fatal(err)
	}
	gs := &site.GalleryState{PostID: "g1", Title: "First", PublishedAt: at, PhotoCount: 2,
		Photos: []site.Photo{{PhotoID: "photoA", Filename: "a.jpg"}, {PhotoID: "not-in-any-library", Filename: "b.jpg"}}}
	if err := site.SaveGalleryState(filepath.Join(g1, "gallery.json"), gs); err != nil {
		t.Fatal(err)
	}
	return mgr, chStore, libID, hint
}

func metaKeys(t *testing.T, mgr *lib.Manager, libID string) map[string]bool {
	t.Helper()
	store, _ := mgr.OpenStore(libID)
	defer store.Close()
	entries, _ := store.GetMeta("photoA")
	keys := map[string]bool{}
	for _, e := range entries {
		keys[e.Key] = true
	}
	return keys
}

// Unpublishing a gallery takes it out of its photos, like a site album: the
// sidecar record and the library's keys for that one gallery go, everything
// else — another gallery of the destination, another destination — stays.
func TestUnpublishGallery_ClearsItsSidecarEntryAndMetaKeysOnly(t *testing.T) {
	mgr, chStore, libID, hint := galleryFixture(t)

	rec := deleteAlbumRequest(t, chStore, mgr, "shares", "g1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	pubs, _ := media.ReadSidecar(hint)
	if len(pubs) != 2 || pubs[0].PostID != "g2" || pubs[1].Channel != "instagram" {
		t.Errorf("sidecar = %+v, want g2 and the instagram record", pubs)
	}
	keys := metaKeys(t, mgr, libID)
	for _, gone := range []string{"built:shares:g1", "built:shares:g1:title"} {
		if keys[gone] {
			t.Errorf("%s must be gone", gone)
		}
	}
	for _, kept := range []string{"built:shares", "built:shares:g2", "built:shares:g2:title", "built:instagram", "built:instagram:ig"} {
		if !keys[kept] {
			t.Errorf("%s must stay", kept)
		}
	}

	var result map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result["sidecarsNotCleared"] != float64(1) { // the photo that is in no library here
		t.Errorf("result = %v, want the unreachable photo counted", result)
	}
}

// The channel marker goes with the photo's last gallery of that destination —
// the channel filter matches on it.
func TestUnpublishGallery_LastGalleryOfAPhotoDropsTheChannelMarker(t *testing.T) {
	mgr, chStore, libID, hint := galleryFixture(t)
	_ = media.RemovePublication(hint, "shares", "g2")
	store, _ := mgr.OpenStore(libID)
	for _, k := range []string{"built:shares:g2", "built:shares:g2:title"} {
		store.DeleteMeta("photoA", k)
	}
	store.Close()

	if rec := deleteAlbumRequest(t, chStore, mgr, "shares", "g1"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	keys := metaKeys(t, mgr, libID)
	if keys["built:shares"] {
		t.Error("the channel marker must go with the last gallery")
	}
	if !keys["built:instagram"] {
		t.Error("another destination's marker must stay")
	}
}

// A deleted site album is cleaned the same way, meta keys included — not only
// the sidecar.
func TestDeleteSiteAlbum_AlsoClearsTheLibrarysKeys(t *testing.T) {
	mgr, chStore, sites, _ := rebuildFixture(t)
	libID := publishedPhoto(t, mgr, "photoA", media.Publication{Channel: "website", PostID: "p1", GalleryTitle: "Iceland", Slug: "iceland", PublishedAt: rebuildTime})
	store, _ := mgr.OpenStore(libID)
	store.UpsertMeta("photoA", "built:website:p1", "2026-02-01T12:00:00Z")
	store.Close()
	album := testAlbum("p1", "Iceland")
	album.Photos = []site.Photo{{PhotoID: "photoA", Filename: "a.jpg"}}
	if err := sites.Upsert(album); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(chStore.OutputDir("website"), "site", "albums", album.Slug), 0o755); err != nil {
		t.Fatal(err)
	}

	if rec := deleteAlbumRequest(t, chStore, mgr, "website", "p1"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	keys := metaKeys(t, mgr, libID)
	if keys["built:website:p1"] || keys["built:website"] {
		t.Errorf("the deleted album's keys are still there: %v", keys)
	}
}
