package pathguard

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInside(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "photos", "2024")
	os.MkdirAll(inner, 0o755)
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	os.Symlink(outside, link)

	cases := []struct {
		path string
		ok   bool
	}{
		{root, true},
		{inner, true},
		{filepath.Join(root, "photos", "..", "photos"), true},
		{outside, false},
		{link, false},
		{"/", false},
		{"photos", false},
		{filepath.Join(root, "missing"), false},
	}
	for _, c := range cases {
		if _, ok := Inside(root, c.path); ok != c.ok {
			t.Errorf("Inside(root, %q) = %v, want %v", c.path, ok, c.ok)
		}
	}
}
