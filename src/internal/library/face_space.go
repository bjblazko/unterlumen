package library

import (
	"math"
	"sort"
)

// The Face space places every face by what it looks like. A face print has
// 128 numbers; the three directions in which the named people differ most
// (the principal axes of their average faces) become the stage's three
// axes, so people stand apart and the faces of one person close together.
// With fewer than four people named, the axes come from the faces
// themselves.

const minPeopleForAxes = 4

// faceAxes returns the middle and the three principal axes of vectors.
func faceAxes(vectors [][]float32) (mean []float64, axes [3][]float64) {
	d := len(vectors[0])
	mean = make([]float64, d)
	for _, v := range vectors {
		for i, x := range v {
			mean[i] += float64(x)
		}
	}
	for i := range mean {
		mean[i] /= float64(len(vectors))
	}
	cov := make([][]float64, d)
	for i := range cov {
		cov[i] = make([]float64, d)
	}
	centred := make([]float64, d)
	for _, v := range vectors {
		for i, x := range v {
			centred[i] = float64(x) - mean[i]
		}
		for i := 0; i < d; i++ {
			ci := centred[i]
			row := cov[i]
			for j := i; j < d; j++ {
				row[j] += ci * centred[j]
			}
		}
	}
	for i := 0; i < d; i++ {
		for j := 0; j < i; j++ {
			cov[i][j] = cov[j][i]
		}
	}
	for k := range axes {
		axes[k] = largestEigenvector(cov)
		deflate(cov, axes[k])
	}
	return mean, axes
}

// largestEigenvector finds it by power iteration from a fixed start, so the
// same faces always give the same picture.
func largestEigenvector(m [][]float64) []float64 {
	d := len(m)
	v := make([]float64, d)
	for i := range v {
		v[i] = 1 / math.Sqrt(float64(d)) * (1 + float64(i%7)/10)
	}
	next := make([]float64, d)
	for range 200 {
		for i := range next {
			s := 0.0
			for j, x := range m[i] {
				s += x * v[j]
			}
			next[i] = s
		}
		n := norm(next)
		if n == 0 {
			return v
		}
		for i := range v {
			v[i] = next[i] / n
		}
	}
	return v
}

// deflate takes the axis v and its share out of m.
func deflate(m [][]float64, v []float64) {
	lambda := 0.0
	for i := range m {
		s := 0.0
		for j, x := range m[i] {
			s += x * v[j]
		}
		lambda += v[i] * s
	}
	for i := range m {
		for j := range m[i] {
			m[i][j] -= lambda * v[i] * v[j]
		}
	}
}

func norm(v []float64) float64 {
	s := 0.0
	for _, x := range v {
		s += x * x
	}
	return math.Sqrt(s)
}

// projectFaces places each print on the axes, scaled so that all but the
// farthest few faces lie within −1 and 1 on each axis.
func projectFaces(prints [][]float32, mean []float64, axes [3][]float64) [][3]float64 {
	out := make([][3]float64, len(prints))
	for n, p := range prints {
		for k, axis := range axes {
			s := 0.0
			for i, x := range p {
				s += (float64(x) - mean[i]) * axis[i]
			}
			out[n][k] = s
		}
	}
	for k := range 3 {
		mags := make([]float64, len(out))
		for n := range out {
			mags[n] = math.Abs(out[n][k])
		}
		sort.Float64s(mags)
		scale := mags[min(len(mags)-1, len(mags)*98/100)]
		if scale == 0 {
			continue
		}
		for n := range out {
			out[n][k] = math.Max(-1.3, math.Min(1.3, out[n][k]/scale))
		}
	}
	return out
}
