package apilibrary

import (
	"net/url"
	"reflect"
	"testing"

	lib "huepattl.de/unterlumen/internal/library"
)

// TestParseListPhotosOpts pins the one filter vocabulary shared by a single
// library's photo list and the search across libraries.
func TestParseListPhotosOpts(t *testing.T) {
	q, _ := url.ParseQuery("ids=a,b&offset=200&limit=50" +
		"&ISOSpeedRatings_min=200&ISOSpeedRatings_max=6400" +
		"&date_taken_min=2024-01-01&date_taken_max=2024-12-31" +
		"&Model=X-T50&meta_rating=5&album_title=Spring&ext=jpg&channel=site&album=site:a1b2")

	got := parseListPhotosOpts(q)
	want := lib.ListPhotosOpts{
		Filters:        map[string]string{"Model": "X-T50"},
		NumericFilters: map[string]lib.NumericFilter{"ISOSpeedRatings": {Min: 200, Max: 6400}},
		DateMin:        "2024-01-01",
		DateMax:        "2024-12-31",
		MetaFilters:    map[string]string{"rating": "5"},
		MetaExists:     []string{"built:site", "built:site:a1b2"},
		AlbumTitle:     "Spring",
		ExtFilter:      "jpg",
		Offset:         200,
		Limit:          50,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseListPhotosOpts:\n got  %+v\n want %+v", got, want)
	}
}

func TestParseListPhotosOptsClampsLimit(t *testing.T) {
	for _, limit := range []string{"", "0", "-1", "501"} {
		q := url.Values{"limit": {limit}}
		if got := parseListPhotosOpts(q).Limit; got != 100 {
			t.Errorf("limit=%q: got %d, want 100", limit, got)
		}
	}
	if got := parseListPhotosOpts(url.Values{"limit": {"500"}}).Limit; got != 500 {
		t.Errorf("limit=500: got %d, want 500", got)
	}
}

// The statistics' scope and marks are filters of their own, never EXIF text.
func TestParseListPhotosOptsStatisticsFilters(t *testing.T) {
	q, _ := url.ParseQuery("pathPrefix=/photos/2024&hour=7&aspect=3:2")
	got := parseListPhotosOpts(q)
	if got.PathPrefix != "/photos/2024" || got.Hour == nil || *got.Hour != 7 || got.Aspect != "3:2" {
		t.Errorf("got path %q, hour %v, aspect %q", got.PathPrefix, got.Hour, got.Aspect)
	}
	if len(got.Filters) != 0 {
		t.Errorf("text filters = %v, want none", got.Filters)
	}
	for _, bad := range []string{"", "24", "-1", "noon"} {
		if h := parseListPhotosOpts(url.Values{"hour": {bad}}).Hour; h != nil {
			t.Errorf("hour=%q: got %d, want none", bad, *h)
		}
	}
}

// The Colour statistics' marks are filters of their own, never EXIF text.
func TestParseListPhotosOptsColourFilters(t *testing.T) {
	q, _ := url.ParseQuery("mono=colour&hue_bin=2&warmth=warm&month=7")
	got := parseListPhotosOpts(q)
	if got.Mono != "colour" || got.HueBin == nil || *got.HueBin != 2 || got.Warmth != "warm" || got.Month == nil || *got.Month != 7 {
		t.Errorf("got mono %q, hue %v, warmth %q, month %v", got.Mono, got.HueBin, got.Warmth, got.Month)
	}
	if len(got.Filters) != 0 {
		t.Errorf("text filters = %v, want none", got.Filters)
	}
	for _, bad := range []url.Values{{"hue_bin": {"12"}}, {"hue_bin": {"-1"}}, {"month": {"0"}}, {"month": {"13"}}} {
		if o := parseListPhotosOpts(bad); o.HueBin != nil || o.Month != nil {
			t.Errorf("%v: got hue %v, month %v, want none", bad, o.HueBin, o.Month)
		}
	}
}
