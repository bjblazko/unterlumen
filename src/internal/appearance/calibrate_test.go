//go:build calibrate

package appearance

// Prints the measurements of real photos, to set the thresholds against:
//
//	cd src && CALIBRATE_DIR=../e2e/fixtures/photos go test -tags calibrate -run Calibrate -v ./internal/appearance/

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCalibrate(t *testing.T) {
	dir := os.Getenv("CALIBRATE_DIR")
	if dir == "" {
		t.Skip("set CALIBRATE_DIR to a folder of JPEGs")
	}
	var total time.Duration
	n := 0
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		ext := strings.ToLower(filepath.Ext(p))
		if err != nil || d.IsDir() || (ext != ".jpg" && ext != ".jpeg") {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		start := time.Now()
		r, err := AnalyseJPEG(f)
		if err != nil {
			return nil
		}
		total += time.Since(start)
		n++
		fmt.Printf("%-48.48s %-6s tint=%3.0f C=%5.1f warm=%5.2f key=%-6s med=%.2f con=%.2f ent=%.1f sharp=%6.0f edge=%.3f pal=%s\n",
			filepath.Base(p), r.Mono, r.TintHue, r.Colourfulness, r.Warmth, r.Tone.Key,
			r.Tone.Median, r.Tone.Contrast, r.Texture.Entropy, r.Texture.Sharpness, r.Texture.EdgeDensity, paletteString(r.Palette))
		return nil
	})
	if n > 0 {
		fmt.Printf("%d photos, %v per photo (decode of the original included)\n", n, total/time.Duration(n))
	}
}

func paletteString(p []Swatch) string {
	var b strings.Builder
	for _, s := range p {
		fmt.Fprintf(&b, " %3.0f°/%.2f/%2.0f%%", s.Colour.Hue(), s.Colour.Chroma(), s.Share*100)
	}
	return b.String()
}
