package media

import (
	"strconv"
	"strings"
)

// numericExifFields maps EXIF field names to their parser functions.
// Only these four fields are given numeric_value in the exif_index.
var numericExifFields = map[string]func(string) (float64, bool){
	"ExposureTime":     ParseExposureSeconds,
	"FNumber":          ParseFNumber,
	"FocalLength":      ParseFocalLengthMM,
	"ISOSpeedRatings":  ParseISO,
}

// NormalizeExifNumbers returns a map of EXIF field → float64 for all numeric
// fields found in tags that can be parsed successfully.
func NormalizeExifNumbers(tags map[string]string) map[string]float64 {
	out := make(map[string]float64)
	for field, parse := range numericExifFields {
		raw, ok := tags[field]
		if !ok {
			continue
		}
		if v, ok := parse(raw); ok {
			out[field] = v
		}
	}
	return out
}

// ParseExposureSeconds parses an EXIF ExposureTime value into seconds.
// Handles rational fractions ("1/500", "2/1", "10/4"), decimals ("0.004"),
// and variants with unit suffixes ("1/250 s", "0.004 sec", "1/250 second").
func ParseExposureSeconds(s string) (float64, bool) {
	s = cleanNumericTag(s)
	s = stripTimeSuffix(s)
	return parseRationalOrFloat(s)
}

// ParseFNumber parses an EXIF FNumber value into an f-number float.
// Handles rationals ("28/10"), decimals ("2.8"), and prefixed forms ("f/2.8").
func ParseFNumber(s string) (float64, bool) {
	s = cleanNumericTag(s)
	s = strings.TrimPrefix(strings.ToLower(s), "f/")
	s = strings.TrimPrefix(strings.ToLower(s), "f")
	v, ok := parseRationalOrFloat(s)
	if !ok || v <= 0 {
		return 0, false
	}
	return v, true
}

// ParseFocalLengthMM parses an EXIF FocalLength value into millimetres.
// Handles rationals ("50/1", "240/10"), decimals ("50", "24.0"),
// and unit suffixes ("50 mm"). Zoom ranges ("24-70") are rejected.
func ParseFocalLengthMM(s string) (float64, bool) {
	s = cleanNumericTag(s)
	// Reject zoom-lens ranges like "24-70"; a bare hyphen mid-string after digits means a range.
	if strings.Count(s, "-") > 0 {
		return 0, false
	}
	s = strings.TrimSuffix(strings.ToLower(s), " mm")
	s = strings.TrimSuffix(strings.ToLower(s), "mm")
	v, ok := parseRationalOrFloat(s)
	if !ok || v <= 0 {
		return 0, false
	}
	return v, true
}

// ParseISO parses an EXIF ISOSpeedRatings value into a float64.
// Handles plain integers ("400"), "ISO 400", and rationals (unusual but possible).
func ParseISO(s string) (float64, bool) {
	s = cleanNumericTag(s)
	s = strings.TrimPrefix(strings.ToUpper(s), "ISO ")
	s = strings.TrimPrefix(strings.ToUpper(s), "ISO")
	v, ok := parseRationalOrFloat(s)
	if !ok || v <= 0 {
		return 0, false
	}
	return v, true
}

// cleanNumericTag strips surrounding quotes that goexif adds to string tags.
func cleanNumericTag(s string) string {
	return strings.Trim(s, `"`)
}

// stripTimeSuffix removes trailing time unit words from a shutter-speed string.
func stripTimeSuffix(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	for _, suffix := range []string{" seconds", " second", " secs", " sec", " s"} {
		if strings.HasSuffix(lower, suffix) {
			return strings.TrimSpace(s[:len(s)-len(suffix)])
		}
	}
	return s
}

// parseRationalOrFloat parses either "num/den" or a plain float string.
func parseRationalOrFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "/"); idx >= 0 {
		num, err1 := strconv.ParseFloat(strings.TrimSpace(s[:idx]), 64)
		den, err2 := strconv.ParseFloat(strings.TrimSpace(s[idx+1:]), 64)
		if err1 != nil || err2 != nil || den == 0 {
			return 0, false
		}
		v := num / den
		if v <= 0 {
			return 0, false
		}
		return v, true
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}
