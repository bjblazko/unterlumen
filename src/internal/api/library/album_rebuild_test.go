package apilibrary

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

var rebuildTime = time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)

// publishedPhoto puts a photo in a fresh library, records the publication in
// its sidecar and marks it in the library the way indexing would.
func publishedPhoto(t *testing.T, mgr *lib.Manager, photoID string, pub media.Publication) (libID string) {
	t.Helper()
	libID = seedLibraryPhoto(t, mgr, photoID)
	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hint, _ := store.GetPhotoPathHint(photoID)
	if err := media.AppendPublication(hint, pub); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertMeta(photoID, "built:"+pub.Channel, pub.PublishedAt.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	return libID
}

func rebuildFixture(t *testing.T) (*lib.Manager, *channels.Store, *siteStore, *channels.Channel) {
	t.Helper()
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	chStore := channels.NewStore(dir, dir)
	ch := &channels.Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85, SiteExport: true}
	if err := chStore.Save(ch); err != nil {
		t.Fatal(err)
	}
	return mgr, chStore, newSiteStore(chStore, "website"), ch
}

func TestRebuildAlbumRegister_RecreatesAlbumFromSidecars(t *testing.T) {
	mgr, chStore, sites, ch := rebuildFixture(t)
	_ = chStore
	pub := media.Publication{Channel: "website", PostID: "p1", GalleryTitle: "Iceland", Slug: "iceland-3fa9c1d2", Unlisted: true, PublishedAt: rebuildTime}
	publishedPhoto(t, mgr, "photoA", pub)
	pub.PublishedAt = rebuildTime.Add(48 * time.Hour) // added to the album later
	publishedPhoto(t, mgr, "photoB", pub)

	report, err := rebuildAlbumRegister(sites, mgr, ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Added) != 1 || report.Added[0].PostID != "p1" || len(report.Unreadable) != 0 {
		t.Fatalf("report = %+v", report)
	}
	albums, _ := sites.List()
	if len(albums) != 1 {
		t.Fatalf("register holds %d albums", len(albums))
	}
	a := albums[0]
	if a.Slug != "iceland-3fa9c1d2" || a.Title != "Iceland" || !a.Unlisted || a.PhotoCount != 2 || len(a.Photos) != 2 {
		t.Errorf("album = %+v", a)
	}
	if !a.PublishedAt.Equal(rebuildTime) || !a.UpdatedAt.Equal(rebuildTime.Add(48*time.Hour)) {
		t.Errorf("publishedAt %v updatedAt %v", a.PublishedAt, a.UpdatedAt)
	}
	for _, p := range a.Photos {
		if p.PhotoID == "" || p.Filename == "" || p.ThumbFilename != "thumbs/"+p.Filename {
			t.Errorf("photo = %+v", p)
		}
	}
	if a.Photos[0].Filename == a.Photos[1].Filename {
		t.Error("filenames must differ")
	}
}

func TestRebuildAlbumRegister_LeavesAlbumsAlreadyInTheRegister(t *testing.T) {
	mgr, chStore, sites, ch := rebuildFixture(t)
	_ = chStore
	publishedPhoto(t, mgr, "photoA", media.Publication{Channel: "website", PostID: "p1", GalleryTitle: "Iceland", Slug: "iceland", PublishedAt: rebuildTime})
	existing := testAlbum("p1", "Renamed Since")
	existing.Slug = "iceland"
	if err := sites.Upsert(existing); err != nil {
		t.Fatal(err)
	}

	report, err := rebuildAlbumRegister(sites, mgr, ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Added) != 0 || report.Present != 1 {
		t.Fatalf("report = %+v", report)
	}
	albums, _ := sites.List()
	if len(albums) != 1 || albums[0].Title != "Renamed Since" {
		t.Errorf("an album already registered must not be overwritten: %+v", albums)
	}
}

// A slug must never be made up: an album whose sidecars predate the slug is
// listed with the reason instead of being registered under a guessed URL.
func TestRebuildAlbumRegister_ListsAlbumsWithoutSlugInsteadOfDroppingThem(t *testing.T) {
	mgr, chStore, sites, ch := rebuildFixture(t)
	_ = chStore
	publishedPhoto(t, mgr, "photoA", media.Publication{Channel: "website", PostID: "old1", GalleryTitle: "Old Album", PublishedAt: rebuildTime})

	report, err := rebuildAlbumRegister(sites, mgr, ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Added) != 0 || len(report.Unreadable) != 1 {
		t.Fatalf("report = %+v", report)
	}
	u := report.Unreadable[0]
	if u.PostID != "old1" || u.Title != "Old Album" || u.Reason == "" {
		t.Errorf("unreadable = %+v", u)
	}
	if albums, _ := sites.List(); len(albums) != 0 {
		t.Errorf("nothing may be registered for it: %+v", albums)
	}
}

func TestRebuildAlbumRegister_IgnoresOtherChannels(t *testing.T) {
	mgr, chStore, sites, ch := rebuildFixture(t)
	_ = chStore
	publishedPhoto(t, mgr, "photoA", media.Publication{Channel: "website", PostID: "p1", GalleryTitle: "Iceland", Slug: "iceland", PublishedAt: rebuildTime})
	publishedPhoto(t, mgr, "photoB", media.Publication{Channel: "instagram", PostID: "x", GalleryTitle: "Other", PublishedAt: rebuildTime})

	report, err := rebuildAlbumRegister(sites, mgr, ch)
	if err != nil || len(report.Added) != 1 || report.Added[0].Photos != 1 {
		t.Fatalf("report = %+v, err %v", report, err)
	}
}

func TestRebuildAlbumListHandler_ReportsAndRegisters(t *testing.T) {
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	chStore := channels.NewStore(dir, dir)
	if err := chStore.Save(&channels.Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85, SiteExport: true}); err != nil {
		t.Fatal(err)
	}
	if err := chStore.Save(&channels.Channel{Slug: "shares", Name: "Shares", Format: "jpeg", Quality: 85, GalleryExport: true}); err != nil {
		t.Fatal(err)
	}
	publishedPhoto(t, mgr, "photoA", media.Publication{Channel: "website", PostID: "p1", GalleryTitle: "Iceland", Slug: "iceland", PublishedAt: rebuildTime})
	publishedPhoto(t, mgr, "photoB", media.Publication{Channel: "website", PostID: "old", GalleryTitle: "Old", PublishedAt: rebuildTime})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/channels/{slug}/rebuild-album-list", rebuildAlbumList(chStore, mgr))

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/channels/website/rebuild-album-list", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var report albumRegisterReport
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Added) != 1 || report.Added[0].Slug != "iceland" || len(report.Unreadable) != 1 || report.Unreadable[0].PostID != "old" {
		t.Errorf("report = %+v", report)
	}

	// Only a website destination has an album register.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/channels/shares/rebuild-album-list", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("gallery channel: status = %d, want 400", rec.Code)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/channels/nope/rebuild-album-list", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown channel: status = %d, want 404", rec.Code)
	}
}

func TestRebuildAlbumRegister_DoesNotBringBackADeletedAlbum(t *testing.T) {
	mgr, chStore, sites, ch := rebuildFixture(t)
	_ = chStore
	publishedPhoto(t, mgr, "photoA", media.Publication{Channel: "website", PostID: "p1", GalleryTitle: "Iceland", Slug: "iceland", PublishedAt: rebuildTime})
	if err := sites.Upsert(testAlbum("p1", "Iceland")); err != nil {
		t.Fatal(err)
	}
	if err := sites.Delete("p1"); err != nil { // deleted on the other installation, whose photos are not mounted here
		t.Fatal(err)
	}

	report, err := rebuildAlbumRegister(sites, mgr, ch)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Added) != 0 || report.Deleted != 1 {
		t.Fatalf("report = %+v", report)
	}
	if albums, _ := sites.List(); len(albums) != 0 {
		t.Errorf("a deleted album must stay deleted: %+v", albums)
	}
}

func deleteAlbumRequest(t *testing.T, chStore *channels.Store, mgr *lib.Manager, slug, postID string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("DELETE /api/channels/{slug}/galleries/{postID}", deleteGallery(chStore, mgr))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/channels/"+slug+"/galleries/"+postID, strings.NewReader(`{"deleteRemote":false}`)))
	return rec
}

// Deleting an album takes it out of its photos too, or the next rebuild would
// find it again; and it leaves a tombstone for the photos that are not reachable.
func TestDeleteGallery_ClearsSidecarsAndLeavesATombstone(t *testing.T) {
	mgr, chStore, sites, _ := rebuildFixture(t)
	pub := media.Publication{Channel: "website", PostID: "p1", GalleryTitle: "Iceland", Slug: "iceland", PublishedAt: rebuildTime}
	libID := publishedPhoto(t, mgr, "photoA", pub)
	other := pub
	other.PostID, other.Slug, other.GalleryTitle = "p2", "norway", "Norway"
	store, _ := mgr.OpenStore(libID)
	hint, _ := store.GetPhotoPathHint("photoA")
	store.Close()
	if err := media.AppendPublication(hint, other); err != nil { // the photo is in a second album as well
		t.Fatal(err)
	}
	album := testAlbum("p1", "Iceland")
	album.Photos = []SitePhoto{{PhotoID: "photoA", Filename: "a.jpg", ThumbFilename: "thumbs/a.jpg"}}
	if err := sites.Upsert(album); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(chStore.OutputDir("website"), "site", "albums", album.Slug), 0o755); err != nil {
		t.Fatal(err)
	}

	if rec := deleteAlbumRequest(t, chStore, mgr, "website", "p1"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	pubs, _ := media.ReadSidecar(hint)
	if len(pubs) != 1 || pubs[0].PostID != "p2" {
		t.Errorf("sidecar = %+v, want only the other album", pubs)
	}
	if !sites.IsDeleted("p1") || sites.IsDeleted("p2") {
		t.Error("p1, and only p1, must be tombstoned")
	}
}

// A photo taken off a site leaves its sidecar too; an album that ran out of
// photos is deleted for good.
func TestRemovePhotoFromSite_ClearsItsSidecarEntry(t *testing.T) {
	mgr, chStore, sites, ch := rebuildFixture(t)
	pub := media.Publication{Channel: "website", PostID: "p1", GalleryTitle: "Iceland", Slug: "iceland", PublishedAt: rebuildTime}
	libID := publishedPhoto(t, mgr, "photoA", pub)
	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hint, _ := store.GetPhotoPathHint("photoA")
	album := testAlbum("p1", "Iceland")
	album.Photos = []SitePhoto{{PhotoID: "photoA", Filename: "a.jpg", ThumbFilename: "thumbs/a.jpg"}}
	if err := sites.Upsert(album); err != nil {
		t.Fatal(err)
	}

	if err := removePhotoFromSite(store, ch, chStore, "photoA", "website"); err != nil {
		t.Fatal(err)
	}

	if pubs, _ := media.ReadSidecar(hint); len(pubs) != 0 {
		t.Errorf("sidecar still records %+v", pubs)
	}
	if !sites.IsDeleted("p1") {
		t.Error("the album ran out of photos and must stay gone")
	}
}
