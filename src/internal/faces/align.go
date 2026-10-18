package faces

import (
	"image"
	"math"
)

// SFace takes a 112×112 face whose five landmarks sit where they sit in
// this template (OpenCV's FaceRecognizerSF.alignCrop, after ArcFace).
const alignSize = 112

var alignTemplate = [5]point{
	{38.2946, 51.6963}, {73.5318, 51.5014}, {56.0252, 71.7366}, {41.5493, 92.3655}, {70.7299, 92.2041},
}

// similarity is x' = a·x − b·y + tx, y' = b·x + a·y + ty: a turn, a scale
// and a shift, no shear and no mirroring.
type similarity struct{ a, b, tx, ty float64 }

// fitSimilarity finds the similarity that moves src onto dst with the least
// squared error.
func fitSimilarity(src, dst [5]point) similarity {
	sm, dm := centroid(src), centroid(dst)
	var num, cross, den float64
	for i := range src {
		sx, sy := src[i].x-sm.x, src[i].y-sm.y
		dx, dy := dst[i].x-dm.x, dst[i].y-dm.y
		num += sx*dx + sy*dy
		cross += sx*dy - sy*dx
		den += sx*sx + sy*sy
	}
	if den == 0 {
		return similarity{a: 1}
	}
	a, b := num/den, cross/den
	return similarity{a, b, dm.x - (a*sm.x - b*sm.y), dm.y - (b*sm.x + a*sm.y)}
}

func centroid(ps [5]point) point {
	var c point
	for _, p := range ps {
		c.x += p.x / 5
		c.y += p.y / 5
	}
	return c
}

// alignFace cuts the face with the given landmarks out of img, turned and
// scaled to the template.
func alignFace(img *image.RGBA, landmarks [5]point) *image.RGBA {
	t := fitSimilarity(landmarks, alignTemplate)
	out := image.NewRGBA(image.Rect(0, 0, alignSize, alignSize))
	det := t.a*t.a + t.b*t.b
	for y := range alignSize {
		for x := range alignSize {
			// The inverse of t: where in img this pixel of the face comes from.
			u, v := float64(x)-t.tx, float64(y)-t.ty
			sx, sy := (t.a*u+t.b*v)/det, (-t.b*u+t.a*v)/det
			sampleBilinear(img, sx, sy, out.Pix[(y*alignSize+x)*4:])
		}
	}
	return out
}

// sampleBilinear writes the colour of img at (x, y) into px, black outside.
func sampleBilinear(img *image.RGBA, x, y float64, px []uint8) {
	b := img.Bounds()
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	at := func(xi, yi, c int) float64 {
		if xi < b.Min.X || yi < b.Min.Y || xi >= b.Max.X || yi >= b.Max.Y {
			return 0
		}
		return float64(img.Pix[img.PixOffset(xi, yi)+c])
	}
	for c := range 3 {
		top := at(x0, y0, c)*(1-fx) + at(x0+1, y0, c)*fx
		bottom := at(x0, y0+1, c)*(1-fx) + at(x0+1, y0+1, c)*fx
		px[c] = uint8(math.Round(top*(1-fy) + bottom*fy))
	}
	px[3] = 255
}
