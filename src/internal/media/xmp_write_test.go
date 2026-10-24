package media

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestASidecarIsNotWrittenWithoutItsPhoto(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "DSCF3052.HEIC")
	err := changeSidecar(gone, func([]byte) ([]byte, error) { return []byte("<x/>"), nil })
	if !errors.Is(err, ErrPhotoGone) {
		t.Fatalf("err = %v, want ErrPhotoGone", err)
	}
	if _, err := os.Stat(SidecarPath(gone)); !os.IsNotExist(err) {
		t.Fatal("a sidecar was written beside no photo")
	}

	here := filepath.Join(dir, "DSCF3054.HEIC")
	os.WriteFile(here, []byte("photo"), 0o644) //nolint:errcheck
	if err := changeSidecar(here, func([]byte) ([]byte, error) { return []byte("<x/>"), nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SidecarPath(here)); err != nil {
		t.Fatal("the photo's sidecar was not written")
	}
}

// existingPhoto puts an empty file where a test's photo is, since a sidecar
// is written only beside a photo that is there.
func existingPhoto(t *testing.T, path string) string {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
