package things

import "sort"

// How a photo's vector is turned into things. SigLIP gives each word its own
// probability of fitting the photo, so a thing is taken when its probability
// is high enough, whatever the other words score: a photo of nothing in
// particular has no thing likely, and gets none.
const (
	defaultMin     = 0.01 // the probability a thing needs, unless it says otherwise
	nearBest       = 10   // and it needs a tenth of the photo's best thing
	mostThings     = 5    // a photo has at most this many things from SigLIP
	detectorMin    = 0.7  // a box this sure names its thing by itself
	detectorMinBox = 0.01 // and must cover this much of the photo
)

// Found is one thing in a photo and how sure Unterlumen is of it: SigLIP's
// probability or the detector's score.
type Found struct {
	Name   string  `json:"name"`
	Score  float32 `json:"score"`
	Source string  `json:"source"` // "siglip", "detector", or "sidecar" for one only the sidecar names
}

// Score tells the things of a photo from its vector, compared with the
// vocabulary's vectors (VocabularyVectors), and its boxes.
func Score(vector []float32, words [][]float32, boxes []Box) []Found {
	found := fromSigLIP(vector, words, boxes)
	have := map[string]bool{}
	for _, f := range found {
		have[f.Name] = true
	}
	for _, f := range fromDetector(boxes) {
		if !have[f.Name] {
			found, have[f.Name] = append(found, f), true
		}
	}
	return found
}

func fromSigLIP(vector []float32, words [][]float32, boxes []Box) []Found {
	var all []Found
	for i, w := range words {
		all = append(all, Found{Name: Vocabulary[i].Name, Score: Probability(Similarity(vector, w)), Source: "siglip"})
	}
	sort.Slice(all, func(a, b int) bool { return all[a].Score > all[b].Score })
	if len(all) == 0 {
		return nil
	}
	// A photo whose best thing is its quality shows nothing clearly: only
	// its quality is kept.
	onlyQuality := CategoryOf(all[0].Name) == Quality
	var found []Found
	for _, f := range all {
		t := Vocabulary[indexOf[f.Name]]
		if len(found) == mostThings || f.Score < t.min() || f.Score < all[0].Score/nearBest {
			continue
		}
		if onlyQuality && t.Category != Quality {
			continue
		}
		found = append(found, f)
	}
	return found
}

func fromDetector(boxes []Box) []Found {
	var found []Found
	for _, b := range boxes {
		if b.Score < detectorMin || b.W*b.H < detectorMinBox {
			continue
		}
		if t, ok := byCoco[b.Class]; ok {
			found = append(found, Found{Name: t.Name, Score: b.Score, Source: "detector"})
		}
	}
	return found
}

func (t Thing) min() float32 {
	if t.Min > 0 {
		return t.Min
	}
	return defaultMin
}

var indexOf = func() map[string]int {
	m := map[string]int{}
	for i, t := range Vocabulary {
		m[t.Name] = i
	}
	return m
}()

var byCoco = func() map[string]Thing {
	m := map[string]Thing{}
	for _, t := range Vocabulary {
		if t.Coco != "" {
			m[t.Coco] = t
		}
	}
	return m
}()
