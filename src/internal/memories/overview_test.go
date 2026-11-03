package memories

import (
	"fmt"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/library"
)

func date(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

// series is n photos on a day, an hour apart from 09:00.
func series(prefix, day string, n int) []library.MemoryPhoto {
	var out []library.MemoryPhoto
	for i := 0; i < n; i++ {
		out = append(out, photoAt(fmt.Sprintf("%s-%d", prefix, i), fmt.Sprintf("%sT%02d:00:00", day, 9+i)))
	}
	return out
}

func joined(sets ...[]library.MemoryPhoto) []library.MemoryPhoto {
	var out []library.MemoryPhoto
	for _, s := range sets {
		out = append(out, s...)
	}
	return out
}

func TestOnThisDay(t *testing.T) {
	all := joined(series("a", "2022-11-03", 8), series("b", "2025-11-03", 2), series("now", "2026-11-03", 3), series("other", "2025-11-04", 3))
	o := Build(date("2026-11-03"), all, nil)
	if len(o.Years) != 2 {
		t.Fatalf("years = %+v; want 2025 and 2022", o.Years)
	}
	if y := o.Years[0]; y.Year != 2025 || y.Ago != 1 || y.Date != "2025-11-03" || y.Total != 2 || len(y.Picks) != 2 {
		t.Errorf("first year = %+v", y)
	}
	if y := o.Years[1]; y.Year != 2022 || y.Ago != 4 || y.Total != 8 || len(y.Picks) != 5 {
		t.Errorf("second year = %+v", y)
	}
	for i := 1; i < len(o.Years[1].Picks); i++ {
		if o.Years[1].Picks[i-1].Taken > o.Years[1].Picks[i].Taken {
			t.Errorf("picks not in time order: %+v", o.Years[1].Picks)
		}
	}
	if o.Day != "2026-11-03" {
		t.Errorf("day = %q", o.Day)
	}
}

func TestOnThisDayLeapDay(t *testing.T) {
	all := joined(series("leap", "2024-02-29", 1), series("eve", "2024-02-28", 1), series("plain", "2023-02-28", 1))
	o := Build(date("2025-02-28"), all, nil)
	if len(o.Years) != 2 || o.Years[0].Total != 2 || o.Years[1].Total != 1 {
		t.Fatalf("years = %+v; want 2024 with both days, 2023 with one", o.Years)
	}
	d, ok := BuildDetail(date("2025-02-28"), "day", o.Years[0].Date, all, nil)
	if !ok || len(d.Photos) != 2 {
		t.Errorf("detail of %s = %+v, %v; want both days", o.Years[0].Date, d.Photos, ok)
	}
	if o := Build(date("2024-02-28"), joined(series("x", "2020-02-29", 1)), nil); len(o.Years) != 0 {
		t.Errorf("in a leap year 28 February shows 29 February: %+v", o.Years)
	}
}

// trip is photos on consecutive days at a place.
func trip(prefix, start string, days int, lat, lon float64, country string) []library.MemoryPhoto {
	var out []library.MemoryPhoto
	for d := 0; d < days; d++ {
		day := date(start).AddDate(0, 0, d).Format("2006-01-02")
		for _, p := range series(fmt.Sprintf("%s%d", prefix, d), day, 2) {
			p.Lat, p.Lon, p.Located, p.Country = lat, lon, true, country
			out = append(out, p)
		}
	}
	return out
}

func homePhotos(n int) []library.MemoryPhoto {
	return trip("home", "2019-01-01", n, 52.52, 13.40, "Germany")
}

func TestFindTripsMatchesStatistics(t *testing.T) {
	// Lisbon on 3 days, a hop of 3 days later (joined), then 4 days later (split).
	all := joined(homePhotos(20),
		trip("l", "2025-10-01", 3, 38.72, -9.14, "Portugal"),
		trip("m", "2025-10-06", 1, 38.72, -9.14, "Portugal"), // 10-03 → 10-06: 3 days, one trip
		trip("r", "2025-10-10", 2, 41.90, 12.50, "Italy"),    // 10-06 → 10-10: 4 days, another
	)
	trips := FindTrips(all, FindHome(all))
	if len(trips) != 2 {
		t.Fatalf("trips = %+v; want 2", trips)
	}
	if tr := trips[0]; tr.Start != "2025-10-01" || tr.End != "2025-10-06" || tr.Country != "Portugal" || tr.Photos != 8 {
		t.Errorf("first trip = %+v", tr)
	}
	if tr := trips[1]; tr.Start != "2025-10-10" || tr.End != "2025-10-11" || tr.Country != "Italy" {
		t.Errorf("second trip = %+v", tr)
	}
}

func cardOf(o Overview, kind string) (Card, bool) {
	for _, c := range o.Cards {
		if c.Kind == kind {
			return c, true
		}
	}
	return Card{}, false
}

func TestTripCardPrefersAnniversary(t *testing.T) {
	all := joined(homePhotos(20),
		trip("old", "2020-06-01", 2, 41.90, 12.50, "Italy"),
		trip("anniv", "2023-11-08", 4, 38.72, -9.14, "Portugal"),
		trip("other", "2024-03-01", 2, 48.85, 2.35, "France"),
	)
	for i := 0; i < 10; i++ { // every day within ±7 days gives the anniversary
		day := date("2026-11-01").AddDate(0, 0, i)
		c, ok := cardOf(Build(day, all, nil), "trip")
		if !ok || c.Key != "2023-11-08" || c.Country != "Portugal" || c.Days != 4 || c.Total != 8 {
			t.Errorf("%s: trip card = %+v, %v; want Portugal 2023-11-08", day.Format("2006-01-02"), c, ok)
		}
	}
	if c, ok := cardOf(Build(date("2026-08-01"), all, nil), "trip"); !ok || c.Key == "" {
		t.Errorf("without an anniversary there is still a trip: %+v, %v", c, ok)
	}
}

func named(ps []library.MemoryPhoto, names ...string) []library.MemoryPhoto {
	for i := range ps {
		ps[i].People = names
	}
	return ps
}

func TestPersonBirthdayFirst(t *testing.T) {
	all := joined(
		named(series("anna18", "2018-05-01", 2), "Anna"), named(series("anna22", "2022-05-01", 2), "Anna"),
		named(series("ben1", "2015-01-01", 1), "Ben"), named(series("ben2", "2016-01-01", 1), "Ben"), named(series("ben3", "2017-01-01", 1), "Ben"),
	)
	births := map[string]string{"anna": "2018-11-06"}
	c, ok := cardOf(Build(date("2026-11-03"), all, births), "person")
	if !ok || c.Name != "Anna" || c.Birthday != "2026-11-06" || c.Age == nil || *c.Age != 8 {
		t.Fatalf("person card = %+v; want Anna turning 8 on 2026-11-06", c)
	}
	d, ok := BuildDetail(date("2026-11-03"), "person", "Anna", all, births)
	if !ok || len(d.Picks) != 2 || len(d.Photos) != 4 {
		t.Errorf("detail = %d picks, %d photos, %v; want one pick per year, all 4 photos", len(d.Picks), len(d.Photos), ok)
	}
	if c, _ := cardOf(Build(date("2026-10-01"), all, births), "person"); c.Name != "Ben" || c.Birthday != "" {
		t.Errorf("without a birthday soon: %+v; want Ben over the years", c)
	}
}

func TestPersonNeedsThreeYears(t *testing.T) {
	all := joined(named(series("a", "2018-05-01", 2), "Anna"), named(series("b", "2022-05-01", 2), "Anna"))
	if c, ok := cardOf(Build(date("2026-10-01"), all, nil), "person"); ok {
		t.Errorf("a person in two years has a card: %+v", c)
	}
}

func TestMonthCard(t *testing.T) {
	all := joined(series("a", "2020-11-20", 3), series("b", "2023-11-02", 2), series("c", "2023-10-02", 2), series("d", "2026-11-01", 1))
	c, ok := cardOf(Build(date("2026-11-03"), all, nil), "month")
	if !ok || c.Key != "11" || c.Month != 11 || c.Total != 5 {
		t.Fatalf("month card = %+v, %v", c, ok)
	}
	d, _ := BuildDetail(date("2026-11-03"), "month", "11", all, nil)
	if len(d.Picks) != 2 {
		t.Errorf("month picks = %d; want one per year", len(d.Picks))
	}
	if _, ok := cardOf(Build(date("2026-11-03"), series("a", "2020-11-20", 3), nil), "month"); ok {
		t.Error("a month card from one year only")
	}
}

func rareLibrary() []library.MemoryPhoto {
	var all []library.MemoryPhoto
	for i := 0; i < 400; i++ {
		p := photoAt(fmt.Sprintf("p%03d", i), date("2015-01-01").AddDate(0, 0, i*3).Format("2006-01-02")+"T10:00:00")
		p.Things = []string{"tree"}
		if i%10 == 0 {
			p.Things = []string{"tree", fmt.Sprintf("rare%d", i)}
		}
		all = append(all, p)
	}
	return all
}

func TestRediscoveredRotatesAndSkipsShown(t *testing.T) {
	all := rareLibrary()
	a, _ := cardOf(Build(date("2026-11-03"), all, nil), "rediscovered")
	b, _ := cardOf(Build(date("2026-11-04"), all, nil), "rediscovered")
	if a.Cover.ID == "" || a.Cover.ID == b.Cover.ID {
		t.Errorf("covers on two days: %q, %q; want two different", a.Cover.ID, b.Cover.ID)
	}
	o := Build(date("2026-11-03"), all, nil)
	shown := map[string]bool{}
	for _, y := range o.Years {
		for _, p := range y.Picks {
			shown[p.ID] = true
		}
	}
	d, ok := BuildDetail(date("2026-11-03"), "rediscovered", "2026-11-03", all, nil)
	if !ok || len(d.Picks) != 5 {
		t.Fatalf("rediscovered = %d picks, %v", len(d.Picks), ok)
	}
	for _, p := range d.Picks {
		if shown[p.ID] {
			t.Errorf("%s is shown twice", p.ID)
		}
	}
}

func TestOverviewWithoutPlacesPeopleOrThings(t *testing.T) {
	o := Build(date("2026-11-03"), joined(series("a", "2022-11-03", 3)), nil)
	if len(o.Years) != 1 {
		t.Errorf("years = %+v", o.Years)
	}
	for _, kind := range []string{"trip", "person", "month", "rediscovered"} {
		if c, ok := cardOf(o, kind); ok {
			t.Errorf("a %s card without anything to show: %+v", kind, c)
		}
	}
	if o := Build(date("2026-11-03"), nil, nil); o.Years == nil || o.Cards == nil {
		t.Errorf("empty overview has nil lists: %+v", o)
	}
}

func TestOverviewCountsMatchDetail(t *testing.T) {
	all := joined(homePhotos(20), trip("anniv", "2023-11-08", 4, 38.72, -9.14, "Portugal"),
		named(series("b1", "2015-11-01", 1), "Ben"), named(series("b2", "2016-11-02", 1), "Ben"), named(series("b3", "2017-01-01", 1), "Ben"),
		series("d", "2021-11-03", 4), rareLibrary())
	shot := photoAt("shot", "2021-11-03T20:00:00")
	shot.Things = []string{"screenshot"}
	all = append(all, shot)
	day := date("2026-11-03")
	o := Build(day, all, nil)
	if len(o.Cards) != 4 {
		t.Errorf("cards = %+v; want all four", o.Cards)
	}
	for _, y := range o.Years {
		d, ok := BuildDetail(day, "day", y.Date, all, nil)
		if !ok || y.Total != len(d.Photos) {
			t.Errorf("day %s: %d in the overview, %d in the detail", y.Date, y.Total, len(d.Photos))
		}
		for _, p := range d.Photos {
			if p.ID == "shot" {
				t.Error("the screenshot is among the day's photos")
			}
		}
	}
	for _, c := range o.Cards {
		d, ok := BuildDetail(day, c.Kind, c.Key, all, nil)
		if !ok || c.Total != len(d.Photos) {
			t.Errorf("%s %s: total %d, detail %d photos (%v)", c.Kind, c.Key, c.Total, len(d.Photos), ok)
		}
	}
}

func TestDetailUnknownKey(t *testing.T) {
	all := series("a", "2022-11-03", 3)
	for _, c := range [][2]string{{"trip", "1900-01-01"}, {"person", "Nobody"}, {"month", "13"}, {"day", "not-a-day"}, {"what", "x"}, {"rediscovered", "2026-11-03"}} {
		if _, ok := BuildDetail(date("2026-11-03"), c[0], c[1], all, nil); ok {
			t.Errorf("%s/%s exists", c[0], c[1])
		}
	}
}

// Two cards with one cover read as one memory twice: a card's cover is its
// best photo that no row and no card before it shows.
func TestCardsHaveTheirOwnCovers(t *testing.T) {
	all := joined(homePhotos(20), trip("anniv", "2023-11-08", 1, 38.72, -9.14, "Portugal"), series("nov", "2021-11-20", 1), series("day", "2024-11-03", 1))
	o := Build(date("2026-11-03"), all, nil)
	trip, okTrip := cardOf(o, "trip")
	month, okMonth := cardOf(o, "month")
	if !okTrip || !okMonth {
		t.Fatalf("cards = %+v; want a trip and a month", o.Cards)
	}
	if trip.Cover.ID == month.Cover.ID {
		t.Errorf("the trip and the month share the cover %s", trip.Cover.ID)
	}
	for _, c := range o.Cards {
		for _, y := range o.Years {
			for _, p := range y.Picks {
				if c.Cover.ID == p.ID {
					t.Errorf("%s's cover %s is a pick of %s", c.Kind, p.ID, y.Date)
				}
			}
		}
	}
}

// Before 1970 the day counts negative; the turn through the pool must not.
func TestRediscoveredBefore1970(t *testing.T) {
	all := rareLibrary()[:140]
	for _, d := range []string{"1969-12-30", "1900-01-01"} {
		Build(date(d), all, nil)
		BuildDetail(date(d), "rediscovered", d, all, nil)
	}
}

// Last week's trip is no anniversary, though it began in the year before.
func TestTripLastWeekIsNoAnniversary(t *testing.T) {
	all := joined(homePhotos(20), trip("new", "2025-12-28", 5, 38.72, -9.14, "Portugal"))
	c, ok := cardOf(Build(date("2026-01-03"), all, nil), "trip")
	if !ok || c.Anniversary {
		t.Errorf("trip card = %+v, %v; want the trip, not as an anniversary", c, ok)
	}
	c, _ = cardOf(Build(date("2026-12-30"), all, nil), "trip")
	if !c.Anniversary {
		t.Errorf("a year later: %+v; want an anniversary", c)
	}
}
