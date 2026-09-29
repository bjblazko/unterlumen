package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// launcherPath is where the launcher of an installed app is.
func launcherPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "unterlumen", "launch.sh")
}

func platformInstall(execPath string, iconPNG []byte, _ string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot find home directory: %w", err)
	}

	installDir := filepath.Join(home, ".local", "share", "unterlumen")
	appsDir := filepath.Join(home, ".local", "share", "applications")

	fmt.Printf("Installing to %s …\n", installDir)

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}
	if err := os.MkdirAll(appsDir, 0755); err != nil {
		return fmt.Errorf("creating applications directory: %w", err)
	}

	binaryDst := filepath.Join(installDir, "unterlumen")
	if err := copyFile(execPath, binaryDst); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}
	if err := os.Chmod(binaryDst, 0755); err != nil {
		return fmt.Errorf("setting binary permissions: %w", err)
	}

	iconDst := filepath.Join(installDir, "icon.png")
	if err := os.WriteFile(iconDst, iconPNG, 0644); err != nil {
		return fmt.Errorf("writing icon: %w", err)
	}

	launchScript := filepath.Join(installDir, "launch.sh")
	const script = "#!/bin/bash\nDIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\nexec \"$DIR/unterlumen\" -desktop\n"
	if err := os.WriteFile(launchScript, []byte(script), 0755); err != nil {
		return fmt.Errorf("writing launch script: %w", err)
	}

	desktopEntry := fmt.Sprintf(
		"[Desktop Entry]\nVersion=1.0\nType=Application\nName=Unterlumen\nComment=Photo browser and culler\nExec=%s\nIcon=%s\nTerminal=false\nCategories=Graphics;Photography;\n",
		launchScript, iconDst,
	)
	desktopFile := filepath.Join(appsDir, "unterlumen.desktop")
	if err := os.WriteFile(desktopFile, []byte(desktopEntry), 0644); err != nil {
		return fmt.Errorf("writing .desktop file: %w", err)
	}

	exec.Command("update-desktop-database", appsDir).Run() //nolint:errcheck — optional cache refresh

	fmt.Println("done.")
	fmt.Println("Unterlumen is now available in your application launcher.")
	return nil
}
