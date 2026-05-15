package desktop

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// InstallConfig holds the user-configured parameters baked into the installed launcher.
type InstallConfig struct {
	Port   int
	Path   string // photos root directory (positional arg)
	LibDir string // value for -lib-dir
}

// Install runs the interactive install wizard and creates a native app launcher.
// execPath must be the path to the current running binary (from os.Executable).
// iconPNG is the raw PNG bytes used as the app icon.
func Install(execPath string, iconPNG []byte) error {
	defaults := platformDefaults()
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\nUnterlumen Desktop Installer")
	fmt.Println(strings.Repeat("-", 30))

	port := promptInt(scanner, defaults.Port, "Port")
	path := expandPath(promptString(scanner, defaults.Path, "Photos directory"))
	libDir := expandPath(promptString(scanner, defaults.LibDir, "Library directory"))

	fmt.Println()
	return platformInstall(InstallConfig{Port: port, Path: path, LibDir: libDir}, execPath, iconPNG)
}

func promptString(scanner *bufio.Scanner, def, label string) string {
	fmt.Printf("%s [%s]: ", label, def)
	if !scanner.Scan() {
		return def
	}
	if v := strings.TrimSpace(scanner.Text()); v != "" {
		return v
	}
	return def
}

func promptInt(scanner *bufio.Scanner, def int, label string) int {
	fmt.Printf("%s [%d]: ", label, def)
	if !scanner.Scan() {
		return def
	}
	v := strings.TrimSpace(scanner.Text())
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		fmt.Printf("  Invalid number, using %d\n", def)
		return def
	}
	return n
}

// expandPath expands a leading ~/ to the user's home directory.
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
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

// shellescape wraps s in single quotes safe for use in POSIX shell scripts.
func shellescape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
