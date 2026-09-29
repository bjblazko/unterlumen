package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/installation"
)

func isolate(t *testing.T) string {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	return home
}

func TestKeepSettingsFromAnOlderLauncher(t *testing.T) {
	home := isolate(t)
	launcher := filepath.Join(home, "launch")
	os.WriteFile(launcher, []byte("#!/bin/bash\nexec \"$DIR/unterlumen\" -desktop -port 8090 -lib-dir '/lib' -channels-dir '/nas/.unterlumen-shared' '/'\n"), 0o755) //nolint:errcheck

	if err := keepSettings(launcher); err != nil {
		t.Fatal(err)
	}
	got, found, _ := installation.Load()
	want := installation.Config{PhotosDir: "/", LibDir: "/lib", ChannelsDir: "/nas/.unterlumen-shared", Port: 8090}
	if !found || got != want {
		t.Errorf("config.json = %+v, want %+v", got, want)
	}
}

func TestKeepSettingsOnAFirstInstall(t *testing.T) {
	home := isolate(t)
	if err := keepSettings(filepath.Join(home, "no-launcher")); err != nil {
		t.Fatal(err)
	}
	got, found, _ := installation.Load()
	if !found || got != (installation.Config{Port: installation.DesktopPort}) {
		t.Errorf("config.json = %+v; want only the port, the photo folder is chosen in the app", got)
	}
}

func TestKeepSettingsLeavesAConfigAlone(t *testing.T) {
	home := isolate(t)
	installation.Save(installation.Config{PhotosDir: "/mine", Port: 9000}) //nolint:errcheck
	launcher := filepath.Join(home, "launch")
	os.WriteFile(launcher, []byte("exec unterlumen -desktop -port 8090 '/old'\n"), 0o755) //nolint:errcheck

	if err := keepSettings(launcher); err != nil {
		t.Fatal(err)
	}
	if got, _, _ := installation.Load(); got.PhotosDir != "/mine" {
		t.Errorf("config.json overwritten: %+v", got)
	}
}
