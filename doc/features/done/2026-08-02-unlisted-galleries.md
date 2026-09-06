# Unlisted Galleries

*Last modified: 2026-09-05*

## Summary

Site-export albums can be marked **Unlisted** at creation, so a gallery URL can be shared directly with specific people without appearing in the site's own navigation, index, or search-engine results. This document backfills the feature (originally shipped in commit `8318332`, 2026-08-02) which had no ADR, feature doc, or CHANGELOG entry until now.

## Details

### What "unlisted" changes

An unlisted album's slug is always the human-readable title slug plus a random 8-character hex token generated with `crypto/rand` (`randomSlugToken()`, `site.go`), appended regardless of whether the base slug would otherwise collide with an existing one — the normal collision fallback (date suffix) is left completely untouched for listed albums. This makes the album's folder name, and therefore its URL, unguessable.

Unlisted albums are:
- Excluded from the site's root `index.html` album listing.
- Excluded from `sitemap.xml`.
- Served with `<meta name="robots" content="noindex, nofollow">` in the album page's `<head>`, so a search engine that does discover the URL by some other means (an inbound link, a browser history sync, etc.) is still asked not to index it.

The `Unlisted` flag is set at album creation and is immutable thereafter (`SiteAlbum.Unlisted`, `site.go:56`) — there is no UI to toggle it after publish, since changing it would require also regenerating the slug (and therefore the URL), breaking any link already shared.

### Why not `robots.txt`

`generateRobotsTxt` (`site.go`) intentionally stays permissive (`User-agent: *` / `Allow: /`, plus an optional `Sitemap:` line) for every site, listed or not. `robots.txt` is a single file with no per-path secrecy: listing an unlisted album's path there — even under a `Disallow:` rule — would publish that path in a document every crawler fetches first, which is exactly the opposite of what "unlisted" is for. Privacy here comes entirely from the URL being unguessable (the random token) plus the noindex meta tag asking well-behaved crawlers not to index it if they do find it some other way. This is a deliberate trade-off, not an oversight: unlisted galleries are undiscoverable-by-crawling, not access-controlled — anyone who obtains the exact URL can view the gallery.

### Where it surfaces

- The publish/build modal (`library.js`) has an "Unlisted" checkbox alongside the normal site-album fields.
- The per-channel Albums popup (`channels.js`'s `_showAlbums`) shows an "Unlisted" badge next to any unlisted album's row.
- The cross-channel [Published Galleries overview](2026-09-05-published-galleries-overview.md) also surfaces the Unlisted flag per row.

## Acceptance Criteria

- [x] Unlisted albums get a slug with a random, unguessable token appended, regardless of title collisions
- [x] Unlisted albums are excluded from the site's root `index.html`
- [x] Unlisted albums are excluded from `sitemap.xml`
- [x] Unlisted album pages emit `<meta name="robots" content="noindex, nofollow">`
- [x] `robots.txt` remains unchanged (permissive) regardless of whether any album is unlisted
- [x] The Unlisted flag is immutable after album creation
- [x] Test coverage in `site_test.go` for slug generation, index/sitemap exclusion, and the noindex tag
