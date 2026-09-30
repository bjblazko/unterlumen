package desktop

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestLaunchedAsMacApp(t *testing.T) {
	inApp := LaunchedAsMacApp("/Applications/Unterlumen.app/Contents/MacOS/unterlumen")
	if runtime.GOOS == "darwin" && !inApp {
		t.Error("the program in an app bundle was not taken as the app")
	}
	if LaunchedAsMacApp("/usr/local/bin/unterlumen") {
		t.Error("a program outside a bundle was taken as the app")
	}
}

func TestWithoutProcessSerial(t *testing.T) {
	got := WithoutProcessSerial([]string{"-psn_0_12345", "-port", "8090"})
	if len(got) != 2 || got[0] != "-port" || got[1] != "8090" {
		t.Errorf("args = %v", got)
	}
}

func TestAdoptLauncherTakesOverAnOlderInstallation(t *testing.T) {
	home := isolate(t)
	launcher := filepath.Join(home, "launch")
	os.WriteFile(launcher, []byte("exec \"$DIR/unterlumen\" -desktop -port 8090 -lib-dir '/lib' -channels-dir '/Volumes/nas/Bilder/.unterlumen-shared' '/'\n"), 0o755) //nolint:errcheck

	adopted, err := adoptLauncher(launcher)
	if !adopted || err != nil {
		t.Fatalf("adopted=%v err=%v", adopted, err)
	}
	got, _, _ := installation.Load()
	if got.PhotosDir != "/" || got.ChannelsDir != "/Volumes/nas/Bilder/.unterlumen-shared" || got.LibDir != "/lib" {
		t.Errorf("config.json = %+v", got)
	}
	if again, _ := adoptLauncher(launcher); again {
		t.Error("an existing config.json was replaced")
	}
}

func TestAdoptLauncherWithoutOneWritesNothing(t *testing.T) {
	home := isolate(t)
	if adopted, err := adoptLauncher(filepath.Join(home, "no-launcher")); adopted || err != nil {
		t.Errorf("adopted=%v err=%v", adopted, err)
	}
	if _, found, _ := installation.Load(); found {
		t.Error("config.json written without an older installation; the setup would not be asked")
	}
}
