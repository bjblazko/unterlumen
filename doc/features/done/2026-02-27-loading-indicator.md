# Loading Indicator for Folder Navigation

*Last modified: 2026-02-27*

## Summary

Show a loading spinner in the browse pane content area while the backend scans a directory. Prevents user confusion when navigating into large folders where EXIF extraction takes several seconds.

## Details

When `BrowsePane.load()` is called, the backend performs a synchronous directory scan including EXIF extraction for every image — this can take several seconds for large folders. Currently the UI shows no feedback: old content stays on screen until the response arrives, causing users to click repeatedly thinking nothing happened.

The fix is purely frontend:

1. `load()` clears `entries` and calls `render()` before the `await`, so the spinner appears immediately on navigation.
2. `render()` checks `this._loading` first and renders a centered spinner instead of the grid/list/empty state.
3. A CSS spinner (24px ring, 2px stroke, orange `border-top-color`) is added near the `.empty` rule.

The breadcrumb and controls still render during loading, so the user sees the destination path update immediately.

Works automatically in both single-pane and Commander mode (both use `BrowsePane`).

## Acceptance Criteria

- [ ] Spinner appears immediately when navigating into a folder
- [ ] Spinner disappears and content renders when the API responds
- [ ] Breadcrumb updates to destination path during loading
- [ ] Empty folder state still works correctly after loading
- [ ] Error state still works correctly on API failure
- [ ] Commander mode shows spinner in each pane independently
- [ ] No regression on normal navigation
