package toolinstall

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// fetch downloads url; the makers' servers redirect, which http follows.
func fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s answered %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func fetchText(ctx context.Context, url string) (string, error) {
	data, err := fetch(ctx, url)
	return strings.TrimSpace(string(data)), err
}

// fetchZip unpacks a zip into dir. With only set, just that entry is taken,
// under its own base name; otherwise every file, at its path in the archive.
func fetchZip(ctx context.Context, url, dir, only string) error {
	data, err := fetch(ctx, url)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || (only != "" && f.Name != only) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = writeFile(dir, entryName(f.Name, only, ""), rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// fetchTarGz unpacks a .tar.gz into dir. only names one entry to take, under
// its base name; a prefix ending in "/" takes that folder's content instead.
func fetchTarGz(ctx context.Context, url, dir, only string) error {
	data, err := fetch(ctx, url)
	if err != nil {
		return err
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	prefix := ""
	if strings.HasSuffix(only, "/") {
		prefix, only = only, ""
	}
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg || !strings.HasPrefix(h.Name, prefix) || (only != "" && h.Name != only) {
			continue
		}
		if err := writeFile(dir, entryName(h.Name, only, prefix), tr); err != nil {
			return err
		}
	}
}

// entryName is where an archive entry goes below the target folder.
func entryName(name, only, prefix string) string {
	if only != "" {
		return filepath.Base(name)
	}
	return filepath.FromSlash(strings.TrimPrefix(name, prefix))
}

// writeFile writes an unpacked file below dir, executable: these are
// programs, and exiftool's Perl modules do not mind the bit.
func writeFile(dir, name string, r io.Reader) error {
	path := filepath.Join(dir, name)
	if rel, err := filepath.Rel(dir, path); err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("archive entry %q leaves its folder", name)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
