---
name: test-e2e-live
description: Run the Unterlumen Playwright e2e tests with the interactive UI. Use when the user says "test-e2e-live", "run e2e with ui", "open playwright ui", or "watch e2e tests".
disable-model-invocation: true
allowed-tools: Bash
---

Launch the Playwright interactive test UI for the Unterlumen e2e suite.

## Steps

1. Make sure the binary is built:

```bash
cd /Users/blazko/Development/unterlumen/src && go build -o ../unterlumen .
```

2. Launch the Playwright UI:

```bash
cd /Users/blazko/Development/unterlumen/e2e && npx playwright test --ui
```

The browser opens the Playwright Test UI at `http://localhost:` (port assigned automatically). From there you can:
- Run individual tests or entire spec files by clicking the play button
- Watch tests execute live in the embedded browser preview
- Inspect DOM snapshots, network requests, and console logs for each step
- Re-run failing tests in isolation

3. Report that the UI is open and remind the user they can run individual specs or all tests from the sidebar.
