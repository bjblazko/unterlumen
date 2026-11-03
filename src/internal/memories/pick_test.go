package memories

import (
	"fmt"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/library"
)

// photoAt is a plain photo taken at an ISO time.
func photoAt(id, taken string) library.MemoryPhoto {
	return library.MemoryPhoto{ID: id, Filename: id + ".jpg", Taken: taken, HueBin: -1}
}

func TestEligibleLeavesOutScreenshotsAndFailures(t *testing.T) {
	for _, thing := range []string{"screenshot", "document", "blurred", "underexposed", "overexposed"} {
		p := photoAt("x", "2020-01-01T10:00:00")
		p.Things = []string{"sea", thing}
		if Eligible(p) {
			t.Errorf("a photo with %q is eligible", thing)
		}
	}
	p := photoAt("x", "2020-01-01T10:00:00")
	p.Things = []string{"sea", "dog"}
	if !Eligible(p) {
		t.Error("a photo of the sea and a dog is not eligible")
	}
}

func TestPickOneMomentOneBurst(t *testing.T) {
	var cands []library.MemoryPhoto
	start := time.Date(2020, 6, 1, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 300; i++ {
		cands = append(cands, photoAt(fmt.Sprintf("burst%03d", i), start.Add(time.Duration(i)*200*time.Millisecond).Format("2006-01-02T15:04:05")))
	}
	for i := 1; i <= 4; i++ {
		cands = append(cands, photoAt(fmt.Sprintf("later%d", i), start.Add(time.Duration(i)*3*time.Hour).Format("2006-01-02T15:04:05")))
	}
	picks := Pick(cands, NewRarity(cands, Home{}), "seed", 5)
	if len(picks) != 5 {
		t.Fatalf("got %d picks, want 5", len(picks))
	}
	bursts := 0
	for _, p := range picks {
		if p.ID[:5] == "burst" {
			bursts++
		}
	}
	if bursts != 1 {
		t.Errorf("%d picks from the burst, want 1", bursts)
	}
}

func TestPickPeopleFirst(t *testing.T) {
	rare := photoAt("rare", "2020-01-01T10:00:00")
	rare.HueBin = 9
	people := photoAt("people", "2020-01-01T10:01:00")
	people.People = []string{"Anna", "Ben"}
	people.HueBin = 1
	cands := []library.MemoryPhoto{rare, people}
	all := append([]library.MemoryPhoto{}, cands...)
	for i := 0; i < 50; i++ {
		common := photoAt(fmt.Sprintf("c%d", i), "2019-01-01T10:00:00")
		common.HueBin = 1
		all = append(all, common)
	}
	picks := Pick(cands, NewRarity(all, Home{}), "seed", 1)
	if len(picks) != 1 || picks[0].ID != "people" {
		t.Errorf("picks = %v, want the photo with people", ids(picks))
	}
}

func TestPickRareSecond(t *testing.T) {
	rare := photoAt("rare", "2020-01-01T10:00:00")
	rare.Things = []string{"owl"}
	common := photoAt("common", "2020-01-01T10:01:00")
	common.Things = []string{"tree"}
	all := []library.MemoryPhoto{rare, common}
	for i := 0; i < 998; i++ {
		p := photoAt(fmt.Sprintf("t%d", i), "2019-01-01T10:00:00")
		p.Things = []string{"tree"}
		all = append(all, p)
	}
	picks := Pick([]library.MemoryPhoto{rare, common}, NewRarity(all, Home{}), "seed", 1)
	if len(picks) != 1 || picks[0].ID != "rare" {
		t.Errorf("picks = %v, want the owl", ids(picks))
	}
}

func TestPickSkipsSameLookAcrossMoments(t *testing.T) {
	a := photoAt("a", "2020-01-01T10:00:00")
	b := photoAt("b", "2020-01-01T15:00:00")
	a.DHash, a.Hashed = 0, true
	for _, c := range []struct {
		hash uint64
		want int
	}{{0b111, 1}, {1<<20 - 1, 2}} {
		b.DHash, b.Hashed = c.hash, true
		cands := []library.MemoryPhoto{a, b}
		if got := len(Pick(cands, NewRarity(cands, Home{}), "seed", 5)); got != c.want {
			t.Errorf("distance %d: %d picks, want %d", appearanceDistance(a, b), got, c.want)
		}
	}
}

func TestPickIsDeterministicPerSeed(t *testing.T) {
	var cands []library.MemoryPhoto
	for i := 0; i < 10; i++ {
		cands = append(cands, photoAt(fmt.Sprintf("p%d", i), fmt.Sprintf("2020-01-%02dT10:00:00", i+1)))
	}
	r := NewRarity(cands, Home{})
	first := map[string]bool{}
	for day := 0; day < 30; day++ {
		seed := fmt.Sprintf("day%d", day)
		a, b := Pick(cands, r, seed, 1), Pick(cands, r, seed, 1)
		if a[0].ID != b[0].ID {
			t.Fatalf("seed %s: %s then %s", seed, a[0].ID, b[0].ID)
		}
		first[a[0].ID] = true
	}
	if len(first) < 2 {
		t.Errorf("30 seeds always picked %v; ties should rotate", first)
	}
}

func TestPickLeavesOutTheIneligible(t *testing.T) {
	shot := photoAt("shot", "2020-01-01T10:00:00")
	shot.Things = []string{"screenshot"}
	shot.People = []string{"Anna"}
	cands := []library.MemoryPhoto{shot, photoAt("ok", "2020-01-01T15:00:00")}
	picks := Pick(cands, NewRarity(cands, Home{}), "seed", 5)
	if len(picks) != 1 || picks[0].ID != "ok" {
		t.Errorf("picks = %v, want only ok", ids(picks))
	}
}

func ids(photos []library.MemoryPhoto) []string {
	out := make([]string, len(photos))
	for i, p := range photos {
		out[i] = p.ID
	}
	return out
}
