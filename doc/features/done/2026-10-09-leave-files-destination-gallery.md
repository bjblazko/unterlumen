# Leave a files destination's gallery

*Last modified: 2026-10-09*

## Summary

A photo exported to a files destination (a folder whose files are posted by
hand, such as Instagram) can be taken out of that gallery again from the info
panel. The photo stays; its record of the gallery and its exported file go.

## Details

- The info panel's gallery card shows a × only for files destinations. It
  learns which destinations those are from the destination list.
- The server (`DeleteBuiltMeta`, `removeFromFilesDestination`) removes the
  album from the photo's XMP sidecar, so the next scan does not bring it back
  (ADR-0048), drops the library keys, and deletes the exported file.
- The exported file is found by its name, `{slug}_{stamp}_{base}{ext}`. The
  stamp is not recorded, so when more than one file matches (the same photo in
  two galleries of one destination, or two photos with one name) no file is
  deleted rather than the wrong one.
- Galleries and websites are unchanged: leaving them from the info panel would
  only make the library forget while their pages keep the photo.

## Acceptance Criteria

- [x] A files destination's gallery card in the info panel has a ×; other cards have none.
- [x] The × removes the gallery from the sidecar and the library, and deletes the exported file.
- [x] The photo itself is untouched.
- [x] Galleries shows one photo fewer for that gallery.
- [x] Go test (`files_removal_test.go`) and e2e (`publish-workflow.spec.js`, Files channel).
