---
name: unterlumen-dev
description: Build the Unterlumen dev binary from source and launch it on port 8080. Use when the user says "dev run", "launch dev", "build and run", "run unterlumen", or "check changes".
disable-model-invocation: true
allowed-tools: Bash
---

Build Unterlumen from source and launch the dev binary on port 8080.

## Steps

1. Kill any process currently holding port 8080:

```bash
lsof -ti:8080 | xargs kill -9 2>/dev/null || true
```

2. Build from source:

```bash
cd /Users/blazko/Development/unterlumen/src && go build -o ../unterlumen . && echo "Build succeeded."
```

If the build fails, report the errors and stop.

3. Launch the dev binary in the background, logging to `/tmp/unterlumen-dev.log`:

```bash
nohup /Users/blazko/Development/unterlumen/unterlumen -port 8080 ~/Pictures > /tmp/unterlumen-dev.log 2>&1 &
echo $!
```

4. Wait briefly and confirm it started:

```bash
sleep 1 && curl -sf http://localhost:8080/api/config > /dev/null && echo "Running." || echo "Did not respond yet — check /tmp/unterlumen-dev.log"
```

5. Report: dev build is running at http://localhost:8080. Logs at `/tmp/unterlumen-dev.log`.
