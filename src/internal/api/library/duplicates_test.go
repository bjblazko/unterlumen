package apilibrary

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSidecarSharedWithARawBeside(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a [1].jpg", "a [1].raf", "a [1].xmp", "b.jpg", "b.xmp"} {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if !sidecarShared(filepath.Join(dir, "a [1].jpg")) {
		t.Error("a [1].jpg shares its sidecar with a [1].raf")
	}
	if sidecarShared(filepath.Join(dir, "b.jpg")) {
		t.Error("b.jpg has its sidecar to itself")
	}
}
