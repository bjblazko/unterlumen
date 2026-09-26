package publish

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/site"
)

// albumRegisterReport says what rebuilding the album register from the photos'
// sidecars found, what it added and what it could not read.
type albumRegisterReport struct {
	Added   []rebuiltAlbum `json:"added"`
	Present int            `json:"present"` // already in the register, left as they are
	Deleted int            `json:"deleted"` // deleted on purpose, so not restored
	// SidecarsCompleted counts photos whose sidecar got a registered album's
	// membership or address, because they predate addresses being recorded.
	SidecarsCompleted int `json:"sidecarsCompleted"`
	// Unreachable counts members of registered albums that could not be
	// written to: not in a library here, gone from disk, or a legacy entry
	// that names no photo.
	Unreachable int               `json:"unreachable"`
	Unreadable  []unreadableAlbum `json:"unreadable"` // found in sidecars but not registrable
	Photos      []unreadablePhoto `json:"photos"`     // sidecars that could not be read
}

type rebuiltAlbum struct {
	PostID string `json:"postID"`
	Title  string `json:"title"`
	Slug   string `json:"slug"`
	Photos int    `json:"photos"`
}

type unreadableAlbum struct {
	PostID string `json:"postID"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

type unreadablePhoto struct {
	Filename string `json:"filename"`
	Reason   string `json:"reason"`
}

// sidecarAlbum is what the sidecars of one album's photos add up to.
type sidecarAlbum struct {
	postID, title, slug string
	unlisted            bool
	first, last         time.Time
	photos              []site.Photo
	unaddressed         []string // photos whose sidecar records this album without an address
}

// rebuildAlbumRegister writes the albums the photos' sidecars record for a
// site channel into the register, for the albums that are missing there. An
// album already in the register is left alone: the register is the newer
// truth (titles are renamed there, not in the sidecars). A slug is never
// derived — an album whose sidecars carry none is reported, not registered.
func rebuildAlbumRegister(sites *site.Store, mgr *lib.Manager, ch *channels.Channel) (albumRegisterReport, error) {
	report := albumRegisterReport{Added: []rebuiltAlbum{}, Unreadable: []unreadableAlbum{}, Photos: []unreadablePhoto{}}
	found, unread := collectSidecarAlbums(mgr, ch)
	report.Photos = append(report.Photos, unread...)

	registered, err := sites.List()
	if err != nil {
		return report, err
	}
	have := make(map[string]site.Album, len(registered))
	for _, a := range registered {
		have[a.PostID] = a
	}

	for _, sa := range found {
		switch {
		case sites.IsDeleted(sa.postID):
			report.Deleted++
		case have[sa.postID].PostID != "":
			report.Present++
			completeSidecars(&report, ch, have[sa.postID], sa.unaddressed)
		case sa.slug == "":
			report.Unreadable = append(report.Unreadable, unreadableAlbum{
				PostID: sa.postID, Title: sa.title,
				Reason: "its photos were published before the album's address was recorded, so it cannot be restored without inventing a new address",
			})
		default:
			album := sa.toSiteAlbum()
			if err := sites.Upsert(album); err != nil {
				return report, err
			}
			report.Added = append(report.Added, rebuiltAlbum{PostID: album.PostID, Title: album.Title, Slug: album.Slug, Photos: album.PhotoCount})
		}
	}
	// The register knows every registered album's members. Their sidecars may
	// name only older albums, so write the membership into them.
	paths := photoPathIndex(mgr)
	for _, a := range registered {
		writeMembership(&report, ch, a, paths)
	}
	return report, nil
}

// photoPathIndex maps a photo's ID to its file for every photo the libraries
// on this installation know.
func photoPathIndex(mgr *lib.Manager) map[string]string {
	paths := map[string]string{}
	libs, err := mgr.ListLibraries()
	if err != nil {
		return paths
	}
	for _, l := range libs {
		store, err := mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		refs, _ := store.ListAllPhotoRefs()
		store.Close()
		for _, ref := range refs {
			if _, ok := paths[ref.ID]; !ok {
				paths[ref.ID] = ref.PathHint
			}
		}
	}
	return paths
}

// writeMembership records a registered album in the sidecar of each of its
// members that lacks it, or lacks its address, so the album can be restored
// from the photos. A sidecar is never created for a photo that is not there.
func writeMembership(report *albumRegisterReport, ch *channels.Channel, album site.Album, paths map[string]string) {
	if album.Slug == "" {
		return
	}
	for _, sp := range album.Photos {
		path := paths[sp.PhotoID]
		if sp.PhotoID == "" || path == "" {
			report.Unreachable++
			continue
		}
		if _, err := os.Stat(path); err != nil {
			report.Unreachable++
			continue
		}
		pubs, err := media.ReadSidecar(path)
		if err != nil {
			report.Photos = append(report.Photos, unreadablePhoto{Filename: filepath.Base(path), Reason: err.Error()})
			continue
		}
		recorded := false
		for _, p := range pubs {
			recorded = recorded || (p.Channel == ch.Slug && p.PostID == album.PostID)
		}
		if !recorded {
			pub := media.Publication{Channel: ch.Slug, PostID: album.PostID, GalleryTitle: album.Title, Slug: album.Slug, Unlisted: album.Unlisted, PublishedAt: album.PublishedAt}
			if err := media.AppendPublication(path, pub); err != nil {
				report.Photos = append(report.Photos, unreadablePhoto{Filename: filepath.Base(path), Reason: err.Error()})
			} else {
				report.SidecarsCompleted++
			}
			continue
		}
		completeSidecars(report, ch, album, []string{path})
	}
}

// completeSidecars gives the photos of a registered album the address the
// register knows and their sidecars lack, so the album can be restored from the
// photos as well.
func completeSidecars(report *albumRegisterReport, ch *channels.Channel, album site.Album, paths []string) {
	if album.Slug == "" {
		return
	}
	for _, path := range paths {
		changed, err := media.SetPublicationAddress(path, ch.Slug, album.PostID, album.Slug, album.Unlisted)
		switch {
		case err != nil:
			report.Photos = append(report.Photos, unreadablePhoto{Filename: filepath.Base(path), Reason: err.Error()})
		case changed:
			report.SidecarsCompleted++
		}
	}
}

func (sa *sidecarAlbum) toSiteAlbum() site.Album {
	sort.Slice(sa.photos, func(i, j int) bool { return sa.photos[i].Filename < sa.photos[j].Filename })
	album := site.Album{
		PostID: sa.postID, Slug: sa.slug, Title: sa.title, PublishedAt: sa.first,
		PhotoCount: len(sa.photos), CoverFile: "cover.jpg", Photos: sa.photos, Unlisted: sa.unlisted,
	}
	if sa.last.After(sa.first) {
		album.UpdatedAt = sa.last
	}
	return album
}

// collectSidecarAlbums reads the sidecar of every photo the libraries know to
// be published to the channel and groups the publications by album. The
// libraries only say which photos to look at; what is recorded is read from the
// photos' own sidecars.
func collectSidecarAlbums(mgr *lib.Manager, ch *channels.Channel) ([]*sidecarAlbum, []unreadablePhoto) {
	byPost := map[string]*sidecarAlbum{}
	var order []string
	var unread []unreadablePhoto
	seen := map[string]bool{} // photo ID, so a photo in two libraries counts once

	libs, err := mgr.ListLibraries()
	if err != nil {
		return nil, nil
	}
	for _, l := range libs {
		store, err := mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		for offset := 0; ; offset += 500 {
			res, err := store.ListPhotos(lib.ListPhotosOpts{MetaExists: []string{"built:" + ch.Slug}, Offset: offset, Limit: 500})
			if err != nil || len(res.Photos) == 0 {
				break
			}
			for _, p := range res.Photos {
				if seen[p.ID] {
					continue
				}
				seen[p.ID] = true
				pubs, err := media.ReadSidecar(p.PathHint)
				if err != nil {
					unread = append(unread, unreadablePhoto{Filename: p.Filename, Reason: err.Error()})
					continue
				}
				for _, pub := range pubs {
					if pub.Channel != ch.Slug || pub.PostID == "" {
						continue
					}
					sa := byPost[pub.PostID]
					if sa == nil {
						sa = &sidecarAlbum{postID: pub.PostID, first: pub.PublishedAt, last: pub.PublishedAt}
						byPost[pub.PostID] = sa
						order = append(order, pub.PostID)
					}
					sa.add(ch, p, pub)
				}
			}
			if offset+len(res.Photos) >= res.Total {
				break
			}
		}
		store.Close()
	}

	out := make([]*sidecarAlbum, 0, len(order))
	for _, id := range order {
		out = append(out, byPost[id])
	}
	return out, unread
}

func (sa *sidecarAlbum) add(ch *channels.Channel, p lib.Photo, pub media.Publication) {
	if pub.PublishedAt.Before(sa.first) {
		sa.first = pub.PublishedAt
	}
	if pub.PublishedAt.After(sa.last) {
		sa.last = pub.PublishedAt
	}
	if sa.title == "" {
		sa.title = pub.GalleryTitle
	}
	if sa.slug == "" {
		sa.slug, sa.unlisted = pub.Slug, pub.Unlisted
	}
	if pub.Slug == "" {
		sa.unaddressed = append(sa.unaddressed, p.PathHint)
	}
	name := exportedFilename(ch, pub.PublishedAt, p.PathHint)
	sa.photos = append(sa.photos, site.Photo{PhotoID: p.ID, Filename: name, ThumbFilename: "thumbs/" + name})
}

// exportedFilename is the name a photo gets in an album folder: channel,
// publish time and the photo's own name, the same scheme buildOne writes.
func exportedFilename(ch *channels.Channel, publishedAt time.Time, pathHint string) string {
	ext := "." + ch.Format
	if ch.Format == "jpeg" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
	return ch.Slug + "_" + publishedAt.UTC().Format("20060102T150405Z") + "_" + base + ext
}

// rebuildAlbumList is POST /api/channels/{slug}/rebuild-album-list: it restores
// a website destination's missing albums in the shared register from the
// photos' own sidecars and answers with what it found.
func rebuildAlbumList(chStore *channels.Store, mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil || mgr == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusNotFound)
			return
		}
		if !ch.SiteExport {
			http.Error(w, "only a website destination keeps an album list", http.StatusBadRequest)
			return
		}
		report, err := rebuildAlbumRegister(site.NewStore(chStore, slug), mgr, ch)
		if err != nil {
			http.Error(w, "rebuild album list: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, report)
	}
}
