package media

import (
	"bytes"
	"image/jpeg"
	"os"
	"testing"
	"time"
)

// examplePortraitJPEG is a real Fujifilm JPEG whose EXIF orientation tag says
// "Rotate 270 CW" while its stored pixel array remains in the camera's native
// landscape shape (7728x5152) — i.e. genuinely unbaked, the opposite of what
// heif-convert produces. Used here only as a convenient real source of JPEG
// bytes carrying a non-1 orientation tag.
const examplePortraitJPEG = "../../examples/folder-a/a3/2025-08-26_00-09-23_X-T50_DSF1756.jpg"

const exampleNormalJPEG = "../../examples/folder-a/a3/_DSF1321.jpg"

// TestStripStaleHeifConvertOrientationRemovesTagWithoutRotating is a
// regression test for a double-rotation bug: heif-convert bakes the correct
// display rotation into its own output's pixels — even for Fujifilm-style
// HEIC/HIF files with no irot box, whose primary image plane it decodes
// already display-ready — but copies the source file's stale EXIF
// orientation tag into the output JPEG unchanged. Reapplying that tag rotates
// an already-correct image a second time. This verifies the fix strips the
// tag without touching pixel dimensions (i.e. without rotating).
func TestStripStaleHeifConvertOrientationRemovesTagWithoutRotating(t *testing.T) {
	data, err := os.ReadFile(examplePortraitJPEG)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if ori := extractJPEGOrientation(data); ori != 8 {
		t.Fatalf("fixture precondition failed: orientation = %d, want 8 (Rotate 270 CW)", ori)
	}
	origCfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode fixture config: %v", err)
	}

	result := stripStaleHeifConvertOrientation(data)

	if ori := extractJPEGOrientation(result); ori > 1 {
		t.Errorf("orientation tag still present after strip: %d", ori)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(result))
	if err != nil {
		t.Fatalf("decode stripped result: %v", err)
	}
	if cfg.Width != origCfg.Width || cfg.Height != origCfg.Height {
		t.Errorf("dimensions changed from %dx%d to %dx%d — a rotation was applied when none should be",
			origCfg.Width, origCfg.Height, cfg.Width, cfg.Height)
	}
}

// TestStripStaleHeifConvertOrientationNoOpWhenAlreadyNormal verifies files
// with no orientation tag (heif-convert's common-case output) pass through
// byte-for-byte, avoiding needless re-encoding/quality loss.
func TestStripStaleHeifConvertOrientationNoOpWhenAlreadyNormal(t *testing.T) {
	data, err := os.ReadFile(exampleNormalJPEG)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if ori := extractJPEGOrientation(data); ori != 1 {
		t.Fatalf("fixture precondition failed: orientation = %d, want 1", ori)
	}

	result := stripStaleHeifConvertOrientation(data)
	if !bytes.Equal(result, data) {
		t.Error("expected byte-identical passthrough when orientation is already normal")
	}
}

// TestCommandWithTimeoutKillsHungProcess is a regression test for a hang
// risk: every external decoder/converter call (ffmpeg, sips, heif-convert,
// exiftool, cwebp) used to run via bare exec.Command with no deadline, so a
// malformed input that made one of them hang would block the calling HTTP
// handler goroutine forever. Verifies commandWithTimeout actually bounds
// execution instead of just plumbing a context through unused.
func TestCommandWithTimeoutKillsHungProcess(t *testing.T) {
	old := externalCmdTimeout
	externalCmdTimeout = 50 * time.Millisecond
	defer func() { externalCmdTimeout = old }()

	cmd, cancel := commandWithTimeout("sleep", "5")
	defer cancel()

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected the timeout to kill `sleep 5`, but it exited cleanly")
	}
	if elapsed > 2*time.Second {
		t.Errorf("command took %v to be killed, want well under the 5s sleep duration", elapsed)
	}
}
