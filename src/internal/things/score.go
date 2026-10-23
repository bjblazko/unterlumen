package things

import "sort"

// How a photo's vector is turned into things. CLIP's similarities of a photo
// to words lie close together, roughly 0.15–0.33, so a thing is taken only
// when it is above its own floor and near the photo's best word.
const (
	defaultFloor   = 0.25
	nearBest       = 0.02 // a thing further below the best word is not taken
	mostThings     = 5    // a photo has at most this many things from CLIP
	detectorMin    = 0.7  // a box this sure names its thing by itself
	detectorMinBox = 0.01 // and must cover this much of the photo
)

// Found is one thing in a photo and how sure Unterlumen is of it, from CLIP's
// similarity or the detector's score.
type Found struct {
	Name   string  `json:"name"`
	Score  float32 `json:"score"`
	Source string  `json:"source"` // "clip", "detector", or "sidecar" for one only the sidecar names
}

// Score tells the things of a photo from its vector, compared with the
// vocabulary's vectors (VocabularyVectors), and its boxes.
func Score(vector []float32, words [][]float32, boxes []Box) []Found {
	found := fromCLIP(vector, words, boxes)
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

func fromCLIP(vector []float32, words [][]float32, boxes []Box) []Found {
	sims := make([]float32, len(words))
	order := make([]int, len(words))
	for i, w := range words {
		sims[i], order[i] = Similarity(vector, w), i
	}
	sort.Slice(order, func(a, b int) bool { return sims[order[a]] > sims[order[b]] })
	best := sims[order[0]]
	var found []Found
	for _, i := range order {
		if len(found) == mostThings || sims[i] < best-nearBest {
			break
		}
		if t := Vocabulary[i]; t.Category != none && sims[i] >= t.floor() && t.confirmed(sims[i], boxes) {
			found = append(found, Found{Name: t.Name, Score: sims[i], Source: "clip"})
		}
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

func (t Thing) floor() float32 {
	if t.Floor > 0 {
		return t.Floor
	}
	return defaultFloor
}

// confirmed asks of an animal more than of other things: CLIP sees animals
// in fur-coloured blurs. One the detector knows needs its box; any other
// needs a clear similarity.
func (t Thing) confirmed(sim float32, boxes []Box) bool {
	if t.Category != Animals {
		return true
	}
	if t.Coco == "" {
		return sim >= animalFloor
	}
	for _, b := range boxes {
		if b.Class == t.Coco {
			return true
		}
	}
	return sim >= animalFloor
}

// animalFloor is the similarity an animal needs without a box.
const animalFloor = 0.28

var byCoco = func() map[string]Thing {
	m := map[string]Thing{}
	for _, t := range Vocabulary {
		if t.Coco != "" {
			m[t.Coco] = t
		}
	}
	return m
}()
