package appearance

import (
	"image"
	"math"
)

// Texture is how much detail a photo has and where its weight sits.
type Texture struct {
	Entropy     float64 // bits, 0 for a flat image up to 8
	Sharpness   float64 // variance of the Laplacian; low means blurred
	EdgeDensity float64 // share of pixels on an edge
	CentroidX   float64 // where the light sits, 0 left … 1 right
	CentroidY   float64 // 0 top … 1 bottom
}

// edgeThreshold is the Sobel magnitude, on 0…255 grey, at which a pixel
// lies on an edge.
const edgeThreshold = 100

func measureTexture(large *image.RGBA, small pixels) Texture {
	grey, w, h := greyOf(large)
	cx, cy := centroid(small)
	return Texture{
		Entropy:     entropy(grey),
		Sharpness:   laplacianVariance(grey, w, h),
		EdgeDensity: edgeDensity(grey, w, h),
		CentroidX:   cx,
		CentroidY:   cy,
	}
}

// greyOf is an image's luma (Rec. 601, on the sRGB values), 0…255.
func greyOf(img *image.RGBA) ([]float64, int, int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	g := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			g[y*w+x] = 0.299*float64(img.Pix[o]) + 0.587*float64(img.Pix[o+1]) + 0.114*float64(img.Pix[o+2])
		}
	}
	return g, w, h
}

// entropy is the Shannon entropy of the grey histogram in bits.
func entropy(grey []float64) float64 {
	var counts [256]int
	for _, v := range grey {
		counts[min(max(int(v+0.5), 0), 255)]++
	}
	n := float64(len(grey))
	var e float64
	for _, c := range counts {
		if c > 0 {
			p := float64(c) / n
			e -= p * math.Log2(p)
		}
	}
	return e
}

// laplacianVariance is the variance of the four-neighbour Laplacian, the
// usual measure for telling a sharp photo from a blurred one.
func laplacianVariance(g []float64, w, h int) float64 {
	var sum, sum2 float64
	n := 0
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			v := g[i-1] + g[i+1] + g[i-w] + g[i+w] - 4*g[i]
			sum += v
			sum2 += v * v
			n++
		}
	}
	if n == 0 {
		return 0
	}
	mean := sum / float64(n)
	return sum2/float64(n) - mean*mean
}

// edgeDensity is the share of pixels whose Sobel gradient passes
// edgeThreshold.
func edgeDensity(g []float64, w, h int) float64 {
	edges, n := 0, 0
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			gx := g[i-w+1] + 2*g[i+1] + g[i+w+1] - g[i-w-1] - 2*g[i-1] - g[i+w-1]
			gy := g[i+w-1] + 2*g[i+w] + g[i+w+1] - g[i-w-1] - 2*g[i-w] - g[i-w+1]
			if math.Hypot(gx, gy) >= edgeThreshold {
				edges++
			}
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(edges) / float64(n)
}

// centroid is the lightness-weighted centre of the image, each axis 0…1.
func centroid(px pixels) (float64, float64) {
	var sx, sy, sw float64
	for y := 0; y < px.h; y++ {
		for x := 0; x < px.w; x++ {
			l := px.lab[y*px.w+x].L
			sx += l * (float64(x) + 0.5)
			sy += l * (float64(y) + 0.5)
			sw += l
		}
	}
	if sw == 0 {
		return 0.5, 0.5
	}
	return sx / sw / float64(px.w), sy / sw / float64(px.h)
}
