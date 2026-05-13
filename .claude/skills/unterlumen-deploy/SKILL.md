---
name: unterlumen-deploy
description: Build and deploy Unterlumen as a local background service (launchd). Prompts for port and photo directory. Use when the user says "deploy unterlumen", "install unterlumen locally", or "set up unterlumen service".
allowed-tools: Bash
---

Build Unterlumen from source, install the binary to `~/.local/bin/`, register it as a launchd LaunchAgent, and start it.

## Steps

### Step 1 — Detect existing install

Run this to check whether a previous deployment exists and read its settings:

```bash
PLIST="$HOME/Library/LaunchAgents/com.unterlumen.app.plist"
if [ -f "$PLIST" ]; then
    EXISTING_PORT=$(/usr/libexec/PlistBuddy -c "Print :ProgramArguments:2" "$PLIST" 2>/dev/null)
    EXISTING_DIR=$(/usr/libexec/PlistBuddy -c "Print :ProgramArguments:3" "$PLIST" 2>/dev/null)
    echo "port=$EXISTING_PORT dir=$EXISTING_DIR"
else
    echo "no-existing-install"
fi
```

### Step 2 — Ask the user what to do

**If an existing install was found**, tell the user the current settings and ask:
> "Existing install found — port **X**, directory **Y**. Update the binary and restart with the same settings, or change the configuration?"

- If the user wants to **update only** (keep same settings): proceed to Step 3a.
- If the user wants to **change config**: ask for the new port (default: existing port) and photo directory (default: existing dir), then proceed to Step 3b.

**If no existing install was found**, ask:
> "No existing install. What port and photo directory? (defaults: 8080, ~/Pictures)"

Then proceed to Step 3b (fresh install).

### Step 3a — Update only (same settings, new binary)

Build and install the binary:

```bash
mkdir -p "$HOME/.local/bin" && \
  cd /Users/blazko/Development/unterlumen/src && \
  go build -o "$HOME/.local/bin/unterlumen" . && \
  echo "Build succeeded."
```

If the build fails, report the errors and stop.

Stop the running service cleanly (before the binary is replaced, so launchd releases the file):

```bash
launchctl bootout gui/$(id -u)/com.unterlumen.app 2>/dev/null || true
```

Start the service again with the existing plist (no plist rewrite needed):

```bash
launchctl bootstrap gui/$(id -u) "$HOME/Library/LaunchAgents/com.unterlumen.app.plist"
```

Skip to Step 5 to verify.

### Step 3b — Fresh install or config change

Build and install the binary:

```bash
mkdir -p "$HOME/.local/bin" && \
  cd /Users/blazko/Development/unterlumen/src && \
  go build -o "$HOME/.local/bin/unterlumen" . && \
  echo "Build succeeded."
```

If the build fails, report the errors and stop.

Write the launchd plist with the chosen PORT and fully-expanded PHOTO_DIR (launchd does not expand `~`; resolve it to the absolute path before writing):

```bash
cat > "$HOME/Library/LaunchAgents/com.unterlumen.app.plist" << PLIST
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

Unload any existing registration, then load the new one:

```bash
launchctl bootout gui/$(id -u)/com.unterlumen.app 2>/dev/null || true
launchctl bootstrap gui/$(id -u) "$HOME/Library/LaunchAgents/com.unterlumen.app.plist"
```

### Step 5 — Verify

```bash
launchctl print gui/$(id -u)/com.unterlumen.app 2>&1 | grep -E 'state|pid'
```

Report success: the URL (`http://localhost:PORT`) and the log file (`/tmp/unterlumen.log`). If the service did not start, show the relevant launchctl output.
