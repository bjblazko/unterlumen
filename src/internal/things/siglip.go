package things

import (
	"image"
	"math"

	"golang.org/x/image/draw"

	"huepattl.de/unterlumen/internal/onnx"
)

// SigLIP 2 looks at the whole photo scaled to a square of imageSize, its
// RGB values taken from 0–1 to -1–1.
const imageSize = 224

// What SigLIP 2 learned with the encoders: how likely a text fits an image
// is sigmoid(logitScale · similarity + logitBias). Read from the model's
// weights (logit_scale is stored as its logarithm, 4.7245).
const (
	logitScale = 112.6689
	logitBias  = -16.7717
)

// describeImage is SigLIP's vector of an image, of unit length.
func (e *Engine) describeImage(img image.Image) ([]float32, error) {
	out, err := e.image.Run(imageInput(square(img, imageSize)))
	if err != nil {
		return nil, err
	}
	return normalise(pooled(e.image, out)), nil
}

// describeText is SigLIP's vector of a text, of unit length.
func (e *Engine) describeText(text string) ([]float32, error) {
	out, err := e.text.Run(onnx.Tensor{Shape: []int64{1, contextLength}, Ints: e.tok.encode(text)})
	if err != nil {
		return nil, err
	}
	return normalise(pooled(e.text, out)), nil
}

// pooled is the encoder's output named pooler_output, its vector.
func pooled(s *onnx.Session, out []onnx.Tensor) []float32 {
	for i, name := range s.OutputNames() {
		if name == "pooler_output" {
			return out[i].Data
		}
	}
	return out[len(out)-1].Data
}

// square scales img to size × size, as SigLIP's preprocessing does: the
// whole photo, not cut.
func square(img image.Image, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
	return dst
}

// imageInput lays the square out as SigLIP's RGB planes, -1 to 1.
func imageInput(img *image.RGBA) onnx.Tensor {
	n := imageSize * imageSize
	data := make([]float32, 3*n)
	for i := range n {
		for c := range 3 {
			data[c*n+i] = float32(img.Pix[i*4+c])/127.5 - 1
		}
	}
	return onnx.Tensor{Shape: []int64{1, 3, imageSize, imageSize}, Data: data}
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

// Similarity is the cosine of two vectors of unit length; 0 for vectors
// of different models, which cannot be compared.
func Similarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// Probability is how likely SigLIP takes a text to fit an image whose
// vectors have this similarity: each text on its own, not one against the
// others.
func Probability(similarity float32) float32 {
	return float32(1 / (1 + math.Exp(-(logitScale*float64(similarity) + logitBias))))
}
