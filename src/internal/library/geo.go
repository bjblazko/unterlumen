package library

import (
	"strconv"
	"strings"
)

// ParseGPSCoord converts a goexif GPS tag string to decimal degrees.
// Handles rational DMS format "[48/1, 52/1, 4746/100]" and plain decimals.
func ParseGPSCoord(coord, ref string) (float64, bool) {
	coord = strings.TrimSpace(coord)
	if coord == "" {
		return 0, false
	}
	deg, ok := parseDecimalOrDMS(coord)
	if !ok {
		return 0, false
	}
	if isSouthOrWest(ref) {
		deg = -deg
	}
	return deg, true
}

func parseDecimalOrDMS(coord string) (float64, bool) {
	// Plain decimal (e.g. "48.879850").
	if v, err := strconv.ParseFloat(coord, 64); err == nil {
		return v, true
	}
	// Rational DMS: "[d/1, m/1, s/100]".
	parts := strings.SplitN(strings.Trim(coord, "[] "), ",", 3)
	if len(parts) != 3 {
		return 0, false
	}
	vals := make([]float64, 3)
	for i, p := range parts {
		n, d, ok := parseRat(strings.TrimSpace(p))
		if !ok || d == 0 {
			return 0, false
		}
		vals[i] = n / d
	}
	return vals[0] + vals[1]/60 + vals[2]/3600, true
}

func isSouthOrWest(ref string) bool {
	ref = strings.TrimSpace(ref)
	return strings.EqualFold(ref, "S") || strings.EqualFold(ref, "W")
}

func parseRat(s string) (float64, float64, bool) {
	idx := strings.IndexByte(s, '/')
	if idx < 0 {
		return 0, 0, false
	}
	n, err1 := strconv.ParseFloat(s[:idx], 64)
	d, err2 := strconv.ParseFloat(s[idx+1:], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return n, d, true
}
