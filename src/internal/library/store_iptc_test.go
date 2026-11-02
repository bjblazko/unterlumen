package library

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// The IPTC fields reach the library filter: a plain field by its value, a
// list field (Person Shown) by one of its entries (ADR-0061).
func TestIPTCFilterFindsByValueAndEntry(t *testing.T) {
	s := listFixture(t)
	res, _ := s.ListPhotos(context.Background(), ListPhotosOpts{Limit: 1000})
	if len(res.Photos) < 2 {
		t.Skip("the fixture has too few photos")
	}
	a, b := res.Photos[0].ID, res.Photos[1].ID
	s.UpsertExifIndex(a, map[string]string{"City": "Kiel", "PersonInImage": "Ann; Bob"}, nil) //nolint:errcheck
	s.UpsertExifIndex(b, map[string]string{"City": "Lübeck", "PersonInImage": "Bob"}, nil)    //nolint:errcheck
	count := func(field, value string) int {
		got, err := s.ListPhotos(context.Background(), ListPhotosOpts{Limit: 1000, Filters: map[string]string{field: value}})
		if err != nil {
			t.Fatal(err)
		}
		return got.Total
	}
	if n := count("City", "Kiel"); n != 1 {
		t.Errorf("Kiel: %d photos, want 1", n)
	}
	if n := count("PersonInImage", "Bob"); n != 2 {
		t.Errorf("Bob: %d photos, want 2", n)
	}
	if n := count("PersonInImage", "Ann; Bob"); n != 0 {
		t.Errorf("the joined list matched as one entry: %d", n)
	}
	vals, _ := s.GetExifFieldValues("PersonInImage")
	if !slices.Equal(vals, []string{"Ann", "Bob"}) {
		t.Errorf("values %v", vals)
	}
	s.UpsertExifIndex(a, map[string]string{"City": "Kiel"}, nil) //nolint:errcheck
	if n := count("PersonInImage", "Ann"); n != 0 {
		t.Errorf("an entry stayed after the photo lost it: %d", n)
	}
}

func TestEntryFilterIsNotCorrelated(t *testing.T) {
	s := listFixture(t)
	fromSQL, whereSQL, args := newPhotoFilter(ListPhotosOpts{Filters: map[string]string{"PersonInImage": "Ann"}}).sql()
	rows, err := s.db.Query(`EXPLAIN QUERY PLAN SELECT COUNT(*) `+fromSQL+` WHERE `+whereSQL, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		rows.Scan(&id, &parent, &unused, &detail) //nolint:errcheck
		plan = append(plan, detail)
	}
	joined := strings.Join(plan, "; ")
	if strings.Contains(joined, "CORRELATED") || !strings.Contains(joined, "exif_entries") {
		t.Fatalf("the entry filter runs once per photo or not from its table: %s", joined)
	}
}
