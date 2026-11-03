package memories

import (
	"sort"
	"strconv"
	"time"

	"huepattl.de/unterlumen/internal/birthdays"
	"huepattl.de/unterlumen/internal/library"
)

const (
	birthdayDays = 7 // how far ahead a birthday brings someone back
	personYears  = 3 // the years with photos someone needs to come back without one
	yearsShown   = 5 // the newest years shown of someone without a birthday
)

// personCard is someone whose birthday is in the next days, the soonest;
// otherwise someone with photos in several years, of the day's choosing.
func (w *world) personCard() (memory, bool) {
	years := w.peopleYears()
	if name, ok := w.nextBirthday(years); ok {
		return w.personMemory(name)
	}
	var many []string
	for name, ys := range years {
		if len(ys) >= personYears {
			many = append(many, name)
		}
	}
	if len(many) == 0 {
		return memory{}, false
	}
	sort.Strings(many)
	return w.personMemory(many[choose(w.today()+"|person", len(many))])
}

// peopleYears is the years each named person has photos in.
func (w *world) peopleYears() map[string]map[int]bool {
	out := map[string]map[int]bool{}
	for _, p := range w.all {
		y, err := strconv.Atoi(p.Taken[:min(4, len(p.Taken))])
		if err != nil {
			continue
		}
		for _, name := range p.People {
			if out[name] == nil {
				out[name] = map[int]bool{}
			}
			out[name][y] = true
		}
	}
	return out
}

func (w *world) nextBirthday(people map[string]map[int]bool) (string, bool) {
	best, bestIn := "", birthdayDays+1
	for name := range people {
		next, ok := w.birthdayOf(name)
		if !ok {
			continue
		}
		in := int(next.Sub(w.day).Hours() / 24)
		if in < bestIn || (in == bestIn && name < best) {
			best, bestIn = name, in
		}
	}
	return best, best != ""
}

// birthdayOf is name's next birthday from today, when it is within
// birthdayDays.
func (w *world) birthdayOf(name string) (time.Time, bool) {
	birth, ok := birthdays.Of(w.births, name)
	if !ok {
		return time.Time{}, false
	}
	next := time.Date(w.day.Year(), birth.Month(), birth.Day(), 0, 0, 0, 0, time.UTC)
	if next.Before(w.day) {
		next = next.AddDate(1, 0, 0)
	}
	if next.Sub(w.day).Hours()/24 > birthdayDays || next.Year() <= birth.Year() {
		return time.Time{}, false
	}
	return next, true
}

// personMemory is a person's photos, a pick for each year: since they were
// born when their birthday is near, else their newest years.
func (w *world) personMemory(name string) (memory, bool) {
	years := byYear(w.where(func(p library.MemoryPhoto) bool { return hasPerson(p, name) }))
	if len(years) == 0 {
		return memory{}, false
	}
	card := Card{Kind: "person", Key: name, Name: name}
	if next, ok := w.birthdayOf(name); ok {
		birth, _ := birthdays.Of(w.births, name)
		age := birthdays.AgeAt(birth, next)
		card.Birthday, card.Age = next.Format(dayLayout), &age
		keepYears(years, func(y int) bool { return y >= birth.Year() })
	} else {
		newest := sortedDesc(keysOf(years))
		keepYears(years, func(y int) bool { return len(newest) <= yearsShown || y >= newest[yearsShown-1] })
	}
	var photos []library.MemoryPhoto
	for _, ps := range years {
		photos = append(photos, ps...)
	}
	sort.SliceStable(photos, func(i, j int) bool { return photos[i].Taken < photos[j].Taken })
	return memory{card: card, photos: photos, picks: w.picksPerYear("person", name, years)}, len(photos) > 0
}

func keepYears(years map[int][]library.MemoryPhoto, keep func(int) bool) {
	for y := range years {
		if !keep(y) {
			delete(years, y)
		}
	}
}

func hasPerson(p library.MemoryPhoto, name string) bool {
	for _, n := range p.People {
		if n == name {
			return true
		}
	}
	return false
}
