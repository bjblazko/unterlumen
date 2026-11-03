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

// More vectors than the axes are found from: every one is still placed,
// the clusters still apart, and the axis does not flip with the order.
func TestPrincipalAxesFromASample(t *testing.T) {
	n := axesSample + 1900
	var vecs [][]float32
	for i := 0; i < n; i++ {
		v := make([]float32, 32)
		for j := range v {
			v[j] = float32(math.Sin(float64(i*31+j*17))) * 0.05
		}
		if i%3 == 0 {
			v[5] -= 1
		} else {
			v[5] += 0.5
		}
		vecs = append(vecs, v)
	}
	out := principalAxes(vecs)
	if len(out) != n {
		t.Fatalf("%d places, want %d", len(out), n)
	}
	for i, p := range out {
		if (p[0] < 0) != (i%3 == 0) {
			t.Fatalf("photo %d is on the wrong side of the first axis: %v", i, p)
		}
	}
	reversed := make([][]float32, n)
	for i := range vecs {
		reversed[n-1-i] = vecs[i]
	}
	if back := principalAxes(reversed); (back[n-1][0] < 0) != (out[0][0] < 0) {
		t.Errorf("the first axis flips with the order of the photos")
	}
}
