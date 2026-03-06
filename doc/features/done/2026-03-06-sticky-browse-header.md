# Sticky Browse Header

*Last modified: 2026-03-06*

## Summary

Make the browse header (breadcrumb navigation, View button, and image count status bar) sticky so it remains visible while scrolling through images.

## Details

The browse container is split into two child divs: `.browse-header` (flex-shrink: 0, stays at top) and `.browse-content` (flex: 1, overflow-y: auto, scrollable). The header contains warnings, breadcrumb, and controls. The content contains the grid/list/justified layout and the scroll sentinel for lazy loading.

The IntersectionObserver for chunked rendering uses `.browse-content` as its root so the sentinel triggers correctly within the scrollable area.

## Acceptance Criteria

- [x] Breadcrumb, View button, and status bar remain visible at top while scrolling
- [x] Lazy loading (chunked rendering via IntersectionObserver) still works
- [x] Justified layout still works (relayout on resize)
- [x] Scroll position restoration on reload works
- [x] `go vet ./...` passes
