# ADR-0005: Norton Commander-Style Dual-Pane Culling

*Last modified: 2026-02-21*

## Status

Accepted

## Context

Photo culling (selecting keepers from a larger set) is typically done by:

1. **Flagging/rating** — Mark photos with stars or pick/reject flags, then filter and export. Requires persistent metadata (conflicts with ADR-0002).
2. **Drag-and-drop to folders** — Visual but requires precise mouse interaction.
3. **Dual-pane file manager** — Two directory panels side by side. Select files in one, copy/move to the other. A proven paradigm (Norton Commander, Midnight Commander, Total Commander).

## Decision

Implement culling as a dual-pane Commander mode. Each pane navigates directories independently. Users multi-select files in one pane and copy or move them to the other pane's current directory.

## Consequences

- **No metadata needed** — Culling is expressed as filesystem operations (copy/move), not stored flags. Fully consistent with ADR-0002.
- **Familiar paradigm** — Users experienced with file managers will recognize the interaction pattern immediately.
- **Irreversible moves** — Moving a file is a destructive operation (removes it from the source). The UI shows a confirmation dialog with the file count before executing.
- **Both panes are full browsers** — Each pane has its own breadcrumb, sort controls, and view mode (grid/list). The browse rendering logic is shared between Browse mode and Commander mode via the `BrowsePane` class.
- **Active pane concept** — Only the active pane's selection is used for copy/move. The Tab key switches the active pane.
