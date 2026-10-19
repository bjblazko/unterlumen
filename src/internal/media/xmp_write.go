package media

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// sidecarAttempts is how often changeSidecar starts again when the sidecar
// keeps changing under it before it gives up.
const sidecarAttempts = 5

// errSidecarBusy says the sidecar changed under every attempt to change it.
var errSidecarBusy = errors.New("the sidecar kept changing while it was written; try again")

// changeSidecar is the one way a sidecar is written. It reads the sidecar
// beside photoPath and lets change make the new content from what is there
// now — nil data when there is no sidecar; nil out for "nothing to write".
// The result is written beside the sidecar under another name and renamed
// over it, so a reader — the other installation's scan — sees the whole old
// or the whole new file, never half of one. If the sidecar changed while the
// new one was made, it starts again from the changed one, so what another
// installation wrote in the meantime stays. Sidecars on a shared folder are
// written by several installations, and a lock across SMB or NFS is neither
// reliable nor released by an app that is killed.
func changeSidecar(photoPath string, change func(data []byte) ([]byte, error)) error {
	path := SidecarPath(photoPath)
	for range sidecarAttempts {
		data, mode, err := readSidecarFile(path)
		if err != nil {
			return err
		}
		out, err := change(data)
		if err != nil || out == nil {
			return err
		}
		tmp, err := writeBeside(path, out, mode)
		if err != nil {
			return err
		}
		if now, _, err := readSidecarFile(path); err != nil || !sameSidecar(now, data) {
			os.Remove(tmp) //nolint:errcheck
			if err != nil {
				return err
			}
			continue
		}
		if err := os.Rename(tmp, path); err != nil {
			os.Remove(tmp) //nolint:errcheck
			return err
		}
		return nil
	}
	return fmt.Errorf("%s: %w", path, errSidecarBusy)
}

// readSidecarFile returns the sidecar's content and mode; nil content and
// 0o644 when there is none.
func readSidecarFile(path string) ([]byte, fs.FileMode, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0o644, nil
	}
	if err != nil {
		return nil, 0, err
	}
	mode := fs.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		mode = fi.Mode().Perm()
	}
	return data, mode, nil
}

// sameSidecar tells whether two reads found the same sidecar, a missing one
// included.
func sameSidecar(a, b []byte) bool {
	return (a == nil) == (b == nil) && bytes.Equal(a, b)
}

// writeBeside writes data into a new hidden file in path's folder and
// returns its name. The name does not end in .xmp, so a scan passing by
// never takes it for a sidecar.
func writeBeside(path string, data []byte, mode fs.FileMode) (string, error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".unterlumen-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, mode)
	}
	if err != nil {
		os.Remove(tmp) //nolint:errcheck
		return "", err
	}
	return tmp, nil
}
