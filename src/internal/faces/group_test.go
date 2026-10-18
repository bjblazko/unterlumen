package faces

import (
	"math/rand/v2"
	"testing"
)

// person makes n noisy faces around a random direction, as one person's
// faces lie around a point.
func person(r *rand.Rand, n int, noise float32) [][]float32 {
	centre := randomUnit(r)
	out := make([][]float32, n)
	for i := range out {
		v := make([]float32, len(centre))
		for j := range v {
			v[j] = centre[j] + noise*float32(r.NormFloat64())/11
		}
		out[i] = normalise(v)
	}
	return out
}

func randomUnit(r *rand.Rand) []float32 {
	v := make([]float32, 128)
	for i := range v {
		v[i] = float32(r.NormFloat64())
	}
	return normalise(v)
}

func TestGroupKeepsPeopleApart(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	a, b := person(r, 20, 0.8), person(r, 15, 0.8)
	got := Group(append(append([][]float32{}, a...), b...))
	for i := 1; i < 20; i++ {
		if got[i] != got[0] {
			t.Fatalf("face %d of the first person is in group %d, not %d", i, got[i], got[0])
		}
	}
	for i := 21; i < 35; i++ {
		if got[i] != got[20] {
			t.Fatalf("face %d of the second person is in group %d, not %d", i, got[i], got[20])
		}
	}
	if got[0] == got[20] {
		t.Fatal("two people share a group")
	}
}

func TestGroupLeavesStrangersAlone(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	strangers := [][]float32{randomUnit(r), randomUnit(r), randomUnit(r)}
	got := Group(strangers)
	if got[0] == got[1] || got[1] == got[2] || got[0] == got[2] {
		t.Fatalf("strangers grouped: %v", got)
	}
}

func TestNearestOfNoneIsMinusOne(t *testing.T) {
	if g, _ := Nearest(nil, []float32{1}); g != -1 {
		t.Fatalf("got %d", g)
	}
}
