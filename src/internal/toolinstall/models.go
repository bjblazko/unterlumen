package toolinstall

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

// Face recognition and things each need ONNX Runtime and their models
// (ADR-0051, ADR-0054). They come from their makers — Microsoft's releases
// on GitHub, the models' own pages on Hugging Face — at fixed versions, and
// each download is checked against the checksum recorded here before
// anything is written.

type download struct {
	url, sha256 string
	entry       string // the file to take from an archive; "" for a plain file
	size        int64  // bytes, for the page to say before it is done
}

// A model is one file of a models folder and where it comes from.
type model struct {
	name string
	download
}

// ModelPlan says whether a set of models can be installed on this system
// and how much it downloads, ONNX Runtime included when it is not there yet.
type ModelPlan struct {
	Supported bool  `json:"supported"`
	Bytes     int64 `json:"bytes"`
}

func planModels(runtimeDir string, models []model) ModelPlan {
	ort, ok := ortDownloads[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return ModelPlan{}
	}
	var total int64
	if !RuntimeInstalled(runtimeDir) {
		total = ort.size
	}
	for _, m := range models {
		total += m.size
	}
	return ModelPlan{Supported: true, Bytes: total}
}

// installModels makes sure ONNX Runtime is in runtimeDir, then downloads
// models into dir. The files go into a folder beside dir first and take its
// place only when all are there and checked, so an interrupted download
// leaves nothing half done.
func installModels(ctx context.Context, report Reporter, runtimeDir, dir string, models []model) error {
	if err := installRuntime(ctx, report, runtimeDir); err != nil {
		return err
	}
	tmp := dir + ".download"
	os.RemoveAll(tmp) //nolint:errcheck
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck
	for _, m := range models {
		report.Step("Downloading " + m.name + "…")
		if err := fetchInto(ctx, m.download, tmp, m.name); err != nil {
			return fmt.Errorf("%s: %w", m.name, err)
		}
	}
	os.RemoveAll(dir) //nolint:errcheck
	return os.Rename(tmp, dir)
}

// fetchInto downloads d, checks it, and writes it — or its entry — to dir/name.
func fetchInto(ctx context.Context, d download, dir, name string) error {
	data, err := fetch(ctx, d.url)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != d.sha256 {
		return fmt.Errorf("the download from %s is not the expected file (checksum %s)", d.url, got)
	}
	if d.entry == "" {
		return writeFile(dir, name, bytes.NewReader(data))
	}
	r, err := archiveEntry(data, d.entry, strings.HasSuffix(d.url, ".zip"))
	if err != nil {
		return err
	}
	return writeFile(dir, name, r)
}

// archiveEntry finds one file in a .tgz, or in a .zip.
func archiveEntry(data []byte, entry string, isZip bool) (io.Reader, error) {
	if !isZip {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if err != nil {
				return nil, fmt.Errorf("%s is not in the archive: %w", entry, err)
			}
			if h.Name == entry && h.Typeflag == tar.TypeReg {
				return tr, nil
			}
		}
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name == entry {
			return f.Open()
		}
	}
	return nil, fmt.Errorf("%s is not in the archive", entry)
}
