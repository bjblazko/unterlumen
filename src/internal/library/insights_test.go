package library

import (
	"context"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// insightsFixture: a phone photo and two camera photos with what the gear,
// frame and catalogue views read.
func insightsFixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	seedStatsPhotos(t, s, []statsPhoto{
		{id: "a", path: "/lib/a.jpg", ext: "jpeg", date: "2024-05-01T10:00:00", exifJSON: `{"width":4000,"height":3000,"latitude":54.32,"longitude":10.12}`,
			fields: map[string]string{"Make": `"Apple"`, "Model": `"iPhone 12"`, "LensModel": `"iPhone back"`, "FocalLength": "4", "FocalLengthIn35mmFilm": "26",
				"ExposureTime": `"1/10"`, "FNumber": "2", "ISOSpeedRatings": "100", "Flash": "16", "Orientation": "6", "WhiteBalance": "0", "Rating": "4",
				"Country": "Deutschland", "Keywords": "sea; harbour"},
			numeric: map[string]float64{"FocalLength": 4, "FocalLengthIn35mmFilm": 26, "ExposureTime": 0.1, "FNumber": 2, "ISOSpeedRatings": 100}},
		{id: "b", path: "/lib/b.jpg", ext: "jpeg", date: "2024-05-02T10:00:00", exifJSON: `{"width":6000,"height":4000}`,
			fields: map[string]string{"Make": `"FUJIFILM"`, "Model": `"X-T50"`, "LensModel": `"XF23"`, "FocalLength": "23", "FocalLengthIn35mmFilm": "35",
				"ExposureTime": `"1/250"`, "FNumber": "8", "ISOSpeedRatings": "100", "Flash": "9", "ExposureBiasValue": `"-1/3"`, "WhiteBalance": "1", "Keywords": "sea"},
			numeric: map[string]float64{"FocalLength": 23, "FocalLengthIn35mmFilm": 35, "ExposureTime": 0.004, "FNumber": 8, "ISOSpeedRatings": 100}},
		{id: "c", path: "/lib/c.jpg", ext: "jpeg", date: "2023-01-02T10:00:00", exifJSON: `{"width":3000,"height":2000}`,
			fields: map[string]string{"Make": `"FUJIFILM"`, "Model": `"X-T50"`, "ExposureBiasValue": `"2/3"`}},
	})
	s.UpsertMeta("a", "stock", "Portra") //nolint:errcheck
	return s
}

func TestEveryInsightGroupRuns(t *testing.T) {
	s := insightsFixture(t)
	for group, count := range insightGroups {
		if _, err := count(s, s.statsScope("")); err != nil {
			t.Errorf("%s: %v", group, err)
		}
	}
}

func key(parts ...string) string { return strings.Join(parts, InsightSep) }

func TestInsightsCountWhatTheViewsShow(t *testing.T) {
	s := insightsFixture(t)
	sc := s.statsScope("")
	gear, _ := s.gearInsights(sc)
	frame, _ := s.frameInsights(sc)
	cat, _ := s.catalogueInsights(sc)
	places, _ := s.placesInsights(sc)
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"phone in 2024-05", gear["device"][key("2024-05", "phone")], 1},
		{"camera in 2024-05", gear["device"][key("2024-05", "camera")], 1},
		{"XF23 at 23 mm", gear["lensFocal"][key("XF23", "23")], 1},
		{"phone lens slow (1/10 at 26 mm)", gear["shake"][key("iPhone back", "slow")], 1},
		{"XF23 sharp (1/250 at 35 mm)", gear["shake"][key("XF23", "ok")], 1},
		{"EV 5 (f/2, 1/10, ISO 100)", gear["ev"]["5"], 1},
		{"flash fired in 2024-05", gear["flash"][key("2024-05", "fired")], 1},
		{"flash not fired in 2024-05", gear["flash"][key("2024-05", "not")], 1},
		{"minus in 2024-05", gear["bias"][key("2024-05", "minus")], 1},
		{"plus in 2023-01", gear["bias"][key("2023-01", "plus")], 1},
		{"auto white balance", gear["whiteBalance"]["0"], 1},
		{"iPhone turned to portrait", frame["orient"][key("iPhone 12", "portrait")], 1},
		{"X-T50 landscape", frame["orient"][key("X-T50", "landscape")], 2},
		{"X-T50 cropped in 2023-01", frame["cropped"][key("2023-01", "cropped")], 1},
		{"ratio 1.50", frame["ratio"]["1.50"], 2},
		{"four stars in 2024-05", cat["stars"][key("2024-05", "4")], 1},
		{"no stars in 2024-05", cat["stars"][key("2024-05", "0")], 1},
		{"keyword sea", cat["keywords"]["sea"], 2},
		{"own field", cat["fields"][key("stock", "Portra")], 1},
		{"a day in Kiel", places["days"][key("2024-05-01", "543", "101")], 1},
		{"Germany on that day", places["countryDays"][key("2024-05-01", "Deutschland")], 1},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, want %d", c.name, c.got, c.want)
		}
	}
}

// Each click shows the photos its chart counted.
func TestInsightClicksFindTheirPhotos(t *testing.T) {
	s := insightsFixture(t)
	five, ratioLo, ratioHi := 5, 1.45, 1.55
	for _, c := range []struct {
		name string
		opts ListPhotosOpts
		want []string
	}{
		{"phone", ListPhotosOpts{Device: "phone"}, []string{"a"}},
		{"camera", ListPhotosOpts{Device: "camera"}, []string{"b", "c"}},
		{"slow", ListPhotosOpts{Slow: true}, []string{"a"}},
		{"EV 5", ListPhotosOpts{EVMin: &five, EVMax: &five}, []string{"a"}},
		{"flash", ListPhotosOpts{Flash: true}, []string{"b"}},
		{"portrait", ListPhotosOpts{Orient: "portrait"}, []string{"a"}},
		{"ratio 1.5", ListPhotosOpts{RatioMin: &ratioLo, RatioMax: &ratioHi}, []string{"b", "c"}},
		{"cropped", ListPhotosOpts{Cropped: true}, []string{"c"}},
		{"no lens", ListPhotosOpts{NoLens: true}, []string{"c"}},
	} {
		ids, _ := listedIDs(t, s, c.opts)
		slices.Sort(ids)
		if !slices.Equal(ids, c.want) {
			t.Errorf("%s: %v, want %v", c.name, ids, c.want)
		}
	}
}

func TestInsightClicksAreNotCorrelated(t *testing.T) {
	s := insightsFixture(t)
	five, lo, hi := 5, 1.0, 2.0
	opts := ListPhotosOpts{Device: "phone", Slow: true, EVMin: &five, EVMax: &five, Flash: true, Orient: "portrait",
		RatioMin: &lo, RatioMax: &hi, Cropped: true, NoLens: true}
	fromSQL, whereSQL, args := newPhotoFilter(opts).sql()
	rows, err := s.db.QueryContext(context.Background(), `EXPLAIN QUERY PLAN SELECT COUNT(*) `+fromSQL+` WHERE `+whereSQL, args...)
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
	if joined := strings.Join(plan, "; "); strings.Contains(joined, "CORRELATED") {
		t.Errorf("a click's condition runs once per photo: %s", joined)
	}
}

// A tally that reads photos p finds the scope's photos by their status in an
// index, never one by one by id: "p.id IN (ok photos)" read each photo's row
// with its EXIF JSON and took 3 s where the index takes 0.08 s (33,000
// photos). With a folder too; with the shared filter, whose set is small and
// may lead to the photos by id, only never correlated.
func TestInsightTalliesFindPhotosByStatus(t *testing.T) {
	type heard struct {
		chart, query string
		args         []any
		filtered     bool
	}
	var all []heard
	s := insightsFixture(t)
	var mu sync.Mutex // a group's tallies run side by side
	tallyHeard = func(chart, query string, args []any, _ time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		all = append(all, heard{chart, query, args, !s.filter.Empty()})
	}
	defer func() { tallyHeard = nil }()
	for _, prefix := range []string{"", "/lib"} {
		for _, f := range []Filter{{}, {Models: []string{`"X-T50"`}}} {
			s.filter = f
			for name, group := range insightGroups {
				if _, err := group(s, s.statsScope(prefix)); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
			}
		}
	}
	s.filter = Filter{}
	for _, h := range all {
		plan := queryPlan(t, s, h.query, h.args)
		for _, step := range plan {
			byID := (strings.HasPrefix(step, "SEARCH p") || strings.HasPrefix(step, "SCAN p")) && strings.Contains(step, "sqlite_autoindex_photos")
			if strings.Contains(step, "CORRELATED") || byID && !h.filtered {
				t.Errorf("%s reads photos one by one: %s", h.chart, strings.Join(plan, "; "))
				break
			}
		}
	}
}

func queryPlan(t *testing.T, s *Store, query string, args []any) []string {
	t.Helper()
	rows, err := s.db.Query("EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	return plan
}

func TestColourPeopleAndThingsInsights(t *testing.T) {
	s := insightsFixture(t)
	for _, q := range []string{
		`INSERT INTO photo_appearance (photo_id, version, analysed_at, mono_class, tone_key, colourfulness) VALUES ('b', 1, '2024-01-01', 'colour', 'mid', 42)`,
		`INSERT INTO photo_palette (photo_id, rank, l, c, h, hue_bin, share) VALUES ('a', 0, 0.6, 0.1, 30, 1, 0.5), ('a', 1, 0.3, 0.05, 200, 7, 0.3)`,
		`INSERT INTO people (id, name) VALUES (1, 'Ann')`,
		`INSERT INTO faces (photo_id, x, y, w, h, score, embedding, person_id, confirmed) VALUES ('a', 0, 0, 1, 1, 1, x'', 1, 1)`,
		`INSERT INTO photo_things (photo_id, thing, score, source) VALUES ('a', 'dog', 0.9, 'clip')`,
		`INSERT INTO exif_index (photo_id, field, value) VALUES ('b', 'FilmSimulation', '"Velvia"')`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	sc := s.statsScope("")
	colour, _ := s.colourInsights(sc)
	people, _ := s.peopleInsights(sc)
	things, _ := s.thingsInsights(sc)
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"Velvia at 40", colour["colourfulFilm"][key("Velvia", "40")], 1},
		{"X-T50 at 40", colour["colourfulCamera"][key("X-T50", "40")], 1},
		{"hue 1 at ten", colour["hueHour"][key("10", "1")], 1},
		{"Ann in 2024-05", people["personMonth"][key("Ann", "2024-05")], 1},
		{"Ann in Germany", people["personCountry"][key("Ann", "Deutschland")], 1},
		{"Ann with the dog", people["personThing"][key("Ann", "dog")], 1},
		{"dog in May", things["thingMonth"][key("dog", "5")], 1},
		{"dog in Germany", things["thingCountry"][key("dog", "Deutschland")], 1},
	} {
		if c.got != c.want {
			t.Errorf("%s: %d, want %d", c.name, c.got, c.want)
		}
	}
	places, _ := s.placesInsights(sc)
	if places["density"][key("109", "20", "1")] != 1 {
		t.Errorf("density %v", places["density"])
	}
}

// The palette space: three colours per photo, the last repeated.
func TestPalettesTakeThreeColoursAPhoto(t *testing.T) {
	s := insightsFixture(t)
	s.db.Exec(`INSERT INTO photo_palette (photo_id, rank, l, c, h, hue_bin, share) VALUES ('a', 0, 0.6, 0.1, 30, 1, 0.5), ('a', 1, 0.3, 0.05, 200, 7, 0.3)`) //nolint:errcheck
	pc := &PointCloud{}
	if err := s.addPalettes(pc, s.statsScope(""), 0, 10); err != nil {
		t.Fatal(err)
	}
	if len(pc.ID) != 1 || len(pc.Colours) != 12 || pc.Total != 1 || pc.Colours[8] != pc.Colours[4] {
		t.Errorf("palette cloud %+v", pc)
	}
}

func TestSessionsSplitAtTwoHours(t *testing.T) {
	ss := sessionsOf([]string{"2024-05-01T10:00:00", "2024-05-01T11:30:00", "2024-05-01T14:00:00", "bad"})
	if len(ss) != 2 || ss[0].n != 2 || ss[0].last != "2024-05-01T11:30:00" || ss[1].first != "2024-05-01T14:00:00" {
		t.Errorf("sessions %+v", ss)
	}
	s := insightsFixture(t)
	in, err := s.timeInsights(s.statsScope(""))
	if err != nil || in["sessionSizes"]["1"] != 3 || len(in["sessions"]) != 3 {
		t.Errorf("time insights %v, %v", in, err)
	}
	from, to := "2024-05-01T09:00:00", "2024-05-01T10:30:00"
	if ids, _ := listedIDs(t, s, ListPhotosOpts{TakenFrom: from, TakenTo: to}); !slices.Equal(ids, []string{"a"}) {
		t.Errorf("a session's photos: %v", ids)
	}
}

func TestAreaFindsTheColumnsPhotos(t *testing.T) {
	s := insightsFixture(t)
	if ids, _ := listedIDs(t, s, ListPhotosOpts{Area: &[4]float64{54.25, 54.75, 10, 10.25}}); !slices.Equal(ids, []string{"a"}) {
		t.Errorf("Kiel's column: %v", ids)
	}
}
