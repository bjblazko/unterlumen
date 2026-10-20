package apipeople

import (
	"image"
	"testing"

	lib "huepattl.de/unterlumen/internal/library"
)

func TestAFaceAtTheEdgeIsCutFromInsideThePhoto(t *testing.T) {
	b := image.Rect(0, 0, 400, 300)
	for name, box := range map[string]lib.FaceBox{
		"middle":       {X: 0.4, Y: 0.4, W: 0.1, H: 0.1},
		"top left":     {X: 0, Y: 0, W: 0.2, H: 0.2},
		"bottom right": {X: 0.9, Y: 0.85, W: 0.1, H: 0.15},
		"larger":       {X: 0.1, Y: 0, W: 0.8, H: 1},
	} {
		r := faceSquare(b, box)
		if !r.In(b) {
			t.Errorf("%s: %v reaches outside the photo %v", name, r, b)
		}
		if d := r.Dx() - r.Dy(); d < -1 || d > 1 {
			t.Errorf("%s: %v is not square", name, r)
		}
	}
	// A face in the middle keeps its margin and stays centred.
	r := faceSquare(b, lib.FaceBox{X: 0.4, Y: 0.4, W: 0.1, H: 0.1})
	if c := (r.Min.X + r.Max.X) / 2; c < 179 || c > 181 {
		t.Errorf("centre x = %d, want 180", c)
	}
}
