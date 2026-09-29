package toolinstall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"huepattl.de/unterlumen/internal/installation"
)

// The versions and addresses install/install.sh uses; keep them the same.
const (
	webpVersion   = "1.6.0"
	exiftoolVer   = "https://exiftool.org/ver.txt"
	exiftoolTar   = "https://sourceforge.net/projects/exiftool/files/Image-ExifTool-%s.tar.gz/download"
	ffmpegMacZip  = "https://ffmpeg.martin-riedl.de/redirect/latest/macos/%s/release/ffmpeg.zip"
	webpBase      = "https://storage.googleapis.com/downloads.webmproject.org/releases/webp/"
	brewFormulaOf = "ffmpeg:ffmpeg exiftool:exiftool cwebp:webp"
)

// brewLocations are where Homebrew puts brew on Apple Silicon and on Intel.
var brewLocations = []string{"/opt/homebrew/bin/brew", "/usr/local/bin/brew"}

func brewPath() string {
	for _, p := range brewLocations {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func installMacOS(ctx context.Context, report Reporter, missing []string) error {
	if brew := brewPath(); brew != "" {
		return installWithBrew(ctx, report, brew, missing)
	}
	tools, err := installation.ToolsDir()
	if err != nil {
		return err
	}
	for _, tool := range missing {
		report.Step("Downloading " + tool + "…")
		if err := downloadMacTool(ctx, tool, tools); err != nil {
			return fmt.Errorf("%s: %w", tool, err)
		}
	}
	installation.AddToolsToPath()
	return nil
}

func installWithBrew(ctx context.Context, report Reporter, brew string, missing []string) error {
	formulas := map[string]string{}
	for _, pair := range strings.Fields(brewFormulaOf) {
		tool, formula, _ := strings.Cut(pair, ":")
		formulas[tool] = formula
	}
	for _, tool := range missing {
		report.Step("Installing " + tool + " with Homebrew…")
		out, err := exec.CommandContext(ctx, brew, "install", formulas[tool]).CombinedOutput()
		if err != nil {
			return fmt.Errorf("brew install %s: %v: %s", formulas[tool], err, lastLine(out))
		}
	}
	// A server started from the Dock may not have Homebrew's folder on PATH.
	os.Setenv("PATH", filepath.Dir(brew)+string(os.PathListSeparator)+os.Getenv("PATH")) //nolint:errcheck
	return nil
}

func downloadMacTool(ctx context.Context, tool, tools string) error {
	arch, webpArch := "amd64", "x86-64"
	if runtime.GOARCH == "arm64" {
		arch, webpArch = "arm64", "arm64"
	}
	switch tool {
	case "ffmpeg":
		return fetchZip(ctx, fmt.Sprintf(ffmpegMacZip, arch), tools, "")
	case "exiftool":
		ver, err := fetchText(ctx, exiftoolVer)
		if err != nil {
			return err
		}
		return fetchTarGz(ctx, fmt.Sprintf(exiftoolTar, ver), filepath.Join(tools, "exiftool"), "Image-ExifTool-"+ver+"/")
	case "cwebp":
		dir := "libwebp-" + webpVersion + "-mac-" + webpArch
		return fetchTarGz(ctx, webpBase+dir+".tar.gz", tools, dir+"/bin/cwebp")
	}
	return nil
}

func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return lines[len(lines)-1]
}
