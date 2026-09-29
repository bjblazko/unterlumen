// Package toolinstall installs the helper programs Unterlumen uses — ffmpeg,
// exiftool, cwebp — from inside the app, for installs that did not run the
// one-line installer (the .dmg, the Windows setup). It does what
// install/install.sh does: the system's package manager where there is one,
// otherwise the makers' own downloads into installation.ToolsDir (ADR-0042).
package toolinstall

import (
	"context"
	"errors"
	"os/exec"
	"runtime"

	"huepattl.de/unterlumen/internal/media"
)

// Plan is what would be installed, and how, for the page to say before it
// is done.
type Plan struct {
	Missing    []string `json:"missing"`
	CanInstall bool     `json:"canInstall"`
	// How names the source in words: "Homebrew", "winget", "the makers' downloads".
	How string `json:"how,omitempty"`
	// Command is what to run by hand where the app cannot install (Linux needs root).
	Command string `json:"command,omitempty"`
}

// Reporter is told which step runs; a *jobs.Handle is one.
type Reporter interface {
	Step(step string)
}

// Missing lists the helper programs this system lacks. sips does HEIF on
// macOS, so heif-convert is asked for only elsewhere; cwebp only when ffmpeg
// cannot write WebP itself.
func Missing() []string {
	ff := media.CheckFFmpeg()
	var missing []string
	if !ff.Available {
		missing = append(missing, "ffmpeg")
	}
	if !media.CheckExiftool() {
		missing = append(missing, "exiftool")
	}
	if !ff.WebPSupport && !media.CheckCwebp() {
		missing = append(missing, "cwebp")
	}
	if runtime.GOOS == "linux" && !media.CheckHeifConvert() {
		missing = append(missing, "heif-convert")
	}
	return missing
}

// PlanNow says what is missing and how it would be installed.
func PlanNow() Plan {
	p := Plan{Missing: Missing()}
	if len(p.Missing) == 0 {
		return p
	}
	switch runtime.GOOS {
	case "darwin":
		p.CanInstall = true
		p.How = "the makers' downloads"
		if brewPath() != "" {
			p.How = "Homebrew"
		}
	case "windows":
		p.CanInstall = true
		p.How = "the makers' downloads"
		if _, err := exec.LookPath("winget"); err == nil {
			p.How = "winget"
		}
	default:
		p.Command = linuxCommand()
	}
	return p
}

// ErrCannotInstall means the helper programs have to be installed by hand here.
var ErrCannotInstall = errors.New("the helper programs have to be installed by hand on this system")

// Install installs what is missing and makes the app look for it again.
func Install(ctx context.Context, report Reporter) error {
	missing := Missing()
	if len(missing) == 0 {
		return nil
	}
	var err error
	switch runtime.GOOS {
	case "darwin":
		err = installMacOS(ctx, report, missing)
	case "windows":
		err = installWindows(ctx, report, missing)
	default:
		err = ErrCannotInstall
	}
	media.RecheckTools()
	return err
}
