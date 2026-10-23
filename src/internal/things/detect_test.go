package things

import "testing"

// One row per grid cell of every stride, all empty but one dog in the cell
// (2,3) of stride 32: centred there, 64 px square.
func TestDecodeBoxesPlacesOneDog(t *testing.T) {
	width := 5 + len(cocoClasses)
	rows := 0
	for _, s := range detectStrides {
		rows += (detectSize / s) * (detectSize / s)
	}
	data := make([]float32, rows*width)
	cell := (detectSize/8)*(detectSize/8) + (detectSize/16)*(detectSize/16) + 3*(detectSize/32) + 2
	r := data[cell*width : (cell+1)*width]
	r[0], r[1], r[2], r[3], r[4] = 0.5, 0.5, 0.6931472, 0.6931472, 0.9 // exp(ln 2)·32 = 64
	r[5+16] = 0.8                                                      // dog
	boxes := decodeBoxes(data, 640, 320)
	if len(boxes) != 1 || boxes[0].Class != "dog" {
		t.Fatalf("got %+v", boxes)
	}
	b := boxes[0]
	// centre (2.5·32, 3.5·32) = (80, 112), 64 wide: x 48–112 of 640, y 80–144 of 320
	if !near(b.X, 48.0/640) || !near(b.Y, 80.0/320) || !near(b.W, 64.0/640) || !near(b.H, 64.0/320) {
		t.Fatalf("box %+v", b)
	}
}

func TestSuppressOverlapsKeepsOtherClasses(t *testing.T) {
	kept := suppressOverlaps([]Box{
		{Class: "dog", Score: 0.6, X: 0.1, Y: 0.1, W: 0.5, H: 0.5},
		{Class: "dog", Score: 0.9, X: 0.12, Y: 0.1, W: 0.5, H: 0.5},
		{Class: "cat", Score: 0.5, X: 0.1, Y: 0.1, W: 0.5, H: 0.5},
	})
	if len(kept) != 2 || kept[0].Score != 0.9 || kept[1].Class != "cat" {
		t.Fatalf("got %+v", kept)
	}
}

func near(a, b float64) bool { return a-b < 1e-4 && b-a < 1e-4 }
