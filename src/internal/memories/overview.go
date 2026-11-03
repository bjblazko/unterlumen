package memories

import (
	"sort"
	"time"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/timeline"
)

// picksPer is how many photos a memory shows at first.
const picksPer = 5

const dayLayout = "2006-01-02"

// Photo is a photo as the browser opens it.
type Photo struct {
	Lib   string  `json:"lib"`
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Taken string  `json:"taken"`
	Ratio float64 `json:"ratio"`
}

// Year is on this day, that many years ago.
type Year struct {
	Year  int     `json:"year"`
	Ago   int     `json:"ago"`
	Date  string  `json:"date"`
	Total int     `json:"total"`
	Picks []Photo `json:"picks"`
}

// Card is one of the other memories: a trip, a person, this month back
// then, or rediscovered photos.
type Card struct {
	Kind     string `json:"kind"`
	Key      string `json:"key"`
	Total    int    `json:"total"`
	Cover    Photo  `json:"cover"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
	Country  string `json:"country,omitempty"`
	Days     int    `json:"days,omitempty"`
	Name     string `json:"name,omitempty"`
	Birthday string `json:"birthday,omitempty"`
	Age      *int   `json:"age,omitempty"`
	Month    int    `json:"month,omitempty"`
}

// Overview is the Memories place on one day.
type Overview struct {
	Day   string `json:"day"`
	Years []Year `json:"years"`
	Cards []Card `json:"cards"`
}

// Detail is one memory: its picks and all of its photos, in time order
// (rediscovered: rarest first). Its Cover is its best pick; the overview's
// card may show another, one no other row or card shows.
type Detail struct {
	Card
	Picks  []Photo `json:"picks"`
	Photos []Photo `json:"photos"`
}

// memory is a memory before it is shown: its card, its photos and its
// picks, best first.
type memory struct {
	card   Card
	photos []library.MemoryPhoto
	picks  []library.MemoryPhoto
}

// world is what every memory of a day is chosen from.
type world struct {
	day    time.Time
	all    []library.MemoryPhoto // eligible, in time order
	rarity Rarity
	home   Home
	births map[string]string
}

func newWorld(day time.Time, all []library.MemoryPhoto, births map[string]string) *world {
	ok := eligible(all)
	sort.SliceStable(ok, func(i, j int) bool { return ok[i].Taken < ok[j].Taken })
	home := FindHome(ok)
	return &world{day: day, all: ok, rarity: NewRarity(ok, home), home: home, births: births}
}

func (w *world) today() string { return w.day.Format(dayLayout) }

// pick chooses a memory's picks, seeded by the day and the memory.
func (w *world) pick(kind, key string, photos []library.MemoryPhoto, n int) []library.MemoryPhoto {
	return Pick(photos, w.rarity, w.today()+"|"+kind+"|"+key, n)
}

// Build lays out the Memories place on day.
func Build(day time.Time, all []library.MemoryPhoto, births map[string]string) Overview {
	w := newWorld(day, all, births)
	o := Overview{Day: w.today(), Years: []Year{}, Cards: []Card{}}
	years, cards := w.others()
	if m, ok := w.rediscovered(shownIn(years, cards)); ok {
		cards = append(cards, m)
	}
	used := shownIn(years)
	for _, m := range years {
		o.Years = append(o.Years, m.year(day))
	}
	for _, m := range cards {
		c := m.shown().Card
		c.Cover = photoOf(m.coverBeside(used))
		used[c.Cover.ID] = true
		o.Cards = append(o.Cards, c)
	}
	return o
}

// coverBeside is the memory's best photo that is not used yet: its picks
// best first, then its other photos; its best pick when all are used.
func (m memory) coverBeside(used map[string]bool) library.MemoryPhoto {
	for _, set := range [][]library.MemoryPhoto{m.picks, m.photos} {
		for _, p := range set {
			if !used[p.ID] {
				return p
			}
		}
	}
	return m.picks[0]
}

// others are the day's years and the cards but rediscovered, which leaves
// out what they show.
func (w *world) others() (years, cards []memory) {
	years = w.onThisDay()
	for _, find := range []func() (memory, bool){w.tripCard, w.personCard, w.monthCard} {
		if m, ok := find(); ok {
			cards = append(cards, m)
		}
	}
	return years, cards
}

func shownIn(sets ...[]memory) map[string]bool {
	shown := map[string]bool{}
	for _, set := range sets {
		for _, m := range set {
			for _, p := range m.picks {
				shown[p.ID] = true
			}
		}
	}
	return shown
}

// BuildDetail is one memory on day; false when there is no such memory.
func BuildDetail(day time.Time, kind, key string, all []library.MemoryPhoto, births map[string]string) (Detail, bool) {
	w := newWorld(day, all, births)
	var m memory
	var ok bool
	switch kind {
	case "day":
		m, ok = w.dayMemory(key)
	case "trip":
		m, ok = w.tripMemory(key)
	case "person":
		m, ok = w.personMemory(key)
	case "month":
		m, ok = w.monthMemory(key)
	case "rediscovered":
		if key == w.today() {
			m, ok = w.rediscovered(shownIn(w.others()))
		}
	}
	if !ok {
		return Detail{}, false
	}
	return m.shown(), true
}

// shown turns a memory into what the browser gets: the picks in time
// order, the best one as the cover.
func (m memory) shown() Detail {
	d := Detail{Card: m.card, Picks: photosOf(m.picks), Photos: photosOf(m.photos)}
	d.Total = len(m.photos)
	if len(m.picks) > 0 {
		d.Cover = photoOf(m.picks[0])
	}
	sort.SliceStable(d.Picks, func(i, j int) bool { return d.Picks[i].Taken < d.Picks[j].Taken })
	return d
}

func (m memory) year(day time.Time) Year {
	d := m.shown()
	y, _ := time.Parse(dayLayout, m.card.Key)
	return Year{Year: y.Year(), Ago: day.Year() - y.Year(), Date: m.card.Key, Total: d.Total, Picks: d.Picks}
}

func photosOf(ps []library.MemoryPhoto) []Photo {
	out := make([]Photo, len(ps))
	for i, p := range ps {
		out[i] = photoOf(p)
	}
	return out
}

func photoOf(p library.MemoryPhoto) Photo {
	return Photo{Lib: p.LibraryID, ID: p.ID, Name: p.Filename, Taken: p.Taken, Ratio: timeline.DisplayRatio(p.Width, p.Height, p.Orientation)}
}
