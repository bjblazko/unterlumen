package library

import (
	"math"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := openStore(":memory:", t.TempDir())
	if err != nil {
		t.Fatalf("openStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func insertPhoto(t *testing.T, s *Store, id string, fl, fl35 *float64) {
	t.Helper()
	if err := s.UpsertPhoto(id, "", id+".jpg", 0, time.Now(), "", ""); err != nil {
		t.Fatalf("UpsertPhoto %s: %v", id, err)
	}
	fields := map[string]string{}
	numeric := map[string]float64{}
	if fl != nil {
		fields["FocalLength"] = "x"
		numeric["FocalLength"] = *fl
	}
	if fl35 != nil {
		fields["FocalLengthIn35mmFilm"] = "x"
		numeric["FocalLengthIn35mmFilm"] = *fl35
	}
	if err := s.UpsertExifIndex(id, fields, numeric); err != nil {
		t.Fatalf("UpsertExifIndex %s: %v", id, err)
	}
}

func fp(v float64) *float64 { return &v }

// TestFocalLength35Range verifies GetExifRanges returns the correct combined
// range for the virtual "FocalLength35" key, preferring FocalLengthIn35mmFilm
// where present and falling back to FocalLength otherwise.
func TestFocalLength35Range(t *testing.T) {
	s := newTestStore(t)

	// Photo A: has 35mm data (35mm=75) and native (fl=50) → 35mm value used
	insertPhoto(t, s, "a", fp(50), fp(75))
	// Photo B: no 35mm data, native fl=28 → fl used as 35mm equiv
	insertPhoto(t, s, "b", fp(28), nil)
	// Photo C: 35mm=200, no native fl
	insertPhoto(t, s, "c", nil, fp(200))

	ranges, err := s.GetExifRanges([]string{"FocalLength35"})
	if err != nil {
		t.Fatalf("GetExifRanges: %v", err)
	}
	r, ok := ranges["FocalLength35"]
	if !ok {
		t.Fatal("FocalLength35 range not returned")
	}
	if math.Abs(r.Min-28.0) > 1e-9 {
		t.Errorf("Min = %v, want 28.0", r.Min)
	}
	if math.Abs(r.Max-200.0) > 1e-9 {
		t.Errorf("Max = %v, want 200.0", r.Max)
	}
}

// TestListPhotosFocalLength35Filter verifies the fallback filter:
// matches photos whose 35mm equiv (or FocalLength fallback) is in range.
func TestListPhotosFocalLength35Filter(t *testing.T) {
	s := newTestStore(t)

	// Photo A: FocalLengthIn35mmFilm=35, FocalLength=24 → 35mm value matches [30,40]
	insertPhoto(t, s, "a", fp(24), fp(35))
	// Photo B: no 35mm data, FocalLength=35 → fallback matches [30,40]
	insertPhoto(t, s, "b", fp(35), nil)
	// Photo C: FocalLengthIn35mmFilm=85, FocalLength=50 → 35mm value outside [30,40]
	insertPhoto(t, s, "c", fp(50), fp(85))
	// Photo D: no focal length at all → excluded
	insertPhoto(t, s, "d", nil, nil)

	numericFilters := map[string]NumericFilter{
		"FocalLength35": {Min: 30, Max: 40},
	}
	result, err := s.ListPhotos("", nil, numericFilters, 0, 100)
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	got := map[string]bool{}
	for _, p := range result.Photos {
		got[p.ID] = true
	}
	if !got["a"] {
		t.Error("photo a (35mm=35) should match [30,40]")
	}
	if !got["b"] {
		t.Error("photo b (fallback fl=35) should match [30,40]")
	}
	if got["c"] {
		t.Error("photo c (35mm=85) should NOT match [30,40]")
	}
	if got["d"] {
		t.Error("photo d (no focal length) should NOT match [30,40]")
	}
}

// TestPurgeMissingPhotos verifies that PurgeMissingPhotos removes missing photos
// and their dependent rows, leaving ok photos untouched.
func TestPurgeMissingPhotos(t *testing.T) {
	s := newTestStore(t)

	insertPhoto(t, s, "keep", fp(50), nil)
	insertPhoto(t, s, "gone", fp(35), nil)

	// Simulate a re-scan that only found "keep".
	if _, err := s.db.Exec(`UPDATE photos SET status='missing' WHERE id='gone'`); err != nil {
		t.Fatalf("mark missing: %v", err)
	}

	n, err := s.PurgeMissingPhotos()
	if err != nil {
		t.Fatalf("PurgeMissingPhotos: %v", err)
	}
	if n != 1 {
		t.Errorf("purged %d, want 1", n)
	}

	// "keep" must still be present with status ok.
	result, err := s.ListPhotos("", nil, nil, 0, 10)
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if result.Total != 1 || result.Photos[0].ID != "keep" {
		t.Errorf("expected only 'keep', got %+v", result.Photos)
	}

	// "gone" must be fully removed — no orphan rows.
	var count int
	s.db.QueryRow(`SELECT COUNT(1) FROM photos WHERE id='gone'`).Scan(&count)       //nolint:errcheck
	s.db.QueryRow(`SELECT COUNT(1) FROM exif_index WHERE photo_id='gone'`).Scan(&count) //nolint:errcheck
	if count != 0 {
		t.Error("orphan exif_index rows remain after purge")
	}
}
