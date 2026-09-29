package toolinstall

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"huepattl.de/unterlumen/internal/media"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(body)) //nolint:errcheck
	}
	zw.Close()
	return buf.Bytes()
}

func tarGzOf(t *testing.T, files map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}) //nolint:errcheck
		tw.Write([]byte(body))                                                                              //nolint:errcheck
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func serve(t *testing.T, body []byte) string {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(body) })) //nolint:errcheck
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestFetchZipTakesOneEntryUnderItsName(t *testing.T) {
	dir := t.TempDir()
	url := serve(t, zipOf(t, map[string]string{"libwebp/bin/cwebp.exe": "cwebp", "libwebp/README": "x"}))
	if err := fetchZip(context.Background(), url, dir, "libwebp/bin/cwebp.exe"); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "cwebp.exe")); string(data) != "cwebp" {
		t.Errorf("cwebp.exe = %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "README")); err == nil {
		t.Error("an entry that was not asked for was unpacked")
	}
}

func TestFetchTarGzTakesAFolderWithoutItsPrefix(t *testing.T) {
	dir := t.TempDir()
	url := serve(t, tarGzOf(t, map[string]string{
		"Image-ExifTool-13.59/exiftool":          "#!/usr/bin/env perl",
		"Image-ExifTool-13.59/lib/Image/Exif.pm": "1;",
		"other/file":                             "no",
	}))
	if err := fetchTarGz(context.Background(), url, dir, "Image-ExifTool-13.59/"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"exiftool", "lib/Image/Exif.pm"} {
		if _, err := os.Stat(filepath.Join(dir, want)); err != nil {
			t.Errorf("%s missing: %v", want, err)
		}
	}
	if info, _ := os.Stat(filepath.Join(dir, "exiftool")); info != nil && info.Mode()&0o100 == 0 {
		t.Error("exiftool is not executable")
	}
	if _, err := os.Stat(filepath.Join(dir, "other")); err == nil {
		t.Error("an entry outside the prefix was unpacked")
	}
}

func TestAnEntryCannotLeaveItsFolder(t *testing.T) {
	dir := t.TempDir()
	url := serve(t, tarGzOf(t, map[string]string{"../evil": "x"}))
	if err := fetchTarGz(context.Background(), url, dir, ""); err == nil {
		t.Error("an entry with ../ was written")
	}
}

func TestFetchReportsAFailedDownload(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if err := fetchZip(context.Background(), srv.URL, t.TempDir(), ""); err == nil {
		t.Error("a 404 was taken as a download")
	}
}

type steps []string

func (s *steps) Step(step string) { *s = append(*s, step) }

// TestInstallFromTheMakers downloads the real programs; it runs only when
// asked, with UNTERLUMEN_TOOLINSTALL_LIVE=1, on a Mac, into a temporary home
// and with a PATH that has neither Homebrew nor the programs.
func TestInstallFromTheMakers(t *testing.T) {
	if os.Getenv("UNTERLUMEN_TOOLINSTALL_LIVE") != "1" || runtime.GOOS != "darwin" {
		t.Skip("set UNTERLUMEN_TOOLINSTALL_LIVE=1 on a Mac to download the real programs")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin:/usr/sbin:/sbin")
	media.RecheckTools()
	brewLocations = nil // as on a Mac without Homebrew
	var s steps
	if err := Install(context.Background(), &s); err != nil {
		t.Fatal(err)
	}
	if missing := Missing(); len(missing) != 0 {
		t.Errorf("still missing after installing: %v (steps %v)", missing, s)
	}
}
