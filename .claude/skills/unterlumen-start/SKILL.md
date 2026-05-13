---
name: unterlumen-start
description: Start or restart the locally deployed Unterlumen service. Use when the user says "start unterlumen", "restart unterlumen", or "launch unterlumen service".
disable-model-invocation: true
allowed-tools: Bash
---

Start or restart the Unterlumen launchd service.

## Steps

1. Check whether the plist exists:

```bash
test -f "$HOME/Library/LaunchAgents/com.unterlumen.app.plist" && echo "found" || echo "missing"
```

If it is missing, report that Unterlumen has not been deployed yet and the user should run `/unterlumen-deploy` first. Stop here.

2. Check whether the service is already loaded:

```bash
launchctl print gui/$(id -u)/com.unterlumen.app >/dev/null 2>&1 && echo "loaded" || echo "not loaded"
```

3a. If already loaded, restart it:

```bash
launchctl kickstart -k gui/$(id -u)/com.unterlumen.app
```

3b. If not loaded, bootstrap it:

```bash
launchctl bootstrap gui/$(id -u) "$HOME/Library/LaunchAgents/com.unterlumen.app.plist"
```

4. Read the configured port from the plist:

```bash
/usr/libexec/PlistBuddy -c "Print :ProgramArguments:2" "$HOME/Library/LaunchAgents/com.unterlumen.app.plist"
```

5. Report: "Unterlumen started at http://localhost:PORT" (use the port from step 4). Also note the log file at `/tmp/unterlumen.log`.
