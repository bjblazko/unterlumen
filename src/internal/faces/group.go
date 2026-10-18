package faces

// GroupThreshold is how close a face must be to the average of a group to
// join it. It is a little stricter than SFace's line between two single
// faces, since a wrong face in a group misleads more than a group split in two.
const GroupThreshold = 0.42

// SuggestThreshold is how close a group must be to a named person for
// Unterlumen to ask whether it is that person.
const SuggestThreshold = 0.45

// Group sorts faces that look alike into groups and returns each face's
// group, numbered from 0. Faces earlier in embs found groups first, so the
// clearest faces should come first.
func Group(embs [][]float32) []int {
	var sums [][]float32
	assign := make([]int, len(embs))
	for i, e := range embs {
		g, sim := nearest(sums, e)
		if g < 0 || sim < GroupThreshold {
			sums = append(sums, append([]float32(nil), e...))
			assign[i] = len(sums) - 1
			continue
		}
		add(sums[g], e)
		assign[i] = g
	}
	// One more round against the finished groups undoes most of what the
	// order decided: a face that founded a group early may fit a later one better.
	means := make([][]float32, len(sums))
	for g, s := range sums {
		means[g] = Mean([][]float32{s})
	}
	for i, e := range embs {
		if g, sim := nearest(means, e); sim >= GroupThreshold {
			assign[i] = g
		}
	}
	return assign
}

// Mean is the unit-length average of embeddings.
func Mean(embs [][]float32) []float32 {
	if len(embs) == 0 {
		return nil
	}
	m := make([]float32, len(embs[0]))
	for _, e := range embs {
		add(m, e)
	}
	return normalise(m)
}

// Nearest returns which of the unit-length candidates is closest to e, and
// how close; -1 when there are none.
func Nearest(candidates [][]float32, e []float32) (int, float32) {
	return nearest(candidates, e)
}

// nearest compares e with sums, which need not have unit length.
func nearest(sums [][]float32, e []float32) (int, float32) {
	best, bestSim := -1, float32(-2)
	for g, s := range sums {
		if sim := Similarity(s, e) / length(s); sim > bestSim {
			best, bestSim = g, sim
		}
	}
	return best, bestSim
}

func add(sum, e []float32) {
	for i := range sum {
		sum[i] += e[i]
	}
}

func length(v []float32) float32 {
	return float32(sqrt(float64(Similarity(v, v))))
}
