package library

import "math"

// axesSample is how many vectors, taken evenly, the axes are found from;
// every vector is then placed along them. A few thousand show the
// directions of most spread as well as all of them do, at a tenth of the
// time (3 s for 33,000 photos before, ADR-0063).
const axesSample = 4096

// axesRounds is how often the axes are refined.
const axesRounds = 30

// principalAxes are the three directions in which vectors spread most
// (principal component analysis by orthogonal iteration: the three axes
// refined together, each kept at right angles to those before it), and
// each vector's place along them. A vision model's vectors have hundreds of
// dimensions; three is what a view can show. The places are scaled so the
// spread of the first axis is about one.
func principalAxes(vecs [][]float32) [][3]float32 {
	n := len(vecs)
	if n == 0 {
		return nil
	}
	d := len(vecs[0])
	fit := centred(evenly(vecs, axesSample), d)
	axes := threeAxes(fit.rows, d)
	out := make([][3]float32, n)
	x := make([]float64, d)
	var spread float64
	for i, v := range vecs {
		centre(x, v, fit.mean)
		for k := range axes {
			out[i][k] = float32(dot(x, axes[k]))
		}
		spread += float64(out[i][0] * out[i][0])
	}
	scale := float32(1 / math.Max(1e-9, 2*math.Sqrt(spread/float64(n))))
	for i := range out {
		for k := range out[i] {
			out[i][k] = clampUnit(out[i][k] * scale)
		}
	}
	return out
}

func clampUnit(v float32) float32 { return max(-1, min(1, v)) }

// evenly takes at most limit of vecs, spread over all of them.
func evenly(vecs [][]float32, limit int) [][]float32 {
	if len(vecs) <= limit {
		return vecs
	}
	out := make([][]float32, limit)
	for i := range out {
		out[i] = vecs[i*len(vecs)/limit]
	}
	return out
}

type centredRows struct {
	rows [][]float64 // each vector less the mean
	mean []float64
}

func centred(vecs [][]float32, d int) centredRows {
	mean := make([]float64, d)
	for _, v := range vecs {
		for j := 0; j < d && j < len(v); j++ {
			mean[j] += float64(v[j])
		}
	}
	for j := range mean {
		mean[j] /= float64(len(vecs))
	}
	rows := make([][]float64, len(vecs))
	for i, v := range vecs {
		rows[i] = make([]float64, d)
		centre(rows[i], v, mean)
	}
	return centredRows{rows, mean}
}

// centre writes v less the mean into x; a short v counts as zeros.
func centre(x []float64, v []float32, mean []float64) {
	for j := range x {
		if j < len(v) {
			x[j] = float64(v[j]) - mean[j]
		} else {
			x[j] = -mean[j]
		}
	}
}

func dot(a, b []float64) float64 {
	var s float64
	for j := range a {
		s += a[j] * b[j]
	}
	return s
}

// threeAxes refines three axes together: one pass over the rows per round
// multiplies each by the rows' spread, then they are set at right angles
// in order, so the first becomes the direction of most spread, the second
// the most at right angles to it, and so on.
func threeAxes(rows [][]float64, d int) [3][]float64 {
	var axes [3][]float64
	for k := range axes {
		axes[k] = make([]float64, d)
		for j := range axes[k] {
			axes[k][j] = math.Sin(float64(j*7+k*3) + 1) // any start but zero
		}
	}
	orthonormalAll(axes[:])
	for round := 0; round < axesRounds; round++ {
		var next [3][]float64
		for k := range next {
			next[k] = make([]float64, d)
		}
		for _, x := range rows {
			for k := range axes {
				p := dot(x, axes[k])
				nk := next[k]
				for j, xj := range x {
					nk[j] += p * xj
				}
			}
		}
		orthonormalAll(next[:])
		axes = next
	}
	for _, ax := range axes {
		pointUp(ax)
	}
	return axes
}

// pointUp turns an axis so its largest part is positive: an axis has no
// direction of its own, and a view should not flip when a photo is added.
func pointUp(ax []float64) {
	largest := 0
	for j := range ax {
		if math.Abs(ax[j]) > math.Abs(ax[largest]) {
			largest = j
		}
	}
	if ax[largest] < 0 {
		for j := range ax {
			ax[j] = -ax[j]
		}
	}
}

// orthonormalAll sets each axis at right angles to those before it, at
// length one (Gram–Schmidt).
func orthonormalAll(axes [][]float64) {
	for k := range axes {
		orthonormal(axes[k], axes[:k])
	}
}

// orthonormal takes from v its share along each axis before and scales it to
// length one.
func orthonormal(v []float64, before [][]float64) {
	for _, b := range before {
		dv := dot(v, b)
		for j := range v {
			v[j] -= dv * b[j]
		}
	}
	norm := math.Sqrt(dot(v, v))
	if norm == 0 {
		return
	}
	for j := range v {
		v[j] /= norm
	}
}
