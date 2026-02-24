# Sort by Size

*Last modified: 2026-02-22*

## Summary

Add file size as a sort option so users can order images by file size in addition to name and date.

## Details

A new `SortBySize` constant and corresponding sort case are added to `internal/media/scanner.go`. The frontend sort dropdown in the View menu gains a "Size" option. The backend already passes the sort parameter through without validation, so no API handler changes are needed.

Directories always sort first regardless of sort field. Size sort compares the `Size` field on entries (populated from `os.FileInfo` during directory scanning). Directories have no size value and are unaffected.

## Acceptance Criteria

- [x] "Size" appears as an option in the Sort dropdown
- [x] Sorting by size ascending orders smallest files first
- [x] Sorting by size descending orders largest files first
- [x] Directories remain sorted before files regardless of sort field
- [x] `go vet ./...` passes cleanly
