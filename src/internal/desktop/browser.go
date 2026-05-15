package desktop

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// AppInstance represents a running Chrome app window.
type AppInstance struct {
	cmd     *exec.Cmd
	tempDir string
}

// Wait blocks until the Chrome window is closed, then cleans up the temp profile directory.
func (a *AppInstance) Wait() {
	a.cmd.Wait() //nolint:errcheck — non-zero exit is normal when user closes the window
	if a.tempDir != "" {
		os.RemoveAll(a.tempDir)
	}
}

// LaunchApp opens url in Chrome app mode (no URL bar, no tabs).
// Returns a non-nil AppInstance if Chrome was found and started.
// Returns nil, nil if Chrome was not found and the URL was opened in the default browser instead.
// Returns nil, err only on a hard failure (default browser also failed).
func LaunchApp(url string) (*AppInstance, error) {
	chromePath := findChrome()
	if chromePath == "" {
		log.Println("Chrome not found, falling back to default browser")
		return nil, openDefault(url)
	}

	// A dedicated user-data-dir forces Chrome to start as its own process
	// even when another Chrome instance is already running (required on macOS).
	tmpDir, _ := os.MkdirTemp("", "unterlumen-chrome-*")

	args := []string{"--app=" + url}
	if tmpDir != "" {
		args = append(args, "--user-data-dir="+tmpDir)
	}

	cmd := exec.Command(chromePath, args...)
	if err := cmd.Start(); err != nil {
		os.RemoveAll(tmpDir)
		log.Printf("Chrome launch failed (%v), falling back to default browser", err)
		return nil, openDefault(url)
	}

	return &AppInstance{cmd: cmd, tempDir: tmpDir}, nil
}

// findChrome returns the path to Chrome or Chromium, or "" if not found.
func findChrome() string {
	switch runtime.GOOS {
	case "darwin":
		candidates := []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "linux":
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium-browser", "chromium"} {
			if p, err := exec.LookPath(name); err == nil {
				return p
			}
		}
	case "windows":
		dirs := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")}
		for _, dir := range dirs {
			if dir == "" {
				continue
			}
			p := filepath.Join(dir, "Google", "Chrome", "Application", "chrome.exe")
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// openDefault opens url in the OS default browser.
func openDefault(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		log.Printf("Cannot open browser on %s, navigate to %s manually", runtime.GOOS, url)
		return nil
	}
	return cmd.Start()
}
