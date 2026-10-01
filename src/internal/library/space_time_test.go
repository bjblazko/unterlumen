package library

import (
	"math"
	"testing"
)

// A located photo without a date has no height and is left out; one not
// analysed yet keeps its place with L -1.
func TestBuildSpaceTime(t *testing.T) {
	st := BuildSpaceTime([]LibrarySpaceTime{{
		LibraryID: "x",
		Places: []GeoPoint{
			{ID: "home", Lat: 50.94, Lon: 6.96, Taken: "2024-05-01T10:30:00"},
			{ID: "new", Lat: 57.1, Lon: -2.1, Taken: "2024-06-01T09:00:00"},
			{ID: "undated", Lat: 1, Lon: 1},
		},
		Colours: map[string]LCh{"home": {L: 0.6, C: 0.1, H: 90}},
	}})
	if st.Photos != 2 || st.Libraries[0] != "x" {
		t.Fatalf("got %+v", st)
	}
	if st.Points.L[0] != 0.6 || st.Points.B[0] != 0.1 || st.Points.L[1] != -1 {
		t.Errorf("colours %v %v %v", st.Points.L, st.Points.A, st.Points.B)
	}
}

// LocatedPhotos reads the tags as exif_index keeps them, quotes included,
// and leaves out photos without a date, without a fix, or in another folder.
func TestLocatedPhotos(t *testing.T) {
	s := newTestStore(t)
	seed := func(id, path, date, lat, latRef, lon, lonRef string) {
		seedExposure(t, s, id, path, date, map[string]any{
			"GPSLatitude": lat, "GPSLatitudeRef": latRef, "GPSLongitude": lon, "GPSLongitudeRef": lonRef})
	}
	seed("trip", "/lib/trip/a.jpg", "2024-05-01T10:00:00", `["57/1","9/1","0/1"]`, `"N"`, `["2/1","6/1","0/1"]`, `"W"`)
	seed("home", "/lib/home/b.jpg", "2024-05-02T10:00:00", `["50/1","56/1","0/1"]`, `"N"`, `["6/1","57/1","0/1"]`, `"E"`)
	seed("undated", "/lib/trip/c.jpg", "", `["57/1","9/1","0/1"]`, `"N"`, `["2/1","6/1","0/1"]`, `"W"`)
	seed("nofix", "/lib/trip/d.jpg", "2024-05-03T10:00:00", `["0/1","0/1","0/1"]`, `"N"`, `["0/1","0/1","0/1"]`, `"E"`)
	points, err := s.LocatedPhotos("/lib/trip")
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].ID != "trip" {
		t.Fatalf("points = %+v, want only the dated trip photo with a fix", points)
	}
	if p := points[0]; math.Abs(p.Lat-57.15) > 1e-9 || math.Abs(p.Lon+2.1) > 1e-9 {
		t.Errorf("trip at %v, %v, want 57.15, -2.1", p.Lat, p.Lon)
	}
	if all, _ := s.LocatedPhotos(""); len(all) != 2 {
		t.Errorf("whole library: %d, want 2", len(all))
	}
}
