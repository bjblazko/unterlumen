# View Menu Consolidation

*Last modified: 2026-02-22*

## Summary

Move the Grid/List layout toggle from the standalone controls bar into the View popup menu, reducing toolbar clutter and grouping all view-related options together.

## Details

The Grid/List toggle buttons previously sat as a separate `.view-toggle` element in the controls bar, occupying space alongside the View menu button. This change moves them into the View menu as the first section, labeled "Layout", using a segmented-control appearance identical to their previous standalone styling.

The existing `[data-view]` event delegation continues to work without modification since it matches buttons by data attribute regardless of DOM location.

## Acceptance Criteria

- [x] Grid/List buttons no longer appear as standalone controls in the toolbar
- [x] View menu contains a "Layout" section as its first entry with Grid/List buttons
- [x] Clicking Grid/List inside the menu changes the layout correctly
- [x] Segmented-control styling (`.view-menu-toggle`) matches the previous `.view-toggle` appearance
- [x] Old `.view-toggle` CSS rules are removed
