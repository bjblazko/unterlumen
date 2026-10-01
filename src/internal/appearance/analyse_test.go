package appearance

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"math/rand"
	"testing"

	"golang.org/x/image/draw"
)

func fill(w, h int, at func(x, y int) color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, at(x, y))
		}
	}
	return img
}

func flat(c color.RGBA) *image.RGBA {
	return fill(400, 300, func(int, int) color.RGBA { return c })
}

func hueNear(t *testing.T, got, want, tol float64) {
	t.Helper()
	d := math.Abs(math.Mod(got-want+540, 360) - 180)
	if d > tol {
		t.Errorf("hue %.1f°, want %.1f° ± %.0f°", got, want, tol)
	}
}

func TestOKLabReferenceColours(t *testing.T) {
	if l := labFromSRGB(255, 255, 255); math.Abs(l.L-1) > 1e-3 || l.Chroma() > 1e-3 {
		t.Errorf("white = %+v", l)
	}
	if l := labFromSRGB(0, 0, 0); l.L > 1e-6 {
		t.Errorf("black = %+v", l)
	}
	hueNear(t, labFromSRGB(255, 0, 0).Hue(), 29, 2)
	hueNear(t, labFromSRGB(0, 0, 255).Hue(), 264, 2)
}

func TestGreyIsMono(t *testing.T) {
	r := Analyse(fill(400, 300, func(x, _ int) color.RGBA {
		v := uint8(x * 255 / 399)
		return color.RGBA{v, v, v, 255}
	}))
	if r.Mono != Mono {
		t.Errorf("grey gradient: %s, want mono", r.Mono)
	}
	if r.Colourfulness > 1 {
		t.Errorf("grey colourfulness %.2f", r.Colourfulness)
	}
}

func TestSepiaIsTinted(t *testing.T) {
	r := Analyse(fill(400, 300, func(x, _ int) color.RGBA {
		v := float64(x) / 399 // sepia: grey ramp with a warm cast
		return color.RGBA{uint8(40 + 200*v), uint8(30 + 180*v), uint8(20 + 140*v), 255}
	}))
	if r.Mono != Tinted {
		t.Fatalf("sepia: %s, want tinted", r.Mono)
	}
	hueNear(t, r.TintHue, 70, 15)
	if r.Warmth < 0.5 {
		t.Errorf("sepia warmth %.2f, want warm", r.Warmth)
	}
}

func TestOrangeAndTealPalette(t *testing.T) {
	orange, teal := color.RGBA{232, 120, 30, 255}, color.RGBA{20, 128, 128, 255}
	r := Analyse(fill(400, 300, func(x, _ int) color.RGBA {
		if x < 200 {
			return orange
		}
		return teal
	}))
	if r.Mono != Colour {
		t.Errorf("orange and teal: %s", r.Mono)
	}
	if len(r.Palette) < 2 {
		t.Fatalf("palette %+v, want two swatches", r.Palette)
	}
	hues := []float64{r.Palette[0].Colour.Hue(), r.Palette[1].Colour.Hue()}
	if math.Abs(hues[0]-hues[1]) < 90 {
		t.Fatalf("hues %v, want orange and teal", hues)
	}
	if hues[0] > hues[1] {
		hues[0], hues[1] = hues[1], hues[0]
	}
	hueNear(t, hues[0], 55, 15)
	hueNear(t, hues[1], 195, 15)
	for _, s := range r.Palette[:2] {
		if math.Abs(s.Share-0.5) > 0.06 {
			t.Errorf("share %.2f, want about half", s.Share)
		}
	}
}

func TestFlatColourHasOneSwatch(t *testing.T) {
	r := Analyse(flat(color.RGBA{200, 40, 40, 255}))
	if len(r.Palette) != 1 || r.Palette[0].Share != 1 {
		t.Errorf("palette %+v, want one swatch", r.Palette)
	}
}

func TestHueBin(t *testing.T) {
	if b := (Swatch{Colour: labFromSRGB(128, 128, 128)}).HueBin(); b != -1 {
		t.Errorf("grey bin %d, want -1", b)
	}
	if b := (Swatch{Colour: labFromSRGB(255, 0, 0)}).HueBin(); b != 0 {
		t.Errorf("red bin %d, want 0", b)
	}
}

func TestDarkFrameIsLowKey(t *testing.T) {
	r := Analyse(fill(400, 300, func(x, y int) color.RGBA {
		if x > 150 && x < 250 && y > 100 && y < 200 {
			return color.RGBA{60, 60, 60, 255}
		}
		return color.RGBA{0, 0, 0, 255}
	}))
	if r.Tone.Key != LowKey {
		t.Errorf("key %s, want low", r.Tone.Key)
	}
	if r.Tone.ClipLow < 0.8 {
		t.Errorf("clipped shadows %.2f, want most", r.Tone.ClipLow)
	}
	if r.Tone.Histogram[0] != 255 {
		t.Errorf("histogram %v, want the first bin fullest", r.Tone.Histogram)
	}
}

func TestWhiteIsHighKey(t *testing.T) {
	r := Analyse(flat(color.RGBA{250, 250, 250, 255}))
	if r.Tone.Key != HighKey || r.Tone.ClipHigh != 1 {
		t.Errorf("tone %+v, want high key, all clipped", r.Tone)
	}
}

func TestNoiseHasMoreEntropyThanFlat(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	noise := fill(400, 300, func(int, int) color.RGBA {
		v := uint8(rng.Intn(256))
		return color.RGBA{v, v, v, 255}
	})
	n, f := Analyse(noise).Texture, Analyse(flat(color.RGBA{90, 90, 90, 255})).Texture
	if n.Entropy < 6 || f.Entropy != 0 {
		t.Errorf("entropy noise %.2f, flat %.2f", n.Entropy, f.Entropy)
	}
	if n.EdgeDensity < 0.3 || f.EdgeDensity != 0 {
		t.Errorf("edges noise %.3f, flat %.3f", n.EdgeDensity, f.EdgeDensity)
	}
}

func TestBlurLowersSharpness(t *testing.T) {
	stripes := fill(1024, 768, func(x, _ int) color.RGBA {
		if (x/16)%2 == 0 {
			return color.RGBA{255, 255, 255, 255}
		}
		return color.RGBA{0, 0, 0, 255}
	})
	// Scaling down to an eighth and back is a strong blur.
	tiny := image.NewRGBA(image.Rect(0, 0, 128, 96))
	draw.BiLinear.Scale(tiny, tiny.Bounds(), stripes, stripes.Bounds(), draw.Src, nil)
	blurred := image.NewRGBA(stripes.Bounds())
	draw.BiLinear.Scale(blurred, blurred.Bounds(), tiny, tiny.Bounds(), draw.Src, nil)

	s, b := Analyse(stripes).Texture, Analyse(blurred).Texture
	if s.Sharpness <= 2*b.Sharpness {
		t.Errorf("sharpness sharp %.0f, blurred %.0f", s.Sharpness, b.Sharpness)
	}
}

func TestCentroidFollowsTheLight(t *testing.T) {
	r := Analyse(fill(400, 300, func(x, _ int) color.RGBA {
		if x > 300 {
			return color.RGBA{255, 255, 255, 255}
		}
		return color.RGBA{0, 0, 0, 255}
	}))
	if r.Texture.CentroidX < 0.8 || math.Abs(r.Texture.CentroidY-0.5) > 0.02 {
		t.Errorf("centroid %.2f, %.2f, want right of centre", r.Texture.CentroidX, r.Texture.CentroidY)
	}
}

func TestHashSurvivesResizeAndJPEG(t *testing.T) {
	wave := func(fx, fy float64) func(x, y int) color.RGBA {
		return func(x, y int) color.RGBA {
			v := uint8(128 + 100*math.Sin(float64(x)/fx)*math.Cos(float64(y)/fy))
			return color.RGBA{v, v, v, 255}
		}
	}
	scene := fill(1200, 800, wave(80, 60))
	smaller := image.NewRGBA(image.Rect(0, 0, 600, 400))
	draw.BiLinear.Scale(smaller, smaller.Bounds(), scene, scene.Bounds(), draw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, smaller, &jpeg.Options{Quality: 70}); err != nil {
		t.Fatal(err)
	}
	recompressed, err := AnalyseJPEG(&buf)
	if err != nil {
		t.Fatal(err)
	}
	other := Analyse(fill(1200, 800, wave(45, 110)))
	original := Analyse(scene)
	if d := HashDistance(original.Hash, recompressed.Hash); d > 6 {
		t.Errorf("same scene differs in %d bits", d)
	}
	if d := HashDistance(original.Hash, other.Hash); d < 16 {
		t.Errorf("different scene differs in only %d bits", d)
	}
}

func TestAnalyseIsDeterministic(t *testing.T) {
	img := fill(300, 200, func(x, y int) color.RGBA {
		return color.RGBA{uint8(x), uint8(y), uint8(x ^ y), 255}
	})
	a, b := Analyse(img), Analyse(img)
	if len(a.Palette) != len(b.Palette) || a.Palette[0] != b.Palette[0] || a.Hash != b.Hash {
		t.Errorf("two runs differ: %+v vs %+v", a.Palette, b.Palette)
	}
}

func BenchmarkAnalyseThumbnail(b *testing.B) {
	img := fill(1200, 800, func(x, y int) color.RGBA {
		return color.RGBA{uint8(x), uint8(y), uint8(x ^ y), 255}
	})
	var buf bytes.Buffer
	jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}) //nolint:errcheck
	data := buf.Bytes()
	b.ResetTimer()
	for range b.N {
		if _, err := AnalyseJPEG(bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}
