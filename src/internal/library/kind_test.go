package library

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func kindFixture(t *testing.T) *Store {
	t.Helper()
	s := listFixture(t)
	if err := s.UpsertPhoto("v1", "/lib/2024/clip.mov", "clip.mov", 5000, time.Now(), `{"tags":{"Duration":"12.50"}}`, "thumbs/v1.jpg", "2024-05-02T09:00:00", "mov"); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestKindFilterSeparatesPhotosAndVideos(t *testing.T) {
	s := kindFixture(t)
	videos, n := listedIDs(t, s, ListPhotosOpts{Scope: Filter{Kind: KindVideo}})
	if n != 1 || videos[0] != "v1" {
		t.Errorf("videos = %v, want [v1]", videos)
	}
	photos, _ := listedIDs(t, s, ListPhotosOpts{Scope: Filter{Kind: KindPhoto}})
	sort.Strings(photos)
	if want := []string{"p1", "p2", "p3", "p5"}; !reflect.DeepEqual(photos, want) {
		t.Errorf("photos = %v, want %v", photos, want)
	}
	if (Filter{Kind: KindVideo}).Key() == (Filter{}).Key() {
		t.Error("the kind is not part of the filter's cache key")
	}
}

func TestKindFilterIsNotCorrelated(t *testing.T) {
	s := kindFixture(t)
	fromSQL, whereSQL, args := newPhotoFilter(ListPhotosOpts{}).sql()
	scope, _ := Filter{Kind: KindVideo}.Cond("p.id")
	rows, err := s.db.Query(`EXPLAIN QUERY PLAN SELECT COUNT(*) `+fromSQL+` WHERE `+whereSQL+scope, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	if joined := strings.Join(plan, "; "); strings.Contains(joined, "CORRELATED") {
		t.Fatalf("the kind filter runs once per photo: %s", joined)
	}
}

func TestNothingLooksForFacesOrThingsInVideos(t *testing.T) {
	s := kindFixture(t)
	faces, err := s.PhotosNeedingFaces()
	if err != nil {
		t.Fatal(err)
	}
	things, err := s.PhotosNeedingThings()
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range faces {
		if task.ID == "v1" {
			t.Error("faces are looked for in a video")
		}
	}
	for _, task := range things {
		if task.ID == "v1" {
			t.Error("things are looked for in a video")
		}
	}
}

func TestWalkListsVideosButNotLivePhotoClips(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"IMG_1.HEIC", "IMG_1.MOV", "IMG_2.MOV", "trip.mp4", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := collectFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}
	sort.Strings(names)
	if want := []string{"IMG_1.HEIC", "IMG_2.MOV", "trip.mp4"}; !reflect.DeepEqual(names, want) {
		t.Errorf("walk found %v, want %v", names, want)
	}
}

func TestALibraryCountsItsVideos(t *testing.T) {
	s := kindFixture(t)
	if n, err := s.CountVideos(); err != nil || n != 1 {
		t.Errorf("CountVideos = %d, %v; want 1", n, err)
	}
}

func TestAVideoWithAPlaceCarriesTheGPSBadge(t *testing.T) {
	s := newTestStore(t)
	if err := s.UpsertPhoto("v2", "/lib/trip/clip.mov", "clip.mov", 10, time.Now(), `{"latitude":50.98,"longitude":6.89,"tags":{"Duration":"3.00"}}`, "", "", "mov"); err != nil {
		t.Fatal(err)
	}
	res, err := s.BrowseFolder("/lib/trip")
	if err != nil || len(res.Photos) != 1 {
		t.Fatalf("browse: %v %v", res, err)
	}
	if res.Photos[0].Exif["GPSLatitude"] == "" || res.Photos[0].Exif["Duration"] != "3.00" {
		t.Errorf("overlay details = %v", res.Photos[0].Exif)
	}
}
