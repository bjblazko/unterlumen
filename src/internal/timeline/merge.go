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
