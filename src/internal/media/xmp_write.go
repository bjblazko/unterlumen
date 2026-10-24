package media

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/atomicfile"
)

// ErrPhotoGone is returned for a sidecar change of a photo that is not at
// its path: renamed or moved since the index last saw it.
var ErrPhotoGone = errors.New("the photo is not in its folder any more")

// changeSidecar is the one way a sidecar is written: it reads the sidecar
// beside photoPath, lets change make the new content from what is there now
// (nil data when there is none; nil out for nothing to write), and puts the
// result in place whole through atomicfile.Change. A scan on the other
// installation then never reads half a sidecar, and what that installation
// wrote in the meantime stays.
//
// A sidecar belongs beside its photo. Without the photo nothing is written:
// a pass working from an index that still had a renamed photo's old name
// left sidecars under names no photo has, beside the photos renamed with
// theirs.
func changeSidecar(photoPath string, change func(data []byte) ([]byte, error)) error {
	if info, err := os.Stat(photoPath); err != nil || info.IsDir() {
		return fmt.Errorf("%s: %w; scan the library to find it again", filepath.Base(photoPath), ErrPhotoGone)
	}
	return atomicfile.Change(SidecarPath(photoPath), change)
}
