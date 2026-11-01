package library

import (
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"huepattl.de/unterlumen/internal/onnx"
)

func fillDir(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveModelsWaitsForARunningPass(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fillDir(t, m.RecognitionDir(), "model.onnx")

	m.faces.running.Store("lib", new(atomic.Bool))
	if err := m.RemoveRecognition(); !errors.Is(err, ErrModelsBusy) {
		t.Fatalf("removing during a face pass: %v, want ErrModelsBusy", err)
	}
	m.faces.running.Delete("lib")
	if err := m.RemoveRecognition(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(m.RecognitionDir()); !os.IsNotExist(err) {
		t.Error("the face models are still there")
	}
}

func TestRemoveRuntimeWhenNothingNeedsIt(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	fillDir(t, m.RuntimeDir(), onnx.LibraryFile())
	if err := m.RemoveRuntime(); err != nil {
		t.Fatal(err)
	}
	if m.runtimeInstalled() {
		t.Error("ONNX Runtime is still there")
	}
}
