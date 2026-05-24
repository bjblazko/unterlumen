---
name: unterlumen-dev-stop
description: Stop the Unterlumen dev server running on port 8080. Use when the user says "stop dev server", "stop unterlumen", "kill port 8080", or "shut down dev".
allowed-tools: Bash
---

Stop the Unterlumen dev server on port 8080.

## Steps

1. Kill any process holding port 8080:

```bash
lsof -ti:8080 | xargs kill -9 2>/dev/null && echo "Stopped." || echo "Nothing running on 8080."
```

2. Report whether anything was stopped.
