package appearance

import (
	"image"
	"math/bits"

	"golang.org/x/image/draw"
)

// dHash is the difference hash: the image in grey at 9×8 pixels, one bit
// per pair of neighbours saying whether the left one is brighter. Resizing,
// recompressing or a slight change of exposure flip few bits.
func dHash(img *image.RGBA) uint64 {
	tiny := image.NewRGBA(image.Rect(0, 0, 9, 8))
	draw.BiLinear.Scale(tiny, tiny.Bounds(), img, img.Bounds(), draw.Src, nil)
	grey, _, _ := greyOf(tiny)
	var h uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			h <<= 1
			if grey[y*9+x] > grey[y*9+x+1] {
				h |= 1
			}
		}
	}
	return h
}

// HashDistance is the number of bits in which two hashes differ: up to
// about 10 of 64 the photos are near copies of each other.
func HashDistance(a, b uint64) int { return bits.OnesCount64(a ^ b) }
