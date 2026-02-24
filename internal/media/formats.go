package media

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var supportedExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".heif": true,
	".heic": true,
	".hif":  true,
}

func IsSupportedImage(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return supportedExtensions[ext]
}

func IsHEIF(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".heif" || ext == ".heic" || ext == ".hif"
}

// FFmpegStatus describes the availability of ffmpeg and its HEIF support.
type FFmpegStatus struct {
	Available    bool
	HEIFSupport  bool
	ErrorMessage string
}

var (
	ffmpegStatus     FFmpegStatus
	ffmpegStatusOnce sync.Once
)

// CheckFFmpeg checks whether ffmpeg is installed and supports HEIF decoding.
// The result is cached for the lifetime of the process.
func CheckFFmpeg() FFmpegStatus {
	ffmpegStatusOnce.Do(func() {
		path, err := exec.LookPath("ffmpeg")
		if err != nil || path == "" {
			ffmpegStatus = FFmpegStatus{
				Available:    false,
				ErrorMessage: "ffmpeg is not installed. HEIF/HEIC files cannot be displayed. Install ffmpeg (https://ffmpeg.org) to enable HEIF support.",
			}
			return
		}

		ffmpegStatus.Available = true

		var out bytes.Buffer
		cmd := exec.Command("ffmpeg", "-decoders")
		cmd.Stdout = &out
		cmd.Stderr = &bytes.Buffer{}
		if err := cmd.Run(); err != nil {
			ffmpegStatus.HEIFSupport = false
			ffmpegStatus.ErrorMessage = "ffmpeg is installed but its decoder list could not be checked. HEIF/HEIC files may not display correctly."
			return
		}

		if strings.Contains(out.String(), "hevc") {
			ffmpegStatus.HEIFSupport = true
		} else {
			ffmpegStatus.HEIFSupport = false
			ffmpegStatus.ErrorMessage = "ffmpeg is installed but lacks HEVC/HEIF decoder support. HEIF/HEIC files cannot be displayed. Reinstall ffmpeg with HEIF support (e.g. 'brew install ffmpeg' on macOS or install libheif/libde265)."
		}
	})

	return ffmpegStatus
}

// --- Disk cache in OS temp directory ---

var (
	cacheDir     string
	cacheDirOnce sync.Once
)

// getCacheDir returns the cache directory, creating it if needed.
func getCacheDir() string {
	cacheDirOnce.Do(func() {
		cacheDir = filepath.Join(os.TempDir(), "unterlumen-cache")
		os.MkdirAll(cacheDir, 0700)
	})
	return cacheDir
}

// cacheKey returns a unique filename for a source file + purpose.
func cacheKey(path string, purpose string) string {
	info, err := os.Stat(path)
	var modStr string
	if err == nil {
		modStr = info.ModTime().String()
	}
	h := sha256.Sum256([]byte(path + "|" + modStr + "|" + purpose))
	return fmt.Sprintf("%x.jpg", h[:12])
}

// readCache returns cached bytes or nil.
func readCache(key string) []byte {
	data, err := os.ReadFile(filepath.Join(getCacheDir(), key))
	if err != nil {
		return nil
	}
	return data
}

// writeCache stores bytes in the cache.
func writeCache(key string, data []byte) {
	os.WriteFile(filepath.Join(getCacheDir(), key), data, 0600)
}

// --- HEIF conversion ---

// ConvertHEIFToJPEG extracts the best available JPEG from a HEIF file.
// It prefers the embedded full-frame JPEG preview (stream copy, fast),
// falls back to sips (macOS) for reliable multi-tile HEIF support,
// and finally to HEVC decoding for simple HEIF files without previews.
// Results are cached to disk.
func ConvertHEIFToJPEG(path string) ([]byte, error) {
	key := cacheKey(path, "full-v3")
	if cached := readCache(key); cached != nil {
		return cached, nil
	}

	data, err := extractBestJPEG(path)
	if err != nil {
		return nil, err
	}

	// Apply HEIF container rotation (irot box)
	orientation := ExtractHEIFOrientation(path)
	data, _ = applyOrientationJPEG(data, orientation, 92)

	writeCache(key, data)
	return data, nil
}

// ExtractHEIFPreview extracts a JPEG preview suitable for thumbnails.
// Uses the embedded preview or falls back to conversion.
// Results are cached to disk.
func ExtractHEIFPreview(path string) ([]byte, error) {
	key := cacheKey(path, "preview-v3")
	if cached := readCache(key); cached != nil {
		return cached, nil
	}

	data, err := extractBestJPEG(path)
	if err != nil {
		return nil, err
	}

	// Apply HEIF container rotation (irot box)
	orientation := ExtractHEIFOrientation(path)
	data, _ = applyOrientationJPEG(data, orientation, 80)

	writeCache(key, data)
	return data, nil
}

// extractBestJPEG probes a HEIF file and extracts the best available JPEG.
// Priority: largest embedded JPEG preview (stream copy) > HEVC decode fallback.
func extractBestJPEG(path string) ([]byte, error) {
	// Probe for embedded JPEG streams
	probeOut := ffmpegProbe(path)

	// Find the largest embedded JPEG preview stream (not 160x120 thumbnails)
	bestStream := -1
	for _, line := range strings.Split(probeOut, "\n") {
		if strings.Contains(line, "mjpeg") && strings.Contains(line, "Stream #0:") {
			idx := parseStreamIndex(line)
			if idx >= 0 && !strings.Contains(line, "160x") {
				bestStream = idx
				break
			}
		}
	}

	// Extract embedded JPEG via stream copy (instant, no re-encode)
	if bestStream >= 0 {
		data, err := ffmpegRun(path,
			"-map", fmt.Sprintf("0:%d", bestStream),
			"-c", "copy",
			"-f", "image2pipe",
			"pipe:1",
		)
		if err == nil && len(data) > 0 {
			return data, nil
		}
	}

	// Try sips (macOS) — handles multi-tile HEIF reliably via native decoder
	if data, err := sipsConvert(path); err == nil && len(data) > 0 {
		return data, nil
	}

	// Fallback: decode HEVC to JPEG (for simple HEIF/HEIC without embedded previews)
	return ffmpegRun(path,
		"-f", "image2pipe",
		"-vcodec", "mjpeg",
		"-q:v", "2",
		"-frames:v", "1",
		"pipe:1",
	)
}

// sipsConvert uses macOS sips to convert HEIF to JPEG.
// sips uses Apple's native HEIF decoder which correctly assembles multi-tile grids.
func sipsConvert(path string) ([]byte, error) {
	tmp, err := os.CreateTemp("", "unterlumen-sips-*.jpg")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	var stderr bytes.Buffer
	cmd := exec.Command("sips", "-s", "format", "jpeg", "-s", "formatOptions", "92", path, "--out", tmpPath)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("sips failed: %v: %s", err, stderr.String())
	}

	return os.ReadFile(tmpPath)
}

func ffmpegRun(inputPath string, args ...string) ([]byte, error) {
	var out bytes.Buffer
	var stderr bytes.Buffer

	cmdArgs := make([]string, 0, len(args)+3)
	cmdArgs = append(cmdArgs, "-i", inputPath)
	cmdArgs = append(cmdArgs, args...)

	cmd := exec.Command("ffmpeg", cmdArgs...)
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %v: %s", err, stderr.String())
	}

	return out.Bytes(), nil
}

func ffmpegProbe(path string) string {
	var stderr bytes.Buffer
	cmd := exec.Command("ffmpeg", "-i", path)
	cmd.Stderr = &stderr
	cmd.Run() // always exits non-zero (no output file)
	return stderr.String()
}

func parseStreamIndex(line string) int {
	idx := strings.Index(line, "Stream #0:")
	if idx < 0 {
		return -1
	}
	rest := line[idx+len("Stream #0:"):]
	n := 0
	for _, c := range rest {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
