package memories

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"huepattl.de/unterlumen/internal/library"
)

// onThisDay is a memory for each earlier year with photos on today's
// month and day, newest first.
func (w *world) onThisDay() []memory {
	years := map[int]bool{}
	for _, p := range w.all {
		if y, ok := w.onToday(p); ok {
			years[y] = true
		}
	}
	var out []memory
	for _, y := range sortedDesc(years) {
		if m, ok := w.dayMemory(fmt.Sprintf("%04d-%s", y, w.today()[5:])); ok {
			out = append(out, m)
		}
	}
	return out
}

// onToday says the photo was taken on today's month and day in an earlier
// year, and which. On 28 February of a year without a 29th, the 29th of
// earlier years counts too.
func (w *world) onToday(p library.MemoryPhoto) (int, bool) {
	if len(p.Taken) < 10 {
		return 0, false
	}
	y, err := strconv.Atoi(p.Taken[:4])
	if err != nil || y >= w.day.Year() {
		return 0, false
	}
	md := p.Taken[5:10]
	return y, md == w.today()[5:] || (md == "02-29" && w.leapEve())
}

// leapEve says today is 28 February of a year without a 29th.
func (w *world) leapEve() bool {
	return w.day.Month() == time.February && w.day.Day() == 28 && !isLeap(w.day.Year())
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

// dayMemory is the photos of one date; on a leap eve the 28th of a year
// takes its 29th along.
func (w *world) dayMemory(key string) (memory, bool) {
	d, err := time.Parse(dayLayout, key)
	if err != nil {
		return memory{}, false
	}
	days := map[string]bool{key: true}
	if w.leapEve() && d.Month() == time.February && d.Day() == 28 {
		days[key[:5]+"02-29"] = true
	}
	photos := w.where(func(p library.MemoryPhoto) bool { return len(p.Taken) >= 10 && days[p.Taken[:10]] })
	if len(photos) == 0 {
		return memory{}, false
	}
	return memory{card: Card{Kind: "day", Key: key}, photos: photos, picks: w.pick("day", key, photos)}, true
}

// monthCard is this month in earlier years, when there are at least two.
func (w *world) monthCard() (memory, bool) {
	m, ok := w.monthMemory(fmt.Sprintf("%02d", int(w.day.Month())))
	if !ok || len(m.picks) < 2 {
		return memory{}, false
	}
	return m, true
}

// monthMemory is one month ("01"…"12") in the years before this one, a
// pick for each year.
func (w *world) monthMemory(key string) (memory, bool) {
	month, err := strconv.Atoi(key)
	if err != nil || month < 1 || month > 12 || len(key) != 2 {
		return memory{}, false
	}
	thisYear := strconv.Itoa(w.day.Year())
	photos := w.where(func(p library.MemoryPhoto) bool {
		return len(p.Taken) >= 7 && p.Taken[5:7] == key && p.Taken[:4] < thisYear
	})
	if len(photos) == 0 {
		return memory{}, false
	}
	card := Card{Kind: "month", Key: key, Month: month}
	return memory{card: card, photos: photos, picks: w.picksPerYear("month", key, byYear(photos))}, true
}

// where keeps the photos that match, in time order.
func (w *world) where(match func(library.MemoryPhoto) bool) []library.MemoryPhoto {
	out := []library.MemoryPhoto{}
	for _, p := range w.all {
		if match(p) {
			out = append(out, p)
		}
	}
	return out
}

// byYear groups photos by the year they were taken.
func byYear(photos []library.MemoryPhoto) map[int][]library.MemoryPhoto {
	out := map[int][]library.MemoryPhoto{}
	for _, p := range photos {
		if y, err := strconv.Atoi(p.Taken[:min(4, len(p.Taken))]); err == nil {
			out[y] = append(out[y], p)
		}
	}
	return out
}

// picksPerYear takes the best photo of some of the years, best first: as
// many years as the memory shows photos, the day's choice of them — not
// every year in a row.
func (w *world) picksPerYear(kind, key string, years map[int][]library.MemoryPhoto) []library.MemoryPhoto {
	chosen := sortedDesc(keysOf(years))
	seed := w.seed(kind, key)
	sort.SliceStable(chosen, func(i, j int) bool {
		return unit(seed+"|"+strconv.Itoa(chosen[i])) < unit(seed+"|"+strconv.Itoa(chosen[j]))
	})
	var picks []library.MemoryPhoto
	for _, y := range chosen[:min(len(chosen), w.picksFor(kind, key))] {
		picks = append(picks, Pick(years[y], w.rarity, seed, 1)...)
	}
	sort.SliceStable(picks, func(i, j int) bool {
		return Score(picks[i], w.rarity, seed) > Score(picks[j], w.rarity, seed)
	})
	return picks
}

func keysOf(m map[int][]library.MemoryPhoto) map[int]bool {
	out := map[int]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}

func sortedDesc(set map[int]bool) []int {
	out := make([]int, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}
