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
	"path/filepath"
	"runtime"
	"strings"

	"huepattl.de/unterlumen/internal/faces"
)

// Face recognition needs ONNX Runtime and two models (ADR-0051). They come
// from their makers — Microsoft's releases on GitHub, OpenCV's model zoo on
// Hugging Face — at fixed versions, and each download is checked against
// the checksum recorded here before anything is written.

const ortVersion = "1.31.0"

type download struct {
	url, sha256 string
	entry       string // the file to take from an archive; "" for a plain file
	size        int64  // bytes, for the page to say before it is done
}

var ortDownloads = map[string]download{
	"darwin/arm64": {
		url:    ortRelease("onnxruntime-osx-arm64-" + ortVersion + ".tgz"),
		sha256: "1031b36dd3bdaa8d976a6130d9cf2a05e9be320b93ae804f8427b91dc3432afc",
		entry:  "./onnxruntime-osx-arm64-" + ortVersion + "/lib/libonnxruntime." + ortVersion + ".dylib",
		size:   36549009,
	},
	"linux/amd64": {
		url:    ortRelease("onnxruntime-linux-x64-" + ortVersion + ".tgz"),
		sha256: "cc5c72baf5ae5c8238a6841f0897227be2d02826b9cf98eaf02fdefaeeb03a57",
		entry:  "onnxruntime-linux-x64-" + ortVersion + "/lib/libonnxruntime.so." + ortVersion,
		size:   11408003,
	},
	"linux/arm64": {
		url:    ortRelease("onnxruntime-linux-aarch64-" + ortVersion + ".tgz"),
		sha256: "c5b8b3cca31f3d643a3b313b8f42e2d0f16a4fcefd02cda343d97bd5306afaab",
		entry:  "onnxruntime-linux-aarch64-" + ortVersion + "/lib/libonnxruntime.so." + ortVersion,
		size:   10331104,
	},
	"windows/amd64": {
		url:    ortRelease("onnxruntime-win-x64-" + ortVersion + ".zip"),
		sha256: "3958d8a44984160692c1b4ffd0de3cb48f8c39f9d34bb31e433db06e30b82195",
		entry:  "onnxruntime-win-x64-" + ortVersion + "/lib/onnxruntime.dll",
		size:   82887349,
	},
}

func ortRelease(file string) string {
	return "https://github.com/microsoft/onnxruntime/releases/download/v" + ortVersion + "/" + file
}

// zooRevision pins OpenCV's model zoo on Hugging Face to one commit.
const zooRevision = "d4938dfc9d4ec5d098bfa33e98b3f3345a236586"

var modelDownloads = map[string]download{
	faces.DetectorFile: {
		url:    zooModel("face_detection_yunet/" + faces.DetectorFile),
		sha256: "8f2383e4dd3cfbb4553ea8718107fc0423210dc964f9f4280604804ed2552fa4",
		size:   232589,
	},
	faces.RecognizerFile: {
		url:    zooModel("face_recognition_sface/" + faces.RecognizerFile),
		sha256: "0ba9fbfa01b5270c96627c4ef784da859931e02f04419c829e83484087c34e79",
		size:   38696353,
	},
}

func zooModel(path string) string {
	return "https://huggingface.co/opencv/opencv_zoo/resolve/" + zooRevision + "/models/" + path
}

// RecognitionPlan says whether face recognition can be installed on this
// system and how much it downloads.
type RecognitionPlan struct {
	Supported bool  `json:"supported"`
	Bytes     int64 `json:"bytes"`
}

func PlanRecognition() RecognitionPlan {
	ort, ok := ortDownloads[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return RecognitionPlan{}
	}
	total := ort.size
	for _, m := range modelDownloads {
		total += m.size
	}
	return RecognitionPlan{Supported: true, Bytes: total}
}

// InstallRecognition downloads ONNX Runtime and the face models into dir.
// The files go into a folder beside it first and take its place only when
// all are there and checked, so an interrupted download leaves nothing half done.
func InstallRecognition(ctx context.Context, report Reporter, dir string) error {
	ort, ok := ortDownloads[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return fmt.Errorf("ONNX Runtime %s has no build for %s on %s", ortVersion, runtime.GOARCH, runtime.GOOS)
	}
	tmp := dir + ".download"
	os.RemoveAll(tmp) //nolint:errcheck
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck
	report.Step("Downloading ONNX Runtime " + ortVersion + "…")
	if err := fetchInto(ctx, ort, tmp, faces.RuntimeFile()); err != nil {
		return fmt.Errorf("ONNX Runtime: %w", err)
	}
	for _, name := range []string{faces.DetectorFile, faces.RecognizerFile} {
		report.Step("Downloading " + name + "…")
		if err := fetchInto(ctx, modelDownloads[name], tmp, name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
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

// RemoveRecognition deletes what InstallRecognition put into dir.
func RemoveRecognition(dir string) error {
	if filepath.Base(dir) != "recognition" {
		return fmt.Errorf("%s is not a recognition folder", dir)
	}
	return os.RemoveAll(dir)
}
