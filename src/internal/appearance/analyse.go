package appearance

import (
	"image"
	"image/jpeg"
	"io"

	"golang.org/x/image/draw"
)

// Version names the way the measurements are taken. Raise it when one of
// them changes; values stored under an older version are then measured again.
const Version = 1

// Sizes the image is scaled to before measuring, by its longest side, so a
// value means the same for every photo whatever its thumbnail's size.
const (
	colourSize  = 256 // colours and tones
	textureSize = 512 // sharpness and edges need finer detail
)

// Result is everything measured of one photo.
type Result struct {
	Mono          MonoClass
	TintHue       float64 // degrees; only when Mono is Tinted
	Average       Lab
	Colourfulness float64 // Hasler and Süsstrunk; about 0 for grey, above 100 for very vivid
	Warmth        float64 // -1 all blue … +1 all orange
	Palette       []Swatch
	Tone          Tone
	Texture       Texture
	Hash          uint64 // dHash; near-identical photos differ in few bits
}

// AnalyseJPEG decodes a JPEG, such as a library thumbnail, and measures it.
func AnalyseJPEG(r io.Reader) (Result, error) {
	img, err := jpeg.Decode(r)
	if err != nil {
		return Result{}, err
	}
	return Analyse(img), nil
}

// Analyse measures an image. The image should already be the right way up.
func Analyse(img image.Image) Result {
	large := scaleTo(img, textureSize)
	small := scaleTo(large, colourSize)
	px := readPixels(small)

	mono, tint := monoClass(px.lab)
	return Result{
		Mono:          mono,
		TintHue:       tint,
		Average:       averageColour(px.lab),
		Colourfulness: colourfulness(px.rgb),
		Warmth:        warmth(px.lab),
		Palette:       palette(px.lab),
		Tone:          measureTone(px),
		Texture:       measureTexture(large, px),
		Hash:          dHash(small),
	}
}

// scaleTo returns img scaled down so its longest side is at most size. A
// smaller image is copied as it is, never enlarged.
func scaleTo(img image.Image, size int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if longest := max(w, h); longest > size {
		w = max(1, w*size/longest)
		h = max(1, h*size/longest)
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if w == b.Dx() && h == b.Dy() {
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
		return dst
	}
	draw.BiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// pixels is an image's pixels in the two forms the measurements read.
type pixels struct {
	w, h int
	rgb  [][3]uint8
	lab  []Lab
}

func readPixels(img *image.RGBA) pixels {
	b := img.Bounds()
	px := pixels{w: b.Dx(), h: b.Dy()}
	n := px.w * px.h
	px.rgb = make([][3]uint8, n)
	px.lab = make([]Lab, n)
	for y := 0; y < px.h; y++ {
		for x := 0; x < px.w; x++ {
			o := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			r, g, bl := img.Pix[o], img.Pix[o+1], img.Pix[o+2]
			i := y*px.w + x
			px.rgb[i] = [3]uint8{r, g, bl}
			px.lab[i] = labFromSRGB(r, g, bl)
		}
	}
	return px
}
