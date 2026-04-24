package pathguard_test

import (
	"os"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/pathguard"
)

func TestSafePath(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	os.Mkdir(sub, 0755)

	tests := []struct {
		name     string
		relative string
		wantOK   bool
	}{
		{"empty relative returns root", "", true},
		{"valid file in root", "photo.jpg", true},
		{"valid nested path", "sub/../photo.jpg", true},
		{"traversal attempt", "../escaped.jpg", false},
		{"absolute path rejected", "/etc/passwd", false},
		{"double-dot traversal", "../../etc/passwd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := pathguard.SafePath(root, tt.relative)
			if ok != tt.wantOK {
				t.Errorf("SafePath(%q, %q) ok=%v, want %v", root, tt.relative, ok, tt.wantOK)
			}
		})
	}
}

func TestSafePath_ExistingFile(t *testing.T) {
	root := t.TempDir()
	f, _ := os.Create(filepath.Join(root, "photo.jpg"))
	f.Close()

	got, ok := pathguard.SafePath(root, "photo.jpg")
	if !ok {
		t.Fatal("expected ok=true for existing file")
	}
	if got == "" {
		t.Fatal("expected non-empty path")
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}

func TestSafePath_NonExistentDestination(t *testing.T) {
	root := t.TempDir()
	// A non-existent destination (e.g. copy target) should be accepted
	// as long as the parent directory is within root.
	_, ok := pathguard.SafePath(root, "newfile.jpg")
	if !ok {
		t.Error("expected ok=true for non-existent file within root")
	}
}
