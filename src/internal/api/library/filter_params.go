package apilibrary

import (
	"math"
	"net/url"
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
		ExtFilter:      q.Get("ext"),
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
		if k == "channel" || k == "album" || k == "album_title" || k == "ext" || k == "date_taken_min" || k == "date_taken_max" {
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
