package library

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/appearance"
)

// addPhotoWithThumb indexes a photo at path whose thumbnail is a flat
// colour, written where the indexer looks for it.
func addPhotoWithThumb(t *testing.T, s *Store, libDir, id, path string, c color.RGBA) {
	t.Helper()
	rel := filepath.Join("thumbs", id[:2], id+".jpg")
	if err := os.MkdirAll(filepath.Join(libDir, filepath.Dir(rel)), 0o700); err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
	}
	f, err := os.Create(filepath.Join(libDir, rel))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertPhoto(id, path, filepath.Base(path), 0, time.Now(), "{}", rel, "", "jpeg"); err != nil {
		t.Fatal(err)
	}
}

func needing(t *testing.T, s *Store) int {
	t.Helper()
	tasks, err := s.PhotosNeedingAppearance()
	if err != nil {
		t.Fatal(err)
	}
	return len(tasks)
}

func runAnalyse(t *testing.T, s *Store, libDir string) []Progress {
	t.Helper()
	ch := make(chan Progress, 64)
	go NewIndexer(s, libDir, "/photos").RunAnalyseMissing(context.Background(), ch)
	var got []Progress
	for p := range ch {
		got = append(got, p)
	}
	return got
}

func TestAnalyseMissingStoresAppearanceAndPalette(t *testing.T) {
	s := newTestStore(t)
	libDir := t.TempDir()
	addPhotoWithThumb(t, s, libDir, "aa01", "/photos/a/red.jpg", color.RGBA{220, 40, 30, 255})
	addPhotoWithThumb(t, s, libDir, "bb02", "/photos/b/grey.jpg", color.RGBA{128, 128, 128, 255})
	if n := needing(t, s); n != 2 {
		t.Fatalf("needing before = %d, want 2", n)
	}

	got := runAnalyse(t, s, libDir)
	if last := got[len(got)-1]; !last.Finished || last.Done != 2 || last.Total != 2 {
		t.Errorf("last progress %+v, want 2 of 2, finished", last)
	}
	if n := needing(t, s); n != 0 {
		t.Errorf("needing after = %d, want 0", n)
	}

	var mono string
	s.db.QueryRow(`SELECT mono_class FROM photo_appearance WHERE photo_id='bb02'`).Scan(&mono) //nolint:errcheck
	if mono != string(appearance.Mono) {
		t.Errorf("grey photo mono_class %q", mono)
	}
	var bin *int
	var share float64
	if err := s.db.QueryRow(`SELECT hue_bin, share FROM photo_palette WHERE photo_id='aa01' AND rank=0`).Scan(&bin, &share); err != nil {
		t.Fatal(err)
	}
	if bin == nil || *bin != 0 || share != 1 {
		t.Errorf("red photo swatch bin %v share %.2f, want bin 0, whole photo", bin, share)
	}
	var greyBin *int
	s.db.QueryRow(`SELECT hue_bin FROM photo_palette WHERE photo_id='bb02'`).Scan(&greyBin) //nolint:errcheck
	if greyBin != nil {
		t.Errorf("grey swatch has hue bin %d, want none", *greyBin)
	}
}

func TestOutdatedAppearanceIsMeasuredAgain(t *testing.T) {
	s := newTestStore(t)
	libDir := t.TempDir()
	addPhotoWithThumb(t, s, libDir, "aa01", "/photos/a.jpg", color.RGBA{20, 128, 128, 255})
	runAnalyse(t, s, libDir)
	s.db.Exec(`UPDATE photo_appearance SET version = ?`, appearance.Version-1) //nolint:errcheck
	if n := needing(t, s); n != 1 {
		t.Errorf("needing = %d, want the outdated photo", n)
	}
}

func TestUnreadableThumbnailIsPassedOver(t *testing.T) {
	s := newTestStore(t)
	libDir := t.TempDir()
	addPhotoWithThumb(t, s, libDir, "aa01", "/photos/a.jpg", color.RGBA{20, 128, 128, 255})
	os.WriteFile(filepath.Join(libDir, "thumbs", "aa", "aa01.jpg"), []byte("not a jpeg"), 0o600) //nolint:errcheck
	got := runAnalyse(t, s, libDir)
	if last := got[len(got)-1]; !last.Finished || last.Error != "" {
		t.Errorf("last progress %+v, want finished without error", last)
	}
	if n := needing(t, s); n != 1 {
		t.Errorf("needing = %d, want it still to be measured", n)
	}
}

func TestClearAppearanceInFolderLeavesSiblingFolder(t *testing.T) {
	s := newTestStore(t)
	libDir := t.TempDir()
	addPhotoWithThumb(t, s, libDir, "aa01", "/photos/2024_Trip/a.jpg", color.RGBA{200, 90, 20, 255})
	addPhotoWithThumb(t, s, libDir, "bb02", "/photos/2024XTrip/b.jpg", color.RGBA{200, 90, 20, 255})
	runAnalyse(t, s, libDir)
	if err := s.ClearAppearanceInFolder("/photos/2024_Trip"); err != nil {
		t.Fatal(err)
	}
	tasks, _ := s.PhotosNeedingAppearance()
	if len(tasks) != 1 || tasks[0].ID != "aa01" {
		t.Errorf("needing %+v, want only the cleared folder's photo", tasks)
	}
}

func TestDeletingAPhotoDeletesItsAppearance(t *testing.T) {
	s := newTestStore(t)
	libDir := t.TempDir()
	addPhotoWithThumb(t, s, libDir, "aa01", "/photos/a.jpg", color.RGBA{200, 90, 20, 255})
	addPhotoWithThumb(t, s, libDir, "bb02", "/photos/b.jpg", color.RGBA{200, 90, 20, 255})
	runAnalyse(t, s, libDir)

	if _, _, err := s.DeletePhotoByID("aa01"); err != nil {
		t.Fatalf("DeletePhotoByID: %v", err)
	}
	s.db.Exec(`UPDATE photos SET status='missing' WHERE id='bb02'`) //nolint:errcheck
	if _, err := s.PurgeMissingPhotos(); err != nil {
		t.Fatalf("PurgeMissingPhotos: %v", err)
	}
	var left int
	s.db.QueryRow(`SELECT (SELECT COUNT(*) FROM photo_appearance) + (SELECT COUNT(*) FROM photo_palette)`).Scan(&left) //nolint:errcheck
	if left != 0 {
		t.Errorf("%d appearance rows left after deleting the photos", left)
	}
}

func TestSavingSkipsAPhotoDeletedSinceItWasMeasured(t *testing.T) {
	s := newTestStore(t)
	libDir := t.TempDir()
	addPhotoWithThumb(t, s, libDir, "aa01", "/photos/a.jpg", color.RGBA{200, 90, 20, 255})
	addPhotoWithThumb(t, s, libDir, "bb02", "/photos/b.jpg", color.RGBA{20, 90, 200, 255})
	if _, _, err := s.DeletePhotoByID("aa01"); err != nil {
		t.Fatal(err)
	}
	r := appearance.Result{Mono: appearance.Colour, Tone: appearance.Tone{Key: appearance.NormalKey}}
	if err := s.SaveAppearances([]MeasuredAppearance{{ID: "aa01", Result: r}, {ID: "bb02", Result: r}}); err != nil {
		t.Fatalf("SaveAppearances: %v", err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM photo_appearance`).Scan(&n) //nolint:errcheck
	if n != 1 {
		t.Errorf("%d rows, want only the photo that still exists", n)
	}
}
