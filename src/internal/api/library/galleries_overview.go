package apilibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/deploy"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/pathguard"
)

// PublishedGallery is one published album/gallery merged across every
// channel, for the cross-channel "Published Galleries" overview.
type PublishedGallery struct {
	galleryListItem
	ChannelSlug string `json:"channelSlug"`
	ChannelName string `json:"channelName"`
	// ChannelHandler mirrors Channel.Handler ("rsync", or empty) — lets the
	// frontend offer a remote-delete option only where one is possible.
	ChannelHandler string `json:"channelHandler,omitempty"`
	// URL is the resolved absolute public URL for this gallery, if computable.
	URL string `json:"url,omitempty"`
	// URLGuessed is true when URL was derived from the rsync handler's host
	// rather than the channel's explicitly configured SiteURL.
	URLGuessed bool `json:"urlGuessed,omitempty"`
}

// resolveGalleryURL computes the public URL for a published gallery item, if
// one can be determined. This mirrors _deployBaseURL in channels.js — kept in
// sync deliberately rather than shared, since the reachability check (below)
// needs a URL server-side before the client has rendered anything.
func resolveGalleryURL(ch *channels.Channel, item galleryListItem) (url string, guessed bool) {
	base := ""
	isGuess := false
	if ch.SiteURL != "" {
		base = strings.TrimRight(ch.SiteURL, "/")
	} else if ch.Handler == "rsync" && ch.HandlerConfig["host"] != "" {
		base = "https://" + ch.HandlerConfig["host"]
		isGuess = true
	} else {
		return "", false
	}

	folder := item.FolderName
	if folder == "" {
		folder = item.PostID
	}
	if folder == "" {
		return base, isGuess
	}
	return base + "/" + folder + "/", isGuess
}

// listAllGalleries returns every published gallery across every channel,
// merged into one flat, newest-first list.
func listAllGalleries(chStore *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		chs, err := chStore.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		out := []PublishedGallery{}
		for _, ch := range chs {
			if !ch.SiteExport && !ch.GalleryExport {
				continue
			}
			channelDir := chStore.OutputDir(ch.Slug)
			items, err := collectGalleryItems(ch, channelDir)
			if err != nil {
				// One channel's broken/missing statefile shouldn't take
				// down the whole overview.
				continue
			}
			for _, it := range items {
				url, guessed := resolveGalleryURL(ch, it)
				out = append(out, PublishedGallery{
					galleryListItem: it,
					ChannelSlug:     ch.Slug,
					ChannelName:     ch.Name,
					ChannelHandler:  ch.Handler,
					URL:             url,
					URLGuessed:      guessed,
				})
			}
		}

		sort.Slice(out, func(i, j int) bool {
			return out[i].PublishedAt.After(out[j].PublishedAt)
		})
		writeJSON(w, out)
	}
}

const (
	reachabilityMaxConcurrency = 8
	reachabilityTimeout        = 5 * time.Second
)

type reachabilityTarget struct {
	ChannelSlug string `json:"channelSlug"`
	PostID      string `json:"postID"`
	URL         string `json:"url"`
}

type reachabilityResult struct {
	ChannelSlug string `json:"channelSlug,omitempty"`
	PostID      string `json:"postID,omitempty"`
	Reachable   bool   `json:"reachable,omitempty"`
	Error       string `json:"error,omitempty"`
	Complete    bool   `json:"complete,omitempty"`
}

// checkGalleryReachability streams a live HEAD-request reachability check
// over Server-Sent Events for a client-supplied list of gallery URLs, bounded
// to reachabilityMaxConcurrency in-flight requests at a time so a large
// published-gallery collection doesn't open unbounded outbound connections.
func checkGalleryReachability() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Targets []reachabilityTarget `json:"targets"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		results := make(chan reachabilityResult)
		sem := make(chan struct{}, reachabilityMaxConcurrency)
		var wg sync.WaitGroup
		client := &http.Client{Timeout: reachabilityTimeout}

		// Dispatch runs in its own goroutine so it can't deadlock against the
		// receive loop below: with more targets than reachabilityMaxConcurrency,
		// filling sem here would otherwise block before anyone starts draining
		// results, and the in-flight workers would in turn block sending to the
		// unbuffered results channel.
		go func() {
			for _, t := range body.Targets {
				if t.URL == "" {
					continue
				}
				wg.Add(1)
				sem <- struct{}{}
				go func(t reachabilityTarget) {
					defer wg.Done()
					defer func() { <-sem }()
					results <- probeReachability(r.Context(), client, t)
				}(t)
			}
			wg.Wait()
			close(results)
		}()

		enc := json.NewEncoder(w)
		for res := range results {
			writeSSEEvent(w, enc, res)
			flusher.Flush()
		}
		writeSSEEvent(w, enc, reachabilityResult{Complete: true})
		flusher.Flush()
	}
}

func probeReachability(parent context.Context, client *http.Client, t reachabilityTarget) reachabilityResult {
	ctx, cancel := context.WithTimeout(parent, reachabilityTimeout)
	defer cancel()

	res := reachabilityResult{ChannelSlug: t.ChannelSlug, PostID: t.PostID}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, t.URL, nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	resp, err := client.Do(req)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()
	res.Reachable = resp.StatusCode < 400
	if !res.Reachable {
		res.Error = resp.Status
	}
	return res
}

func writeSSEEvent(w http.ResponseWriter, enc *json.Encoder, res reachabilityResult) {
	fmt.Fprint(w, "data: ")
	enc.Encode(res) //nolint:errcheck
	fmt.Fprint(w, "\n")
}

// renameGallery changes the title of one already-published album/gallery and
// regenerates its HTML (and, for site-export, the site index/sitemap, since
// those also list every album's title). Only the title can be changed —
// Unlisted is immutable after creation by design (see SiteAlbum.Unlisted):
// changing it would change the slug/URL and break any link already shared.
func renameGallery(chStore *channels.Store, mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		postID := r.PathValue("postID")
		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusNotFound)
			return
		}

		var body struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		title := strings.TrimSpace(body.Title)
		if title == "" {
			http.Error(w, "title must not be empty", http.StatusBadRequest)
			return
		}

		switch {
		case ch.SiteExport:
			siteDir := filepath.Join(chStore.OutputDir(slug), "site")
			statePath := filepath.Join(siteDir, "site.json")
			albums, err := loadSiteState(statePath)
			if err != nil {
				http.Error(w, "read site state: "+err.Error(), http.StatusInternalServerError)
				return
			}
			idx := indexOfAlbum(albums, postID)
			if idx == -1 {
				http.Error(w, "gallery not found", http.StatusNotFound)
				return
			}
			albums[idx].Title = title
			if err := saveSiteState(statePath, albums); err != nil {
				http.Error(w, "save site state: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if _, _, err := rebuildSiteChannel(chStore, mgr, ch); err != nil {
				http.Error(w, "regenerate site: "+err.Error(), http.StatusInternalServerError)
				return
			}
		case ch.GalleryExport:
			outDir, ok := pathguard.SafePath(chStore.OutputDir(slug), postID)
			if !ok {
				http.Error(w, "invalid gallery id", http.StatusBadRequest)
				return
			}
			statePath := filepath.Join(outDir, "gallery.json")
			gs, err := loadGalleryState(statePath)
			if err != nil || gs == nil {
				http.Error(w, "gallery not found", http.StatusNotFound)
				return
			}
			gs.Title = title
			if err := saveGalleryState(statePath, gs); err != nil {
				http.Error(w, "save gallery state: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if err := regenerateGalleryFolder(outDir, gs); err != nil {
				http.Error(w, "regenerate gallery: "+err.Error(), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "channel is not configured for gallery or site export", http.StatusBadRequest)
			return
		}

		writeJSON(w, map[string]any{"success": true, "title": title})
	}
}

// indexOfAlbum returns the index of the album with the given postID, or -1.
func indexOfAlbum(albums []SiteAlbum, postID string) int {
	for i := range albums {
		if albums[i].PostID == postID {
			return i
		}
	}
	return -1
}

// deleteGallery removes one published album/gallery: its statefile entry and
// local output folder always; its remote copy too, best-effort, only when
// the caller opts in via {"deleteRemote": true} and the channel uses the
// rsync handler. Remote deletion runs over SSH and is entirely separate from
// deploy (which never uses rsync --delete — see rsyncArgs — so a gallery
// removed locally has never been automatically removed from the remote).
func deleteGallery(chStore *channels.Store, mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		postID := r.PathValue("postID")
		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusNotFound)
			return
		}

		var body struct {
			DeleteRemote bool `json:"deleteRemote"`
		}
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck // absent/empty body just means deleteRemote=false
		}

		var localDir, remoteSubpath string
		var regenErr error

		switch {
		case ch.SiteExport:
			siteDir := filepath.Join(chStore.OutputDir(slug), "site")
			statePath := filepath.Join(siteDir, "site.json")
			albums, err := loadSiteState(statePath)
			if err != nil {
				http.Error(w, "read site state: "+err.Error(), http.StatusInternalServerError)
				return
			}
			idx := indexOfAlbum(albums, postID)
			if idx == -1 {
				http.Error(w, "gallery not found", http.StatusNotFound)
				return
			}
			folder := albumFolderName(albums[idx])
			safeDir, ok := pathguard.SafePath(siteDir, filepath.Join("albums", folder))
			if !ok {
				http.Error(w, "invalid gallery path", http.StatusBadRequest)
				return
			}
			localDir = safeDir
			remoteSubpath = "albums/" + folder

			remaining := append(append([]SiteAlbum{}, albums[:idx]...), albums[idx+1:]...)
			if err := saveSiteState(statePath, remaining); err != nil {
				http.Error(w, "save site state: "+err.Error(), http.StatusInternalServerError)
				return
			}
			os.RemoveAll(localDir) //nolint:errcheck
			if _, _, err := rebuildSiteChannel(chStore, mgr, ch); err != nil {
				regenErr = err
			}
		case ch.GalleryExport:
			safeDir, ok := pathguard.SafePath(chStore.OutputDir(slug), postID)
			if !ok {
				http.Error(w, "invalid gallery id", http.StatusBadRequest)
				return
			}
			if _, err := os.Stat(filepath.Join(safeDir, "gallery.json")); err != nil {
				http.Error(w, "gallery not found", http.StatusNotFound)
				return
			}
			localDir = safeDir
			remoteSubpath = postID
			os.RemoveAll(localDir) //nolint:errcheck
		default:
			http.Error(w, "channel is not configured for gallery or site export", http.StatusBadRequest)
			return
		}

		result := map[string]any{"success": true}
		if regenErr != nil {
			result["regenerateError"] = regenErr.Error()
		}
		if body.DeleteRemote {
			if ch.Handler != "rsync" {
				result["remoteDeleteError"] = "channel does not use the rsync handler"
			} else if target, err := deploy.TargetFromConfig(ch.HandlerConfig); err != nil {
				result["remoteDeleteError"] = err.Error()
			} else if out, err := deploy.DeleteRemote(target, remoteSubpath); err != nil {
				result["remoteDeleteError"] = err.Error() + "\n" + out
			} else {
				result["remoteDeleted"] = true
			}
		}
		writeJSON(w, result)
	}
}
