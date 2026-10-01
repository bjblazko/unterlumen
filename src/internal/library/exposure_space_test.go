package library

import (
	"testing"
	"time"
)

func seedExposure(t *testing.T, s *Store, id, path, date string, exif map[string]any) {
	t.Helper()
	if err := s.UpsertPhoto(id, path, id+".jpg", 1, time.Now(), "{}", "", date, "jpeg"); err != nil {
		t.Fatal(err)
	}
	for field, v := range exif {
		var err error
		switch v := v.(type) {
		case string:
			_, err = s.db.Exec(`INSERT INTO exif_index VALUES (?, ?, ?, NULL)`, id, field, v)
		case float64:
			_, err = s.db.Exec(`INSERT INTO exif_index VALUES (?, ?, '', ?)`, id, field, v)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

// A photo needs all three settings; the 35 mm focal length wins over the
// one taken, which stands in when the camera wrote none.
func TestExposurePoints(t *testing.T) {
	s := newTestStore(t)
	seedExposure(t, s, "both", "/lib/a/1.jpg", "2024-07-01T10:00:00", map[string]any{
		"FocalLength": 23.0, "FocalLengthIn35mmFilm": 35.0, "FNumber": 2.0, "ISOSpeedRatings": 200.0, "Model": `"X-T50"`})
	seedExposure(t, s, "taken", "/lib/b/2.jpg", "2024-07-02T10:00:00", map[string]any{
		"FocalLength": 50.0, "FNumber": 4.0, "ISOSpeedRatings": 800.0})
	seedExposure(t, s, "noiso", "/lib/b/3.jpg", "2024-07-03T10:00:00", map[string]any{
		"FocalLength": 50.0, "FNumber": 4.0})
	points, err := s.ExposurePoints("")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ExposurePoint{}
	for _, p := range points {
		got[p.ID] = p
	}
	if len(points) != 2 {
		t.Fatalf("points %+v, want the two with all settings", points)
	}
	if b := got["both"]; b.Focal != 35 || b.FNum != 2 || b.ISO != 200 || b.Camera != `"X-T50"` {
		t.Errorf("both = %+v", b)
	}
	if got["taken"].Focal != 50 || got["taken"].Camera != "" {
		t.Errorf("taken = %+v", got["taken"])
	}
	if scoped, _ := s.ExposurePoints("/lib/b"); len(scoped) != 1 || scoped[0].ID != "taken" {
		t.Errorf("scoped = %+v", scoped)
	}
}

func TestBuildExposureSpace(t *testing.T) {
	var points []ExposurePoint
	for i, cam := range []string{`"A"`, `"A"`, `"A"`, `"B"`, `"B"`, `"C"`, `"D"`, `"E"`, `"F"`, `"G "`, ""} {
		points = append(points, ExposurePoint{ID: string(rune('a' + i)), Date: "2024-01-0" + string(rune('1'+i%9)), Camera: cam,
			Focal: float64(10 * (i + 1)), FNum: 2, ISO: 100})
	}
	es := BuildExposureSpace([]LibraryExposure{{LibraryID: "x", Points: points}}, "")
	if len(es.Cameras) != topCameraCount+1 || es.Cameras[0] != "A" || es.Cameras[1] != "B" || es.Cameras[topCameraCount] != "Other" {
		t.Errorf("cameras %v, want A and B first and Other last", es.Cameras)
	}
	if es.Photos != len(points) || len(es.Points.Camera) != len(points) {
		t.Errorf("photos %d, points %d", es.Photos, len(es.Points.Camera))
	}
	if es.Points.Camera[len(points)-1] != topCameraCount {
		t.Errorf("a photo without a camera tag falls in Other, got %d", es.Points.Camera[len(points)-1])
	}
	if len(es.Path) != 1 || es.Path[0].Photos != len(points) || es.Path[0].Focal != 60 {
		t.Errorf("path %+v, want one month with the median focal length 60", es.Path)
	}
}

func TestMedianBetweenTwoStops(t *testing.T) {
	if got := median([]float64{100, 400}, func(v float64) float64 { return v }); got != 200 {
		t.Errorf("median of ISO 100 and 400 = %v, want 200, the stop between them", got)
	}
}

func TestCameraLabel(t *testing.T) {
	for in, want := range map[string]string{`"X-T50"`: "X-T50", `"Spca1628 "`: "Spca1628", "": "Unknown camera"} {
		if got := cameraLabel(in); got != want {
			t.Errorf("cameraLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
