package things

import (
	"image"
	"math"

	"golang.org/x/image/draw"

	"huepattl.de/unterlumen/internal/onnx"
)

// clipSize is the side of the square CLIP looks at.
const clipSize = 224

// CLIP's normalisation of the RGB channels.
var (
	clipMean = [3]float32{0.48145466, 0.4578275, 0.40821073}
	clipStd  = [3]float32{0.26862954, 0.26130258, 0.27577711}
)

// describeImage is CLIP's vector of an image, of unit length.
func (e *Engine) describeImage(img image.Image) ([]float32, error) {
	out, err := e.image.Run(clipInput(centreSquare(img, clipSize)))
	if err != nil {
		return nil, err
	}
	return normalise(out[0].Data), nil
}

// describeText is CLIP's vector of a phrase, of unit length.
func (e *Engine) describeText(phrase string) ([]float32, error) {
	ids := e.tok.encode(phrase)
	out, err := e.text.Run(onnx.Tensor{Shape: []int64{1, contextLength}, Ints: ids})
	if err != nil {
		return nil, err
	}
	return normalise(out[0].Data), nil
}

// centreSquare scales img so its shorter side is size and cuts the middle
// square out of it, as CLIP's preprocessing does.
func centreSquare(img image.Image, size int) *image.RGBA {
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(0, 0, side, side).Add(b.Min).Add(image.Pt((b.Dx()-side)/2, (b.Dy()-side)/2))
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, crop, draw.Src, nil)
	return dst
}

// clipInput lays the square out as CLIP's normalised RGB planes.
func clipInput(img *image.RGBA) onnx.Tensor {
	n := clipSize * clipSize
	data := make([]float32, 3*n)
	for i := range n {
		for c := range 3 {
			v := float32(img.Pix[i*4+c]) / 255
			data[c*n+i] = (v - clipMean[c]) / clipStd[c]
		}
	}
	return onnx.Tensor{Shape: []int64{1, 3, clipSize, clipSize}, Data: data}
}

func normalise(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	inv := float32(1 / math.Sqrt(sum))
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x * inv
	}
	return out
}

// Similarity is the cosine of two vectors of unit length.
func Similarity(a, b []float32) float32 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}
