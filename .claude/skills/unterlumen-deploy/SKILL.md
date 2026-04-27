---
name: unterlumen-deploy
description: Build and deploy Unterlumen as a local background service (launchd). Prompts for port and photo directory. Use when the user says "deploy unterlumen", "install unterlumen locally", or "set up unterlumen service".
allowed-tools: Bash
---

Build Unterlumen from source, install the binary to `~/.local/bin/`, register it as a launchd LaunchAgent, and start it.

## Steps

1. Ask the user for the port (default: `8080`) and the photo directory (default: `~/Pictures`). Confirm or use the defaults if the user does not specify.

2. Build the binary and install it:

```bash
mkdir -p "$HOME/.local/bin" && \
  cd /Users/blazko/Development/unterlumen/src && \
  go build -o "$HOME/.local/bin/unterlumen" . && \
  echo "Build succeeded: $HOME/.local/bin/unterlumen"
```

If the build fails, report the errors and stop.

3. Write the launchd plist using the port and directory chosen in step 1. Replace `PORT` and `PHOTO_DIR` with the actual values:

```bash
cat > "$HOME/Library/LaunchAgents/com.unterlumen.app.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.unterlumen.app</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/blazko/.local/bin/unterlumen</string>
        <string>-port</string>
        <string>PORT</string>
        <string>PHOTO_DIR</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
    <key>StandardOutPath</key>
    <string>/tmp/unterlumen.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/unterlumen.log</string>
</dict>
</plist>
PLIST
```

Note: the `PHOTO_DIR` value must be fully expanded (e.g., `/Users/blazko/Pictures`, not `~/Pictures`), because launchd does not expand `~`. Use `$HOME` resolution when constructing the command.

4. If the service is already registered, unload it first to allow a clean reload:

```bash
launchctl bootout gui/$(id -u)/com.unterlumen.app 2>/dev/null || true
```

5. Load and start the service:

```bash
launchctl bootstrap gui/$(id -u) "$HOME/Library/LaunchAgents/com.unterlumen.app.plist"
```

6. Verify the service is running:

```bash
launchctl print gui/$(id -u)/com.unterlumen.app 2>&1 | grep -E 'state|pid'
```

7. Report success: the URL (`http://localhost:PORT`) and the log file path (`/tmp/unterlumen.log`). If the service did not start, show the relevant launchctl output.
