package library

import (
	"math"
	"testing"
)

// Two clusters apart along one dimension land apart on the first axis.
func TestPrincipalAxesSeparateClusters(t *testing.T) {
	var vecs [][]float32
	for i := 0; i < 40; i++ {
		v := make([]float32, 16)
		for j := range v {
			v[j] = float32(math.Sin(float64(i*j))) * 0.05
		}
		if i%2 == 0 {
			v[3] += 1
		} else {
			v[3] -= 1
		}
		vecs = append(vecs, v)
	}
	out := principalAxes(vecs)
	side := out[0][0] > 0
	for i, p := range out {
		if (p[0] > 0) != (side == (i%2 == 0)) {
			t.Errorf("photo %d is on the wrong side of the first axis: %v", i, p)
		}
		for _, x := range p {
			if x < -1 || x > 1 {
				t.Errorf("outside the unit box: %v", p)
			}
		}
	}
}
