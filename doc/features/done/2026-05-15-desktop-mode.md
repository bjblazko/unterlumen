# Desktop Mode

*Last modified: 2026-05-23*

## Summary

A `-desktop` flag that opens Unterlumen in a Chrome app window (no URL bar, no tab strip) and automatically shuts down the server when the window is closed.

## Details

When `-desktop` is passed:

1. The HTTP server starts and binds its port before the browser is launched, so Chrome can connect immediately.
2. Chrome is launched with `--app=<url>` and a temporary `--user-data-dir` (so it always starts as its own isolated process, even if another Chrome window is already open).
3. The process monitors the Chrome window; when it is closed, the server shuts down gracefully (5-second timeout).
4. OS signals (Ctrl+C, SIGTERM) also trigger a clean shutdown at any time.

If Chrome or Chromium is not found, the URL is opened in the default system browser (`open` on macOS, `xdg-open` on Linux, `rundll32` on Windows) and the server keeps running until interrupted — same behaviour as today but with an automatic browser open.

Chrome lookup order:
- **macOS**: `/Applications/Google Chrome.app/...`, `/Applications/Chromium.app/...`
- **Linux**: `google-chrome`, `google-chrome-stable`, `chromium-browser`, `chromium` (via `PATH`)
- **Windows**: `%ProgramFiles%\Google\Chrome\Application\chrome.exe`, then `%ProgramFiles(x86)%\...`

No external Go dependencies are introduced.

See also: [Desktop Install](2026-05-15-desktop-install.md) — the companion flag that creates a native app icon so the app can be launched without a terminal.

## Acceptance Criteria

- [x] `./unterlumen -desktop ~/Pictures` opens a frameless Chrome app window
- [x] Closing the Chrome window exits the server process
- [x] Ctrl+C also exits cleanly in desktop mode
- [x] With Chrome absent, the default browser opens and the server stays alive
- [x] `./unterlumen ~/Pictures` (no flag) behaves exactly as before
- [x] `go vet ./...` and `go build` pass with no errors
