package browse

import "testing"

func TestParseThumbnailQuality(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "default", input: "", want: thumbnailQualityStandard},
		{name: "unknown", input: "preview", want: thumbnailQualityStandard},
		{name: "high", input: thumbnailQualityHigh, want: thumbnailQualityHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseThumbnailQuality(tt.input); got != tt.want {
				t.Fatalf("parseThumbnailQuality(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
