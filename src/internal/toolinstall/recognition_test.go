package toolinstall

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func tgz(t *testing.T, name, body string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: "other.txt", Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})         //nolint:errcheck
	tw.Write([]byte("x"))                                                                               //nolint:errcheck
	tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}) //nolint:errcheck
	tw.Write([]byte(body))                                                                              //nolint:errcheck
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func TestArchiveEntryFindsTheFileInATgzAndAZip(t *testing.T) {
	r, err := archiveEntry(tgz(t, "pkg/lib/libonnxruntime.so.1", "elf"), "pkg/lib/libonnxruntime.so.1", false)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(r); string(got) != "elf" {
		t.Fatalf("got %q", got)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("pkg/lib/onnxruntime.dll")
	w.Write([]byte("pe")) //nolint:errcheck
	zw.Close()
	r, err = archiveEntry(buf.Bytes(), "pkg/lib/onnxruntime.dll", true)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := io.ReadAll(r); string(got) != "pe" {
		t.Fatalf("got %q", got)
	}
	if _, err := archiveEntry(buf.Bytes(), "missing.dll", true); err == nil {
		t.Fatal("a missing entry was found")
	}
}

func TestFetchIntoRefusesAFileWithTheWrongChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("model")) })) //nolint:errcheck
	defer srv.Close()
	dir := t.TempDir()
	if err := fetchInto(context.Background(), download{url: srv.URL, sha256: "00"}, dir, "m.onnx"); err == nil {
		t.Fatal("a file with the wrong checksum was written")
	}
	if _, err := os.Stat(filepath.Join(dir, "m.onnx")); err == nil {
		t.Fatal("the refused file is on disk")
	}
	sum := sha256.Sum256([]byte("model"))
	if err := fetchInto(context.Background(), download{url: srv.URL, sha256: hex.EncodeToString(sum[:])}, dir, "m.onnx"); err != nil {
		t.Fatal(err)
	}
}

func TestPlanRecognitionCountsEveryDownload(t *testing.T) {
	p := PlanRecognition()
	if p.Supported && p.Bytes < 38_000_000 {
		t.Fatalf("plan %+v counts less than the recognition model alone", p)
	}
}
