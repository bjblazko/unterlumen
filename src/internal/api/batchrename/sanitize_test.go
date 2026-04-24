package batchrename

import "testing"

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello-world"},
		{"IMG_1234", "IMG_1234"},
		{"file@#$%name", "filename"},
		{"---leading-trailing---", "leading-trailing"},
		{"...dotty...", "dotty"},
		{"multi---hyphen", "multi-hyphen"},
		{"under__score", "under_score"},
		{"", "unnamed"},
		{"@#$%", "unnamed"},
		{"2024-03-15_photo", "2024-03-15_photo"},
		{"Fujifilm X-T5", "Fujifilm-X-T5"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
