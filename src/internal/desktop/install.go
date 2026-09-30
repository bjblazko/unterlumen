package desktop

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"huepattl.de/unterlumen/internal/installation"
)

// Install creates a native app launcher for the binary at execPath (from
// os.Executable), with iconPNG as its icon. It asks nothing: the photo folder
// is chosen in the app on its first start (#setup), and the settings an older
// launcher carried as flags are kept in config.json first.
func Install(execPath string, iconPNG []byte, version string) error {
	if err := keepSettings(launcherPath()); err != nil {
		return fmt.Errorf("keeping the settings of the installed version: %w", err)
	}
	return platformInstall(execPath, iconPNG, version)
}

// keepSettings writes config.json when there is none yet: from the flags of
// an older launcher at path, or with the app's own port on a first install.
func keepSettings(path string) error {
	adopted, err := adoptLauncher(path)
	if adopted || err != nil {
		return err
	}
	if _, found, err := installation.Load(); found || err != nil {
		return err
	}
	return installation.Save(installation.Config{Port: installation.DesktopPort})
}

// AdoptOlderSettings takes over the settings of an older installation on the
// installed app's first start, when there is no config.json yet: an app from
// the .dmg or the Windows setup, installed beside one made by an older
// -desktop-install, then starts where that one left off — its photo folder,
// data folder, shared destinations and port — instead of asking again.
func AdoptOlderSettings() (bool, error) {
	return adoptLauncher(launcherPath())
}

// adoptLauncher saves the flags of the launcher at path as config.json,
// unless there is a config.json or no such launcher.
func adoptLauncher(path string) (bool, error) {
	if _, found, err := installation.Load(); found || err != nil {
		return false, err
	}
	script, err := os.ReadFile(path)
	if err != nil {
		return false, nil
	}
	old, ok := installation.FromLauncher(string(script))
	if !ok {
		return false, nil
	}
	return true, installation.Save(old)
}

// LaunchedAsMacApp says the program runs as the main executable of a macOS
// app bundle, started from the Finder, the Dock or Spotlight: it then opens
// its own window (-desktop) without being told.
func LaunchedAsMacApp(execPath string) bool {
	return runtime.GOOS == "darwin" && strings.Contains(execPath, ".app/Contents/MacOS/")
}

// WithoutProcessSerial drops the -psn_ argument older macOS versions pass to
// an app started from the Finder; the flag parser would stop at it.
func WithoutProcessSerial(args []string) []string {
	out := args[:0:0]
	for _, a := range args {
		if !strings.HasPrefix(a, "-psn_") {
			out = append(out, a)
		}
	}
	return out
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
