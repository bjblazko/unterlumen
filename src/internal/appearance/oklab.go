// Package appearance measures what a photo looks like from its pixels: its
// colours, its tones, how busy and how sharp it is, and a hash for finding
// look-alikes. It reads an image and returns numbers; storing them is the
// library's business (ADR-0044).
package appearance

import "math"

// Lab is a colour in OKLab (Björn Ottosson, 2020): L is lightness from 0 to
// 1, A runs green to red, B blue to yellow. Distances in it match how far
// apart colours look, which plain RGB does not.
type Lab struct{ L, A, B float64 }

// srgbLinear maps an 8-bit sRGB channel to linear light.
var srgbLinear = func() [256]float64 {
	var t [256]float64
	for i := range t {
		c := float64(i) / 255
		if c <= 0.04045 {
			t[i] = c / 12.92
		} else {
			t[i] = math.Pow((c+0.055)/1.055, 2.4)
		}
	}
	return t
}()

// labFromSRGB converts an 8-bit sRGB colour to OKLab.
func labFromSRGB(r, g, b uint8) Lab {
	lr, lg, lb := srgbLinear[r], srgbLinear[g], srgbLinear[b]
	l := math.Cbrt(0.4122214708*lr + 0.5363325363*lg + 0.0514459929*lb)
	m := math.Cbrt(0.2119034982*lr + 0.6806995451*lg + 0.1073969566*lb)
	s := math.Cbrt(0.0883024619*lr + 0.2817188376*lg + 0.6299787005*lb)
	return Lab{
		L: 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		A: 1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		B: 0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

// Chroma is how colourful the colour is: 0 for a grey.
func (c Lab) Chroma() float64 { return math.Hypot(c.A, c.B) }

// Hue is the colour's angle in degrees, 0 to 360: about 30 for red, 70 for
// orange-yellow, 140 for green, 200 for teal and 265 for blue.
func (c Lab) Hue() float64 {
	h := math.Atan2(c.B, c.A) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return h
}

func (c Lab) dist2(o Lab) float64 {
	dl, da, db := c.L-o.L, c.A-o.A, c.B-o.B
	return dl*dl + da*da + db*db
}
