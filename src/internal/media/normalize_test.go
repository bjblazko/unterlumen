package media

import (
	"math"
	"testing"
)

func TestParseExposureSeconds(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		// goexif rational format
		{`"1/500"`, 1.0 / 500, true},
		{`"1/250"`, 1.0 / 250, true},
		{`"2/1"`, 2.0, true},
		{`"10/4"`, 2.5, true},
		{`"1/8000"`, 1.0 / 8000, true},
		{`"30/1"`, 30.0, true},
		// No surrounding quotes
		{"1/500", 1.0 / 500, true},
		// With unit suffix
		{"1/250 s", 1.0 / 250, true},
		{"1/250 sec", 1.0 / 250, true},
		{"1/250 second", 1.0 / 250, true},
		{"1/250 seconds", 1.0 / 250, true},
		// Decimal
		{"0.004", 0.004, true},
		{"0.004 sec", 0.004, true},
		{"0.053", 0.053, true},
		// Invalid
		{"0", 0, false},
		{"-1/250", 0, false},
		{"", 0, false},
		{"abc", 0, false},
		{"1/0", 0, false},
	}
	for _, c := range cases {
		v, ok := ParseExposureSeconds(c.in)
		if ok != c.ok {
			t.Errorf("ParseExposureSeconds(%q) ok=%v want %v", c.in, ok, c.ok)
			continue
		}
		if ok && math.Abs(v-c.want) > 1e-9 {
			t.Errorf("ParseExposureSeconds(%q) = %v, want %v", c.in, v, c.want)
		}
	}
}

func TestParseFNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{`"28/10"`, 2.8, true},
		{`"14/5"`, 2.8, true},
		{`"56/10"`, 5.6, true},
		{"2.8", 2.8, true},
		{"f/2.8", 2.8, true},
		{"f2.8", 2.8, true},
		{"F/2.8", 2.8, true},
		{"1/1", 1.0, true},
		// Invalid
		{"0", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		v, ok := ParseFNumber(c.in)
		if ok != c.ok {
			t.Errorf("ParseFNumber(%q) ok=%v want %v", c.in, ok, c.ok)
			continue
		}
		if ok && math.Abs(v-c.want) > 1e-9 {
			t.Errorf("ParseFNumber(%q) = %v, want %v", c.in, v, c.want)
		}
	}
}

func TestParseFocalLengthMM(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{`"50/1"`, 50.0, true},
		{`"240/10"`, 24.0, true},
		{"50", 50.0, true},
		{"50 mm", 50.0, true},
		{"50mm", 50.0, true},
		{"24.0", 24.0, true},
		// Zoom range — rejected
		{"24-70", 0, false},
		// Invalid
		{"0", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		v, ok := ParseFocalLengthMM(c.in)
		if ok != c.ok {
			t.Errorf("ParseFocalLengthMM(%q) ok=%v want %v", c.in, ok, c.ok)
			continue
		}
		if ok && math.Abs(v-c.want) > 1e-9 {
			t.Errorf("ParseFocalLengthMM(%q) = %v, want %v", c.in, v, c.want)
		}
	}
}

func TestParseISO(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"400", 400, true},
		{`"400"`, 400, true},
		{"ISO 400", 400, true},
		{"ISO400", 400, true},
		{"3200", 3200, true},
		{"100", 100, true},
		// Invalid
		{"0", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		v, ok := ParseISO(c.in)
		if ok != c.ok {
			t.Errorf("ParseISO(%q) ok=%v want %v", c.in, ok, c.ok)
			continue
		}
		if ok && math.Abs(v-c.want) > 1e-9 {
			t.Errorf("ParseISO(%q) = %v, want %v", c.in, v, c.want)
		}
	}
}

func TestNormalizeExifNumbers(t *testing.T) {
	tags := map[string]string{
		"ExposureTime":          `"1/500"`,
		"FNumber":               `"28/10"`,
		"FocalLength":           `"50/1"`,
		"FocalLengthIn35mmFilm": "75",
		"ISOSpeedRatings":       "400",
		"Make":                  "Canon", // non-numeric, must be ignored
	}
	got := NormalizeExifNumbers(tags)
	if len(got) != 5 {
		t.Fatalf("expected 5 numeric values, got %d: %v", len(got), got)
	}
	if math.Abs(got["ExposureTime"]-0.002) > 1e-9 {
		t.Errorf("ExposureTime = %v, want 0.002", got["ExposureTime"])
	}
	if math.Abs(got["FNumber"]-2.8) > 1e-9 {
		t.Errorf("FNumber = %v, want 2.8", got["FNumber"])
	}
	if math.Abs(got["FocalLength"]-50.0) > 1e-9 {
		t.Errorf("FocalLength = %v, want 50.0", got["FocalLength"])
	}
	if math.Abs(got["FocalLengthIn35mmFilm"]-75.0) > 1e-9 {
		t.Errorf("FocalLengthIn35mmFilm = %v, want 75.0", got["FocalLengthIn35mmFilm"])
	}
	if math.Abs(got["ISOSpeedRatings"]-400) > 1e-9 {
		t.Errorf("ISOSpeedRatings = %v, want 400", got["ISOSpeedRatings"])
	}
}
