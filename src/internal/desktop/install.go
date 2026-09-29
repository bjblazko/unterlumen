package desktop

import (
	"fmt"
	"io"
	"os"

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
	if _, found, err := installation.Load(); found || err != nil {
		return err
	}
	cfg := installation.Config{Port: installation.DesktopPort}
	if script, err := os.ReadFile(path); err == nil {
		if old, ok := installation.FromLauncher(string(script)); ok {
			cfg = old
		}
	}
	return installation.Save(cfg)
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
