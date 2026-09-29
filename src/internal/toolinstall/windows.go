package toolinstall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/installation"
)

var wingetID = map[string]string{"ffmpeg": "Gyan.FFmpeg", "exiftool": "OliverBetz.ExifTool"}

func installWindows(ctx context.Context, report Reporter, missing []string) error {
	winget, _ := exec.LookPath("winget")
	tools, err := installation.ToolsDir()
	if err != nil {
		return err
	}
	for _, tool := range missing {
		id, viaWinget := wingetID[tool]
		switch {
		case viaWinget && winget != "":
			report.Step("Installing " + tool + " with winget…")
			out, err := exec.CommandContext(ctx, winget, "install", "--id", id, "-e", "--silent",
				"--accept-source-agreements", "--accept-package-agreements").CombinedOutput()
			if err != nil {
				return fmt.Errorf("winget install %s: %v: %s", id, err, lastLine(out))
			}
		case tool == "cwebp":
			report.Step("Downloading cwebp…")
			dir := "libwebp-" + webpVersion + "-windows-x64"
			if err := fetchZip(ctx, webpBase+dir+".zip", tools, dir+"/bin/cwebp.exe"); err != nil {
				return fmt.Errorf("cwebp: %w", err)
			}
		default:
			return fmt.Errorf("%s: winget is not installed; install %s yourself", tool, tool)
		}
	}
	refreshWindowsPath()
	installation.AddToolsToPath()
	return nil
}

// refreshWindowsPath reads PATH as winget left it in the registry, so the
// running app finds what was just installed without a restart.
func refreshWindowsPath() {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"[Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')").Output()
	if err != nil {
		return
	}
	path := strings.TrimSpace(string(out))
	if links := filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WinGet", "Links"); links != "" {
		path = links + ";" + path
	}
	os.Setenv("PATH", path) //nolint:errcheck
}
