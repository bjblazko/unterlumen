// Package atomicfile changes a file that several installations write — a
// sidecar, the shared birthdays — so that a reader sees the whole old or
// the whole new file, never half of one, and a change made by another
// writer in the meantime is kept. A lock across SMB or NFS is neither
// reliable nor released by an app that is killed, so there is none.
package atomicfile

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// attempts is how often Change starts again when the file keeps changing
// under it before it gives up.
const attempts = 5

// ErrBusy says the file changed under every attempt to change it.
var ErrBusy = errors.New("the file kept changing while it was written; try again")

// Change reads the file at path and lets change make the new content from
// what is there now — nil data when there is no file; nil out for "nothing
// to write". The result is written beside the file under a hidden name and
// renamed over it. If the file changed while the new one was made, it
// starts again from the changed one.
func Change(path string, change func(data []byte) ([]byte, error)) error {
	for range attempts {
		data, mode, err := read(path)
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
		if now, _, err := read(path); err != nil || !same(now, data) {
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
	return fmt.Errorf("%s: %w", path, ErrBusy)
}

// read returns the file's content and mode; nil content and 0o644 when
// there is none.
func read(path string) ([]byte, fs.FileMode, error) {
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

// same tells whether two reads found the same file, a missing one included.
func same(a, b []byte) bool {
	return (a == nil) == (b == nil) && bytes.Equal(a, b)
}

// writeBeside writes data into a new hidden file in path's folder and
// returns its name. The name does not end in the file's extension, so a
// scan passing by never takes it for the file.
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
