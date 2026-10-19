package library

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// errIncomplete means a walk could not read every folder or file below its
// root: the files it returns are real, but a file missing from them may
// only be unreadable for now — a share that dropped, a folder without
// permission. Whoever removes photos for files not found must not act on
// such a walk.
var errIncomplete = errors.New("some folders could not be read")

// collectFiles lists the supported photos below root. A root that cannot be
// read — a share that is not mounted — is an error and returns no files;
// anything below it that cannot be read makes the result incomplete
// (errIncomplete) but keeps what was found.
func collectFiles(root string) ([]string, error) {
	files, _, err := collectFilesAndSidecars(root)
	return files, err
}

// collectFilesAndSidecars is collectFiles that also notes the sidecars it
// passes, by lower-cased path (sidecarKey), so a scan need not ask the disk
// — a round trip each on a network share — for the sidecar of every photo.
func collectFilesAndSidecars(root string) ([]string, map[string]bool, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("%s is not a folder", root)
	}
	var files []string
	sidecars := map[string]bool{}
	var unreadable error
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err
			}
			if unreadable == nil {
				unreadable = fmt.Errorf("%w: %s: %v", errIncomplete, path, err)
			}
			return nil
		}
		switch {
		case d.IsDir():
		case media.IsSupportedImage(d.Name()):
			files = append(files, path)
		case strings.EqualFold(filepath.Ext(path), ".xmp"):
			sidecars[sidecarKey(path)] = true
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return files, sidecars, unreadable
}

// sidecarKey is how collectFilesAndSidecars notes a sidecar: shares and the
// Mac's disks match names without regard to case.
func sidecarKey(path string) string { return strings.ToLower(path) }

// walkFailed says err stops a run that only adds or refreshes photos: an
// incomplete walk does not, since every file it found is real.
func walkFailed(err error) bool {
	return err != nil && !errors.Is(err, errIncomplete)
}

func fileSet(files []string) map[string]bool {
	set := make(map[string]bool, len(files))
	for _, f := range files {
		set[f] = true
	}
	return set
}
