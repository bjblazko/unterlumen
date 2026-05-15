package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func platformDefaults() InstallConfig {
	home, _ := os.UserHomeDir()
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return InstallConfig{
		Port:   8090,
		Path:   filepath.Join(home, "Pictures"),
		LibDir: filepath.Join(appData, "Unterlumen"),
	}
}

func platformInstall(config InstallConfig, execPath string, iconPNG []byte) error {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}

	installDir := filepath.Join(localAppData, "Unterlumen")
	fmt.Printf("Installing to %s …\n", installDir)

	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("creating install directory: %w", err)
	}

	binaryDst := filepath.Join(installDir, "unterlumen.exe")
	if err := copyFile(execPath, binaryDst); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}

	icoDst := filepath.Join(installDir, "icon.ico")
	if err := writePNGasICO(iconPNG, icoDst); err != nil {
		fmt.Printf("Warning: icon creation failed (%v)\n", err)
		icoDst = ""
	}

	batPath := filepath.Join(installDir, "launch.bat")
	bat := fmt.Sprintf("@echo off\r\n\"%s\" -desktop -port %d -lib-dir \"%s\" \"%s\"\r\n",
		binaryDst, config.Port, config.LibDir, config.Path)
	if err := os.WriteFile(batPath, []byte(bat), 0644); err != nil {
		return fmt.Errorf("writing launch.bat: %w", err)
	}

	lnkPath := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Unterlumen.lnk")
	if err := createWindowsShortcut(lnkPath, batPath, icoDst); err != nil {
		fmt.Printf("Warning: Start Menu shortcut creation failed (%v)\n", err)
	}

	fmt.Println("done.")
	fmt.Println("Unterlumen is now available in the Start Menu.")
	return nil
}

func createWindowsShortcut(lnkPath, targetPath, iconPath string) error {
	lines := []string{
		fmt.Sprintf(`$s = (New-Object -COM WScript.Shell).CreateShortcut('%s')`, escapePS(lnkPath)),
		fmt.Sprintf(`$s.TargetPath = '%s'`, escapePS(targetPath)),
	}
	if iconPath != "" {
		lines = append(lines, fmt.Sprintf(`$s.IconLocation = '%s'`, escapePS(iconPath)))
	}
	lines = append(lines, `$s.Save()`)
	return exec.Command("powershell", "-NoProfile", "-Command", strings.Join(lines, "; ")).Run()
}

func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// writePNGasICO creates a minimal ICO file with the PNG embedded directly.
// Modern Windows (Vista+) supports PNG images inside ICO containers.
func writePNGasICO(pngData []byte, path string) error {
	if len(pngData) == 0 {
		return fmt.Errorf("no icon data")
	}

	const (
		width  = 96
		height = 96
		offset = 22 // 6-byte file header + 16-byte ICONDIRENTRY
	)
	size := len(pngData)

	var buf []byte
	// File header: reserved(2), type=ICO(2), image count(2)
	buf = append(buf, 0, 0, 1, 0, 1, 0)
	// ICONDIRENTRY: width, height, colorCount, reserved, planes(2), bitCount(2), size(4), offset(4)
	buf = append(buf,
		byte(width), byte(height),
		0, 0,
		1, 0,
		32, 0,
		byte(size), byte(size>>8), byte(size>>16), byte(size>>24),
		byte(offset), byte(offset>>8), byte(offset>>16), byte(offset>>24),
	)
	buf = append(buf, pngData...)

	return os.WriteFile(path, buf, 0644)
}
