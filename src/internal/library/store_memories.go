package library

import (
	"context"
	"database/sql"
	"sort"
	"strings"
)

// MemoryPhoto is a dated photo with what Memories choose by (ADR-0064):
// where it was taken, what and who it shows, and how it looks.
type MemoryPhoto struct {
	LibraryID, ID, Filename string
	Taken                   string // ISO date taken, local, as indexed
	Width, Height           int    // stored size, before orientation; 0 when unknown
	Orientation             string // EXIF Orientation tag; "" when absent
	Lat, Lon                float64
	Located                 bool
	Country                 string   // "" when none
	Things                  []string // sorted
	People                  []string // the names of its confirmed faces, sorted, once each
	HueBin                  int      // the main colour's hue bin; -1 when none
	Tone                    string   // low, normal or high; "" when not measured
	DHash                   uint64
	Hashed                  bool
}

// The queries Memories read a library with. {where} is the condition on
// photos p that makes a photo one to choose from; each query takes the set
// once, never per row (ADR-0045). The + before a photo_id keeps SQLite from
// looking each photo up by its primary key; the table is read through its
// own index and the set checked with a Bloom filter — 0.1 s instead of 1.4 s
// for the countries of 31,000 photos.
const (
	memoryPhotosQuery = `SELECT p.id, p.filename, p.date_taken, COALESCE(p.width, 0), COALESCE(p.height, 0),
		COALESCE(CAST(json_extract(p.exif_json, '$.tags.Orientation') AS TEXT), ''), p.lat, p.lon
		FROM photos p WHERE {where}`
	memoryCountryQuery = `SELECT photo_id, TRIM(value, '" ') FROM exif_index
		WHERE field = 'Country' AND +photo_id IN (SELECT p.id FROM photos p WHERE {where})`
	memoryThingsQuery = `SELECT photo_id, thing FROM photo_things
		WHERE photo_id IN (SELECT p.id FROM photos p WHERE {where})`
	memoryPeopleQuery = `SELECT DISTINCT fa.photo_id, pe.name FROM faces fa JOIN people pe ON pe.id = fa.person_id
		WHERE fa.confirmed = 1 AND fa.photo_id IN (SELECT p.id FROM photos p WHERE {where})`
	memoryLookQuery = `SELECT a.photo_id, a.tone_key, a.dhash, pl.hue_bin FROM photo_appearance a
		LEFT JOIN photo_palette pl ON pl.photo_id = a.photo_id AND pl.rank = 0
		WHERE a.photo_id IN (SELECT p.id FROM photos p WHERE {where})`
)

var memoryQueries = []string{memoryPhotosQuery, memoryCountryQuery, memoryThingsQuery, memoryPeopleQuery, memoryLookQuery}

// memoryQuery fills in the photos to choose from: ok, dated, not a video,
// within the store's filter.
func (s *Store) memoryQuery(q string) (string, []any) {
	fw, fa := s.filter.Cond("p.id")
	where := "p.status = 'ok' AND p.date_taken IS NOT NULL AND " + notVideo + fw
	return strings.Replace(q, "{where}", where, 1), fa
}

// MemoryPhotos returns the photos Memories choose from, LibraryID empty.
func (s *Store) MemoryPhotos(ctx context.Context) ([]MemoryPhoto, error) {
	photos, err := s.memoryBasics(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*MemoryPhoto, len(photos))
	for i := range photos {
		byID[photos[i].ID] = &photos[i]
	}
	for _, st := range []struct {
		query string
		scan  memoryScan
	}{{memoryCountryQuery, scanCountry}, {memoryThingsQuery, scanThing}, {memoryPeopleQuery, scanPerson}, {memoryLookQuery, scanLook}} {
		if err := s.eachMemoryRow(ctx, st.query, byID, st.scan); err != nil {
			return nil, err
		}
	}
	for i := range photos {
		sort.Strings(photos[i].Things)
		sort.Strings(photos[i].People)
	}
	return photos, nil
}

func (s *Store) memoryBasics(ctx context.Context) ([]MemoryPhoto, error) {
	q, args := s.memoryQuery(memoryPhotosQuery)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	photos := []MemoryPhoto{}
	for rows.Next() {
		p := MemoryPhoto{HueBin: -1}
		var lat, lon sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.Filename, &p.Taken, &p.Width, &p.Height, &p.Orientation, &lat, &lon); err != nil {
			return nil, err
		}
		p.Lat, p.Lon, p.Located = lat.Float64, lon.Float64, lat.Valid && lon.Valid
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// memoryScan reads one row: the photo it is about, and what it adds to it.
type memoryScan func(*sql.Rows) (string, func(*MemoryPhoto), error)

// eachMemoryRow runs a query and adds each row to its photo.
func (s *Store) eachMemoryRow(ctx context.Context, query string, byID map[string]*MemoryPhoto, scan memoryScan) error {
	q, args := s.memoryQuery(query)
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		id, add, err := scan(rows)
		if err != nil {
			return err
		}
		if p := byID[id]; p != nil {
			add(p)
		}
	}
	return rows.Err()
}

func scanCountry(r *sql.Rows) (string, func(*MemoryPhoto), error) {
	var id, country string
	err := r.Scan(&id, &country)
	return id, func(p *MemoryPhoto) { p.Country = country }, err
}

func scanThing(r *sql.Rows) (string, func(*MemoryPhoto), error) {
	var id, thing string
	err := r.Scan(&id, &thing)
	return id, func(p *MemoryPhoto) { p.Things = append(p.Things, thing) }, err
}

func scanPerson(r *sql.Rows) (string, func(*MemoryPhoto), error) {
	var id, name string
	err := r.Scan(&id, &name)
	return id, func(p *MemoryPhoto) { p.People = append(p.People, name) }, err
}

func scanLook(r *sql.Rows) (string, func(*MemoryPhoto), error) {
	var id, tone string
	var hash, bin sql.NullInt64
	err := r.Scan(&id, &tone, &hash, &bin)
	return id, func(p *MemoryPhoto) {
		p.Tone = tone
		p.DHash, p.Hashed = uint64(hash.Int64), hash.Valid
		if bin.Valid {
			p.HueBin = int(bin.Int64)
		}
	}, err
}
