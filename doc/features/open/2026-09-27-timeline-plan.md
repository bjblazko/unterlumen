# Timeline Implementation Plan

*Last modified: 2026-09-27*

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A new place "Timeline" (Explore, below Map). It shows every dated library photo on one time axis: on a desk as a horizontal band over a time bar, on a phone as a list with a scrubber.

**Architecture:** The Go package `internal/timeline` merges all libraries into one stream sorted by date taken, deduplicated by photo ID, and cached per version. `internal/api/timeline` serves the stream in two forms: a skeleton (day and aspect ratio of every photo) and detail pages by index range. The browser lays out, virtualises and draws the graph from the skeleton, and fetches details and thumbnails only for what is near the screen.

**Tech Stack:** Go 1.27 (stdlib, SQLite through the existing `library.Store`), vanilla JS (global classes, no build step), Canvas 2D, Playwright e2e.

**Spec:** `doc/features/open/2026-09-27-timeline.md` and `doc/architecture/adr/0040-timeline-place.md`. Clickdummy: https://claude.ai/artifact/N2RjBab31exPDAy4A2Xxh5 (tabs B and Mobile 1). Its source is the model for the layout and drawing code below.

## Global Constraints

- The UI follows `huepattl-rams-design` and the project CLAUDE.md:
  - IBM Plex Sans for UI text; IBM Plex Mono only for data (dates, counts, file names).
  - Sentence case, no uppercase labels.
  - Orange (`--accent`) only on the frame or handle that is being dragged right now.
  - The graph uses `--chart-seq-*`, `--chart-grid` and `--chart-axis`.
  - Flat: the time bar is a surface step (`--bg-2`) without a hairline.
  - Spacing on the 4 px scale; one control height per row (`--control-h-sm` 30 px).
- Coding standards (ADR-0015, `huepattl-code-quality`):
  - Functions of about 30–40 lines at most.
  - One responsibility per file.
  - No new dependency.
  - Domain packages with imports in one direction: `api/timeline → timeline → library`.
- The CSS goes into the new `src/web/css/timeline.css`, not into `style.css`.
- Navigation: `<a class="nav-item" id="mode-timeline" href="#timeline" data-mode="timeline">`, number key `8`.
- Copy: "Timeline", "Rows", "Show all", "Shown", "212 without a date are not shown", "Reading the timeline…".
- Waits go through `Activity` (nothing for 400 ms, no spinner). Errors are inline sentences, never `alert()`.
- Details come in pages of `500`, at most `40` pages kept. The viewer gets the `1000` photos around the one opened.
- Desktop and phone are split at `(max-width: 700px)`, as in `style.css`.
- Every changed documentation file carries `*Last modified: YYYY-MM-DD*`. CHANGELOG entries go under `## [Unreleased]`, merged into the existing subsection.
- Stage exact files only. The user keeps uncommitted edits in the tree.

## Review Focus

1. **A malformed date taken** (`"0000:00:00"`, `"2019"`, an empty string after the index migration): the photo counts as undated. It must not end up at day 0 or crash the day calculation. Test: Task 2, `TestMergeTreatsUnreadableDatesAsUndated`.
2. **No libraries, or no dated photos:** the place explains in one sentence why it is empty. No division by zero in the calendar, axis or layout. Tests: Task 2 `TestSkeletonOfEmptyStreamHasEmptyArrays`, Task 8 e2e `explains an empty timeline`.
3. **A library changes while the place is open** (scan, delete): the detail request answers 409. The place reloads the skeleton and stays at the date it showed. Tests: Task 3 `TestPhotosWithStaleVersionIs409`, Task 8 e2e `reloads when the timeline changed`.
4. **Portrait photos with EXIF orientation 6/8:** the tile is upright (ratio < 1), not lying on its side. Test: Task 2 `TestDisplayRatioSwapsForQuarterTurns`.
5. **All photos on a single day** (a library of one event): axis, range strip and scrubber get a domain of one day. They must draw without `NaN` and the frame must stay usable. Test: Task 4 e2e unit `a one-day stream has a usable calendar`, and the fixture library in Task 8 is small.

---

## File Structure

| File | Responsibility |
|---|---|
| `src/internal/library/store_dated.go` (new) | Store queries: dated photos with their sizes, undated IDs, and a content stamp |
| `src/internal/library/store_dated_test.go` (new) | Tests for those queries |
| `src/internal/timeline/ratio.go` | Display aspect ratio from width, height and EXIF orientation |
| `src/internal/timeline/merge.go` | Merge the libraries, dedupe by ID, sort, count days |
| `src/internal/timeline/stream.go` | `Stream`, `Skeleton()`, `Page()` |
| `src/internal/timeline/builder.go` | Read the libraries in sidebar order, compute the version, cache |
| `src/internal/timeline/*_test.go` | Tests |
| `src/internal/api/timeline/handler.go` (+ `_test.go`) | `GET /api/timeline`, `GET /api/timeline/photos` |
| `src/internal/api/routes.go` | Wire the handlers |
| `src/web/js/timeline-calendar.js` | Day and month arithmetic in UTC, date labels |
| `src/web/js/timeline-stream.js` | Skeleton in typed arrays, detail pages, version handling |
| `src/web/js/timeline-chart.js` | Canvas helpers: step graph, year and month labels, theme redraw |
| `src/web/js/timeline-tiles.js` | Keyed tile layer: create and release tiles near the screen |
| `src/web/js/timeline-axis.js` | Detail axis with the frame (desktop) |
| `src/web/js/timeline-range.js` | Overview with bracket handles to limit the range (desktop) |
| `src/web/js/timeline-band.js` | Horizontal band in 2–4 rows (desktop) |
| `src/web/js/timeline-list.js` | Vertical list, newest first, month headings (phone) |
| `src/web/js/timeline-scrubber.js` | Vertical scrubber with a month bubble (phone) |
| `src/web/js/timeline-place.js` | The place: head, menu, desktop or phone, info panel, viewer, reload |
| `src/web/css/timeline.css` | All styles of the place |
| `src/web/index.html`, `js/app.js`, `js/app-keyboard.js` | Nav entry, tab bar item, scripts, place wiring, keys `8` and `i` |
| `e2e/specs/timeline-units.spec.js`, `e2e/specs/timeline.spec.js` | Browser-side unit checks, and place behaviour on desktop and phone |

---

### Task 1: Store queries for the timeline

**Files:**
- Create: `src/internal/library/store_dated.go`
- Test: `src/internal/library/store_dated_test.go`

**Interfaces:**
- Consumes: `Store.db`, the test helpers `newTestStore`, `seedStatsPhotos` and `statsPhoto` (in `store_test.go` and `store_stats_test.go`).
- Produces:
  - `type DatedPhoto struct { ID, Filename, Taken string; Width, Height int; Orientation string }`
  - `func (s *Store) DatedPhotos() ([]DatedPhoto, error)`: oldest first, then by ID; photos marked missing are left out.
  - `func (s *Store) UndatedPhotoIDs() ([]string, error)`
  - `func (s *Store) ContentStamp() (string, error)`: changes whenever a photo is added, re-indexed or marked missing.

- [ ] **Step 1: Write the failing tests**

```go
package library

import (
	"reflect"
	"testing"
)

func datedFixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	seedStatsPhotos(t, s, []statsPhoto{
		{id: "b", path: "/l/b.jpg", date: "2024-05-01T10:30:00", exifJSON: `{"width":6000,"height":4000,"tags":{"Orientation":"6"}}`},
		{id: "a", path: "/l/a.jpg", date: "2023-01-02T08:00:00"},
		{id: "c", path: "/l/c.jpg", date: "2024-05-01T10:30:00"},
		{id: "u", path: "/l/u.jpg"},
		{id: "m", path: "/l/m.jpg", date: "2022-01-01T00:00:00", missing: true},
	})
	return s
}

func TestDatedPhotosSortedWithoutMissingOrUndated(t *testing.T) {
	got, err := datedFixture(t).DatedPhotos()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	if !reflect.DeepEqual(ids, []string{"a", "b", "c"}) {
		t.Fatalf("ids = %v, want a b c", ids)
	}
	if b := got[1]; b.Width != 6000 || b.Height != 4000 || b.Orientation != "6" || b.Filename != "b.jpg" || b.Taken != "2024-05-01T10:30:00" {
		t.Errorf("b = %+v", b)
	}
	if a := got[0]; a.Width != 0 || a.Height != 0 || a.Orientation != "" {
		t.Errorf("a without size = %+v", a)
	}
}

func TestUndatedPhotoIDsLeaveOutMissing(t *testing.T) {
	ids, err := datedFixture(t).UndatedPhotoIDs()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []string{"u"}) {
		t.Fatalf("undated = %v, want [u]", ids)
	}
}

func TestContentStampChangesWithPhotos(t *testing.T) {
	s := newTestStore(t)
	empty, err := s.ContentStamp()
	if err != nil {
		t.Fatal(err)
	}
	seedStatsPhotos(t, s, []statsPhoto{{id: "p", path: "/l/p.jpg", date: "2024-01-01T00:00:00"}})
	one, _ := s.ContentStamp()
	if one == empty {
		t.Fatalf("stamp did not change after adding a photo: %q", one)
	}
	if err := s.MarkPhotoMissing("p"); err != nil {
		t.Fatal(err)
	}
	if gone, _ := s.ContentStamp(); gone == one {
		t.Fatalf("stamp did not change after marking missing: %q", gone)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd src && go test ./internal/library/ -run 'Dated|Undated|ContentStamp' -v`
Expected: FAIL, `s.DatedPhotos undefined` (and the same for the other two).

- [ ] **Step 3: Implement**

```go
package library

import "database/sql"

// DatedPhoto is an indexed photo with a date taken, with what the timeline
// needs to lay it out (ADR-0040).
type DatedPhoto struct {
	ID          string
	Filename    string
	Taken       string // ISO date taken, as indexed
	Width       int    // stored size, before orientation; 0 when unknown
	Height      int
	Orientation string // EXIF Orientation tag; "" when absent
}

// DatedPhotos returns the photos that have a date taken, oldest first.
// Photos marked missing are left out.
func (s *Store) DatedPhotos() ([]DatedPhoto, error) {
	rows, err := s.db.Query(`
		SELECT id, filename, date_taken,
		       json_extract(exif_json, '$.width'),
		       json_extract(exif_json, '$.height'),
		       json_extract(exif_json, '$.tags.Orientation')
		  FROM photos
		 WHERE status='ok' AND date_taken IS NOT NULL
		 ORDER BY date_taken, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	photos := []DatedPhoto{}
	for rows.Next() {
		var p DatedPhoto
		var w, h sql.NullInt64
		var o sql.NullString
		if err := rows.Scan(&p.ID, &p.Filename, &p.Taken, &w, &h, &o); err != nil {
			return nil, err
		}
		p.Width, p.Height, p.Orientation = int(w.Int64), int(h.Int64), o.String
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// UndatedPhotoIDs returns the IDs of the photos without a date taken.
func (s *Store) UndatedPhotoIDs() ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM photos WHERE status='ok' AND date_taken IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ContentStamp changes whenever the library's photos do: it is the number of
// photos that are not missing and the latest time one was indexed.
func (s *Store) ContentStamp() (string, error) {
	var n int
	var latest string
	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(CAST(MAX(indexed_at) AS TEXT), '')
		FROM photos WHERE status='ok'`).Scan(&n, &latest)
	return strconv.Itoa(n) + "|" + latest, err
}
```

Add `"strconv"` to the imports.

- [ ] **Step 4: Run the tests to see them pass**

Run: `cd src && go test ./internal/library/ -run 'Dated|Undated|ContentStamp' -v && go vet ./internal/library/`
Expected: PASS, and no vet output.

- [ ] **Step 5: Commit**

```bash
git add src/internal/library/store_dated.go src/internal/library/store_dated_test.go
git commit -m "feat(library): dated photos, undated IDs and a content stamp for the timeline"
```

---

### Task 2: The `timeline` package: ratio, merge, stream, builder

**Files:**
- Create: `src/internal/timeline/ratio.go`, `merge.go`, `stream.go`, `builder.go`
- Test: `src/internal/timeline/ratio_test.go`, `merge_test.go`, `stream_test.go`, `builder_test.go`

**Interfaces:**
- Consumes: `library.DatedPhoto`, `(*library.Store).DatedPhotos`, `UndatedPhotoIDs`, `ContentStamp`, `(*library.Manager).ListLibraries`, `OpenStore`, `CreateLibrary(name, description, sourcePath string)`, `SetLibrarySortOrder([]string)`, `library.NewManager(dir)`, `(*library.Store).UpsertPhoto(id, pathHint, filename string, fileSize int64, indexedAt time.Time, exifJSON, thumbPath, dateTaken, ext string)`.
- Produces:
  - `type Photo struct { LibraryID, ID, Filename, Taken string; Day int; Ratio float64 }`
  - `type Stream struct { Version, Start string; Photos []Photo; Undated int }`
  - `type Skeleton struct { Version string \`json:"version"\`; Start string \`json:"start"\`; Days []int \`json:"days"\`; Ratios []float64 \`json:"ratios"\`; Undated int \`json:"undated"\` }`
  - `func (s *Stream) Skeleton() Skeleton` and `func (s *Stream) Page(from, count int) []Photo`
  - `func NewBuilder(mgr *library.Manager) *Builder` and `func (b *Builder) Current() (*Stream, error)`

- [ ] **Step 1: Write the failing tests**

`ratio_test.go`:

```go
package timeline

import "testing"

func TestDisplayRatioSwapsForQuarterTurns(t *testing.T) {
	cases := []struct {
		w, h int
		o    string
		want float64
	}{
		{6000, 4000, "", 1.5},
		{6000, 4000, "1", 1.5},
		{6000, 4000, "6", 4000.0 / 6000},
		{6000, 4000, "8", 4000.0 / 6000},
		{6000, 4000, "3", 1.5},
		{0, 4000, "6", fallbackRatio},
		{6000, 4000, "rotate", 1.5},
	}
	for _, c := range cases {
		if got := displayRatio(c.w, c.h, c.o); got != c.want {
			t.Errorf("displayRatio(%d, %d, %q) = %v, want %v", c.w, c.h, c.o, got, c.want)
		}
	}
}
```

`merge_test.go`:

```go
package timeline

import (
	"reflect"
	"testing"

	"huepattl.de/unterlumen/internal/library"
)

func ids(photos []Photo) []string {
	out := []string{}
	for _, p := range photos {
		out = append(out, p.ID)
	}
	return out
}

func TestMergeSortsAndKeepsEachPhotoOnce(t *testing.T) {
	photos, undated := merge([]LibraryPhotos{
		{LibraryID: "L1", Dated: []library.DatedPhoto{
			{ID: "x", Taken: "2024-05-01T10:00:00"}, {ID: "shared", Taken: "2021-02-03T09:00:00"},
		}, Undated: []string{"u1", "u2"}},
		{LibraryID: "L2", Dated: []library.DatedPhoto{
			{ID: "shared", Taken: "2021-02-03T09:00:00"}, {ID: "a", Taken: "2020-01-01T00:00:00"},
		}, Undated: []string{"u2"}},
	})
	if got := ids(photos); !reflect.DeepEqual(got, []string{"a", "shared", "x"}) {
		t.Fatalf("order = %v", got)
	}
	if photos[1].LibraryID != "L1" {
		t.Errorf("shared photo came from %s, want the first library L1", photos[1].LibraryID)
	}
	if undated != 2 {
		t.Errorf("undated = %d, want 2 (u2 counted once)", undated)
	}
}

func TestMergeTreatsUnreadableDatesAsUndated(t *testing.T) {
	photos, undated := merge([]LibraryPhotos{{LibraryID: "L", Dated: []library.DatedPhoto{
		{ID: "ok", Taken: "2019-08-14T12:00:00"},
		{ID: "zero", Taken: "0000:00:00 00:00:00"},
		{ID: "short", Taken: "2019"},
		{ID: "ancient", Taken: "0001-01-01T00:00:00"},
	}}})
	if got := ids(photos); !reflect.DeepEqual(got, []string{"ok"}) {
		t.Fatalf("dated = %v, want only ok", got)
	}
	if undated != 3 {
		t.Errorf("undated = %d, want 3", undated)
	}
}

func TestAssignDaysCountsFromTheFirstPhoto(t *testing.T) {
	photos := []Photo{{Taken: "2020-02-28T23:00:00"}, {Taken: "2020-03-01T01:00:00"}, {Taken: "2021-02-28T00:00:00"}}
	start := assignDays(photos)
	if start != "2020-02-28" {
		t.Errorf("start = %q", start)
	}
	var days []int
	for _, p := range photos {
		days = append(days, p.Day)
	}
	if !reflect.DeepEqual(days, []int{0, 2, 366}) {
		t.Errorf("days = %v, want [0 2 366] (2020 is a leap year)", days)
	}
}
```

`stream_test.go`:

```go
package timeline

import (
	"encoding/json"
	"testing"
)

func TestSkeletonRoundsRatiosAndKeepsDays(t *testing.T) {
	s := &Stream{Version: "v", Start: "2020-01-01", Undated: 4, Photos: []Photo{{Day: 0, Ratio: 2.0 / 3}, {Day: 9, Ratio: 1.5}}}
	sk := s.Skeleton()
	if sk.Days[1] != 9 || sk.Ratios[0] != 0.667 || sk.Undated != 4 || sk.Start != "2020-01-01" || sk.Version != "v" {
		t.Fatalf("skeleton = %+v", sk)
	}
}

func TestSkeletonOfEmptyStreamHasEmptyArrays(t *testing.T) {
	b, _ := json.Marshal((&Stream{Version: "v"}).Skeleton())
	if string(b) != `{"version":"v","start":"","days":[],"ratios":[],"undated":0}` {
		t.Fatalf("json = %s", b)
	}
}

func TestPageClampsToTheStream(t *testing.T) {
	s := &Stream{Photos: make([]Photo, 7)}
	for _, c := range []struct{ from, count, want int }{{0, 5, 5}, {5, 5, 2}, {7, 5, 0}, {-1, 5, 0}, {0, 0, 0}} {
		if got := len(s.Page(c.from, c.count)); got != c.want {
			t.Errorf("Page(%d, %d) has %d photos, want %d", c.from, c.count, got, c.want)
		}
	}
}
```

`builder_test.go`:

```go
package timeline

import (
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/library"
)

func seedLibrary(t *testing.T, mgr *library.Manager, name string, photos map[string]string) string {
	t.Helper()
	l, err := mgr.CreateLibrary(name, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for id, taken := range photos {
		if err := s.UpsertPhoto(id, "/"+id, id+".jpg", 1, time.Now(), `{"width":3,"height":2}`, "", taken, "jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	return l.ID
}

func TestBuilderMergesInSidebarOrderAndCaches(t *testing.T) {
	mgr, err := library.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := seedLibrary(t, mgr, "Zeta", map[string]string{"shared": "2022-01-01T10:00:00", "z": "2023-01-01T10:00:00"})
	second := seedLibrary(t, mgr, "Alpha", map[string]string{"shared": "2022-01-01T10:00:00", "u": ""})
	if err := mgr.SetLibrarySortOrder([]string{first, second}); err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(mgr)
	s, err := b.Current()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Photos) != 2 || s.Photos[0].ID != "shared" || s.Photos[0].LibraryID != first || s.Undated != 1 {
		t.Fatalf("stream = %+v", s)
	}
	if s.Photos[1].Day != 365 || s.Start != "2022-01-01" || s.Photos[0].Ratio != 1.5 {
		t.Errorf("days/ratio = %+v, start %s", s.Photos, s.Start)
	}
	again, _ := b.Current()
	if again != s {
		t.Error("an unchanged library built the stream again")
	}
	seedLibrary(t, mgr, "Beta", map[string]string{"new": "2024-01-01T10:00:00"})
	changed, _ := b.Current()
	if changed.Version == s.Version || len(changed.Photos) != 3 {
		t.Errorf("a new library did not change the stream: %+v", changed)
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd src && go test ./internal/timeline/ -v`
Expected: FAIL at compile time, `undefined: displayRatio`, `merge`, `Stream`, `NewBuilder`.

- [ ] **Step 3: Implement `ratio.go`**

```go
// Package timeline is every dated photo of every library on one time axis,
// each photo once (ADR-0040).
package timeline

import "strconv"

// fallbackRatio stands in for a photo whose size is unknown: the most common
// sensor shape.
const fallbackRatio = 1.5

// displayRatio is width ÷ height as the photo is shown. EXIF orientations 5
// to 8 turn it by a quarter, so its sides swap.
func displayRatio(width, height int, orientation string) float64 {
	if width <= 0 || height <= 0 {
		return fallbackRatio
	}
	if o, _ := strconv.Atoi(orientation); o >= 5 && o <= 8 {
		width, height = height, width
	}
	return float64(width) / float64(height)
}
```

- [ ] **Step 4: Implement `merge.go`**

```go
package timeline

import (
	"sort"
	"time"

	"huepattl.de/unterlumen/internal/library"
)

// LibraryPhotos is what one library contributes: its dated photos and the IDs
// of its undated ones.
type LibraryPhotos struct {
	LibraryID string
	Dated     []library.DatedPhoto
	Undated   []string
}

// firstPhotographYear rejects dates no photo can have, such as the 0001-01-01
// of a camera without a clock.
const firstPhotographYear = 1826

// merge joins the libraries into one list, oldest first. A photo in several
// libraries (one ID) is kept from the first of them in libs. A date that is
// not a calendar date makes the photo undated; undated counts each ID once.
func merge(libs []LibraryPhotos) (photos []Photo, undated int) {
	seen := map[string]bool{}
	noDate := map[string]bool{}
	for _, l := range libs {
		for _, p := range l.Dated {
			if seen[p.ID] {
				continue
			}
			if _, ok := parseDay(p.Taken); !ok {
				noDate[p.ID] = true
				continue
			}
			seen[p.ID] = true
			photos = append(photos, Photo{LibraryID: l.LibraryID, ID: p.ID, Filename: p.Filename,
				Taken: p.Taken, Ratio: displayRatio(p.Width, p.Height, p.Orientation)})
		}
		for _, id := range l.Undated {
			noDate[id] = true
		}
	}
	sort.SliceStable(photos, func(i, j int) bool {
		if photos[i].Taken != photos[j].Taken {
			return photos[i].Taken < photos[j].Taken
		}
		return photos[i].ID < photos[j].ID
	})
	for id := range noDate {
		if !seen[id] {
			undated++
		}
	}
	return photos, undated
}

// parseDay reads the calendar day of an ISO date taken, in UTC.
func parseDay(taken string) (time.Time, bool) {
	if len(taken) < 10 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02", taken[:10])
	if err != nil || t.Year() < firstPhotographYear {
		return time.Time{}, false
	}
	return t, true
}

// assignDays numbers each photo's day from the first photo's and returns
// that first day as YYYY-MM-DD; "" when there are no photos. photos must be
// sorted and dated.
func assignDays(photos []Photo) string {
	if len(photos) == 0 {
		return ""
	}
	first, _ := parseDay(photos[0].Taken)
	for i := range photos {
		t, _ := parseDay(photos[i].Taken)
		photos[i].Day = int(t.Sub(first).Hours() / 24)
	}
	return first.Format("2006-01-02")
}
```

- [ ] **Step 5: Implement `stream.go`**

```go
package timeline

import "math"

// Photo is one photo on the timeline.
type Photo struct {
	LibraryID string
	ID        string
	Filename  string
	Taken     string  // ISO date taken
	Day       int     // days since the stream's first day
	Ratio     float64 // width ÷ height as shown
}

// Stream is every dated photo of every library, oldest first, each once.
type Stream struct {
	Version string
	Start   string // the first photo's day, YYYY-MM-DD; "" when empty
	Photos  []Photo
	Undated int
}

// Skeleton is what the browser lays the timeline out from: a day and an
// aspect ratio per photo, in stream order.
type Skeleton struct {
	Version string    `json:"version"`
	Start   string    `json:"start"`
	Days    []int     `json:"days"`
	Ratios  []float64 `json:"ratios"`
	Undated int       `json:"undated"`
}

// Skeleton returns the stream's skeleton; ratios are rounded to three
// decimals, which is finer than a pixel at any tile size.
func (s *Stream) Skeleton() Skeleton {
	sk := Skeleton{Version: s.Version, Start: s.Start, Undated: s.Undated,
		Days: make([]int, len(s.Photos)), Ratios: make([]float64, len(s.Photos))}
	for i, p := range s.Photos {
		sk.Days[i] = p.Day
		sk.Ratios[i] = math.Round(p.Ratio*1000) / 1000
	}
	return sk
}

// Page returns up to count photos from index from on; a range outside the
// stream is empty.
func (s *Stream) Page(from, count int) []Photo {
	if from < 0 || from >= len(s.Photos) || count <= 0 {
		return []Photo{}
	}
	return s.Photos[from:min(from+count, len(s.Photos))]
}
```

- [ ] **Step 6: Implement `builder.go`**

```go
package timeline

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"

	"huepattl.de/unterlumen/internal/library"
)

// Builder keeps the current stream and builds it again when a library has
// changed. A library whose database cannot be read is left out, as on the
// Map.
type Builder struct {
	mgr *library.Manager
	mu  sync.Mutex
	cur *Stream
}

func NewBuilder(mgr *library.Manager) *Builder {
	return &Builder{mgr: mgr}
}

// Current returns the stream for the libraries as they are now.
func (b *Builder) Current() (*Stream, error) {
	libs, err := b.orderedLibraries()
	if err != nil {
		return nil, err
	}
	version := b.version(libs)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cur != nil && b.cur.Version == version {
		return b.cur, nil
	}
	b.cur = b.build(libs, version)
	return b.cur, nil
}

// orderedLibraries lists the libraries in the sidebar's order: by the
// position the owner gave them, then by name.
func (b *Builder) orderedLibraries() ([]*library.Library, error) {
	libs, err := b.mgr.ListLibraries()
	if err != nil {
		return nil, err
	}
	sort.SliceStable(libs, func(i, j int) bool { return libraryBefore(libs[i], libs[j]) })
	return libs, nil
}

func libraryBefore(a, b *library.Library) bool {
	pa, pb := a.SortPosition, b.SortPosition
	if (pa == nil) != (pb == nil) {
		return pa != nil
	}
	if pa != nil && *pa != *pb {
		return *pa < *pb
	}
	return a.Name < b.Name
}

// version hashes each library's content stamp, so it changes with any scan,
// deletion or reorder.
func (b *Builder) version(libs []*library.Library) string {
	h := fnv.New64a()
	for _, l := range libs {
		store, err := b.mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		stamp, err := store.ContentStamp()
		store.Close()
		if err == nil {
			fmt.Fprintf(h, "%s=%s;", l.ID, stamp)
		}
	}
	return strconv.FormatUint(h.Sum64(), 36)
}

func (b *Builder) build(libs []*library.Library, version string) *Stream {
	var all []LibraryPhotos
	for _, l := range libs {
		if lp, err := b.read(l.ID); err == nil {
			all = append(all, lp)
		}
	}
	photos, undated := merge(all)
	start := assignDays(photos)
	return &Stream{Version: version, Start: start, Photos: photos, Undated: undated}
}

func (b *Builder) read(id string) (LibraryPhotos, error) {
	store, err := b.mgr.OpenStore(id)
	if err != nil {
		return LibraryPhotos{}, err
	}
	defer store.Close()
	dated, err := store.DatedPhotos()
	if err != nil {
		return LibraryPhotos{}, err
	}
	undated, err := store.UndatedPhotoIDs()
	if err != nil {
		return LibraryPhotos{}, err
	}
	return LibraryPhotos{LibraryID: id, Dated: dated, Undated: undated}, nil
}
```

- [ ] **Step 7: Run the tests to see them pass**

Run: `cd src && go test ./internal/timeline/ -v && go vet ./internal/timeline/`
Expected: PASS, and no vet output.

- [ ] **Step 8: Commit**

```bash
git add src/internal/timeline/
git commit -m "feat(timeline): one stream of every dated library photo, each once"
```

---

### Task 3: The HTTP API and its wiring

**Files:**
- Create: `src/internal/api/timeline/handler.go`
- Test: `src/internal/api/timeline/handler_test.go`
- Modify: `src/internal/api/routes.go` (imports, and the `if libMgr != nil` block around line 58)

**Interfaces:**
- Consumes: `timeline.NewBuilder`, `(*Builder).Current`, `Stream.Skeleton`, `Stream.Page`, `Stream.Version`.
- Produces:
  - `func Handle(mux *http.ServeMux, b *timeline.Builder)`.
  - `GET /api/timeline` returns a `Skeleton` as JSON.
  - `GET /api/timeline/photos?v=&from=&count=` returns `{"photos": [[libraryID, photoID, filename, taken], …]}`.
  - A stale `v` gives 409; bad numbers give 400; `count` is capped at 500.

- [ ] **Step 1: Write the failing tests**

```go
package apitimeline

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/timeline"
)

func testMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mgr, err := library.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l, err := mgr.CreateLibrary("L", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, _ := mgr.OpenStore(l.ID)
	defer s.Close()
	for i, taken := range []string{"2020-01-01T10:00:00", "2020-01-03T10:00:00", "2021-06-01T10:00:00"} {
		id := string(rune('a' + i))
		if err := s.UpsertPhoto(id, "/"+id, id+".jpg", 1, time.Now(), `{}`, "", taken, "jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	Handle(mux, timeline.NewBuilder(mgr))
	return mux
}

func get(t *testing.T, mux *http.ServeMux, url string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func skeleton(t *testing.T, mux *http.ServeMux) timeline.Skeleton {
	t.Helper()
	var sk timeline.Skeleton
	if err := json.NewDecoder(get(t, mux, "/api/timeline").Body).Decode(&sk); err != nil {
		t.Fatal(err)
	}
	return sk
}

func TestSkeletonListsDays(t *testing.T) {
	sk := skeleton(t, testMux(t))
	if len(sk.Days) != 3 || sk.Days[1] != 2 || sk.Start != "2020-01-01" || sk.Version == "" {
		t.Fatalf("skeleton = %+v", sk)
	}
}

func TestPhotosReturnsAPage(t *testing.T) {
	mux := testMux(t)
	v := skeleton(t, mux).Version
	rec := get(t, mux, "/api/timeline/photos?v="+v+"&from=1&count=5")
	var body struct{ Photos [][4]string }
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(body.Photos) != 2 || body.Photos[0][1] != "b" || body.Photos[0][2] != "b.jpg" || body.Photos[0][3] != "2020-01-03T10:00:00" {
		t.Fatalf("code %d, photos %v", rec.Code, body.Photos)
	}
}

func TestPhotosWithStaleVersionIs409(t *testing.T) {
	if rec := get(t, testMux(t), "/api/timeline/photos?v=old&from=0&count=5"); rec.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409", rec.Code)
	}
}

func TestPhotosRejectsBadRanges(t *testing.T) {
	mux := testMux(t)
	for _, q := range []string{"from=x&count=5", "from=0&count=0", "from=-1&count=5", "count=5"} {
		if rec := get(t, mux, "/api/timeline/photos?v=any&"+q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: code = %d, want 400", q, rec.Code)
		}
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `cd src && go test ./internal/api/timeline/ -v`
Expected: FAIL at compile time, `undefined: Handle`.

- [ ] **Step 3: Implement `handler.go`**

```go
// Package apitimeline serves the timeline (ADR-0040): the skeleton the
// browser lays it out from, and the details of the photos in view.
package apitimeline

import (
	"encoding/json"
	"net/http"
	"strconv"

	"huepattl.de/unterlumen/internal/timeline"
)

// maxPage caps one details request; the browser asks for 500 at a time.
const maxPage = 500

func Handle(mux *http.ServeMux, b *timeline.Builder) {
	mux.HandleFunc("GET /api/timeline", skeleton(b))
	mux.HandleFunc("GET /api/timeline/photos", photos(b))
}

func skeleton(b *timeline.Builder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := b.Current()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, s.Skeleton())
	}
}

// photos answers [libraryID, photoID, filename, taken] for an index range.
// Another version than the current one is a 409: the browser's indexes
// point at other photos now.
func photos(b *timeline.Builder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, count, ok := pageRange(r)
		if !ok {
			http.Error(w, "from and count must be whole numbers, from at least 0 and count at least 1", http.StatusBadRequest)
			return
		}
		s, err := b.Current()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if r.URL.Query().Get("v") != s.Version {
			http.Error(w, "the timeline changed; load it again", http.StatusConflict)
			return
		}
		page := s.Page(from, min(count, maxPage))
		rows := make([][4]string, len(page))
		for i, p := range page {
			rows[i] = [4]string{p.LibraryID, p.ID, p.Filename, p.Taken}
		}
		writeJSON(w, map[string]any{"photos": rows})
	}
}

func pageRange(r *http.Request) (from, count int, ok bool) {
	from, errFrom := strconv.Atoi(r.URL.Query().Get("from"))
	count, errCount := strconv.Atoi(r.URL.Query().Get("count"))
	return from, count, errFrom == nil && errCount == nil && from >= 0 && count >= 1
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Wire it up in `routes.go`**

Add these imports next to `apilibrary`:

```go
	apitimeline "huepattl.de/unterlumen/internal/api/timeline"
	"huepattl.de/unterlumen/internal/timeline"
```

Inside `if libMgr != nil { … }`, directly after `apilibrary.Handle(…)`:

```go
		apitimeline.Handle(mux, timeline.NewBuilder(libMgr))
```

- [ ] **Step 5: Run the tests, vet, build**

Run: `cd src && go test ./internal/api/timeline/ ./internal/timeline/ ./internal/library/ && go vet ./... && go build -o ../unterlumen .`
Expected: `ok` for all three packages, no vet output, and the binary is built.

- [ ] **Step 6: Commit**

```bash
git add src/internal/api/timeline/ src/internal/api/routes.go
git commit -m "feat(api): /api/timeline skeleton and detail pages"
```

---

### Task 4: Browser data layer: calendar and stream

**Files:**
- Create: `src/web/js/timeline-calendar.js`, `src/web/js/timeline-stream.js`
- Modify: `src/web/index.html` (script tags after `map-place.js`)
- Test: `e2e/specs/timeline-units.spec.js`

**Interfaces:**
- Consumes: `/api/timeline`, `/api/timeline/photos`.
- Produces (globals):
  - `TIMELINE_DAY_MS`, `TIMELINE_MONTHS`, `tlClamp(v, a, b)`.
  - `class TimelineCalendar(start: 'YYYY-MM-DD', span: number)`:
    - fields `epoch`, `span`, `year: Uint16Array`, `month: Uint32Array` (year·12+month), `date: Uint8Array`, `firstMonth`, `lastMonth`;
    - methods `monthStart(k)`, `monthEnd(k)` (exclusive), `yearStart(y)`, `dayAt(ms)`, `msOf(d)`, `formatMonth(d)`, `formatMonthLong(d)`, `formatDay(d)`.
  - `class TimelineStream`:
    - `load(): Promise`, `adopt(skeleton)`;
    - fields `version`, `undated`, `count`, `day: Uint32Array`, `ratio: Float32Array`, `span`, `calendar` (`null` when empty), `prefix: Uint32Array` (length `span + 1`, photos before day d);
    - `monthCount(k)`, `detail(i) → {lib, id, name, taken} | null`, `ensure(from, to): Promise`;
    - callbacks `onDetails()`, `onStale()`.

- [ ] **Step 1: Write the failing browser unit spec**

```js
import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';

// Browser-side checks of the timeline's arithmetic (ADR-0040). The classes are
// globals of the app page, so the page is loaded once and they are called in it.

test.describe('Timeline units', () => {
    test.beforeEach(async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
    });

    test('the calendar counts days in UTC and knows months', async ({ page }) => {
        const r = await page.evaluate(() => {
            const c = new TimelineCalendar('2020-02-28', 400);
            return {
                leap: c.formatDay(1), march: c.formatMonth(2), long: c.formatMonthLong(2),
                marchStart: c.monthStart(2020 * 12 + 2), marchEnd: c.monthEnd(2020 * 12 + 2),
                first: c.monthStart(c.firstMonth), y2021: c.yearStart(2021),
                back: c.dayAt(c.msOf(123)),
            };
        });
        expect(r).toEqual({ leap: 'Sat 29 Feb 2020', march: 'Mar 2020', long: 'March 2020',
            marchStart: 2, marchEnd: 33, first: 0, y2021: 308, back: 123 });
    });

    test('a one-day stream has a usable calendar', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-07-14', days: [0, 0, 0], ratios: [1.5, 0.667, 1], undated: 0 });
            return { span: s.span, prefix: [...s.prefix], month: s.monthCount(s.calendar.firstMonth), label: s.calendar.formatMonth(0) };
        });
        expect(r).toEqual({ span: 1, prefix: [0, 3], month: 3, label: 'Jul 2024' });
    });

    test('an empty stream has no calendar', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '', days: [], ratios: [], undated: 5 });
            return { count: s.count, span: s.span, calendar: s.calendar, undated: s.undated };
        });
        expect(r).toEqual({ count: 0, span: 0, calendar: null, undated: 5 });
    });

    test('details load by page and a stale version is reported', async ({ page }) => {
        await page.route('**/api/timeline/photos?*', (route) => {
            const u = new URL(route.request().url());
            if (u.searchParams.get('v') === 'old') return route.fulfill({ status: 409, body: 'changed' });
            const from = Number(u.searchParams.get('from'));
            route.fulfill({ contentType: 'application/json',
                body: JSON.stringify({ photos: [[`L`, `id${from}`, `f${from}.jpg`, '2024-01-01T00:00:00']] }) });
        });
        const r = await page.evaluate(async () => {
            const s = new TimelineStream();
            s.adopt({ version: 'v1', start: '2024-01-01', days: new Array(600).fill(0), ratios: new Array(600).fill(1.5), undated: 0 });
            let details = 0; s.onDetails = () => details++;
            await s.ensure(0, 1);
            const first = s.detail(0), missing = s.detail(501);
            s.version = 'old';
            let stale = 0; s.onStale = () => stale++;
            await s.ensure(500, 501);
            return { first, missing, details, stale };
        });
        expect(r).toEqual({ first: { lib: 'L', id: 'id0', name: 'f0.jpg', taken: '2024-01-01T00:00:00' }, missing: null, details: 1, stale: 1 });
    });
});
```

- [ ] **Step 2: Run it to see it fail**

Run: `cd src && go build -o ../unterlumen . && cd ../e2e && npx playwright test specs/timeline-units.spec.js`
Expected: FAIL, `TimelineCalendar is not defined`.

- [ ] **Step 3: Implement `timeline-calendar.js`**

```js
// TimelineCalendar — the days of a timeline, counted from its first photo's
// day (ADR-0040). Day d is start + d days in UTC, so a photo never moves to
// another day with the browser's time zone. Months are keyed year * 12 + month.

const TIMELINE_DAY_MS = 864e5;
const TIMELINE_MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const TIMELINE_MONTHS_LONG = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const TIMELINE_WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

function tlClamp(v, a, b) {
    return Math.max(a, Math.min(b, v));
}

class TimelineCalendar {
    // start: the first day, 'YYYY-MM-DD'; span: how many days, at least 1.
    constructor(start, span) {
        const [y, m, d] = start.split('-').map(Number);
        this.epoch = Date.UTC(y, m - 1, d);
        this.span = span;
        this.year = new Uint16Array(span);
        this.month = new Uint32Array(span);
        this.date = new Uint8Array(span);
        for (let i = 0; i < span; i++) {
            const t = new Date(this.msOf(i));
            this.year[i] = t.getUTCFullYear();
            this.month[i] = t.getUTCFullYear() * 12 + t.getUTCMonth();
            this.date[i] = t.getUTCDate();
        }
        this.firstMonth = this.month[0];
        this.lastMonth = this.month[span - 1];
        this._monthStarts = [];
        for (let k = this.firstMonth; k <= this.lastMonth + 1; k++) {
            this._monthStarts.push(tlClamp(this.dayAt(Date.UTC(Math.floor(k / 12), k % 12, 1)), 0, span));
        }
    }

    msOf(d) { return this.epoch + d * TIMELINE_DAY_MS; }
    dayAt(ms) { return Math.round((ms - this.epoch) / TIMELINE_DAY_MS); }
    monthStart(k) { return this._monthStarts[k - this.firstMonth]; }
    monthEnd(k) { return this._monthStarts[k - this.firstMonth + 1]; }
    yearStart(y) { return tlClamp(this.dayAt(Date.UTC(y, 0, 1)), 0, this.span); }

    formatMonth(d) { return `${TIMELINE_MONTHS[this.month[d] % 12]} ${this.year[d]}`; }
    formatMonthLong(d) { return `${TIMELINE_MONTHS_LONG[this.month[d] % 12]} ${this.year[d]}`; }
    formatDay(d) {
        const weekday = TIMELINE_WEEKDAYS[new Date(this.msOf(d)).getUTCDay()];
        return `${weekday} ${this.date[d]} ${TIMELINE_MONTHS[this.month[d] % 12]} ${this.year[d]}`;
    }
}
```

- [ ] **Step 4: Implement `timeline-stream.js`**

```js
// TimelineStream — the timeline's photos (ADR-0040). The skeleton (a day and
// an aspect ratio per photo) is loaded once and kept in typed arrays; the
// details (library, ID, file name, date) come by index range, in pages, only
// for what is near the screen. A page used longest ago is dropped first.

const TIMELINE_PAGE = 500;
const TIMELINE_PAGES_KEPT = 40;

class TimelineStream {
    constructor() {
        this.onDetails = null; // a page of details arrived
        this.onStale = null;   // the server's stream changed; load() again
        this.adopt({ version: '', start: '', days: [], ratios: [], undated: 0 });
    }

    async load() {
        const r = await fetch('/api/timeline');
        if (!r.ok) throw new Error(await r.text());
        this.adopt(await r.json());
    }

    adopt(sk) {
        this.version = sk.version;
        this.undated = sk.undated;
        this.count = sk.days.length;
        this.day = Uint32Array.from(sk.days);
        this.ratio = Float32Array.from(sk.ratios);
        this.span = this.count ? this.day[this.count - 1] + 1 : 0;
        this.calendar = this.count ? new TimelineCalendar(sk.start, this.span) : null;
        this.prefix = new Uint32Array(this.span + 1);
        for (let i = 0; i < this.count; i++) this.prefix[this.day[i] + 1]++;
        for (let d = 0; d < this.span; d++) this.prefix[d + 1] += this.prefix[d];
        this._pages = new Map();
        this._pending = new Map();
    }

    monthCount(k) {
        const c = this.calendar;
        return this.prefix[c.monthEnd(k)] - this.prefix[c.monthStart(k)];
    }

    detail(i) {
        const rows = this._pages.get(Math.floor(i / TIMELINE_PAGE));
        const row = rows && rows[i % TIMELINE_PAGE];
        return row ? { lib: row[0], id: row[1], name: row[2], taken: row[3] } : null;
    }

    // ensure loads the pages that hold the indexes from ≤ i < to.
    ensure(from, to) {
        const first = Math.floor(Math.max(0, from) / TIMELINE_PAGE);
        const last = Math.floor((Math.min(this.count, to) - 1) / TIMELINE_PAGE);
        const loads = [];
        for (let p = first; p <= last; p++) loads.push(this._page(p));
        return Promise.all(loads);
    }

    _page(p) {
        if (this._pages.has(p)) {
            const rows = this._pages.get(p);
            this._pages.delete(p);
            this._pages.set(p, rows); // most recently used last
            return Promise.resolve();
        }
        if (this._pending.has(p)) return this._pending.get(p);
        const load = this._fetchPage(p, this.version).finally(() => {
            if (this._pending.get(p) === load) this._pending.delete(p);
        });
        this._pending.set(p, load);
        return load;
    }

    async _fetchPage(p, version) {
        const url = `/api/timeline/photos?v=${encodeURIComponent(version)}&from=${p * TIMELINE_PAGE}&count=${TIMELINE_PAGE}`;
        const r = await fetch(url);
        if (r.status === 409) {
            if (version === this.version) this.onStale?.();
            return;
        }
        if (!r.ok) throw new Error(await r.text());
        const { photos } = await r.json();
        if (version !== this.version) return;
        this._pages.set(p, photos);
        while (this._pages.size > TIMELINE_PAGES_KEPT) this._pages.delete(this._pages.keys().next().value);
        this.onDetails?.();
    }
}
```

- [ ] **Step 5: Add the scripts to `index.html`** (after `map-place.js`)

```html
    <script src="/js/timeline-calendar.js?v=1"></script>
    <script src="/js/timeline-stream.js?v=1"></script>
```

- [ ] **Step 6: Run the spec to see it pass**

Run: `cd src && go build -o ../unterlumen . && cd ../e2e && npx playwright test specs/timeline-units.spec.js`
Expected: 4 passed.

- [ ] **Step 7: Commit**

```bash
git add src/web/js/timeline-calendar.js src/web/js/timeline-stream.js src/web/index.html e2e/specs/timeline-units.spec.js
git commit -m "feat(web): timeline calendar and stream with paged details"
```

---

### Task 5: Charts, the axis and the range (desktop time bar)

**Files:**
- Create: `src/web/js/timeline-chart.js`, `src/web/js/timeline-axis.js`, `src/web/js/timeline-range.js`
- Modify: `src/web/index.html` (scripts)
- Test: `e2e/specs/timeline-units.spec.js` (add tests)

**Interfaces:**
- Consumes: `TimelineStream` (`prefix`, `span`, `calendar`, `monthCount`), `tlClamp`, `TIMELINE_MONTHS`.
- Produces:
  - `TimelineChart.prepare(canvas, box) → {ctx, w, h}`, `TimelineChart.colours()`, `TimelineChart.area(ctx, stream, xOf, a, b, top, bottom, colours, pxPerDay)`, `TimelineChart.calendar(ctx, stream, xOf, a, b, top, bottom, w, colours, withMonths)`, `TimelineChart.onRedraw(fn)`. Here `b` is the exclusive end day.
  - `class TimelineAxis(stream, { height, onSeek(day), onRelease() })`: `el`, `setDomain(a, b)` (inclusive), `setView(a, b)`, `x(day)`, `dayAt(x)`, `dragging`.
  - `class TimelineRange(stream, { height, onLimit(a, b), onText(a, b) })`: `el`, `set(a, b)`, `a`, `b`.

- [ ] **Step 1: Add the failing tests to `timeline-units.spec.js`**

```js
    test('the axis maps days to pixels and back, also for a one-day domain', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-01-01', days: [0, 10, 99], ratios: [1.5, 1.5, 1.5], undated: 0 });
            const axis = new TimelineAxis(s, { height: 92, onSeek() {} });
            axis.el.style.width = '1000px';
            document.body.appendChild(axis.el);
            axis.redraw();
            const wide = { x10: axis.x(10), back: axis.dayAt(axis.x(10) + 1) };
            axis.setDomain(10, 10);
            const one = { x: axis.x(10), end: axis.x(11), back: axis.dayAt(500) };
            axis.el.remove();
            return { wide, one };
        });
        expect(r.wide).toEqual({ x10: 100, back: 10 });
        expect(r.one).toEqual({ x: 0, end: 1000, back: 10 });
    });

    test('the range snaps to whole months', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-01-01', days: [0, 400], ratios: [1.5, 1.5], undated: 0 });
            let limit = null;
            const range = new TimelineRange(s, { height: 40, onLimit: (a, b) => { limit = [a, b]; } });
            range.el.style.width = '800px';
            document.body.appendChild(range.el);
            range.redraw();
            range.set(40, 70);
            range.el.remove();
            return { limit, from: s.calendar.formatDay(limit[0]), until: s.calendar.formatDay(limit[1]) };
        });
        expect(r.from).toBe('Thu 1 Feb 2024');
        expect(r.until).toBe('Sun 31 Mar 2024');
    });
```

- [ ] **Step 2: Run them to see them fail**

Run: `cd e2e && npx playwright test specs/timeline-units.spec.js`
Expected: the two new tests FAIL with `TimelineAxis is not defined`.

- [ ] **Step 3: Implement `timeline-chart.js`**

```js
// TimelineChart — drawing the timeline's graph and calendar on a canvas
// (ADR-0040). The graph is a step shape of photos per day within each bin,
// so a short bin at the edge is not dwarfed; it has no numbers, only a shape
// (ADR-0034: one series, slot of the sequential ramp, no legend).

const TIMELINE_BINS = [1, 2, 4, 7, 14, 30, 61, 91, 182, 365];
const TIMELINE_MONO = '"IBM Plex Mono", ui-monospace, monospace';

const TimelineChart = {
    _redraws: new Set(),

    prepare(canvas, box) {
        const dpr = window.devicePixelRatio || 1, w = box.clientWidth, h = box.clientHeight;
        canvas.width = Math.max(1, Math.round(w * dpr));
        canvas.height = Math.max(1, Math.round(h * dpr));
        const ctx = canvas.getContext('2d');
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        ctx.clearRect(0, 0, w, h);
        return { ctx, w, h };
    },

    colours() {
        const s = getComputedStyle(document.documentElement), v = (n) => s.getPropertyValue(n).trim();
        return { area: v('--chart-seq-2'), bar: v('--chart-seq-3'), line: v('--chart-seq-4'),
            grid: v('--chart-grid'), fg2: v('--fg-2'), fg3: v('--fg-3') };
    },

    area(ctx, stream, xOf, a, b, top, bottom, c, pxPerDay) {
        const bin = TIMELINE_BINS.find(n => n * pxPerDay >= 3) || 365;
        const bins = [];
        let max = 0;
        for (let s = a; s < b; s += bin) {
            const e = Math.min(s + bin, b), v = (stream.prefix[e] - stream.prefix[s]) / (e - s);
            bins.push([s, e, v]);
            max = Math.max(max, v);
        }
        if (!max) return;
        ctx.beginPath();
        ctx.moveTo(xOf(a), bottom);
        for (const [s, e, v] of bins) {
            const y = bottom - v / max * (bottom - top);
            ctx.lineTo(xOf(s), y);
            ctx.lineTo(xOf(e), y);
        }
        ctx.lineTo(xOf(b), bottom);
        ctx.closePath();
        ctx.fillStyle = c.area;
        ctx.fill();
        ctx.lineWidth = 1;
        ctx.strokeStyle = c.line;
        ctx.stroke();
    },

    // Year lines and labels, and month names where a month is 26 px or wider.
    calendar(ctx, stream, xOf, a, b, top, bottom, w, c, withMonths) {
        const cal = stream.calendar, monthY = bottom + 12, yearY = withMonths ? bottom + 27 : bottom + 12;
        let lastYearX = -Infinity;
        for (let k = cal.month[a]; k <= cal.month[b - 1]; k++) {
            const s = Math.max(cal.monthStart(k), a), x0 = xOf(s), mw = xOf(Math.min(cal.monthEnd(k), b)) - x0;
            if (mw < 0.5) continue;
            const isYear = k % 12 === 0 || s === a;
            if (isYear) lastYearX = this._year(ctx, k, x0, lastYearX, top, yearY, w, c);
            if (withMonths) this._month(ctx, k, x0, mw, isYear, bottom, monthY, c);
        }
        ctx.fillStyle = c.grid;
        ctx.fillRect(0, bottom, w, 1);
    },

    _year(ctx, k, x0, lastX, top, yearY, w, c) {
        ctx.fillStyle = c.grid;
        ctx.fillRect(Math.round(x0), top, 1, yearY - top - 9);
        if (x0 - lastX < 40 || x0 + 30 > w + 2) return lastX;
        ctx.fillStyle = c.fg2;
        ctx.font = `500 11px ${TIMELINE_MONO}`;
        ctx.fillText(String(Math.floor(k / 12)), x0 + 4, yearY);
        return x0;
    },

    _month(ctx, k, x0, mw, isYear, bottom, monthY, c) {
        if (mw >= 26) {
            ctx.fillStyle = c.fg3;
            ctx.font = `400 10px ${TIMELINE_MONO}`;
            ctx.fillText(TIMELINE_MONTHS[k % 12], x0 + 4, monthY);
        } else if (mw >= 5 && !isYear) {
            ctx.fillStyle = c.grid;
            ctx.fillRect(Math.round(x0), bottom, 1, 4);
        }
    },

    // Canvases do not follow CSS: they draw again when the theme or the fonts change.
    onRedraw(fn) {
        this._redraws.add(fn);
    },
};

(() => {
    const redraw = () => TimelineChart._redraws.forEach(fn => fn());
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', redraw);
    new MutationObserver(redraw).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
    document.fonts?.ready.then(redraw);
})();
```

- [ ] **Step 4: Implement `timeline-axis.js`**

```js
// TimelineAxis — the desktop's detail axis (ADR-0040): the chosen range
// stretched across the width, its graph and calendar, and the frame marking
// the days the band shows. Dragging the frame or clicking the axis asks the
// band to go there (onSeek); while dragging, the band's reports do not move
// the frame. The frame being dragged is the one orange thing on screen.

const AXIS_LABELS_H = 32;

class TimelineAxis {
    constructor(stream, { height, onSeek, onRelease }) {
        this.stream = stream;
        this.onSeek = onSeek;
        this.onRelease = onRelease;
        this.el = document.createElement('div');
        this.el.className = 'timeline-axis';
        this.el.style.height = height + 'px';
        this.canvas = document.createElement('canvas');
        this.frame = document.createElement('div');
        this.frame.className = 'timeline-frame';
        this.frame.tabIndex = 0;
        this.frame.setAttribute('role', 'slider');
        this.frame.setAttribute('aria-label', 'Time shown');
        this.frame.style.height = (height - AXIS_LABELS_H) + 'px';
        this.tip = document.createElement('div');
        this.tip.className = 'timeline-tip';
        this.tip.hidden = true;
        this.el.append(this.canvas, this.frame, this.tip);
        this.a = 0;
        this.b = Math.max(0, stream.span - 1);
        this.view = [this.b, this.b];
        this.dragging = false;
        this.el.addEventListener('pointerdown', e => this._down(e));
        this.frame.addEventListener('keydown', e => this._key(e));
        new ResizeObserver(() => this.redraw()).observe(this.el);
        TimelineChart.onRedraw(() => this.redraw());
    }

    setDomain(a, b) { this.a = a; this.b = b; this.redraw(); }
    setView(a, b) { if (this.dragging) return; this.view = [a, b]; this._place(); }

    x(d) { return (tlClamp(d, this.a, this.b + 1) - this.a) / (this.b + 1 - this.a) * this.w; }
    dayAt(x) { return tlClamp(Math.floor(this.a + x / this.w * (this.b + 1 - this.a)), this.a, this.b); }

    redraw() {
        this.w = this.el.clientWidth;
        if (!this.w || !this.stream.count) return;
        const { ctx, w, h } = TimelineChart.prepare(this.canvas, this.el), c = TimelineChart.colours();
        const bottom = h - AXIS_LABELS_H, xOf = d => this.x(d), end = this.b + 1;
        TimelineChart.area(ctx, this.stream, xOf, this.a, end, 6, bottom, c, w / (end - this.a));
        TimelineChart.calendar(ctx, this.stream, xOf, this.a, end, 6, bottom, w, c, true);
        this._place();
    }

    _place() {
        if (!this.w) return;
        const [a, b] = this.view, cal = this.stream.calendar;
        const x0 = this.x(a), width = Math.max(8, this.x(b + 1) - x0);
        this.frame.style.left = Math.min(x0, this.w - width) + 'px';
        this.frame.style.width = width + 'px';
        this.frame.setAttribute('aria-valuetext', a === b ? cal.formatMonth(a) : `${cal.formatMonth(a)} to ${cal.formatMonth(b)}`);
    }

    _down(e) {
        if (e.button !== 0) return;
        const r = this.el.getBoundingClientRect(), onFrame = this.frame.contains(e.target);
        const span = this.view[1] - this.view[0], x0 = this.x(this.view[0]);
        const grab = onFrame ? e.clientX - r.left - x0 : (this.x(this.view[1] + 1) - x0) / 2;
        this.dragging = true;
        this.frame.classList.add('is-adjusting');
        this.el.setPointerCapture(e.pointerId);
        const move = ev => this._drag(ev.clientX - r.left - grab, span);
        const up = () => {
            this.dragging = false;
            this.frame.classList.remove('is-adjusting');
            this.tip.hidden = true;
            this.el.removeEventListener('pointermove', move);
            this.el.removeEventListener('pointerup', up);
            this.el.removeEventListener('pointercancel', up);
            this.onRelease?.();
        };
        this.el.addEventListener('pointermove', move);
        this.el.addEventListener('pointerup', up);
        this.el.addEventListener('pointercancel', up);
        if (!onFrame) move(e);
        e.preventDefault();
        this.frame.focus({ preventScroll: true });
    }

    _drag(x, span) {
        const d = tlClamp(this.dayAt(x), this.a, Math.max(this.a, this.b - span));
        this.view = [d, d + span];
        this._place();
        this.tip.hidden = false;
        this.tip.textContent = this.stream.calendar.formatMonth(d);
        this.tip.style.left = tlClamp(this.x(d), 30, this.w - 30) + 'px';
        this.onSeek(d);
    }

    _key(e) {
        const steps = { ArrowLeft: -30, ArrowRight: 30 };
        let d;
        if (e.key in steps) d = this.view[0] + steps[e.key] * (e.shiftKey ? 12 : 1);
        else if (e.key === 'Home') d = this.a;
        else if (e.key === 'End') d = this.b;
        else return;
        e.preventDefault();
        e.stopPropagation();
        this.onSeek(tlClamp(d, this.a, this.b));
    }
}
```

- [ ] **Step 5: Implement `timeline-range.js`**

```js
// TimelineRange — the desktop's overview (ADR-0040): the whole span as a
// small graph with year labels, and two bracket handles that limit the axis
// and the band to a range of whole months. Dragging the space between the
// handles moves the range. The handle being moved is orange.

const RANGE_LABELS_H = 14;
const RANGE_SETTLE_MS = 140;

class TimelineRange {
    constructor(stream, { height, onLimit, onText }) {
        this.stream = stream;
        this.onLimit = onLimit;
        this.onText = onText;
        this.el = document.createElement('div');
        this.el.className = 'timeline-range';
        this.el.style.height = height + 'px';
        this.chartH = height - RANGE_LABELS_H;
        this.canvas = document.createElement('canvas');
        this.outL = this._part('timeline-range-out');
        this.outR = this._part('timeline-range-out');
        this.sel = this._part('timeline-range-sel');
        this.hA = this._handle('min', 'From');
        this.hB = this._handle('max', 'Until');
        this.el.prepend(this.canvas);
        this.a = 0;
        this.b = Math.max(0, stream.span - 1);
        this._timer = 0;
        this.el.addEventListener('pointerdown', e => this._down(e));
        this.hA.addEventListener('keydown', e => this._key(e, 'a'));
        this.hB.addEventListener('keydown', e => this._key(e, 'b'));
        new ResizeObserver(() => this.redraw()).observe(this.el);
        TimelineChart.onRedraw(() => this.redraw());
    }

    _part(cls) {
        const el = document.createElement('div');
        el.className = cls;
        el.style.height = this.chartH + 'px';
        this.el.appendChild(el);
        return el;
    }

    _handle(side, label) {
        const h = document.createElement('button');
        h.type = 'button';
        h.className = `timeline-range-h ${side}`;
        h.setAttribute('role', 'slider');
        h.setAttribute('aria-label', label);
        h.style.height = this.chartH + 'px';
        this.el.appendChild(h);
        return h;
    }

    x(d) { return d / this.stream.span * this.w; }
    dayAt(x) { return tlClamp(Math.floor(x / this.w * this.stream.span), 0, this.stream.span - 1); }

    redraw() {
        this.w = this.el.clientWidth;
        if (!this.w || !this.stream.count) return;
        const { ctx, w } = TimelineChart.prepare(this.canvas, this.el), c = TimelineChart.colours(), span = this.stream.span;
        TimelineChart.area(ctx, this.stream, d => this.x(d), 0, span, 3, this.chartH, c, w / span);
        TimelineChart.calendar(ctx, this.stream, d => this.x(d), 0, span, 2, this.chartH, w, c, false);
        this._place();
    }

    // set takes any two days and snaps them outwards to whole months.
    set(a, b) {
        const cal = this.stream.calendar;
        this.a = cal.monthStart(cal.month[a]);
        this.b = Math.min(this.stream.span, cal.monthEnd(cal.month[b])) - 1;
        this._place();
        this._emit(true);
    }

    _place() {
        if (!this.w) return;
        const xa = this.x(this.a), xb = this.x(this.b + 1), cal = this.stream.calendar;
        Object.assign(this.outL.style, { left: '0px', width: xa + 'px' });
        Object.assign(this.outR.style, { left: xb + 'px', width: (this.w - xb) + 'px' });
        Object.assign(this.sel.style, { left: xa + 'px', width: (xb - xa) + 'px' });
        this.hA.style.left = (xa - 7) + 'px';
        this.hB.style.left = xb + 'px';
        this.hA.setAttribute('aria-valuetext', cal.formatMonth(this.a));
        this.hB.setAttribute('aria-valuetext', cal.formatMonth(this.b));
        this.onText?.(this.a, this.b);
    }

    _emit(now) {
        clearTimeout(this._timer);
        const fire = () => this.onLimit(this.a, this.b);
        if (now) fire(); else this._timer = setTimeout(fire, RANGE_SETTLE_MS);
    }

    _down(e) {
        if (e.button !== 0) return;
        const r = this.el.getBoundingClientRect(), px = e.clientX - r.left;
        const mode = this._modeAt(e.target, px), handle = { a: this.hA, b: this.hB }[mode];
        const start = { a: this.a, b: this.b, d: this.dayAt(px) };
        handle?.classList.add('is-adjusting');
        this.el.setPointerCapture(e.pointerId);
        const move = ev => { this._drag(mode, this.dayAt(ev.clientX - r.left), start); this._place(); this._emit(false); };
        const up = () => {
            handle?.classList.remove('is-adjusting');
            this.el.removeEventListener('pointermove', move);
            this.el.removeEventListener('pointerup', up);
            this.el.removeEventListener('pointercancel', up);
            this._emit(true);
        };
        this.el.addEventListener('pointermove', move);
        this.el.addEventListener('pointerup', up);
        this.el.addEventListener('pointercancel', up);
        if (mode !== 'move') move(e);
        e.preventDefault();
    }

    _modeAt(target, px) {
        if (target === this.hA) return 'a';
        if (target === this.hB) return 'b';
        if (target === this.sel) return 'move';
        return Math.abs(px - this.x(this.a)) < Math.abs(px - this.x(this.b)) ? 'a' : 'b';
    }

    _drag(mode, d, start) {
        const cal = this.stream.calendar, last = this.stream.span - 1;
        if (mode === 'a') this.a = cal.monthStart(cal.month[Math.min(d, this.b)]);
        else if (mode === 'b') this.b = Math.min(last, cal.monthEnd(cal.month[Math.max(d, this.a)]) - 1);
        else {
            const span = start.b - start.a, a = tlClamp(start.a + d - start.d, 0, last - span);
            this.a = cal.monthStart(cal.month[a]);
            this.b = Math.min(last, this.a + span);
        }
    }

    _key(e, which) {
        const step = { ArrowLeft: -1, ArrowRight: 1 }[e.key];
        if (!step) return;
        e.preventDefault();
        e.stopPropagation();
        const cal = this.stream.calendar, n = step * (e.shiftKey ? 12 : 1);
        if (which === 'a') this.a = cal.monthStart(tlClamp(cal.month[this.a] + n, cal.firstMonth, cal.month[this.b]));
        else this.b = Math.min(this.stream.span, cal.monthEnd(tlClamp(cal.month[this.b] + n, cal.month[this.a], cal.lastMonth))) - 1;
        this._place();
        this._emit(false);
    }
}
```

- [ ] **Step 6: Add the scripts to `index.html`** (after `timeline-stream.js`)

```html
    <script src="/js/timeline-chart.js?v=1"></script>
    <script src="/js/timeline-axis.js?v=1"></script>
    <script src="/js/timeline-range.js?v=1"></script>
```

- [ ] **Step 7: Run the spec to see it pass**

Run: `cd src && go build -o ../unterlumen . && cd ../e2e && npx playwright test specs/timeline-units.spec.js`
Expected: 6 passed.

- [ ] **Step 8: Commit**

```bash
git add src/web/js/timeline-chart.js src/web/js/timeline-axis.js src/web/js/timeline-range.js src/web/index.html e2e/specs/timeline-units.spec.js
git commit -m "feat(web): timeline graph, axis with frame, and range overview"
```

---

### Task 6: Tiles and the desktop band

**Files:**
- Create: `src/web/js/timeline-tiles.js`, `src/web/js/timeline-band.js`
- Modify: `src/web/index.html` (scripts)
- Test: `e2e/specs/timeline-units.spec.js` (add tests)

**Interfaces:**
- Consumes: `TimelineStream`, `LibraryAPI.thumbURL(libID, photoID)`, `tlClamp`.
- Produces:
  - `class TimelineTiles(layer, stream)`: `show(items: {i, x, y, w, h}[])`, `fill()`, `select(i)`, `size`, `clear()`. Tiles are `.timeline-tile[data-i]`, holding an `<img>` once the details are there.
  - `class TimelineBand(stream, { rows, onView(a, b), onSelect(i), onOpen(i), onCount(n) })`: `el`, `tiles`, `setRange(a, b)`, `setRows(n)`, `scrollToDay(d)`, `render()`, `leftDay()`, `lo`, `hi`, `selected`.

- [ ] **Step 1: Add the failing tests**

```js
    test('tiles are created near the screen and released when they leave', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-01-01', days: [0, 0, 1], ratios: [1.5, 1.5, 1.5], undated: 0 });
            const layer = document.createElement('div');
            const tiles = new TimelineTiles(layer, s);
            tiles.show([{ i: 0, x: 0, y: 0, w: 10, h: 10 }, { i: 1, x: 12, y: 0, w: 10, h: 10 }]);
            const two = layer.children.length;
            tiles.select(1);
            tiles.show([{ i: 1, x: 0, y: 0, w: 10, h: 10 }, { i: 2, x: 12, y: 0, w: 10, h: 10 }]);
            return { two, after: [...layer.children].map(c => c.dataset.i), selected: layer.querySelector('.is-selected')?.dataset.i, size: tiles.size };
        });
        expect(r).toEqual({ two: 2, after: ['1', '2'], selected: '1', size: 2 });
    });

    test('the band puts each photo in the row that ends furthest left, a month per column', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2024-01-30', days: [0, 0, 0, 3], ratios: [1, 2, 1, 1], undated: 0 });
            s.ensure = () => Promise.resolve();
            const band = new TimelineBand(s, { rows: 2, onView() {}, onSelect() {}, onOpen() {}, onCount() {} });
            Object.assign(band.el.style, { width: '600px', height: '314px', position: 'absolute' });
            document.body.appendChild(band.el);
            band.setRange(0, s.span - 1);
            const out = { rows: [...band.rows_], xs: [...band.xs], labels: band.labels.map(l => l.x) };
            band.el.remove();
            return out;
        });
        // row height = (314 - 14 scrollbar - 16 pad - 28 labels - 4 gap) / 2 = 126
        expect(r.rows).toEqual([0, 1, 0, 0]);
        expect(r.xs[0]).toBe(16);
        expect(r.xs[1]).toBe(16);
        expect(r.xs[2]).toBe(16 + 126 + 4);
        expect(r.labels.length).toBe(2);
        expect(r.xs[3]).toBe(r.labels[1]);
    });
```

In the band, keep the per-photo row in a typed array called `rows_` (`this.rows` holds the row count).

- [ ] **Step 2: Run them to see them fail**

Run: `cd e2e && npx playwright test specs/timeline-units.spec.js`
Expected: the two new tests FAIL, `TimelineTiles is not defined`.

- [ ] **Step 3: Implement `timeline-tiles.js`**

```js
// TimelineTiles — the photo tiles of a timeline lane, kept only while near
// the screen (ADR-0040). show() adds the tiles that are missing and removes
// the rest; a removed tile's image stops loading and is released, so the
// number of images in the page stays bounded however long the stream is.

class TimelineTiles {
    constructor(layer, stream) {
        this.layer = layer;
        this.stream = stream;
        this.tiles = new Map();
        this.selected = -1;
    }

    get size() { return this.tiles.size; }

    show(items) {
        const keep = new Set();
        for (const it of items) {
            keep.add(it.i);
            let el = this.tiles.get(it.i);
            if (!el) {
                el = this._create(it.i);
                this.tiles.set(it.i, el);
                this.layer.appendChild(el);
            }
            const s = el.style;
            s.left = it.x.toFixed(1) + 'px';
            s.top = it.y.toFixed(1) + 'px';
            s.width = it.w.toFixed(1) + 'px';
            s.height = it.h.toFixed(1) + 'px';
        }
        for (const [i, el] of this.tiles) {
            if (!keep.has(i)) { this._release(el); this.tiles.delete(i); }
        }
    }

    // fill gives the tiles whose details have arrived their image.
    fill() {
        for (const [i, el] of this.tiles) if (!el.firstChild) this._image(el, i);
    }

    select(i) {
        this.tiles.get(this.selected)?.classList.remove('is-selected');
        this.selected = i;
        this.tiles.get(i)?.classList.add('is-selected');
    }

    clear() {
        for (const el of this.tiles.values()) this._release(el);
        this.tiles.clear();
    }

    _create(i) {
        const el = document.createElement('div');
        el.className = 'timeline-tile' + (i === this.selected ? ' is-selected' : '');
        el.dataset.i = i;
        this._image(el, i);
        return el;
    }

    _image(el, i) {
        const p = this.stream.detail(i);
        if (!p) return;
        const img = document.createElement('img');
        img.alt = p.name;
        img.decoding = 'async';
        img.draggable = false;
        img.src = LibraryAPI.thumbURL(p.lib, p.id);
        el.appendChild(img);
    }

    _release(el) {
        el.firstChild?.removeAttribute('src');
        el.remove();
    }
}
```

- [ ] **Step 4: Implement `timeline-band.js`**

```js
// TimelineBand — the desktop's photos (ADR-0040): tiles of one height in 2–4
// rows, time running left to right, as the axis below it does. Each photo
// goes into the row that ends furthest left; a month starts a new column
// under its label. Only tiles near the screen exist (TimelineTiles).

const BAND = { gap: 4, pad: 16, labelH: 28, monthGap: 20, scrollbar: 14, margin: 900 };

class TimelineBand {
    constructor(stream, { rows, onView, onSelect, onOpen, onCount }) {
        Object.assign(this, { stream, rows, onView, onSelect, onOpen, onCount });
        this.el = document.createElement('div');
        this.el.className = 'timeline-band';
        this.el.tabIndex = 0;
        this.el.setAttribute('aria-label', 'Photos by date taken');
        this.sizer = document.createElement('div');
        this.sizer.className = 'timeline-band-sizer';
        this.labelsEl = document.createElement('div');
        const layer = document.createElement('div');
        this.sizer.append(this.labelsEl, layer);
        this.el.appendChild(this.sizer);
        this.tiles = new TimelineTiles(layer, stream);
        this.lo = 0; this.hi = stream.count; this.n = 0; this.raf = 0; this.selected = -1;
        this.startDay = Math.max(0, stream.span - 1);
        this._listen();
        new ResizeObserver(() => this.relayout()).observe(this.el);
    }

    _listen() {
        this.el.addEventListener('scroll', () => this.schedule(), { passive: true });
        this.el.addEventListener('wheel', e => {
            if (Math.abs(e.deltaY) <= Math.abs(e.deltaX)) return;
            this.el.scrollLeft += e.deltaY;
            e.preventDefault();
        }, { passive: false });
        this.el.addEventListener('click', e => { const t = e.target.closest('.timeline-tile'); if (t) this.select(+t.dataset.i); });
        this.el.addEventListener('dblclick', e => { const t = e.target.closest('.timeline-tile'); if (t) this.onOpen(+t.dataset.i); });
        this.el.addEventListener('keydown', e => this._key(e));
    }

    setRange(a, b) {
        this.lo = this.stream.prefix[a];
        this.hi = this.stream.prefix[b + 1];
        this._layout();
        this.scrollToDay(a);
        this.render();
    }

    setRows(n) { this.rows = n; this.relayout(); }

    relayout() {
        if (!this.el.clientHeight) return;
        const d = this.n ? this.leftDay() : this.startDay;
        this._layout();
        this.scrollToDay(d);
        this.render();
    }

    _layout() {
        const { gap, pad, labelH, monthGap, scrollbar } = BAND;
        const rowH = (this.el.clientHeight - scrollbar - pad - labelH - gap * (this.rows - 1)) / this.rows;
        const n = this.hi - this.lo, month = this.stream.calendar?.month;
        this.xs = new Float32Array(n); this.ws = new Float32Array(n); this.rows_ = new Uint8Array(n);
        this.labels = [];
        const ends = new Float64Array(this.rows).fill(pad);
        let key = -1;
        for (let j = 0; j < n; j++) {
            const i = this.lo + j, d = this.stream.day[i];
            if (month[d] !== key) {
                const x = Math.max(...ends) + (key === -1 ? 0 : monthGap);
                ends.fill(x);
                this.labels.push({ x, d, j });
                key = month[d];
            }
            let r = 0;
            for (let q = 1; q < this.rows; q++) if (ends[q] < ends[r] - 0.5) r = q;
            this.xs[j] = ends[r]; this.ws[j] = this.stream.ratio[i] * rowH; this.rows_[j] = r;
            ends[r] += this.ws[j] + gap;
        }
        this.rowH = rowH; this.n = n;
        this.sizer.style.width = (Math.max(...ends) + pad) + 'px';
    }

    leftDay() {
        const j = this._firstEndingAfter(this.el.scrollLeft);
        return this.stream.day[this.lo + Math.min(j, this.n - 1)];
    }

    _firstEndingAfter(x) {
        let lo = 0, hi = this.n;
        while (lo < hi) { const m = (lo + hi) >> 1; if (this.xs[m] + this.ws[m] >= x) hi = m; else lo = m + 1; }
        return lo;
    }

    _firstStartingAfter(x) {
        let lo = 0, hi = this.n;
        while (lo < hi) { const m = (lo + hi) >> 1; if (this.xs[m] > x) hi = m; else lo = m + 1; }
        return lo;
    }

    scrollToDay(d) {
        if (!this.n) return;
        const j = tlClamp(this.stream.prefix[tlClamp(d, 0, this.stream.span)] - this.lo, 0, this.n - 1);
        const label = this.labels.find(l => l.j === j);
        this.el.scrollLeft = (label ? label.x : this.xs[j]) - BAND.pad;
        this.schedule();
    }

    schedule() {
        if (!this.raf) this.raf = requestAnimationFrame(() => { this.raf = 0; this.render(); });
    }

    render() {
        if (!this.n) { this.tiles.show([]); this.labelsEl.innerHTML = ''; return; }
        const sl = this.el.scrollLeft, vw = this.el.clientWidth;
        const j0 = this._firstStartingAfter(sl - vw * 0.5 - BAND.margin), j1 = this._firstStartingAfter(sl + vw * 1.5);
        this.tiles.show(this._items(j0, j1));
        this._labels(sl, vw);
        this.stream.ensure(this.lo + j0, this.lo + j1).then(() => this.tiles.fill()).catch(() => {});
        const last = tlClamp(this._firstStartingAfter(sl + vw) - 1, 0, this.n - 1);
        this.onView(this.leftDay(), this.stream.day[this.lo + last]);
        this.onCount?.(this.tiles.size);
    }

    _items(j0, j1) {
        const items = [], top0 = BAND.pad + BAND.labelH - 8;
        for (let j = j0; j < j1; j++) {
            items.push({ i: this.lo + j, x: this.xs[j], y: top0 + this.rows_[j] * (this.rowH + BAND.gap), w: this.ws[j], h: this.rowH });
        }
        return items;
    }

    _labels(sl, vw) {
        const cal = this.stream.calendar;
        this.labelsEl.innerHTML = this.labels
            .filter(l => l.x > sl - vw && l.x < sl + vw * 2)
            .map(l => `<div class="timeline-band-label" style="left:${l.x}px;top:${BAND.pad - 6}px">${cal.formatMonth(l.d)}</div>`)
            .join('');
    }

    select(i) {
        this.selected = i;
        this.tiles.select(i);
        this.onSelect(i);
    }

    _key(e) {
        if (this.selected < 0) return;
        if (e.key === 'Enter') { e.preventDefault(); this.onOpen(this.selected); return; }
        const step = { ArrowRight: 1, ArrowLeft: -1 }[e.key];
        if (!step) return;
        e.preventDefault();
        const i = tlClamp(this.selected + step, this.lo, this.hi - 1), x = this.xs[i - this.lo];
        const sl = this.el.scrollLeft, vw = this.el.clientWidth;
        if (x < sl || x > sl + vw - 100) this.el.scrollLeft = x - vw / 2;
        this.select(i);
    }
}
```

- [ ] **Step 5: Add the scripts to `index.html`** (after `timeline-range.js`)

```html
    <script src="/js/timeline-tiles.js?v=1"></script>
    <script src="/js/timeline-band.js?v=1"></script>
```

- [ ] **Step 6: Run the spec to see it pass**

Run: `cd src && go build -o ../unterlumen . && cd ../e2e && npx playwright test specs/timeline-units.spec.js`
Expected: 8 passed.

- [ ] **Step 7: Commit**

```bash
git add src/web/js/timeline-tiles.js src/web/js/timeline-band.js src/web/index.html e2e/specs/timeline-units.spec.js
git commit -m "feat(web): timeline band in rows with tiles kept near the screen"
```

---

### Task 7: The phone list and the scrubber

**Files:**
- Create: `src/web/js/timeline-list.js`, `src/web/js/timeline-scrubber.js`
- Modify: `src/web/index.html` (scripts)
- Test: `e2e/specs/timeline-units.spec.js` (add a test)

**Interfaces:**
- Consumes: `TimelineStream`, `TimelineTiles`, `TimelineChart`, `tlClamp`.
- Produces:
  - `class TimelineList(stream, { onView(a, b), onOpen(i), onCount(n) })`: newest first; `el` (the wrapper with the sticky heading), `tiles`, `startDay` (where the first layout lands), `relayout()`, `scrollToDay(d)`, `topDay()`, `render()`, `blocks`.
  - `class TimelineScrubber(stream, { onSeek(d), onRelease() })`: `el`, `setDay(d)`, `y(d)`, `dayAt(y)`.

- [ ] **Step 1: Add the failing test**

```js
    test('the phone list starts with the newest month and the scrubber has it on top', async ({ page }) => {
        const r = await page.evaluate(() => {
            const s = new TimelineStream();
            s.adopt({ version: 'v', start: '2023-12-30', days: [0, 1, 40, 41], ratios: [1.5, 1.5, 1.5, 0.667], undated: 0 });
            s.ensure = () => Promise.resolve();
            const list = new TimelineList(s, { onView() {}, onOpen() {}, onCount() {} });
            Object.assign(list.el.style, { width: '390px', height: '700px', position: 'absolute' });
            document.body.appendChild(list.el);
            list.relayout();
            const heads = list.blocks.filter(b => b.t === 'h').map(b => b.label);
            const scrub = new TimelineScrubber(s, { onSeek() {} });
            Object.assign(scrub.el.style, { height: '700px', position: 'absolute' });
            document.body.appendChild(scrub.el);
            scrub.redraw();
            const order = scrub.y(41) < scrub.y(0);
            list.el.remove(); scrub.el.remove();
            return { heads, order, top: list.blocks[0].d };
        });
        expect(r.heads).toEqual(['February 2024', 'December 2023']);
        expect(r.order).toBe(true);
        expect(r.top).toBe(41);
    });
```

- [ ] **Step 2: Run it to see it fail**

Run: `cd e2e && npx playwright test specs/timeline-units.spec.js`
Expected: the new test FAILS, `TimelineList is not defined`.

- [ ] **Step 3: Implement `timeline-list.js`**

```js
// TimelineList — the phone's photos (ADR-0040): justified rows, newest at the
// top, a heading per month that stays at the top while its month scrolls by.
// Only tiles near the screen exist (TimelineTiles). A tap opens the viewer.

const LIST = { rowH: 104, gap: 2, pad: 8, headH: 38 };

class TimelineList {
    constructor(stream, { onView, onOpen, onCount }) {
        Object.assign(this, { stream, onView, onOpen, onCount });
        this.el = document.createElement('div');
        this.el.className = 'timeline-list-wrap';
        this.scroller = document.createElement('div');
        this.scroller.className = 'timeline-list';
        this.scroller.setAttribute('aria-label', 'Photos by date taken, newest first');
        this.sticky = document.createElement('div');
        this.sticky.className = 'timeline-list-sticky';
        this.sticky.hidden = true;
        this.sizer = document.createElement('div');
        this.sizer.className = 'timeline-list-sizer';
        this.headsEl = document.createElement('div');
        const layer = document.createElement('div');
        this.sizer.append(this.headsEl, layer);
        this.scroller.appendChild(this.sizer);
        this.el.append(this.scroller, this.sticky);
        this.tiles = new TimelineTiles(layer, stream);
        this.blocks = []; this.raf = 0;
        this.startDay = Math.max(0, stream.span - 1);
        this.scroller.addEventListener('scroll', () => this.schedule(), { passive: true });
        this.scroller.addEventListener('click', e => { const t = e.target.closest('.timeline-tile'); if (t) this.onOpen(+t.dataset.i); });
        new ResizeObserver(() => this.relayout()).observe(this.scroller);
    }

    relayout() {
        if (!this.scroller.clientWidth) return;
        const d = this.blocks.length ? this.topDay() : this.startDay;
        this._layout();
        this.scrollToDay(d);
        this.render();
    }

    _layout() {
        const { rowH, gap, pad, headH } = LIST, W = this.scroller.clientWidth - pad * 2, cal = this.stream.calendar;
        const blocks = [];
        let y = pad, key = -1, row = [], sum = 0;
        const flush = (last) => {
            if (!row.length) return;
            const h = Math.min((W - gap * (row.length - 1)) / sum, last ? rowH : Infinity);
            blocks.push({ t: 'r', top: y, h, items: row, d: this.stream.day[row[0]] });
            y += h + gap; row = []; sum = 0;
        };
        for (let i = this.stream.count - 1; i >= 0; i--) {
            const d = this.stream.day[i];
            if (cal.month[d] !== key) {
                flush(true);
                if (key !== -1) y += gap * 3;
                blocks.push({ t: 'h', top: y, h: headH, d, label: cal.formatMonthLong(d), n: this.stream.monthCount(cal.month[d]) });
                y += headH; key = cal.month[d];
            }
            row.push(i); sum += this.stream.ratio[i];
            if (sum * rowH + gap * (row.length - 1) >= W) flush(false);
        }
        flush(true);
        this.blocks = blocks;
        this.sizer.style.height = (y + pad) + 'px';
    }

    _blockAt(y) {
        const B = this.blocks;
        let lo = 0, hi = B.length;
        while (lo < hi) { const m = (lo + hi) >> 1; if (B[m].top + B[m].h >= y) hi = m; else lo = m + 1; }
        return Math.min(lo, B.length - 1);
    }

    topDay() { return this.blocks[this._blockAt(this.scroller.scrollTop + 8)].d; }

    scrollToDay(d) {
        const B = this.blocks;
        if (!B.length) return;
        let lo = 0, hi = B.length;
        while (lo < hi) { const m = (lo + hi) >> 1; if (B[m].d <= d) hi = m; else lo = m + 1; }
        let b = Math.min(lo, B.length - 1);
        if (b > 0 && B[b - 1].t === 'h' && B[b - 1].d === B[b].d) b--;
        this.scroller.scrollTop = Math.max(0, B[b].top - LIST.pad);
        this.schedule();
    }

    schedule() {
        if (!this.raf) this.raf = requestAnimationFrame(() => { this.raf = 0; this.render(); });
    }

    render() {
        const B = this.blocks;
        if (!B.length) return;
        const st = this.scroller.scrollTop, vh = this.scroller.clientHeight;
        const items = [], heads = [];
        let lo = Infinity, hi = -1;
        for (let b = this._blockAt(st - vh * 0.5); b < B.length && B[b].top <= st + vh * 1.5; b++) {
            if (B[b].t === 'h') { heads.push(B[b]); continue; }
            this._rowItems(B[b], items);
            lo = Math.min(lo, B[b].items[B[b].items.length - 1]); hi = Math.max(hi, B[b].items[0]);
        }
        this.tiles.show(items);
        this.headsEl.innerHTML = heads.map(h => this._head(h, `top:${h.top}px;left:${LIST.pad}px`)).join('');
        if (hi >= 0) this.stream.ensure(lo, hi + 1).then(() => this.tiles.fill()).catch(() => {});
        this._sticky(st);
        const a = B[this._blockAt(st + 8)].d, z = B[this._blockAt(st + vh - 8)].d;
        this.onView(Math.min(a, z), Math.max(a, z));
        this.onCount?.(this.tiles.size);
    }

    _rowItems(block, items) {
        let x = LIST.pad;
        for (const i of block.items) {
            const w = this.stream.ratio[i] * block.h;
            items.push({ i, x, y: block.top, w, h: block.h });
            x += w + LIST.gap;
        }
    }

    _head(h, style) {
        return `<div class="timeline-list-head" style="${style}">${h.label}<span class="timeline-list-count">${formatCount(h.n)}</span></div>`;
    }

    _sticky(st) {
        const B = this.blocks;
        let h = this._blockAt(st + 1);
        while (h > 0 && B[h].t !== 'h') h--;
        const head = B[h];
        this.sticky.hidden = !(head.t === 'h' && head.top + head.h < st + 34);
        if (!this.sticky.hidden) this.sticky.innerHTML = `${head.label}<span class="timeline-list-count">${formatCount(head.n)}</span>`;
    }
}
```

- [ ] **Step 4: Implement `timeline-scrubber.js`**

```js
// TimelineScrubber — the phone's time axis (ADR-0040): a column down the
// right edge, newest at the top, with year marks and a bar per month for how
// many photos it has. A finger put down anywhere on it and dragged moves the
// list; a bubble shows the month. The thumb being dragged is orange.

const SCRUB_PAD = 14;
const SCRUB_BAR_MAX = 22;

class TimelineScrubber {
    constructor(stream, { onSeek, onRelease }) {
        Object.assign(this, { stream, onSeek, onRelease });
        this.el = document.createElement('div');
        this.el.className = 'timeline-scrubber';
        this.el.tabIndex = 0;
        this.el.setAttribute('role', 'slider');
        this.el.setAttribute('aria-label', 'Jump to a date');
        this.canvas = document.createElement('canvas');
        this.thumb = document.createElement('div');
        this.thumb.className = 'timeline-scrubber-thumb';
        this.bubble = document.createElement('div');
        this.bubble.className = 'timeline-scrubber-bubble';
        this.bubble.hidden = true;
        this.el.append(this.canvas, this.thumb, this.bubble);
        this.day = Math.max(0, stream.span - 1);
        this.dragging = false;
        this.el.addEventListener('pointerdown', e => this._down(e));
        this.el.addEventListener('keydown', e => this._key(e));
        new ResizeObserver(() => this.redraw()).observe(this.el);
        TimelineChart.onRedraw(() => this.redraw());
    }

    y(d) { return SCRUB_PAD + (this.stream.span - d) / this.stream.span * (this.h - SCRUB_PAD * 2); }
    dayAt(y) { return tlClamp(Math.floor(this.stream.span - (y - SCRUB_PAD) / (this.h - SCRUB_PAD * 2) * this.stream.span), 0, this.stream.span - 1); }

    redraw() {
        this.h = this.el.clientHeight;
        if (!this.h || !this.stream.count) return;
        const { ctx, w } = TimelineChart.prepare(this.canvas, this.el), c = TimelineChart.colours();
        this._bars(ctx, w, c);
        this._years(ctx, w, c);
        this._place();
    }

    _bars(ctx, w, c) {
        const cal = this.stream.calendar;
        let max = 0;
        for (let k = cal.firstMonth; k <= cal.lastMonth; k++) max = Math.max(max, this.stream.monthCount(k));
        ctx.fillStyle = c.bar;
        for (let k = cal.firstMonth; k <= cal.lastMonth; k++) {
            const n = this.stream.monthCount(k);
            if (!n) continue;
            const y0 = this.y(cal.monthEnd(k)), y1 = this.y(cal.monthStart(k)), bw = Math.max(1, n / max * SCRUB_BAR_MAX);
            ctx.fillRect(w - 4 - bw, y0, bw, Math.max(0.8, y1 - y0));
        }
    }

    _years(ctx, w, c) {
        const cal = this.stream.calendar;
        ctx.font = `500 10px ${TIMELINE_MONO}`;
        ctx.textBaseline = 'middle';
        let last = -Infinity;
        for (let yr = cal.year[this.stream.span - 1]; yr >= cal.year[0]; yr--) {
            const yy = this.y(cal.yearStart(yr));
            ctx.fillStyle = c.grid;
            ctx.fillRect(2, Math.round(yy), w - 4, 1);
            if (yy - last < 16) continue;
            ctx.fillStyle = c.fg2;
            ctx.fillText('’' + String(yr).slice(2), 4, yy - 7);
            last = yy;
        }
    }

    setDay(d) { if (!this.dragging) { this.day = d; this._place(); } }

    _place() {
        if (!this.h) return;
        this.thumb.style.top = this.y(this.day + 1) + 'px';
        this.el.setAttribute('aria-valuetext', this.stream.calendar.formatMonth(this.day));
    }

    _down(e) {
        const r = this.el.getBoundingClientRect();
        this.dragging = true;
        this.el.classList.add('is-adjusting');
        this.el.setPointerCapture(e.pointerId);
        const move = ev => this._drag(tlClamp(ev.clientY - r.top, SCRUB_PAD, this.h - SCRUB_PAD));
        const up = () => {
            this.dragging = false;
            this.el.classList.remove('is-adjusting');
            this.bubble.hidden = true;
            this.el.removeEventListener('pointermove', move);
            this.el.removeEventListener('pointerup', up);
            this.el.removeEventListener('pointercancel', up);
            this.onRelease?.();
        };
        this.el.addEventListener('pointermove', move);
        this.el.addEventListener('pointerup', up);
        this.el.addEventListener('pointercancel', up);
        move(e);
        e.preventDefault();
    }

    _drag(y) {
        this.day = this.dayAt(y);
        this._place();
        this.bubble.hidden = false;
        this.bubble.style.top = y + 'px';
        this.bubble.textContent = this.stream.calendar.formatMonth(this.day);
        this.onSeek(this.day);
    }

    _key(e) {
        const step = { ArrowUp: 30, ArrowDown: -30 }[e.key];
        if (!step) return;
        e.preventDefault();
        e.stopPropagation();
        this.onSeek(tlClamp(this.day + step, 0, this.stream.span - 1));
    }
}
```

- [ ] **Step 5: Add the scripts to `index.html`** (after `timeline-band.js`)

```html
    <script src="/js/timeline-list.js?v=1"></script>
    <script src="/js/timeline-scrubber.js?v=1"></script>
```

- [ ] **Step 6: Run the spec to see it pass**

Run: `cd src && go build -o ../unterlumen . && cd ../e2e && npx playwright test specs/timeline-units.spec.js`
Expected: 9 passed.

- [ ] **Step 7: Commit**

```bash
git add src/web/js/timeline-list.js src/web/js/timeline-scrubber.js src/web/index.html e2e/specs/timeline-units.spec.js
git commit -m "feat(web): timeline list and scrubber for the phone"
```

---

### Task 8: The place: wiring, CSS, navigation, and its e2e spec

**Files:**
- Create: `src/web/js/timeline-place.js`, `src/web/css/timeline.css`, `e2e/specs/timeline.spec.js`
- Modify:
  - `src/web/index.html`: stylesheet, the Explore nav entry after `#mode-map`, the tab bar item after `#tab-map`, and the script `timeline-place.js` before `settings.js`.
  - `src/web/js/app.js`: `PHONE_PLACES`, `NAV`, `PLACE_ELEMENTS`, `_openPlace`, and the fields `_timelineEl` / `_timelinePane`.
  - `src/web/js/app-keyboard.js`: `PLACE_KEYS` `'8'`, and a `_toggleInfo` branch.
  - `e2e/specs/phone.spec.js` and `e2e/specs/navigation.spec.js`, if they list every tab or nav entry. Check with `grep -n "tab-map\|mode-map" e2e/specs/*.js` and add the timeline next to the map.

**Interfaces:**
- Consumes everything from Tasks 4–7, plus:
  - `InfoPanel(container)` with `.expanded`, `.toggle()`, `.loadFromURL(url, key)`;
  - `Menu({ items })` with `.button`, and an item `{ label, disabled, title, onSelect }`;
  - `Activity.in(container, text, { area: true })` with `.fail(text)`;
  - `App.showViewer(key, keys, { readOnly, imageURLFn, thumbURLFn, infoLoadFn })`;
  - `LibraryAPI.photoURL`, `LibraryAPI.thumbURL`, `formatCount(n)`, `escapeHtml(s)`.
- Produces: `class TimelinePane(container)` with `render()` and `infoPanel`.

- [ ] **Step 1: Write the failing e2e spec `e2e/specs/timeline.spec.js`**

```js
import { test, expect } from '@playwright/test';
import { waitForAppReady } from '../helpers/wait.js';
import { reindexLibrary } from '../helpers/library.js';

// The Timeline place (ADR-0040): every dated library photo on one axis.
// Desktop: a band in rows over a time bar with a frame; phone: a list and a scrubber.

const LIB_NAME = 'E2E Timeline';

async function openTimeline(page) {
    await page.goto('/#timeline');
    await waitForAppReady(page);
    await page.waitForSelector('.timeline-tile img', { timeout: 20_000 });
}

test.describe('Timeline', () => {
    // Narrow enough that the fixtures' photos overflow the band, so it scrolls.
    test.use({ viewport: { width: 800, height: 640 } });

    test.beforeAll(async ({ request }) => {
        const existing = await (await request.get('/api/library/')).json();
        await Promise.all(existing.filter(l => l.name === LIB_NAME).map(l => request.delete(`/api/library/${l.id}`)));
        const res = await request.post('/api/library/', { data: { name: LIB_NAME, description: '', sourcePath: '' } });
        expect(res.status()).toBe(201);
        await reindexLibrary(request, (await res.json()).id);
    });

    test('is a place in Explore, below Map', async ({ page }) => {
        await page.goto('/');
        await waitForAppReady(page);
        const entry = page.locator('#mode-timeline');
        await expect(entry).toHaveAttribute('href', '#timeline');
        await entry.click();
        await expect(entry).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.timeline-title')).toHaveText('Timeline');
    });

    test('shows the photos in rows over a graph with a frame', async ({ page }) => {
        await openTimeline(page);
        await expect(page.locator('.timeline-count')).toContainText('photos');
        await expect(page.locator('.timeline-frame')).toBeVisible();
        const box = await page.locator('.timeline-axis canvas').boundingBox();
        expect(box.width).toBeGreaterThan(200);
    });

    test('scrolling the band moves the frame, dragging the frame scrolls the band', async ({ page }) => {
        await openTimeline(page);
        const band = page.locator('.timeline-band');
        const frame = page.locator('.timeline-frame');
        await band.evaluate(el => { el.scrollLeft = 0; });
        await expect.poll(() => frame.evaluate(el => parseFloat(el.style.left))).toBeLessThan(5);
        const axis = await page.locator('.timeline-axis').boundingBox();
        await page.mouse.click(axis.x + axis.width - 4, axis.y + 20);
        await expect.poll(() => band.evaluate(el => el.scrollLeft)).toBeGreaterThan(0);
    });

    test('Rows in the menu changes the band and is remembered', async ({ page }) => {
        await openTimeline(page);
        await page.locator('.timeline-head .menu-btn').click();
        await page.getByRole('menuitem', { name: '2 rows' }).click();
        await expect.poll(() => page.evaluate(() => localStorage.getItem('timeline-rows'))).toBe('2');
    });

    test('a click shows the info panel, a double-click opens the viewer', async ({ page }) => {
        await openTimeline(page);
        const tile = page.locator('.timeline-tile').first();
        await tile.click();
        await expect(tile).toHaveClass(/is-selected/);
        await tile.dblclick();
        await expect(page.locator('#viewer-container')).toBeVisible();
    });

    test('keeps the number of tiles bounded while scrolling everything', async ({ page }) => {
        await openTimeline(page);
        const band = page.locator('.timeline-band');
        const counts = [];
        const width = await band.evaluate(el => el.scrollWidth);
        for (let x = 0; x <= width; x += 800) {
            await band.evaluate((el, v) => { el.scrollLeft = v; }, x);
            await page.waitForTimeout(50);
            counts.push(await page.locator('.timeline-tile').count());
        }
        expect(Math.max(...counts)).toBeLessThan(400);
    });

    test('explains an empty timeline', async ({ page }) => {
        await page.route('**/api/timeline', route => route.fulfill({ contentType: 'application/json',
            body: JSON.stringify({ version: 'v', start: '', days: [], ratios: [], undated: 3 }) }));
        await page.goto('/#timeline');
        await waitForAppReady(page);
        await expect(page.locator('.timeline-note')).toContainText('date taken');
    });

    test('reloads when the timeline changed', async ({ page }) => {
        let skeletons = 0;
        await page.route('**/api/timeline', route => { skeletons++; route.continue(); });
        await page.route('**/api/timeline/photos?*', route =>
            skeletons === 1 ? route.fulfill({ status: 409, body: 'changed' }) : route.continue());
        await openTimeline(page);
        expect(skeletons).toBeGreaterThanOrEqual(2);
    });
});

test.describe('Timeline on a phone', () => {
    test.use({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 3, isMobile: true, hasTouch: true });

    test('newest first, a scrubber, a tap opens the photo', async ({ page }) => {
        await openTimeline(page);
        await expect(page.locator('#tab-timeline')).toHaveAttribute('aria-current', 'page');
        await expect(page.locator('.timeline-scrubber')).toBeVisible();
        await expect(page.locator('.timeline-band')).toHaveCount(0);
        const scrub = await page.locator('.timeline-scrubber').boundingBox();
        await page.mouse.move(scrub.x + 30, scrub.y + scrub.height - 20);
        await page.mouse.down();
        await expect(page.locator('.timeline-scrubber-bubble')).toBeVisible();
        await page.mouse.up();
        await page.locator('.timeline-tile').first().click();
        await expect(page.locator('#viewer-container')).toBeVisible();
    });
});
```

- [ ] **Step 2: Run it to see it fail**

Run: `cd src && go build -o ../unterlumen . && cd ../e2e && npx playwright test specs/timeline.spec.js`
Expected: FAIL, `#mode-timeline` is not found.

- [ ] **Step 3: Implement `timeline-place.js`**

```js
// timeline-place.js — the Timeline place: every dated library photo on one
// time axis, each photo once (ADR-0040). A desk gets the band in rows and a
// time bar (overview to limit, axis with the frame); a phone gets a list,
// newest first, and a scrubber. Both are built from one TimelineStream.

const TIMELINE_PHONE = '(max-width: 700px)';
const TIMELINE_VIEWER_REACH = 500;

class TimelinePane {
    constructor(container) {
        this.container = container;
        this.stream = new TimelineStream();
        this.stream.onDetails = () => this._lane?.tiles.fill();
        this.stream.onStale = () => this._reload();
        this.infoPanel = null;
        this._rows = readTimelineRows();
        this._phone = matchMedia(TIMELINE_PHONE);
        this._phone.addEventListener('change', () => { if (this._body && this.stream.count) this._build(this._currentMs()); });
    }

    // Reads the stream again on every visit — a scan may have added photos —
    // and stays at the date it showed.
    async render() {
        if (!this.container.firstChild) this._buildShell();
        await this._reload();
    }

    _buildShell() {
        this.container.innerHTML = `
            <div class="timeline-place">
                <div class="timeline-head">
                    <h1 class="timeline-title">Timeline</h1>
                    <span class="timeline-count mono"></span>
                    <span class="timeline-undated"></span>
                    <span class="timeline-head-spacer"></span>
                </div>
                <div class="timeline-body"></div>
                <div class="timeline-note" hidden></div>
            </div>`;
        this._place = this.container.querySelector('.timeline-place');
        this._body = this.container.querySelector('.timeline-body');
        this._noteEl = this.container.querySelector('.timeline-note');
        this._menu = new Menu({ label: 'Timeline options', items: () => this._menuItems() });
        this.container.querySelector('.timeline-head').appendChild(this._menu.button);
    }

    _menuItems() {
        return [2, 3, 4].map(n => ({
            label: `${n} rows`,
            disabled: n === this._rows,
            title: n === this._rows ? 'Shown now' : undefined,
            onSelect: () => { this._rows = n; writeTimelineRows(n); this._lane?.setRows?.(n); },
        }));
    }

    async _reload() {
        const at = this._currentMs();
        if (!this._lane) {
            this._noteEl.hidden = false;
            Activity.in(this._noteEl, 'Reading the timeline…', { area: true });
        }
        try {
            await this.stream.load();
        } catch (err) {
            this._noteEl.hidden = false;
            Activity.in(this._noteEl, '', { area: true }).fail(`The timeline could not be read: ${err.message}. Reload the page to try again.`);
            return;
        }
        this._noteEl.hidden = true;
        this._head();
        this._build(at);
    }

    // The date shown now, as UTC milliseconds, so it survives a new skeleton.
    _currentMs() {
        const cal = this.stream.calendar;
        if (!cal || !this._lane) return null;
        return cal.msOf(this._phone.matches ? this._lane.topDay() : this._lane.leftDay());
    }

    _head() {
        const s = this.stream, cal = s.calendar;
        this.container.querySelector('.timeline-count').textContent = s.count
            ? `${formatCount(s.count)} photos · ${cal.formatMonth(0)} – ${cal.formatMonth(s.span - 1)}` : '';
        this.container.querySelector('.timeline-undated').textContent = s.undated
            ? `${formatCount(s.undated)} without a date are not shown` : '';
        this._menu.button.hidden = this._phone.matches || !s.count;
    }

    _build(atMs) {
        this._lane?.tiles.clear();
        this._lane = null;
        this._body.innerHTML = '';
        if (!this.stream.count) { this._explainEmpty(); return; }
        const day = atMs == null ? this.stream.span - 1 : tlClamp(this.stream.calendar.dayAt(atMs), 0, this.stream.span - 1);
        if (this._phone.matches) this._buildPhone(day); else this._buildDesk(day);
        this._head();
    }

    _explainEmpty() {
        this._noteEl.hidden = false;
        this._noteEl.textContent = this.stream.undated
            ? 'None of the photos in the libraries has a date taken, so there is nothing to place on the timeline.'
            : 'There are no photos in any library yet. Make a library in Libraries, and its dated photos appear here.';
    }

}

// Desk and phone layouts live in their own methods below the class body.
Object.assign(TimelinePane.prototype, {
    _buildDesk(day) {
        const grid = document.createElement('div');
        grid.className = 'timeline-desk';
        grid.innerHTML = `
            <div class="timeline-main"><div class="timeline-band-wrap"></div><div class="timeline-bar"></div></div>
            <div class="lib-info-panel-container timeline-info"></div>`;
        this._body.appendChild(grid);
        this.infoPanel = new InfoPanel(grid.querySelector('.timeline-info'));
        const band = new TimelineBand(this.stream, {
            rows: this._rows,
            onView: (a, b) => axis.setView(a, b),
            onSelect: (i) => this._showInfo(i),
            onOpen: (i) => this._open(i),
        });
        band.startDay = day;
        const axis = new TimelineAxis(this.stream, { height: 92, onSeek: (d) => band.scrollToDay(d), onRelease: () => band.render() });
        grid.querySelector('.timeline-band-wrap').appendChild(band.el);
        grid.querySelector('.timeline-bar').append(...this._barRows(band, axis), axis.el);
        this._lane = band;
    },

    _barRows(band, axis) {
        const text = document.createElement('span');
        text.className = 'timeline-range-text mono';
        const range = new TimelineRange(this.stream, {
            height: 40,
            onLimit: (a, b) => { axis.setDomain(a, b); band.setRange(a, b); },
            onText: (a, b) => { text.textContent = `${this.stream.calendar.formatMonth(a)} – ${this.stream.calendar.formatMonth(b)}`; },
        });
        const all = document.createElement('button');
        all.type = 'button';
        all.className = 'btn btn-sm';
        all.textContent = 'Show all';
        all.addEventListener('click', () => range.set(0, this.stream.span - 1));
        const shown = document.createElement('div');
        shown.className = 'timeline-bar-row';
        const label = document.createElement('span');
        label.className = 'timeline-bar-label';
        label.textContent = 'Shown';
        shown.append(label, text, all);
        return [range.el, shown];
    },

    _buildPhone(day) {
        const wrap = document.createElement('div');
        wrap.className = 'timeline-phone';
        this._body.appendChild(wrap);
        this.infoPanel = null;
        const list = new TimelineList(this.stream, {
            onView: (a, b) => scrub.setDay(b),
            onOpen: (i) => this._open(i),
        });
        const scrub = new TimelineScrubber(this.stream, { onSeek: (d) => list.scrollToDay(d), onRelease: () => list.render() });
        wrap.append(list.el, scrub.el);
        list.startDay = day;
        this._lane = list;
    },

    _showInfo(i) {
        const p = this.stream.detail(i);
        if (!p || !this.infoPanel) return;
        if (!this.infoPanel.expanded) this.infoPanel.toggle();
        this.infoPanel.loadFromURL(`/api/library/${p.lib}/photo/${p.id}/info`, `lib:${p.lib}:${p.id}`);
    },

    // The viewer gets the photos around the one opened, read-only as from
    // the Map: 100 000 keys would be too many to hand over at once.
    async _open(i) {
        const lo = Math.max(0, i - TIMELINE_VIEWER_REACH), hi = Math.min(this.stream.count, i + TIMELINE_VIEWER_REACH);
        await this.stream.ensure(lo, hi);
        const byKey = new Map();
        for (let j = lo; j < hi; j++) {
            const p = this.stream.detail(j);
            if (p) byKey.set(`${p.lib}/${p.id}/${p.name}`, p);
        }
        const opened = this.stream.detail(i);
        if (!opened) return;
        const keys = [...byKey.keys()];
        if (this._phone.matches) keys.reverse();
        App.showViewer(`${opened.lib}/${opened.id}/${opened.name}`, keys, {
            readOnly: true,
            imageURLFn: (k) => LibraryAPI.photoURL(byKey.get(k).lib, byKey.get(k).id),
            thumbURLFn: (k) => LibraryAPI.thumbURL(byKey.get(k).lib, byKey.get(k).id),
            infoLoadFn: (k, panel) => {
                const p = byKey.get(k);
                panel.loadFromURL(`/api/library/${p.lib}/photo/${p.id}/info`, `lib:${p.lib}:${p.id}`);
            },
        });
    },
});

// The number of rows is remembered per browser; without storage it is 3.
function readTimelineRows() {
    try { return [2, 3, 4].includes(Number(localStorage.getItem('timeline-rows'))) ? Number(localStorage.getItem('timeline-rows')) : 3; } catch { return 3; }
}

function writeTimelineRows(n) {
    try { localStorage.setItem('timeline-rows', String(n)); } catch { /* per browser only */ }
}
```

**Note for the implementer:** the `Object.assign(TimelinePane.prototype, …)` block only keeps this plan readable. Move those methods into the class body, which keeps the file to one idiom. If `timeline-place.js` then grows past about 300 lines, split the desktop layout into `timeline-desk.js` (a `TimelineDesk` class with `build(pane, day)`) and report the split.

- [ ] **Step 4: Write `src/web/css/timeline.css`**

```css
/* Timeline place (ADR-0040). Its own file: style.css is past 7 000 lines. */

/* --- Place --- */
.timeline-place { height: 100%; display: flex; flex-direction: column; min-height: 0; }
.timeline-head {
    display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-2) var(--space-4);
    padding: var(--space-3) var(--space-5); border-bottom: 1px solid var(--line);
}
.timeline-title { margin: 0; font-size: var(--text-lg); font-weight: 600; }
.timeline-count { font-size: var(--text-xs); color: var(--fg-3); font-variant-numeric: tabular-nums; }
.timeline-undated { font-size: var(--text-xs); color: var(--fg-3); }
.timeline-head-spacer { flex: 1; }
.timeline-body { flex: 1; min-height: 0; display: flex; }
.timeline-note { padding: var(--space-6); color: var(--fg-2); font-size: var(--text-sm); max-width: 60ch; }

/* --- Desk --- */
.timeline-desk { flex: 1; min-width: 0; display: grid; grid-template-columns: minmax(0, 1fr) auto; }
.timeline-main { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.timeline-band-wrap { flex: 1; position: relative; min-height: 0; }
.timeline-bar { background: var(--bg-2); padding: var(--space-3) var(--space-5); display: flex; flex-direction: column; gap: var(--space-3); }
.timeline-bar-row { display: flex; align-items: center; gap: var(--space-3); }
.timeline-bar-label { font-size: var(--text-sm); font-weight: 500; color: var(--fg-2); }
.timeline-range-text { font-size: var(--text-xs); color: var(--fg-2); min-width: 150px; font-variant-numeric: tabular-nums; }

/* --- Band --- */
.timeline-band { position: absolute; inset: 0; overflow-x: auto; overflow-y: hidden; overscroll-behavior: contain; }
.timeline-band:focus-visible { box-shadow: inset 0 0 0 2px var(--accent); outline: none; }
.timeline-band-sizer { position: relative; height: 100%; }
.timeline-band-label {
    position: absolute; height: 22px; display: flex; align-items: center; padding-left: var(--space-2);
    border-left: 1px solid var(--line-strong); font: 500 var(--text-xs)/1 var(--font-mono); color: var(--fg-2); white-space: nowrap;
}

/* --- Tiles --- */
.timeline-tile { position: absolute; background: var(--bg-3); cursor: pointer; overflow: hidden; }
.timeline-tile img { width: 100%; height: 100%; object-fit: cover; display: block; }
.timeline-tile.is-selected { outline: 2px solid var(--fg); outline-offset: 1px; z-index: 1; }

/* --- Axis and frame --- */
.timeline-axis, .timeline-range, .timeline-scrubber { position: relative; touch-action: none; user-select: none; -webkit-user-select: none; }
.timeline-axis { cursor: pointer; }
.timeline-axis canvas, .timeline-range canvas, .timeline-scrubber canvas { position: absolute; inset: 0; width: 100%; height: 100%; }
.timeline-frame {
    position: absolute; top: 0; box-sizing: border-box; border: 2px solid var(--fg); border-radius: var(--radius-sm);
    background: color-mix(in srgb, var(--fg) 7%, transparent); cursor: grab;
}
.timeline-frame.is-adjusting { border-color: var(--accent); cursor: grabbing; }
.timeline-tip {
    position: absolute; top: -26px; transform: translateX(-50%); padding: 2px var(--space-2);
    background: var(--fg); color: var(--bg); border-radius: var(--radius-sm);
    font: 500 var(--text-xs)/1.4 var(--font-mono); white-space: nowrap; pointer-events: none;
}

/* --- Range --- */
.timeline-range-out { position: absolute; top: 0; background: color-mix(in srgb, var(--bg-2) 72%, transparent); pointer-events: none; }
.timeline-range-sel { position: absolute; top: 0; box-sizing: border-box; border-block: 1px solid var(--line-strong); cursor: grab; }
.timeline-range-h { position: absolute; top: 0; width: 7px; padding: 0; box-sizing: border-box; border: 2px solid var(--fg); background: none; cursor: ew-resize; }
.timeline-range-h::before { content: ''; position: absolute; inset: -6px -12px; }
.timeline-range-h.min { border-right: none; }
.timeline-range-h.max { border-left: none; }
.timeline-range-h:hover { border-color: var(--fg-2); }
.timeline-range-h.is-adjusting { border-color: var(--accent); }

/* --- Phone --- */
.timeline-phone { flex: 1; min-width: 0; display: flex; }
.timeline-list-wrap { flex: 1; position: relative; min-width: 0; }
.timeline-list { position: absolute; inset: 0; overflow-y: auto; overscroll-behavior: contain; }
.timeline-list-sizer { position: relative; }
.timeline-list-head, .timeline-list-sticky {
    display: flex; align-items: flex-end; gap: var(--space-2); font-size: var(--text-sm); font-weight: 600; white-space: nowrap;
}
.timeline-list-head { position: absolute; height: 38px; padding-bottom: 6px; box-sizing: border-box; }
.timeline-list-sticky {
    position: absolute; top: 0; left: 0; right: 0; z-index: 2; height: 32px; align-items: center; padding: 0 var(--space-2);
    background: var(--bg); border-bottom: 1px solid var(--line); pointer-events: none;
}
.timeline-list-count { font: 400 var(--text-xs)/1 var(--font-mono); color: var(--fg-3); }

/* --- Scrubber --- */
.timeline-scrubber { width: 60px; flex-shrink: 0; border-left: 1px solid var(--line); cursor: pointer; }
.timeline-scrubber-thumb { position: absolute; left: 0; right: 0; height: 2px; margin-top: -1px; background: var(--fg); pointer-events: none; }
.timeline-scrubber-thumb::after {
    content: ''; position: absolute; right: var(--space-1); top: -7px; width: 8px; height: 16px;
    border-radius: var(--radius-full); background: var(--fg);
}
.timeline-scrubber.is-adjusting .timeline-scrubber-thumb,
.timeline-scrubber.is-adjusting .timeline-scrubber-thumb::after { background: var(--accent); }
.timeline-scrubber-bubble {
    position: absolute; right: 68px; transform: translateY(-50%); z-index: 3; padding: 6px 10px;
    background: var(--fg); color: var(--bg); border-radius: var(--radius-md); box-shadow: var(--shadow-float);
    font: 500 var(--text-sm)/1.2 var(--font-mono); white-space: nowrap; pointer-events: none;
}
```

`6px 10px` and `right: 68px` are off the spacing scale. Replace them with `var(--space-2) var(--space-3)` and `calc(60px + var(--space-2))` when you write the file.

- [ ] **Step 5: Wire the place in `index.html`, `app.js` and `app-keyboard.js`**

`index.html`:
- After `style.css`: `<link rel="stylesheet" href="/css/timeline.css?v=1">`.
- After the `#mode-map` link, in the same `nav-group`:

```html
                <a class="nav-item" id="mode-timeline" href="#timeline" data-mode="timeline">
                    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" aria-hidden="true">
                        <path d="M1.5 12.5h13M4 12.5V9M7 12.5V5M10 12.5V7.5M13 12.5V10"/>
                    </svg>
                    <span class="nav-text">Timeline</span>
                </a>
```

- After `#tab-map`, the same `<a class="tabbar-item" id="tab-timeline" href="#timeline" data-mode="timeline">`, with the same markup as the other tab bar items and the same icon.
- Before `settings.js`: `<script src="/js/timeline-place.js?v=1"></script>`.
- Bump `app.js?v=` and `app-keyboard.js?v=` by one.

`app.js`:

```js
const PHONE_PLACES = new Set(['browse', 'library', 'map', 'timeline', 'published']);
// in NAV, after map:
        timeline: { hash: 'timeline', id: 'mode-timeline', key: '8' },
// in PLACE_ELEMENTS, after ['_mapEl', 'map']:
        ['_timelineEl', 'timeline'],
// in _openPlace, after case 'map':
            case 'timeline': this._openPane('_timelineEl', '_timelinePane', TimelinePane); break;
```

Also add the fields `_timelineEl: null,` and `_timelinePane: null,` next to the other place fields.

`app-keyboard.js`: add `'8': 'timeline'` to `PLACE_KEYS`, and this branch at the end of `_toggleInfo`:

```js
        } else if (app.mode === 'timeline' && app._timelinePane?.infoPanel) {
            if (this._viewerOpen()) return;
            e.preventDefault();
            app._timelinePane.infoPanel.toggle();
        }
```

- [ ] **Step 6: Run the specs to see them pass**

Run: `cd src && go build -o ../unterlumen . && cd ../e2e && npx playwright test specs/timeline.spec.js specs/timeline-units.spec.js specs/phone.spec.js specs/navigation.spec.js specs/keyboard.spec.js`
Expected: all pass. If a phone or navigation spec lists every tab or entry, add the timeline next to the map and run again.

- [ ] **Step 7: Look at it running**

Invoke the `unterlumen-dev` skill (port 8080) on the e2e fixtures, not on the real instance. Then check:
- Desktop and a 390 px window.
- Light and dark.
- A grayscale screenshot: the frame and the handles must still read.

Report the port to the user.

- [ ] **Step 8: Commit**

```bash
git add src/web/js/timeline-place.js src/web/css/timeline.css src/web/index.html src/web/js/app.js src/web/js/app-keyboard.js e2e/specs/timeline.spec.js
git commit -m "feat(web): the Timeline place in Explore"
```

Also add `e2e/specs/phone.spec.js` / `navigation.spec.js` to that commit if Step 6 changed them.

---

### Task 9: Documentation, full suite, screenshots

**Files:**
- Modify:
  - `README.md`: the Timeline under the places or features, next to the Map.
  - `CHANGELOG.md`: under `## [Unreleased]`, in the existing `### Added`.
  - `doc/architecture/arc42.md`: building blocks (`internal/timeline`, `internal/api/timeline`), the runtime view of skeleton plus pages, and ADR-0040 in the index in section 9.
  - `doc/architecture/adr/0040-timeline-place.md`: Status `Accepted.`
  - `README.md`, if its Documentation section lists ADRs: add ADR-0040 there.
- Move: `doc/features/open/2026-09-27-timeline.md` and this plan to `doc/features/done/`, with every acceptance criterion ticked.

- [ ] **Step 1: CHANGELOG entry** (merge into the existing `### Added`)

```markdown
- **Timeline**: a new place in Explore that puts every dated library photo on one time axis, each photo once. On a desk the photos run left to right in 2–4 rows over a graph of when they were taken; drag the frame, click the axis, or limit the range to a span of months. On a phone the newest come first, with a scrubber down the right edge. Only the photos near the screen are loaded (ADR-0040).
```

- [ ] **Step 2: README and arc42** (as listed under Files, with `*Last modified: 2026-09-27*` on arc42)

- [ ] **Step 3: Run everything**

Run: `cd src && go vet ./... && go test ./... && go build -o ../unterlumen . && cd ../e2e && npm test`
Expected: vet has no output, all Go packages `ok`, and every e2e spec passes. Report the numbers (for example `330 passed`). Any failure is reported with its output, not retried away.

- [ ] **Step 4: Screenshots**

Invoke the `unterlumen-readme-screenshots` skill for the new place (desktop and phone).

- [ ] **Step 5: Commit**

```bash
git add README.md CHANGELOG.md doc/architecture/arc42.md doc/architecture/adr/0040-timeline-place.md doc/features/done/2026-09-27-timeline.md doc/features/done/2026-09-27-timeline-plan.md
git rm --cached doc/features/open/2026-09-27-timeline.md doc/features/open/2026-09-27-timeline-plan.md 2>/dev/null || true
git commit -m "docs: the Timeline place"
```

Add the screenshot files the skill produced to that commit by exact path.
