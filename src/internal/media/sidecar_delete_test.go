package media

import (
	"os"
	"path/filepath"
	"testing"
)

func touch(t *testing.T, dir string, names ...string) {
	t.Helper()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSidecarSharedWithARawBeside(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "a [1].jpg", "a [1].raf", "a [1].xmp", "b.jpg", "b.xmp")
	if !SidecarShared(filepath.Join(dir, "a [1].jpg")) {
		t.Error("a [1].jpg shares its sidecar with a [1].raf")
	}
	if SidecarShared(filepath.Join(dir, "b.jpg")) {
		t.Error("b.jpg has its sidecar to itself")
	}
}

func TestRemoveSidecarOfADeletedPhoto(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "a.xmp", "b.jpg", "b.raf", "b.xmp")
	// a.jpg was deleted; its sidecar goes with it.
	if err := RemoveSidecar(filepath.Join(dir, "a.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "a.xmp")); !os.IsNotExist(err) {
		t.Error("a.xmp should be gone with a.jpg")
	}
	os.Remove(filepath.Join(dir, "b.jpg"))
	if err := RemoveSidecar(filepath.Join(dir, "b.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "b.xmp")); err != nil {
		t.Error("b.xmp still belongs to b.raf")
	}
	if err := RemoveSidecar(filepath.Join(dir, "none.jpg")); err != nil {
		t.Errorf("no sidecar is no error: %v", err)
	}
}

// A video's NFO file goes with it: moved with it, and removed with it even
// when a still of the same name stays beside it.
func TestNFOTravelsWithItsVideo(t *testing.T) {
	dir := t.TempDir()
	video := filepath.Join(dir, "IMG_1.mov")
	still := filepath.Join(dir, "IMG_1.heic")
	for _, p := range []string{video, still, NFOPath(video)} {
		os.WriteFile(p, []byte("x"), 0o644) //nolint:errcheck
	}
	if SidecarShared(still) == false {
		t.Fatal("the still should share its name with the video")
	}
	moved := filepath.Join(dir, "clip.mov")
	if err := CarrySidecar(os.Rename, video, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(NFOPath(moved)); err != nil {
		t.Errorf("the NFO did not move with the video: %v", err)
	}
	if err := RemoveSidecar(moved); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(NFOPath(moved)); err == nil {
		t.Error("the NFO stayed after its video went")
	}
}
