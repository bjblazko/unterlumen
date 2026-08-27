package pathguard_test

import (
	"os"
	"path/filepath"
	"strings"
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

	// Sibling directory whose name has root's basename as a string prefix
	// (e.g. root ".../photos", sibling ".../photosEVIL"). A relative path
	// that resolves into it must be rejected even though the resolved
	// string happens to start with resolvedRoot.
	siblingTests := []struct {
		name     string
		relative string
		wantOK   bool
	}{
		{"sibling dir sharing root's name as a prefix is rejected", "../" + filepath.Base(root) + "EVIL/secret.jpg", false},
	}
	siblingDir := root + "EVIL"
	os.Mkdir(siblingDir, 0755)
	f, _ := os.Create(filepath.Join(siblingDir, "secret.jpg"))
	f.Close()
	for _, tt := range siblingTests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := pathguard.SafePath(root, tt.relative)
			if ok != tt.wantOK {
				t.Errorf("SafePath(%q, %q) ok=%v, want %v", root, tt.relative, ok, tt.wantOK)
			}
		})
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

// TestSafePath_RootIsFilesystemRoot guards against a regression where root
// "/" made every path get rejected: the boundary check built its "is-inside"
// prefix by blindly appending a separator (resolvedRoot + "/"), which for
// root "/" produces "//" — a prefix of nothing, since no real path starts
// with a double slash. Root "/" is a real, deployed configuration (used
// when the app is started with no navigation restriction), not a
// theoretical edge case.
func TestSafePath_RootIsFilesystemRoot(t *testing.T) {
	nested := t.TempDir() // an absolute path guaranteed to exist under "/"
	resolvedNested, err := filepath.EvalSymlinks(nested)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", nested, err)
	}
	relative := strings.TrimPrefix(resolvedNested, string(filepath.Separator))

	got, ok := pathguard.SafePath("/", relative)
	if !ok {
		t.Fatalf("SafePath(\"/\", %q) ok=false, want true — root \"/\" must not reject every path", relative)
	}
	if got != resolvedNested {
		t.Errorf("SafePath(\"/\", %q) = %q, want %q", relative, got, resolvedNested)
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

func TestSafePathLogical(t *testing.T) {
	root := "/some/root"

	tests := []struct {
		name     string
		relative string
		want     string
		wantOK   bool
	}{
		{"empty relative returns root", "", root, true},
		{"valid nested path", "sub/photo.jpg", root + "/sub/photo.jpg", true},
		{"traversal attempt", "../escaped.jpg", "", false},
		{"absolute path rejected", "/etc/passwd", "", false},
		{"sibling dir sharing root's name as a prefix is rejected", "../rootEVIL/secret.jpg", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := pathguard.SafePathLogical(root, tt.relative)
			if ok != tt.wantOK {
				t.Fatalf("SafePathLogical(%q, %q) ok=%v, want %v", root, tt.relative, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("SafePathLogical(%q, %q) = %q, want %q", root, tt.relative, got, tt.want)
			}
		})
	}
}

// TestSafePathLogical_RootIsFilesystemRoot mirrors
// TestSafePath_RootIsFilesystemRoot for the logical (non-filesystem) variant.
func TestSafePathLogical_RootIsFilesystemRoot(t *testing.T) {
	got, ok := pathguard.SafePathLogical("/", "some/nested/path.jpg")
	if !ok {
		t.Fatal("SafePathLogical(\"/\", ...) ok=false, want true — root \"/\" must not reject every path")
	}
	if got != "/some/nested/path.jpg" {
		t.Errorf("SafePathLogical(\"/\", ...) = %q, want %q", got, "/some/nested/path.jpg")
	}
}
