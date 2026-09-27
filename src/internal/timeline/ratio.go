// Package timeline is every dated photo of every library on one time axis,
// each photo once (ADR-0040).
package timeline

import "strconv"

// fallbackRatio stands in for a photo whose size is unknown: the most common
// sensor shape.
const fallbackRatio = 1.5

// displayRatio is width ÷ height as the photo is shown. EXIF orientations 5
// to 8 turn it by a quarter, so its sides swap.
func displayRatio(width, height int, orientation string) float64 {
	if width <= 0 || height <= 0 {
		return fallbackRatio
	}
	if o, _ := strconv.Atoi(orientation); o >= 5 && o <= 8 {
		width, height = height, width
	}
	return float64(width) / float64(height)
}
