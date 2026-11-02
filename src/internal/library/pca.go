package library

import "math"

// principalAxes are the three directions in which vectors spread most
// (principal component analysis by power iteration, each axis deflated from
// the next), and each vector's place along them. A vision model's vectors
// have hundreds of dimensions; three is what a view can show. The places
// are scaled so the spread of the first axis is about one.
func principalAxes(vecs [][]float32) [][3]float32 {
	n := len(vecs)
	if n == 0 {
		return nil
	}
	d := len(vecs[0])
	mean := make([]float64, d)
	for _, v := range vecs {
		for j := 0; j < d && j < len(v); j++ {
			mean[j] += float64(v[j])
		}
	}
	for j := range mean {
		mean[j] /= float64(n)
	}
	var axes [][]float64
	for k := 0; k < 3; k++ {
		axes = append(axes, powerAxis(vecs, mean, axes))
	}
	out := make([][3]float32, n)
	var spread float64
	for i, v := range vecs {
		for k, ax := range axes {
			out[i][k] = float32(project(v, mean, ax))
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

func project(v []float32, mean, ax []float64) float64 {
	var s float64
	for j := range ax {
		if j < len(v) {
			s += (float64(v[j]) - mean[j]) * ax[j]
		}
	}
	return s
}

// powerAxis finds the direction of most spread that stands at right angles
// to the axes found before.
func powerAxis(vecs [][]float32, mean []float64, before [][]float64) []float64 {
	d := len(mean)
	ax := make([]float64, d)
	for j := range ax {
		ax[j] = math.Sin(float64(j*7+len(before)*3) + 1) // any start but zero
	}
	orthonormal(ax, before)
	for iter := 0; iter < 25; iter++ {
		next := make([]float64, d)
		for _, v := range vecs {
			p := project(v, mean, ax)
			for j := range next {
				if j < len(v) {
					next[j] += p * (float64(v[j]) - mean[j])
				}
			}
		}
		orthonormal(next, before)
		ax = next
	}
	return ax
}

// orthonormal takes from v its share along each axis before and scales it to
// length one.
func orthonormal(v []float64, before [][]float64) {
	for _, b := range before {
		var dot float64
		for j := range v {
			dot += v[j] * b[j]
		}
		for j := range v {
			v[j] -= dot * b[j]
		}
	}
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	norm = math.Sqrt(norm)
	if norm == 0 {
		return
	}
	for j := range v {
		v[j] /= norm
	}
}
