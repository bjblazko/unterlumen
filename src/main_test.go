package main

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseConfigTakesDefaultsFromTheEnvironment(t *testing.T) {
	t.Setenv("UNTERLUMEN_PORT", "9090")
	t.Setenv("UNTERLUMEN_BIND", "0.0.0.0")
	t.Setenv("UNTERLUMEN_LIB_DIR", "/data/lib")
	t.Setenv("UNTERLUMEN_CACHE_DIR", "/data/cache")
	t.Setenv("UNTERLUMEN_CHANNELS_DIR", "/shared")
	got := parseConfig(flag.NewFlagSet("t", flag.ContinueOnError), []string{"/photos"})
	want := config{port: 9090, bind: "0.0.0.0", libDir: "/data/lib", cacheDir: "/data/cache", channelsDir: "/shared", args: []string{"/photos"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("config = %+v, want %+v", got, want)
	}
}

func TestParseConfigFlagsOverrideTheEnvironment(t *testing.T) {
	t.Setenv("UNTERLUMEN_PORT", "9090")
	got := parseConfig(flag.NewFlagSet("t", flag.ContinueOnError), []string{"-port", "7000", "-lib-dir", "/x", "-desktop"})
	if got.port != 7000 || got.libDir != "/x" || !got.desktop || len(got.args) != 0 {
		t.Errorf("config = %+v", got)
	}
}

func TestParseConfigDefaults(t *testing.T) {
	for _, k := range []string{"UNTERLUMEN_PORT", "UNTERLUMEN_BIND", "UNTERLUMEN_LIB_DIR", "UNTERLUMEN_CACHE_DIR", "UNTERLUMEN_CHANNELS_DIR"} {
		t.Setenv(k, "")
	}
	t.Setenv("HOME", "/home/someone")
	got := parseConfig(flag.NewFlagSet("t", flag.ContinueOnError), nil)
	if got.port != 8080 || got.bind != "localhost" || got.libDir != "/home/someone/.unterlumen" {
		t.Errorf("config = %+v, want port 8080, localhost, ~/.unterlumen", got)
	}
}

func TestBrowseRoots(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	cases := []struct {
		name            string
		args            []string
		env             string
		start, boundary string
	}{
		{"folder argument wins", []string{"/photos"}, "/srv", "/photos", "/photos"},
		{"then the root path", nil, "/srv", "/srv", "/srv"},
		{"else home, unrestricted", nil, "", "/home/someone", "/"},
	}
	for _, c := range cases {
		start, boundary, err := browseRoots(c.args, c.env)
		if err != nil || start != c.start || boundary != c.boundary {
			t.Errorf("%s: %q, %q, %v; want %q, %q", c.name, start, boundary, err, c.start, c.boundary)
		}
	}
}

func TestAbsoluteRoots(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.txt")
	os.WriteFile(file, nil, 0o644) //nolint:errcheck

	if s, b, err := absoluteRoots(dir, "/"); err != nil || s != dir || b != "/" {
		t.Errorf("unrestricted: %q %q %v", s, b, err)
	}
	if s, b, err := absoluteRoots(dir, dir); err != nil || s != dir || b != dir {
		t.Errorf("restricted: %q %q %v", s, b, err)
	}
	if _, _, err := absoluteRoots(file, "/"); err == nil || !strings.HasPrefix(err.Error(), "Start path is not a valid directory: ") {
		t.Errorf("file as start: %v", err)
	}
	if _, _, err := absoluteRoots(dir, filepath.Join(dir, "gone")); err == nil || !strings.HasPrefix(err.Error(), "Boundary path is not a valid directory: ") {
		t.Errorf("missing boundary: %v", err)
	}
}

func TestRelativeStartAndHome(t *testing.T) {
	cases := []struct {
		fn             func(string, string) string
		boundary, path string
		want           string
	}{
		{relativeStart, "/", "/Users/me/photos", "Users/me/photos"},
		{relativeStart, "/photos", "/photos/2024", "2024"},
		{relativeStart, "/photos", "/photos", ""},
		{homeRelative, "/", "/Users/me", "Users/me"},
		{homeRelative, "/Users", "/Users/me", "me"},
		{homeRelative, "/Users/me", "/Users/me", ""},
		{homeRelative, "/photos", "/Users/me", ""},
		{homeRelative, "/Users/m", "/Users/me", ""},
	}
	for _, c := range cases {
		if got := c.fn(c.boundary, c.path); got != c.want {
			t.Errorf("(%q, %q) = %q, want %q", c.boundary, c.path, got, c.want)
		}
	}
}
