package batchrename

import (
	"strings"
	"testing"
)

func TestResolvePattern_DateTokens(t *testing.T) {
	dateTaken := "2024-03-15T14:07:42"
	tags := map[string]string{}

	result := resolvePattern("{YYYY}-{MM}-{DD}", tags, dateTaken, "original", 1)
	if result != "2024-03-15" {
		t.Errorf("got %q, want %q", result, "2024-03-15")
	}

	result = resolvePattern("{hh}{mm}{ss}", tags, dateTaken, "original", 1)
	if result != "140742" {
		t.Errorf("got %q, want %q", result, "140742")
	}
}

func TestResolvePattern_EmptyDate(t *testing.T) {
	tags := map[string]string{}
	result := resolvePattern("{YYYY}", tags, "", "original", 1)
	if result != "unknown" {
		t.Errorf("empty date should produce 'unknown', got %q", result)
	}
}

func TestResolvePattern_OriginalToken(t *testing.T) {
	tags := map[string]string{}
	result := resolvePattern("{original}", tags, "", "IMG_1234", 1)
	if result != "IMG_1234" {
		t.Errorf("got %q, want 'IMG_1234'", result)
	}
}

func TestResolvePattern_SequenceDefault(t *testing.T) {
	tags := map[string]string{}
	result := resolvePattern("{seq}", tags, "", "x", 7)
	if result != "007" {
		t.Errorf("got %q, want '007'", result)
	}
}

func TestResolvePattern_SequenceCustomWidth(t *testing.T) {
	tags := map[string]string{}
	result := resolvePattern("{seq:5}", tags, "", "x", 3)
	if result != "00003" {
		t.Errorf("got %q, want '00003'", result)
	}
}

func TestResolvePattern_ExifTags(t *testing.T) {
	tags := map[string]string{
		"Make":  "\"Fujifilm\"",
		"Model": "\"X-T5\"",
	}
	result := resolvePattern("{make}-{model}", tags, "", "x", 1)
	if result != "Fujifilm-X-T5" {
		t.Errorf("got %q, want 'Fujifilm-X-T5'", result)
	}
}

func TestResolvePattern_MissingExifTag(t *testing.T) {
	tags := map[string]string{}
	result := resolvePattern("{make}", tags, "", "x", 1)
	if result != "unknown" {
		t.Errorf("missing tag should produce 'unknown', got %q", result)
	}
}

func TestParseDateComponents(t *testing.T) {
	year, month, day, hour, min, sec := parseDateComponents("2024-06-21T09:05:03")
	if year != "2024" || month != "06" || day != "21" {
		t.Errorf("date parts wrong: %s/%s/%s", year, month, day)
	}
	if hour != "09" || min != "05" || sec != "03" {
		t.Errorf("time parts wrong: %s:%s:%s", hour, min, sec)
	}
}

func TestParseDateComponents_Short(t *testing.T) {
	year, _, _, _, _, _ := parseDateComponents("2024-06")
	if year != "unknown" {
		t.Errorf("short date should yield 'unknown', got %q", year)
	}
}

func TestFormatAperture(t *testing.T) {
	tags := map[string]string{"FNumber": "28/10"}
	got := formatAperture(tags)
	if got != "f2.8" {
		t.Errorf("got %q, want 'f2.8'", got)
	}
}

func TestFormatFocal(t *testing.T) {
	tags := map[string]string{"FocalLength": "50/1"}
	got := formatFocal(tags)
	if got != "50mm" {
		t.Errorf("got %q, want '50mm'", got)
	}
}

func TestFormatShutter_Fraction(t *testing.T) {
	tags := map[string]string{"ExposureTime": "1/500"}
	got := formatShutter(tags)
	if !strings.HasPrefix(got, "1-") {
		t.Errorf("fraction shutter should start with '1-', got %q", got)
	}
}

func TestFormatShutter_Slow(t *testing.T) {
	tags := map[string]string{"ExposureTime": "2/1"}
	got := formatShutter(tags)
	if got != "2s" {
		t.Errorf("got %q, want '2s'", got)
	}
}
