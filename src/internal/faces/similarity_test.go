package faces

import (
	"math/rand/v2"
	"testing"
)

// A person seen from the front and in profile: a new profile is measured
// against their profiles, where the average of all their faces lies between
// the two and looks like neither.
func TestResemblanceFindsTheNearestFacesOfAPerson(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	front, profile := person(r, 20, 0.4), person(r, 6, 0.4)
	examples := append(append([][]float32{}, front...), profile[1:]...)
	byAverage := Similarity(Mean(examples), profile[0])
	byNearest := Resemblance(profile[0], examples)
	if byNearest <= byAverage || byNearest < SuggestThreshold {
		t.Fatalf("nearest %.2f, average %.2f: want the nearest above the threshold and the average", byNearest, byAverage)
	}
	if Resemblance(profile[0], nil) != -1 {
		t.Error("no examples should resemble nothing")
	}
	one := Resemblance(profile[0], profile[1:2])
	if one != Similarity(profile[0], profile[1]) {
		t.Errorf("with one example it is that example's similarity, got %.3f", one)
	}
}
