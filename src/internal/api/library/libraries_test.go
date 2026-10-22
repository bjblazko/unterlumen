package apilibrary

import "testing"

// Folders asks where a folder is in the library that covers it, to show
// what the library knows of the folders there.
func TestPathInLibrary(t *testing.T) {
	for _, c := range []struct{ source, abs, want string }{
		{"/photos", "/photos", ""},
		{"/photos", "/photos/2024/rome", "2024/rome"},
		{"/photos/", "/photos/2024", "2024"},
	} {
		if got := pathInLibrary(c.source, c.abs); got != c.want {
			t.Errorf("pathInLibrary(%q, %q) = %q, want %q", c.source, c.abs, got, c.want)
		}
	}
}
