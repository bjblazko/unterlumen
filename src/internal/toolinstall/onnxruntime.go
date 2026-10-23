package toolinstall

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"huepattl.de/unterlumen/internal/onnx"
)

const ortVersion = "1.31.0"

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

// RuntimeInstalled reports whether runtimeDir holds ONNX Runtime.
func RuntimeInstalled(runtimeDir string) bool {
	_, err := os.Stat(filepath.Join(runtimeDir, onnx.LibraryFile()))
	return err == nil
}

// installRuntime downloads ONNX Runtime into runtimeDir unless it is there.
func installRuntime(ctx context.Context, report Reporter, runtimeDir string) error {
	if RuntimeInstalled(runtimeDir) {
		return nil
	}
	ort, ok := ortDownloads[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		return fmt.Errorf("ONNX Runtime %s has no build for %s on %s", ortVersion, runtime.GOARCH, runtime.GOOS)
	}
	tmp := runtimeDir + ".download"
	os.RemoveAll(tmp) //nolint:errcheck
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tmp) //nolint:errcheck
	report.Step("Downloading ONNX Runtime " + ortVersion + "…")
	if err := fetchInto(ctx, ort, tmp, onnx.LibraryFile()); err != nil {
		return fmt.Errorf("ONNX Runtime: %w", err)
	}
	os.RemoveAll(runtimeDir) //nolint:errcheck
	return os.Rename(tmp, runtimeDir)
}
