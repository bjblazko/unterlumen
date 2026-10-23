package library

import (
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/onnx"
)

// RuntimeDir is where this installation keeps ONNX Runtime, which face
// recognition and things share (ADR-0054).
func (m *Manager) RuntimeDir() string {
	return filepath.Join(m.root, "onnxruntime")
}

// runtimeLib is the ONNX Runtime library in RuntimeDir.
func (m *Manager) runtimeLib() string {
	return filepath.Join(m.RuntimeDir(), onnx.LibraryFile())
}

func (m *Manager) runtimeInstalled() bool {
	_, err := os.Stat(m.runtimeLib())
	return err == nil
}

// moveRuntime moves ONNX Runtime out of the recognition folder, where
// versions before things kept it, into RuntimeDir. A failed move leaves it
// where it was and faces wait for a new download.
func (m *Manager) moveRuntime() {
	old := filepath.Join(m.RecognitionDir(), onnx.LibraryFile())
	if _, err := os.Stat(old); err != nil || m.runtimeInstalled() {
		return
	}
	if os.MkdirAll(m.RuntimeDir(), 0o755) == nil {
		os.Rename(old, m.runtimeLib()) //nolint:errcheck
	}
}
