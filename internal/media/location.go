package media

import (
	"bytes"
	"fmt"
	"math"
	"os/exec"
	"sync"
)

var (
	hasExiftool     bool
	exiftoolChecked sync.Once
)

// CheckExiftool returns true if exiftool is available on the system.
func CheckExiftool() bool {
	exiftoolChecked.Do(func() {
		path, err := exec.LookPath("exiftool")
		hasExiftool = err == nil && path != ""
	})
	return hasExiftool
}

// WriteGPSLocation writes GPS coordinates to the image file at absPath using exiftool.
// Existing EXIF data (maker notes, etc.) is preserved.
func WriteGPSLocation(absPath string, lat, lon float64) error {
	if !CheckExiftool() {
		return fmt.Errorf("exiftool is not available")
	}

	latRef := "N"
	if lat < 0 {
		latRef = "S"
	}
	lonRef := "E"
	if lon < 0 {
		lonRef = "W"
	}

	var stderr bytes.Buffer
	cmd := exec.Command("exiftool",
		fmt.Sprintf("-GPSLatitude=%f", math.Abs(lat)),
		fmt.Sprintf("-GPSLatitudeRef=%s", latRef),
		fmt.Sprintf("-GPSLongitude=%f", math.Abs(lon)),
		fmt.Sprintf("-GPSLongitudeRef=%s", lonRef),
		"-overwrite_original",
		absPath,
	)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exiftool GPS write failed: %v: %s", err, stderr.String())
	}
	return nil
}
