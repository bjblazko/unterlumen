# Preserve Scroll Position on Viewer Exit

*Last modified: 2026-03-02*

## Summary

When closing the fullscreen image viewer, restore the browse grid/justified/list view to the same scroll position the user was at before opening the viewer.

## Details

Previously, opening the fullscreen viewer hid the browse container by setting `display: none`, which caused the browser to reset `scrollTop` to 0. On viewer close, the user would lose their place in a large folder.

The fix saves the `scrollTop` of every `.browse-container` element before hiding, and restores those values after the elements are made visible again.

## Acceptance Criteria

- [x] Scroll down in a large folder, open an image, close the viewer — scroll position is preserved
- [x] Scroll position preserved after copy/move reloads panes in File Manager
- [x] Works in grid view
- [x] Works in justified layout view
- [x] Works in list view
- [x] Works in commander mode (both panes)
