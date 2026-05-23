# Desktop Install

*Last modified: 2026-05-23*

## Summary

A `-desktop-install` flag that runs an interactive wizard to create a platform-native app launcher with an icon, so Unterlumen can be opened from Spotlight/Launchpad (macOS), the application grid (Linux), or the Start Menu (Windows) without using a terminal.

## Details

Running `./unterlumen -desktop-install` starts an interactive session that asks for:
- **Port** — HTTP port for the server (default: 8090)
- **Photos directory** — root directory to browse (default: `~/Pictures`)
- **Library directory** — where Unterlumen stores its SQLite database and thumbnails (platform-specific default)

The wizard then copies the binary and creates a self-contained launcher that calls `unterlumen -desktop` with the configured values baked in.

### Platform behaviour

**macOS** — creates `~/Applications/Unterlumen.app`:
- Standard `.app` bundle (no admin password required)
- `Contents/MacOS/unterlumen` — binary copy
- `Contents/MacOS/launch` — shell script, the `CFBundleExecutable`
- `Contents/Resources/icon.icns` — generated from the embedded logo via `sips` + `iconutil` (built into macOS)
- Appears in Spotlight and Launchpad immediately
- Default library: `~/Library/Application Support/Unterlumen`

**Linux** — creates `~/.local/share/unterlumen/` + a `.desktop` entry:
- `unterlumen` binary + `launch.sh` script + `icon.png` in `~/.local/share/unterlumen/`
- `~/.local/share/applications/unterlumen.desktop` — XDG desktop entry
- Appears in GNOME, KDE, and other XDG-compliant launchers
- Default library: `~/.local/share/unterlumen`

**Windows** — creates `%LOCALAPPDATA%\Unterlumen\` + a Start Menu shortcut:
- `unterlumen.exe` + `launch.bat` + `icon.ico` in `%LOCALAPPDATA%\Unterlumen\`
- `.lnk` shortcut in `%APPDATA%\Microsoft\Windows\Start Menu\Programs\`
- ICO generated from the embedded logo (PNG-in-ICO, Windows Vista+ format)
- Shortcut created via PowerShell `WScript.Shell` (no extra dependencies)
- Default library: `%APPDATA%\Unterlumen`

Re-running `-desktop-install` overwrites the previous installation (macOS removes and recreates the whole bundle; Linux/Windows overwrite individual files).

No external Go dependencies are introduced.

## Acceptance Criteria

- [x] `./unterlumen -desktop-install` prompts for port, photos path, library dir with platform defaults
- [x] Pressing Enter at each prompt accepts the default
- [x] macOS: `~/Applications/Unterlumen.app` is created; app opens from Spotlight and Launchpad with icon
- [x] Linux: `.desktop` file created; app appears in application launcher with icon
- [x] Windows: `%LOCALAPPDATA%\Unterlumen\` created; shortcut visible in Start Menu with icon
- [x] All platforms: the launcher starts the server on the configured port and opens Chrome in app mode
- [x] `go vet ./...` and `go build` pass with no errors
