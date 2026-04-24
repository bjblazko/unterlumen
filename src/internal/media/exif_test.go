package media

import "testing"

func TestParseExifDateTag_ValidDate(t *testing.T) {
	tags := map[string]string{
		"DateTimeOriginal": `"2024:03:15 14:07:42"`,
	}
	got := parseExifDateTag(tags, "DateTimeOriginal", "OffsetTimeOriginal")
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if *got != "2024-03-15T14:07:42" {
		t.Errorf("got %q, want %q", *got, "2024-03-15T14:07:42")
	}
}

func TestParseExifDateTag_WithOffset(t *testing.T) {
	tags := map[string]string{
		"DateTimeOriginal":  `"2024:03:15 14:07:42"`,
		"OffsetTimeOriginal": `"+02:00"`,
	}
	got := parseExifDateTag(tags, "DateTimeOriginal", "OffsetTimeOriginal")
	if got == nil {
		t.Fatal("expected non-nil result")
	}
	if *got != "2024-03-15T14:07:42+02:00" {
		t.Errorf("got %q, want %q", *got, "2024-03-15T14:07:42+02:00")
	}
}

func TestParseExifDateTag_MissingKey(t *testing.T) {
	tags := map[string]string{}
	got := parseExifDateTag(tags, "DateTimeOriginal", "OffsetTimeOriginal")
	if got != nil {
		t.Errorf("expected nil for missing key, got %q", *got)
	}
}

func TestParseExifDateTag_InvalidFormat(t *testing.T) {
	tags := map[string]string{
		"DateTimeOriginal": `"not-a-date"`,
	}
	got := parseExifDateTag(tags, "DateTimeOriginal", "OffsetTimeOriginal")
	if got != nil {
		t.Errorf("expected nil for invalid date, got %q", *got)
	}
}

func TestAspectRatioLabel(t *testing.T) {
	tests := []struct {
		w, h int
		want string
	}{
		{3000, 2000, "3:2"},
		{4000, 3000, "4:3"},
		{1920, 1080, "16:9"},
		{1000, 1000, "1:1"},
		{0, 100, ""},
		{100, 0, ""},
		{1234, 5678, "Custom Crop"},
	}

	for _, tt := range tests {
		got := AspectRatioLabel(tt.w, tt.h)
		if got != tt.want {
			t.Errorf("AspectRatioLabel(%d, %d) = %q, want %q", tt.w, tt.h, got, tt.want)
		}
	}
}

func TestIsTIFFHeader(t *testing.T) {
	littleEndian := []byte{0x49, 0x49, 0x2a, 0x00, 0x08, 0x00, 0x00, 0x00}
	bigEndian := []byte{0x4d, 0x4d, 0x00, 0x2a, 0x00, 0x00, 0x00, 0x08}
	notTIFF := []byte{0xFF, 0xD8, 0xFF, 0xE0}

	if !isTIFFHeader(littleEndian) {
		t.Error("little-endian TIFF header not recognised")
	}
	if !isTIFFHeader(bigEndian) {
		t.Error("big-endian TIFF header not recognised")
	}
	if isTIFFHeader(notTIFF) {
		t.Error("JPEG SOI should not be recognised as TIFF")
	}
	if isTIFFHeader([]byte{0x49, 0x49}) {
		t.Error("too-short slice should not be recognised as TIFF")
	}
}
