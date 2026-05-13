---
name: unterlumen-stop
description: Stop the locally deployed Unterlumen service. Use when the user says "stop unterlumen" or "shut down unterlumen".
disable-model-invocation: true
allowed-tools: Bash
---

Stop the Unterlumen launchd service.

## Steps

1. Stop the service:

```bash
launchctl bootout gui/$(id -u)/com.unterlumen.app 2>/dev/null && echo "Unterlumen stopped." || echo "Unterlumen was not running."
```

2. Report the result.
