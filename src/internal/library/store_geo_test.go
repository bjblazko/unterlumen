package library

import (
	"math"
	"testing"
)

func geoFixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	seedStatsPhotos(t, s, []statsPhoto{
		{id: "parsed", path: "/lib/a.jpg", date: "2024-05-01T10:30:00", exifJSON: `{"latitude":48.1,"longitude":11.5}`},
		{id: "tags", path: "/lib/b.jpg", exifJSON: `{"tags":{"GPSLatitude":"[33/1, 51/1, 0/1]","GPSLatitudeRef":"S","GPSLongitude":"[151/1, 12/1, 0/1]","GPSLongitudeRef":"E"}}`},
		{id: "nofix", path: "/lib/c.jpg", exifJSON: `{"latitude":0,"longitude":0}`},
		{id: "nowhere", path: "/lib/d.jpg", exifJSON: `{"width":10}`},
		{id: "broken", path: "/lib/e.jpg", exifJSON: `{"tags":{"GPSLatitude":"north","GPSLongitude":"east"}}`},
		{id: "gone", path: "/lib/f.jpg", exifJSON: `{"latitude":1,"longitude":1}`, missing: true},
	})
	return s
}

func TestGeoPointsKeepsOnlyUsableLocations(t *testing.T) {
	points, err := geoFixture(t).GeoPoints()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]GeoPoint{}
	for _, p := range points {
		byID[p.ID] = p
	}
	if len(byID) != 2 {
		t.Fatalf("points = %+v, want only parsed and tags", points)
	}
	if p := byID["parsed"]; p.Lat != 48.1 || p.Lon != 11.5 || p.Taken != "2024-05-01T10:30:00" || p.Filename != "parsed.jpg" {
		t.Errorf("parsed = %+v", p)
	}
	if p := byID["tags"]; math.Abs(p.Lat+33.85) > 1e-9 || math.Abs(p.Lon-151.2) > 1e-9 || p.Taken != "" {
		t.Errorf("tags = %+v, want -33.85, 151.2, undated", p)
	}
}
