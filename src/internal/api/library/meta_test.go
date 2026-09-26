package apilibrary

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sort"
	"testing"

	"huepattl.de/unterlumen/internal/channels"
)

// deleteBuiltKey seeds a photo in a gallery channel with keys, deletes one
// through the handler and returns the photo's remaining keys.
func deleteBuiltKey(t *testing.T, seed []string, key string) []string {
	t.Helper()
	mgr := newTestManager(t)
	dir := t.TempDir()
	chStore := channels.NewStore(dir, dir)
	mux := http.NewServeMux()
	if err := chStore.Save(&channels.Channel{Slug: "gal", Name: "Gallery", Format: "jpeg", Quality: 85, GalleryExport: true}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	mux.HandleFunc("DELETE /api/library/{id}/photos/{photoID}/meta", deleteMeta(mgr, chStore, nil))
	libID := seedLibraryPhoto(t, mgr, "p1")
	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	for _, k := range seed {
		if err := store.UpsertMeta("p1", k, "v"); err != nil {
			t.Fatalf("UpsertMeta %s: %v", k, err)
		}
	}
	store.Close()

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/library/"+libID+"/photos/p1/meta?key="+url.QueryEscape(key), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var keys []string
	for k := range photoMeta(t, mgr, libID, "p1") {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestDeleteMeta_BuiltAlbumKey_KeepsTheChannelsOtherAlbums(t *testing.T) {
	got := deleteBuiltKey(t, []string{"built:gal", "built:gal:A", "built:gal:A:title", "built:gal:B"}, "built:gal:A")
	want := []string{"built:gal", "built:gal:B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestDeleteMeta_BuiltLastAlbumKey_DropsTheChannelMarker(t *testing.T) {
	got := deleteBuiltKey(t, []string{"built:gal", "built:gal:postid", "built:gal:A", "other"}, "built:gal:A")
	want := []string{"other"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestDeleteMeta_BuiltChannelKey_DropsEveryAlbumOfTheChannel(t *testing.T) {
	got := deleteBuiltKey(t, []string{"built:gal", "built:gal:A", "published:gal:B", "built:other:C"}, "built:gal")
	want := []string{"built:other:C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestDeleteMeta_BuiltReservedSuffix_DeletesOnlyThatKey(t *testing.T) {
	got := deleteBuiltKey(t, []string{"built:gal", "built:gal:title", "built:gal:A"}, "built:gal:title")
	want := []string{"built:gal", "built:gal:A"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}

func TestDeleteMeta_BuiltKeyOfUnknownChannel_DeletesOnlyThatKey(t *testing.T) {
	got := deleteBuiltKey(t, []string{"built:nope", "built:nope:A"}, "built:nope:A")
	want := []string{"built:nope"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}
