---
name: unterlumen-deploy
description: Build from source and update the binary of the local Unterlumen app in /Applications/Unterlumen.app (the installed desktop app). Use when the user says "deploy unterlumen", "update unterlumen", "install unterlumen locally", or "update the installed app".
allowed-tools: Bash
---

Build Unterlumen from source and swap the binary inside `/Applications/Unterlumen.app`, the one installed app. It starts without a folder and reads its settings from `~/Library/Application Support/Unterlumen/config.json` (ADR-0042); port 8090.

There must be only one app. An older `~/Applications/Unterlumen.app` (made by `-desktop-install`, with a `launch` script) was removed on 2026-10-01: two apps of one name made Spotlight start whichever came first, and the two pointed at different shared folders. If one turns up again, tell the user rather than deploying into it.

## Steps

### Step 1 — Check that the app exists, and that it is the only one

```bash
APP=/Applications/Unterlumen.app
[ -f "$APP/Contents/MacOS/unterlumen" ] && echo "app-found" || echo "app-missing"
[ -e "$HOME/Applications/Unterlumen.app" ] && echo "SECOND APP in ~/Applications"
```

If the app is missing, tell the user to install it from the .dmg first, then stop. If a second app exists, ask the user before going on.

### Step 2 — Build the new binary with its version

The version says what it is: the last tag, `-dev`, and the commit, with `+dirty` for uncommitted changes.

```bash
cd /Users/blazko/Development/unterlumen
V="$(git describe --tags --abbrev=0 | sed 's/^v//')-dev+$(git rev-parse --short HEAD)$(git diff --quiet HEAD || echo .dirty)"
cd src && go build -ldflags "-s -w -X main.Version=$V" -o ../unterlumen . && echo "Built $V"
```

If the build fails, report the errors and stop.

### Step 3 — Stop the running app

```bash
pkill -f "Unterlumen.app/Contents/MacOS/unterlumen" 2>/dev/null || true
sleep 2
pkill -9 -f "Unterlumen.app/Contents/MacOS/unterlumen" 2>/dev/null || true
```

### Step 4 — Put the binary into the app, set its version, sign it again

Replacing the binary breaks the app's ad-hoc signature; macOS refuses to open it until it is signed again.

```bash
APP=/Applications/Unterlumen.app
cp /Users/blazko/Development/unterlumen/unterlumen "$APP/Contents/MacOS/unterlumen"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $V" -c "Set :CFBundleVersion $V" "$APP/Contents/Info.plist"
codesign --force --deep -s - "$APP" && codesign -v "$APP" && echo "Signed."
```

### Step 5 — Relaunch and check

The first start after a schema change can take a minute while libraries migrate (ADR-0045); wait for the libraries list, not just the process.

```bash
open /Applications/Unterlumen.app
until curl -sf --max-time 5 localhost:8090/api/library/ >/dev/null; do sleep 2; done
curl -s localhost:8090/api/config | python3 -c "import json,sys; print('Running', json.load(sys.stdin).get('version'))"
```

Report the version that runs.
