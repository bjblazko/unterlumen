package apilibrary

import (
	"archive/zip"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/site"
)

func TestMergePhotoItems_KeepsEachFileOnceInOrder(t *testing.T) {
	existing := []site.SitePhoto{
		{PhotoID: "a", Filename: "a.jpg", ThumbFilename: "thumbs/a.jpg"},
		{PhotoID: "b", Filename: "b.jpg", ThumbFilename: "thumbs/b.jpg"},
		{PhotoID: "a", Filename: "a.jpg", ThumbFilename: "thumbs/a.jpg"}, // a repeat left by an older run
	}
	results := []buildResult{
		{PhotoID: "b", Filename: "b.jpg", ThumbFilename: "thumbs/b.jpg", Width: 10, Height: 5}, // added again
		{PhotoID: "c", Filename: "c.jpg", ThumbFilename: "thumbs/c.jpg"},
		{PhotoID: "d", Filename: "d.jpg", Error: "export failed"},
	}
	got := mergePhotoItems(existing, results)
	var names []string
	for _, it := range got {
		names = append(names, it.Filename)
	}
	if strings.Join(names, ",") != "a.jpg,b.jpg,c.jpg" {
		t.Errorf("items = %v, want a,b,c once each, in order", names)
	}
}

// Adding a photo that is already in the album must not list it twice — on the
// page, in the register or in the ZIP.
func TestGenerateDraft_AddingAPhotoAlreadyInTheAlbumDoesNotDuplicateIt(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85, SiteExport: true}); err != nil {
		t.Fatal(err)
	}
	libID := seedLibraryPhoto(t, mgr, "photoA")
	run := func(target channels.DraftTarget) string {
		d, err := draftStore.Create("website", target, []channels.DraftPhoto{{LibraryID: libID, PhotoID: "photoA"}})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/channels/website/drafts/"+d.ID+"/generate", strings.NewReader(`{"publishedAt":"2026-02-01T12:00:00Z"}`)))
		if rec.Code != 200 {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		albums, _ := site.NewSiteStore(chStore, "website").List()
		if len(albums) != 1 {
			t.Fatalf("albums = %d", len(albums))
		}
		return albums[0].PostID
	}
	postID := run(channels.DraftTarget{Title: "Iceland"})
	run(channels.DraftTarget{PostID: postID}) // the same photo, added again

	albums, _ := site.NewSiteStore(chStore, "website").List()
	if albums[0].PhotoCount != 1 || len(albums[0].Photos) != 1 {
		t.Errorf("photoCount %d, photos %d; want 1 and 1", albums[0].PhotoCount, len(albums[0].Photos))
	}
	zr, err := zip.OpenReader(filepath.Join(chStore.OutputDir("website"), "site", "albums", albums[0].Slug, "photos.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	if len(zr.File) != 1 {
		t.Errorf("zip has %d files, want 1", len(zr.File))
	}
}

// An album that already carries repeats (left by an older run) is repaired by
// rebuilding it.
func TestRebuildSiteChannel_RemovesRepeatedPhotosFromAnAlbum(t *testing.T) {
	_, chStore, sites, ch := rebuildFixture(t)
	album := testAlbum("p1", "Photos 2019")
	album.Photos = []site.SitePhoto{
		{PhotoID: "a", Filename: "a.jpg", ThumbFilename: "thumbs/a.jpg"},
		{PhotoID: "b", Filename: "b.jpg", ThumbFilename: "thumbs/b.jpg"},
		{PhotoID: "a", Filename: "a.jpg", ThumbFilename: "thumbs/a.jpg"},
	}
	album.PhotoCount = 3
	if err := sites.Upsert(album); err != nil {
		t.Fatal(err)
	}

	if _, _, err := rebuildSiteChannel(chStore, nil, ch); err != nil {
		t.Fatal(err)
	}
	got, _ := sites.List()
	if len(got) != 1 || got[0].PhotoCount != 2 || len(got[0].Photos) != 2 {
		t.Fatalf("album = %+v, want the repeat gone and the count 2", got)
	}
}
