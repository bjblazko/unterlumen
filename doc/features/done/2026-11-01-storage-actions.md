# Storage: actions in the data folder

*Last modified: 2026-11-01*

## Summary

Settings → Storage showed the data folder but offered nothing to do with
it, and every library's row repeated the same sentence and link. Rows now
lead to their place by name and carry the action that frees their space.

## Details

- A library's row: its name links to the library. **Delete library…** asks
  in the row with the same words as the library's own dialog, then calls
  `DELETE /api/library/{id}`. What a library keeps is said once, in the
  group's hint.
- Face recognition and Things: **Remove…** → `POST /api/storage/remove`
  (`recognition`, `things`). This is refused with 409 while a pass over a
  library uses the models. The loaded engine is dropped, not closed, because
  a request may still hold it. The faces, names and things already found
  stay.
- ONNX Runtime: **Remove…** only once neither model is installed; the server
  refuses otherwise (`ErrRuntimeNeeded`).
- Destination output has no action: the Galleries overview is read from it.
  Its hint says so.
- The action column has one width, so the sizes line up.

## Acceptance Criteria

- [x] A library can be deleted from its storage row, asked in place
- [x] The face and things models can be removed; this is refused while a pass runs
- [x] ONNX Runtime is removable only when nothing needs it
- [x] No sentence or link repeats on every library row
- [x] Go test (`manager_models_remove_test.go`), e2e (`settings-storage.spec.js`)
