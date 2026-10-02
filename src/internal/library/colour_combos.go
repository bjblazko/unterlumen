package library

import (
	"sort"
)

// ComboShareMin is how much of the frame a hue must cover, over all of a
// photo's swatches of that hue, for the photo to count as having it in a
// colour combination — orange and teal, say (ADR-0049). Lower than
// HueShareMin, since two or three colours share one frame.
const ComboShareMin = 0.10

// HueSet counts the colour photos whose hues of at least ComboShareMin are
// exactly Hues. Any combination's photos are the sum over the sets that
// contain it, so the browser answers every choice without asking again.
type HueSet struct {
	Hues   []int `json:"hues"` // hue bins, ascending
	Photos int   `json:"photos"`
}

// hueSets groups the colour photos by the set of hues they have. Swatches
// read from two libraries that hold the same photo count once.
func hueSets(swatches []ColourSwatch) []HueSet {
	byPhoto := map[string]*[12]float64{}
	seen := map[ColourSwatch]bool{}
	for _, sw := range swatches {
		if sw.HueBin < 0 || sw.HueBin > 11 || seen[sw] {
			continue
		}
		seen[sw] = true
		shares := byPhoto[sw.PhotoID]
		if shares == nil {
			shares = &[12]float64{}
			byPhoto[sw.PhotoID] = shares
		}
		shares[sw.HueBin] += sw.Share
	}
	counts := map[uint16]int{}
	for _, shares := range byPhoto {
		var mask uint16
		for bin, share := range shares {
			if share >= ComboShareMin {
				mask |= 1 << bin
			}
		}
		if mask != 0 {
			counts[mask]++
		}
	}
	out := make([]HueSet, 0, len(counts))
	for mask, n := range counts {
		out = append(out, HueSet{Hues: binsOf(mask), Photos: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Photos != out[j].Photos {
			return out[i].Photos > out[j].Photos
		}
		return lessBins(out[i].Hues, out[j].Hues)
	})
	return out
}

func binsOf(mask uint16) []int {
	var bins []int
	for bin := 0; bin < 12; bin++ {
		if mask&(1<<bin) != 0 {
			bins = append(bins, bin)
		}
	}
	return bins
}

func lessBins(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}
