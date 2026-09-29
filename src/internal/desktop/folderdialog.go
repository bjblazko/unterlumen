package desktop

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"
)

// The system's own folder dialog, for an app that runs on the computer in
// front of the user: it knows the network shares, the external disks and the
// favourites of the Finder or Explorer, which a folder list in the browser
// does not. A browser cannot give a folder's path to the page, so the server
// asks the system instead.

// ErrNoFolderDialog means this system has no folder dialog Unterlumen can open.
var ErrNoFolderDialog = errors.New("no system folder dialog")

// folderDialogCommand is the command that shows the dialog and prints the
// chosen folder, or nil when there is none.
func folderDialogCommand(ctx context.Context, prompt string) *exec.Cmd {
	switch runtime.GOOS {
	case "darwin":
		return exec.CommandContext(ctx, "osascript",
			"-e", "on run argv",
			"-e", "activate",
			"-e", "POSIX path of (choose folder with prompt (item 1 of argv))",
			"-e", "end run", prompt)
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = $args[0]
$d.ShowNewFolderButton = $false
$owner = New-Object System.Windows.Forms.Form -Property @{ TopMost = $true }
if ($d.ShowDialog($owner) -eq 'OK') { $d.SelectedPath }`
		return exec.CommandContext(ctx, "powershell", "-NoProfile", "-STA", "-Command", script, prompt)
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			return exec.CommandContext(ctx, path, "--file-selection", "--directory", "--title="+prompt)
		}
		if path, err := exec.LookPath("kdialog"); err == nil {
			return exec.CommandContext(ctx, path, "--getexistingdirectory", ".", "--title", prompt)
		}
		return nil
	}
}

// CanChooseFolder says whether this system has a folder dialog.
func CanChooseFolder() bool {
	return folderDialogCommand(context.Background(), "") != nil
}

// ChooseFolder shows the system's folder dialog with prompt and waits for it.
// It returns the absolute path of the chosen folder, or "" when the dialog
// was cancelled. Cancelling ctx closes the dialog.
func ChooseFolder(ctx context.Context, prompt string) (string, error) {
	cmd := folderDialogCommand(ctx, prompt)
	if cmd == nil {
		return "", ErrNoFolderDialog
	}
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	path := strings.TrimSpace(string(out))
	var exit *exec.ExitError
	if errors.As(err, &exit) && path == "" {
		return "", nil // every one of these dialogs reports Cancel as a failed exit
	}
	if err != nil {
		return "", err
	}
	if len(path) > 1 && !strings.HasSuffix(path, `:\`) {
		path = strings.TrimRight(path, `/\`) // osascript ends a folder with a slash
	}
	return path, nil
}
