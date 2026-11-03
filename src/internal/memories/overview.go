package memories

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/timeline"
)

// A memory shows between picksMin and picksMin+picksSpread-1 photos at
// first, as many as its day and key choose: blocks of one weight make a
// dull sheet. picksPer is the step of the turn through rediscovered photos.
const (
	picksMin    = 3
	picksSpread = 5
	picksPer    = 5
)

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
	Kind        string `json:"kind"`
	Key         string `json:"key"`
	Total       int    `json:"total"`
	Cover       Photo  `json:"cover"`
	Start       string `json:"start,omitempty"`
	End         string `json:"end,omitempty"`
	Country     string `json:"country,omitempty"`
	Days        int    `json:"days,omitempty"`
	Anniversary bool   `json:"anniversary,omitempty"` // a trip that began around today in an earlier year
	Name        string `json:"name,omitempty"`
	Birthday    string `json:"birthday,omitempty"`
	Age         *int   `json:"age,omitempty"`
	Month       int    `json:"month,omitempty"`
}

// CardView is a card with the photos its block on the sheet shows.
type CardView struct {
	Card
	Picks []Photo `json:"picks"`
}

// Overview is the Memories place on one day.
type Overview struct {
	Day   string     `json:"day"`
	Years []Year     `json:"years"`
	Cards []CardView `json:"cards"`
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

	tripsOnce sync.Once
	tripsAll  []Trip
}

func newWorld(day time.Time, all []library.MemoryPhoto, births map[string]string) *world {
	ok := eligible(all)
	sort.SliceStable(ok, func(i, j int) bool { return ok[i].Taken < ok[j].Taken })
	home := FindHome(ok)
	return &world{day: day, all: ok, rarity: NewRarity(ok, home), home: home, births: births}
}

func (w *world) today() string { return w.day.Format(dayLayout) }

func (w *world) seed(kind, key string) string { return w.today() + "|" + kind + "|" + key }

// picksFor is how many photos the memory shows, the day's choice.
func (w *world) picksFor(kind, key string) int {
	return picksMin + choose(w.seed(kind, key)+"|n", picksSpread)
}

// pick chooses a memory's picks, seeded by the day and the memory.
func (w *world) pick(kind, key string, photos []library.MemoryPhoto) []library.MemoryPhoto {
	return Pick(photos, w.rarity, w.seed(kind, key), w.picksFor(kind, key))
}

// trips are the trips of all the photos, found once.
func (w *world) trips() []Trip {
	w.tripsOnce.Do(func() { w.tripsAll = FindTrips(w.all, w.home) })
	return w.tripsAll
}

// Day is every memory of one day, worked out once: the overview and each
// memory's page read from it.
type Day struct {
	w     *world
	once  sync.Once
	years []memory
	cards []memory // rediscovered last, when there is one
}

func NewDay(day time.Time, all []library.MemoryPhoto, births map[string]string) *Day {
	return &Day{w: newWorld(day, all, births)}
}

func (d *Day) memories() {
	d.once.Do(func() {
		d.years, d.cards = d.w.others()
		if m, ok := d.w.rediscovered(shownIn(d.years, d.cards)); ok {
			d.cards = append(d.cards, m)
		}
	})
}

// Build lays out the Memories place on day.
func Build(day time.Time, all []library.MemoryPhoto, births map[string]string) Overview {
	return NewDay(day, all, births).Overview()
}

// Overview is the Memories place: the years, then the cards, each with a
// cover no row or card before it shows.
func (d *Day) Overview() Overview {
	d.memories()
	o := Overview{Day: d.w.today(), Years: []Year{}, Cards: []CardView{}}
	used := shownIn(d.years)
	for _, m := range d.years {
		o.Years = append(o.Years, m.year(d.w.day))
	}
	for _, m := range d.cards {
		o.Cards = append(o.Cards, m.beside(used))
	}
	return o
}

// beside is the card of a memory on the sheet: its picks that no memory
// before it shows — or, when it has none left, its best photo nobody shows
// yet — the best of them its cover. What it shows is marked used.
func (m memory) beside(used map[string]bool) CardView {
	var picks []library.MemoryPhoto
	for _, p := range m.picks {
		if !used[p.ID] {
			picks = append(picks, p)
		}
	}
	if len(picks) == 0 {
		picks = []library.MemoryPhoto{m.coverBeside(used)}
	}
	for _, p := range picks {
		used[p.ID] = true
	}
	shown := memory{card: m.card, photos: m.photos, picks: picks}.shown()
	return CardView{Card: shown.Card, Picks: shown.Picks}
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
	return NewDay(day, all, births).Detail(kind, key)
}

// Detail is one memory of the day; false when there is no such memory: a
// day must be today's date in an earlier year.
func (d *Day) Detail(kind, key string) (Detail, bool) {
	w := d.w
	var m memory
	var ok bool
	switch kind {
	case "day":
		if w.isOnThisDay(key) {
			m, ok = w.dayMemory(key)
		}
	case "trip":
		m, ok = w.tripMemory(key)
	case "person":
		m, ok = w.personMemory(key)
	case "month":
		m, ok = w.monthMemory(key)
	case "rediscovered":
		if key == w.today() {
			d.memories()
			m, ok = d.cardOf(kind)
		}
	}
	if !ok {
		return Detail{}, false
	}
	return m.shown(), true
}

func (d *Day) cardOf(kind string) (memory, bool) {
	for _, m := range d.cards {
		if m.card.Kind == kind {
			return m, true
		}
	}
	return memory{}, false
}

// isOnThisDay says key is today's month and day in an earlier year.
func (w *world) isOnThisDay(key string) bool {
	y, err := strconv.Atoi(key[:min(4, len(key))])
	return err == nil && len(key) == 10 && key[4:] == w.today()[4:] && y < w.day.Year()
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
