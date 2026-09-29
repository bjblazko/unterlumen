package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// launcherPath is where the launcher of an installed app is.
func launcherPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Applications", "Unterlumen.app", "Contents", "MacOS", "launch")
}

func platformInstall(execPath string, iconPNG []byte, version string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot find home directory: %w", err)
	}

	appDir := filepath.Join(home, "Applications", "Unterlumen.app")
	fmt.Printf("Installing to %s …\n", appDir)
	if err := writeMacBundle(appDir, execPath, iconPNG, version); err != nil {
		return err
	}

	fmt.Println("done.")
	fmt.Println("Unterlumen is now available in Spotlight and Launchpad.")
	return nil
}

// WriteMacBundle makes Unterlumen.app at appDir for the release's .dmg.
func WriteMacBundle(appDir, execPath string, iconPNG []byte, version string) error {
	return writeMacBundle(appDir, execPath, iconPNG, version)
}

// writeMacBundle makes the app bundle at appDir around the binary at
// execPath, replacing whatever is there. -desktop-install puts it in
// ~/Applications; the release puts it in the .dmg (WriteMacBundle).
func writeMacBundle(appDir, execPath string, iconPNG []byte, version string) error {
	macOSDir := filepath.Join(appDir, "Contents", "MacOS")
	resourcesDir := filepath.Join(appDir, "Contents", "Resources")
	if err := os.RemoveAll(appDir); err != nil {
		return fmt.Errorf("removing existing installation: %w", err)
	}
	if err := os.MkdirAll(macOSDir, 0755); err != nil {
		return fmt.Errorf("creating bundle structure: %w", err)
	}
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return fmt.Errorf("creating resources directory: %w", err)
	}

	binaryDst := filepath.Join(macOSDir, "unterlumen")
	if err := copyFile(execPath, binaryDst); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}
	if err := os.Chmod(binaryDst, 0755); err != nil {
		return fmt.Errorf("setting binary permissions: %w", err)
	}
	if err := generateDarwinIcon(iconPNG, resourcesDir); err != nil {
		fmt.Printf("Warning: icon generation failed (%v); bundle will have no icon\n", err)
	}
	if err := writeDarwinLaunchScript(filepath.Join(macOSDir, "launch")); err != nil {
		return fmt.Errorf("writing launch script: %w", err)
	}
	if err := writeDarwinPlist(filepath.Join(appDir, "Contents", "Info.plist"), version); err != nil {
		return fmt.Errorf("writing Info.plist: %w", err)
	}
	return nil
}

func generateDarwinIcon(iconPNG []byte, resourcesDir string) error {
	if len(iconPNG) == 0 {
		return fmt.Errorf("no icon data")
	}

	tmpDir, err := os.MkdirTemp("", "unterlumen-iconset-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "icon-src.png")
	if err := os.WriteFile(srcPath, iconPNG, 0644); err != nil {
		return err
	}

	iconsetDir := filepath.Join(tmpDir, "Unterlumen.iconset")
	if err := os.MkdirAll(iconsetDir, 0755); err != nil {
		return err
	}

	type iconSize struct {
		name string
		px   int
	}
	sizes := []iconSize{
		{"icon_16x16.png", 16},
		{"icon_16x16@2x.png", 32},
		{"icon_32x32.png", 32},
		{"icon_32x32@2x.png", 64},
		{"icon_128x128.png", 128},
		{"icon_128x128@2x.png", 256},
		{"icon_256x256.png", 256},
		{"icon_256x256@2x.png", 512},
		{"icon_512x512.png", 512},
		{"icon_512x512@2x.png", 1024},
	}
	for _, s := range sizes {
		out := filepath.Join(iconsetDir, s.name)
		sz := strconv.Itoa(s.px)
		if err := exec.Command("sips", "-z", sz, sz, srcPath, "--out", out).Run(); err != nil {
			return fmt.Errorf("sips %dx%d: %w", s.px, s.px, err)
		}
	}

	icnsPath := filepath.Join(resourcesDir, "icon.icns")
	return exec.Command("iconutil", "-c", "icns", iconsetDir, "-o", icnsPath).Run()
}

// writeDarwinLaunchScript writes the launcher. It passes no settings; those
// are in config.json. It prepends common Homebrew and system tool locations:
// apps launched from Spotlight or Launchpad receive a minimal PATH that
// excludes these dirs, causing tools like ffmpeg and exiftool to appear
// unavailable.
func writeDarwinLaunchScript(path string) error {
	const script = "#!/bin/bash\nexport PATH=\"/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:$PATH\"\nDIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\nexec \"$DIR/unterlumen\" -desktop\n"
	return os.WriteFile(path, []byte(script), 0755)
}

func writeDarwinPlist(path, version string) error {
	content := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>Unterlumen</string>
	<key>CFBundleDisplayName</key>
	<string>Unterlumen</string>
	<key>CFBundleExecutable</key>
	<string>launch</string>
	<key>CFBundleIconFile</key>
	<string>icon</string>
	<key>CFBundleIdentifier</key>
	<string>de.huepattl.unterlumen</string>
	<key>CFBundleVersion</key>
	<string>` + version + `</string>
	<key>CFBundleShortVersionString</key>
	<string>` + version + `</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>NSHighResolutionCapable</key>
	<true/>
	<key>LSApplicationCategoryType</key>
	<string>public.app-category.photography</string>
</dict>
</plist>
`
	return os.WriteFile(path, []byte(content), 0644)
}
