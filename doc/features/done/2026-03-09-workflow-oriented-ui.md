# Workflow-Oriented UI Redesign

*Last modified: 2026-03-09*

## Summary

Replace the tab-style mode switcher with a sequential workflow indicator using connected chevron/arrow-shaped buttons. Reorder and rename the three modes to reflect a natural photographer's workflow: Select, Review, Organize.

## Details

- **Renamed modes**: Browse & Cull → Select, Marked for Deletion → Review, File Manager → Organize
- **Reordered**: Select (1) → Review (2) → Organize (3) to match the natural culling workflow
- **Chevron stepper**: Three arrow-shaped buttons that interlock gaplessly (flat left edge, pointed right), styled with CSS `clip-path`. Left-to-right z-index ensures each arrow point sits in front of the next step. Future steps use surface color, completed steps use border color, active step uses accent orange.
- **Icons**: Each step has a 12px SVG icon — a 2×2 grid (Select), trash can (Review), dual-pane rectangles (Organize) — rendered in `currentColor` so they adapt to active/inactive state automatically.
- **Count badge**: The Review step shows an inline count badge when photos are marked for deletion.
- **Active/completed states**: Active step is orange with white label and icon; completed steps are gray; future steps are off-white/surface.
- **Transition animation**: Subtle horizontal slide (24px) when switching modes, direction reflects workflow order.
- **Contextual empty state**: Waste bin empty state guides users to the Select step.
- **Keyboard shortcuts**: 1=Select, 2=Review, 3=Organize (reordered to match visual layout).

## Acceptance Criteria

- [x] Three chevron-shaped buttons connect gaplessly, right side pointed, left side flat
- [x] Active step is orange with white text and icon
- [x] Completed steps (before active) show in gray
- [x] Future steps (after active) show in surface/off-white
- [x] Each step has a representative icon (grid, trash, dual-pane)
- [x] Keyboard shortcuts 1/2/3 map to Select/Review/Organize
- [x] Mode transitions include directional slide animation
- [x] Count badge appears on Review step when photos are marked
- [x] Dark mode inherits automatically via CSS variables
- [x] Empty waste bin shows contextual message guiding to Select
- [x] Thumbnail overlays (Show details) enabled by default
