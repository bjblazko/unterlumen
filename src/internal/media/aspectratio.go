package media

import (
	"fmt"
	"math"
)

// AspectRatioLabel returns a human-readable aspect ratio label for the given
// image dimensions, using approximate matching (1.5% tolerance) to handle
// real camera sensors where pixel counts don't reduce to clean integers.
// Returns "Custom Crop" for unrecognised ratios, or "" if dimensions are zero.
func AspectRatioLabel(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	type knownRatio struct {
		w, h  int
		value float64
	}
	ratios := []knownRatio{
		{1, 2, 1.0 / 2.0},
		{9, 16, 9.0 / 16.0},
		{2, 3, 2.0 / 3.0},
		{3, 4, 3.0 / 4.0},
		{4, 5, 4.0 / 5.0},
		{1, 1, 1.0},
		{5, 4, 5.0 / 4.0},
		{4, 3, 4.0 / 3.0},
		{3, 2, 3.0 / 2.0},
		{7, 5, 7.0 / 5.0},
		{16, 10, 16.0 / 10.0},
		{5, 3, 5.0 / 3.0},
		{16, 9, 16.0 / 9.0},
		{2, 1, 2.0},
		{21, 9, 21.0 / 9.0},
	}
	r := float64(w) / float64(h)
	const tol = 0.015
	for _, kr := range ratios {
		if math.Abs(r-kr.value)/kr.value < tol {
			return fmt.Sprintf("%d:%d", kr.w, kr.h)
		}
	}
	return "Custom Crop"
}
