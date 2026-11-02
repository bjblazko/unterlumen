package apilibrary

import (
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	lib "huepattl.de/unterlumen/internal/library"
)

// parseListPhotosOpts reads the photo filter shared by a single library's
// photo list and the search across libraries: EXIF text, numeric and date
// ranges, meta, album (by title or by membership), format, destination and
// paging.
func parseListPhotosOpts(q url.Values) lib.ListPhotosOpts {
	opts := lib.ListPhotosOpts{
		Filters:        parseTextFilters(q),
		NumericFilters: parseNumericFilters(q),
		DateMin:        q.Get("date_taken_min"),
		DateMax:        q.Get("date_taken_max"),
		MetaFilters:    parseMetaFilters(q),
		AlbumTitle:     q.Get("album_title"),
		People:         nonEmpty(q["person"]),
		Things:         nonEmpty(q["thing"]),
		Words:          strings.TrimSpace(q.Get("text")),
		FacesMin:       parseIntIn(q.Get("faces_min"), 0, math.MaxInt32),
		FacesMax:       parseIntIn(q.Get("faces_max"), 0, math.MaxInt32),
		ExtFilter:      q.Get("ext"),
		PathPrefix:     q.Get("pathPrefix"),
		Hour:           parseIntIn(q.Get("hour"), 0, 23),
		Weekday:        parseIntIn(q.Get("weekday"), 0, 6),
		Lacks:          nonEmpty(q["lacks"]),
		Device:         oneOf(q.Get("device"), "phone", "camera"),
		Orient:         oneOf(q.Get("orient"), "portrait", "landscape", "square"),
		Slow:           q.Get("slow") == "1",
		Flash:          q.Get("flash") == "1",
		Cropped:        q.Get("cropped") == "1",
		NoLens:         q.Get("nolens") == "1",
		EVMin:          parseIntIn(q.Get("ev_from"), -20, 30),
		EVMax:          parseIntIn(q.Get("ev_to"), -20, 30),
		RatioMin:       parseFloatIn(q.Get("ratio_from"), 1, 10),
		RatioMax:       parseFloatIn(q.Get("ratio_to"), 1, 10),
		TakenFrom:      momentOf(q.Get("taken_from")),
		TakenTo:        momentOf(q.Get("taken_to")),
		Area:           parseArea(q),
		Aspect:         q.Get("aspect"),
		Month:          parseIntIn(q.Get("month"), 1, 12),
		Mono:           q.Get("mono"),
		HueBin:         parseIntIn(q.Get("hue_bin"), 0, 11),
		Hues:           parseHues(q.Get("hues")),
		Scope:          searchScope(q),
		Warmth:         q.Get("warmth"),
		PhotoID:        q.Get("photo_id"),
	}
	if ch := q.Get("channel"); ch != "" {
		opts.MetaExists = append(opts.MetaExists, "built:"+ch)
	}
	// album=<channel>:<postID> is one gallery's membership, which survives a
	// rename; album_title matches whatever title the photos last recorded.
	if album := q.Get("album"); album != "" {
		opts.MetaExists = append(opts.MetaExists, "built:"+album)
	}
	opts.Offset, _ = strconv.Atoi(q.Get("offset"))
	opts.Limit, _ = strconv.Atoi(q.Get("limit"))
	if opts.Limit <= 0 || opts.Limit > 500 {
		opts.Limit = 100
	}
	return opts
}

// insightParams are the parameters a Statistics click sets
// (library/insight_terms.go); none is an EXIF field.
var insightParams = map[string]bool{"device": true, "orient": true, "slow": true, "flash": true, "cropped": true,
	"nolens": true, "ev_from": true, "ev_to": true, "ratio_from": true, "ratio_to": true, "taken_from": true, "taken_to": true,
	"lat_from": true, "lat_to": true, "lon_from": true, "lon_to": true}

// parseArea is the box lat_from…lat_to, lon_from…lon_to, or nil without all four.
func parseArea(q url.Values) *[4]float64 {
	var a [4]float64
	for i, k := range []string{"lat_from", "lat_to", "lon_from", "lon_to"} {
		lim := 180.0
		if i < 2 {
			lim = 90
		}
		v := parseFloatIn(q.Get(k), -lim-1, lim+1)
		if v == nil {
			return nil
		}
		a[i] = *v
	}
	return &a
}

var momentPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$`)

// momentOf is a moment as taken, "2024-05-01T10:00:00", or "".
func momentOf(v string) string {
	if momentPattern.MatchString(v) {
		return v
	}
	return ""
}

// oneOf is v when it is one of choices, else "".
func oneOf(v string, choices ...string) string {
	for _, c := range choices {
		if v == c {
			return v
		}
	}
	return ""
}

// parseFloatIn is a number from lo to hi, or nil for none or anything else.
func parseFloatIn(s string, lo, hi float64) *float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f < lo || f > hi {
		return nil
	}
	return &f
}

// parseIntIn is a whole number from lo to hi, or nil for none or anything else.
func parseIntIn(s string, lo, hi int) *int {
	n, err := strconv.Atoi(s)
	if err != nil || n < lo || n > hi {
		return nil
	}
	return &n
}

// parseHues reads a colour combination, "1,6": up to three distinct hue
// sectors 0–11; anything else in the list is left out.
func parseHues(s string) []int {
	var out []int
	seen := map[int]bool{}
	for _, part := range strings.Split(s, ",") {
		bin := parseIntIn(strings.TrimSpace(part), 0, 11)
		if bin == nil || seen[*bin] || len(out) == 3 {
			continue
		}
		seen[*bin] = true
		out = append(out, *bin)
	}
	return out
}

func parseIDList(s string) []string {
	if s == "" {
		return nil
	}
	var ids []string
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// parseTextFilters extracts non-reserved query params as EXIF text filters.
// Reserved params: q, offset, limit, ids, date_taken_min/max, _min/_max numeric params,
// and the meta/channel/album/ext params handled separately.
func parseTextFilters(vals map[string][]string) map[string]string {
	out := make(map[string]string)
	for k, vs := range vals {
		if k == "q" || k == "offset" || k == "limit" || k == "ids" || len(vs) == 0 {
			continue
		}
		if strings.HasSuffix(k, "_min") || strings.HasSuffix(k, "_max") {
			continue
		}
		if k == "channel" || k == "album" || k == "album_title" || k == "person" || k == "thing" || k == "text" || k == "ext" || k == "date_taken_min" || k == "date_taken_max" {
			continue
		}
		if k == "pathPrefix" || k == "hour" || k == "aspect" || k == "month" || k == "mono" || k == "hue_bin" || k == "hues" || k == "warmth" || k == "model" || k == "lens" || k == "month_from" || k == "month_until" || k == "photo_id" || k == "kind" || k == "rating" || k == "weekday" || k == "lacks" || insightParams[k] {
			continue
		}
		if strings.HasPrefix(k, "meta_") {
			continue
		}
		if vs[0] != "" {
			out[k] = vs[0]
		}
	}
	return out
}

// parseMetaFilters extracts meta_<key>=<value> params as photo_meta key→value filters.
func parseMetaFilters(vals map[string][]string) map[string]string {
	out := make(map[string]string)
	for k, vs := range vals {
		key, found := strings.CutPrefix(k, "meta_")
		if !found || len(vs) == 0 || vs[0] == "" {
			continue
		}
		out[key] = vs[0]
	}
	return out
}

func parseNumericFilters(vals map[string][]string) map[string]lib.NumericFilter {
	type bounds struct{ min, max *float64 }
	bmap := make(map[string]*bounds)
	ensure := func(field string) *bounds {
		if bmap[field] == nil {
			bmap[field] = &bounds{}
		}
		return bmap[field]
	}
	for k, vs := range vals {
		if len(vs) == 0 {
			continue
		}
		if field, suffix, ok := strings.Cut(k, "_min"); ok && suffix == "" {
			if v, err := strconv.ParseFloat(vs[0], 64); err == nil {
				ensure(field).min = &v
			}
			continue
		}
		if field, suffix, ok := strings.Cut(k, "_max"); ok && suffix == "" {
			if v, err := strconv.ParseFloat(vs[0], 64); err == nil {
				ensure(field).max = &v
			}
		}
	}
	out := make(map[string]lib.NumericFilter)
	for field, b := range bmap {
		f := lib.NumericFilter{Min: 0, Max: math.MaxFloat64}
		if b.min != nil {
			f.Min = *b.min
		}
		if b.max != nil {
			f.Max = *b.max
		}
		out[field] = f
	}
	return out
}

// searchScope is the shared filter of a search. Its people and things are
// the search's own already: a library's filter and a chart's photos both
// send person and thing, and asking twice would only slow the search.
func searchScope(q url.Values) lib.Filter {
	f := filterFromQuery(q)
	f.People, f.Things = nil, nil
	return f
}
