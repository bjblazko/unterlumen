---
name: unterlumen-deploy
description: Build from source and update the binary of the local Unterlumen installation created by 'unterlumen -desktop-install'. Use when the user says "deploy unterlumen", "update unterlumen", "install unterlumen locally", or "update the installed app".
allowed-tools: Bash
---

Build Unterlumen from source and hot-swap the binary inside `~/Applications/Unterlumen.app` (installed via `unterlumen -desktop-install`).

## Steps

### Step 1 — Check that the app bundle exists

```bash
BUNDLE="$HOME/Applications/Unterlumen.app/Contents/MacOS/unterlumen"
if [ -f "$BUNDLE" ]; then
    echo "bundle-found"
else
    echo "bundle-missing"
fi
```

If the bundle is missing, tell the user:
> "No installation found at ~/Applications/Unterlumen.app. Run `./unterlumen -desktop-install` first to create it."

Then stop.

### Step 2 — Build the new binary

```bash
cd /Users/blazko/Development/unterlumen/src && go build -o ../unterlumen . && echo "Build succeeded."
```

If the build fails, report the errors and stop.

### Step 3 — Stop any running instance

```bash
pkill -f "Unterlumen.app/Contents/MacOS/unterlumen" 2>/dev/null || true
sleep 1
```

### Step 4 — Copy the new binary into the bundle

```bash
cp /Users/blazko/Development/unterlumen/unterlumen \
   "$HOME/Applications/Unterlumen.app/Contents/MacOS/unterlumen" && \
echo "Binary updated."
```

### Step 5 — Relaunch the app

```bash
open "$HOME/Applications/Unterlumen.app"
sleep 2
pgrep -f "Unterlumen.app/Contents/MacOS/unterlumen" > /dev/null && echo "Running." || echo "App did not start — launch it manually from Launchpad or Spotlight."
```

Report success: the installed app has been updated and relaunched.
