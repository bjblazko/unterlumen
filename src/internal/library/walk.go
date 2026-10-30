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
// passes, by lower-cased path (sidecarKey), with their stamp, so a scan need
// not ask the disk — a round trip each on a network share — for the sidecar
// of every photo, nor read one that did not change.
func collectFilesAndSidecars(root string) ([]string, map[string]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, nil, err
	}
	if !info.IsDir() {
		return nil, nil, fmt.Errorf("%s is not a folder", root)
	}
	var files []string
	sidecars := map[string]string{}
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
		case media.IsSupportedMedia(d.Name()):
			files = append(files, path)
		case strings.EqualFold(filepath.Ext(path), ".xmp"):
			sidecars[sidecarKey(path)] = stampOf(d)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return withoutLiveClips(files), sidecars, unreadable
}

// withoutLiveClips leaves out the clips of Live Photos: a .mov beside a
// photo of the same name belongs to the photo, not to the library
// (ADR-0058).
func withoutLiveClips(files []string) []string {
	byDir := map[string][]string{}
	for _, f := range files {
		dir := filepath.Dir(f)
		byDir[dir] = append(byDir[dir], filepath.Base(f))
	}
	clips := map[string]bool{}
	for dir, names := range byDir {
		for clip := range media.LiveCompanions(names) {
			clips[filepath.Join(dir, clip)] = true
		}
	}
	if len(clips) == 0 {
		return files
	}
	kept := files[:0]
	for _, f := range files {
		if !clips[f] {
			kept = append(kept, f)
		}
	}
	return kept
}

// stampOf is a file's modification time and size, or "" when they cannot
// be read: a stamp that matches no other, so the file is read.
func stampOf(d os.DirEntry) string {
	info, err := d.Info()
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size())
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
