# A photo's gallery entry leads to the gallery

*Last modified: 2026-09-26*

## Summary

In a library photo's info panel, each gallery the photo is published in had a small ×. What it did was not clear, and it differed by destination: for a share-link gallery it only deleted the library's keys — the photo stayed in the gallery, online and in its sidecar, and the entry came back with the next scan — while for a website it took the photo off the whole site, from every album, without asking. The owner asked on 2026-09-26 what the × does and chose to replace it with a link to the gallery.

## Details

- A published gallery's entry shows the destination, the date and the gallery's title; the title is a link (`<a href="#galleries">`) that opens that gallery's page under Galleries. A gallery without a title reads "Open the gallery". Entries without a gallery ID (plain exports, photos published before those IDs existed) have no page to open and stay plain text.
- The × is gone from published entries. Pending entries — photos collected for a gallery that is not published yet — keep theirs: it removes the photo from the draft, which is what it looks like.
- `App.showGallery(channelSlug, postID)` switches to Galleries and `GalleriesPane.openGallery` opens the matching gallery once the list is read, or shows the list if the gallery is gone.
- Taking one published photo out of a gallery is deliberately not offered anywhere; the owner chose not to build it now. Unpublishing a whole gallery stays on the gallery's page.
- Each card carries its key as `data-key`, so specs find a gallery's entry without a button.

## Acceptance Criteria

- [x] A published gallery entry in the info panel has no ×.
- [x] Its title opens that gallery's page under Galleries.
- [x] Pending entries keep their × and still remove the photo from the draft.
- [x] Entries without a gallery ID show their title as text.
- [x] `gallery-channel-multi-album.spec.js` checks the missing × and the link; `publish-workflow.spec.js` finds cards by `data-key`.
