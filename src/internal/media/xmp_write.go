package media

import "huepattl.de/unterlumen/internal/atomicfile"

// changeSidecar is the one way a sidecar is written: it reads the sidecar
// beside photoPath, lets change make the new content from what is there now
// (nil data when there is none; nil out for nothing to write), and puts the
// result in place whole through atomicfile.Change. A scan on the other
// installation then never reads half a sidecar, and what that installation
// wrote in the meantime stays.
func changeSidecar(photoPath string, change func(data []byte) ([]byte, error)) error {
	return atomicfile.Change(SidecarPath(photoPath), change)
}
