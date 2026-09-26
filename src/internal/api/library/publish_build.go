package apilibrary

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// --- Build ---

type buildResult struct {
	PhotoID       string `json:"photoID"`
	OutputPath    string `json:"outputPath,omitempty"`
	Filename      string `json:"filename,omitempty"`
	ThumbFilename string `json:"thumbFilename,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	Error         string `json:"error,omitempty"`
}

// buildDownload exports selected library photos with channel settings and delivers
// the result as a ZIP download. By default it does not update XMP sidecars or the
// library database; pass RecordXMP=true to opt in.
func buildDownload(mgr *lib.Manager, chStore *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		id := r.PathValue("id")

		var body struct {
			PhotoIDs  []string `json:"photoIDs"`
			Channel   string   `json:"channel"`
			RecordXMP *bool    `json:"recordXMP,omitempty"` // nil → false (default for download)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if len(body.PhotoIDs) == 0 || body.Channel == "" {
			http.Error(w, "photoIDs and channel required", http.StatusBadRequest)
			return
		}

		ch, err := chStore.Get(body.Channel)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		tmpFile, err := os.CreateTemp("", "unterlumen-channel-zip-*.zip")
		if err != nil {
			http.Error(w, "create temp file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		recordXMP := body.RecordXMP != nil && *body.RecordXMP
		publishedAt := time.Now().UTC()
		ts := publishedAt.Format("20060102T150405Z")
		opts := ch.ExportOptions()
		ext := "." + ch.Format
		if ch.Format == "jpeg" {
			ext = ".jpg"
		}

		var pub media.Publication
		if recordXMP {
			pub = media.Publication{
				Channel:     body.Channel,
				PostID:      newPostID(),
				PublishedAt: publishedAt,
			}
		}

		zw := zip.NewWriter(tmpFile)
		for _, photoID := range body.PhotoIDs {
			pathHint, pathErr := store.GetPhotoPathHint(photoID)
			if pathErr != nil || pathHint == "" {
				continue
			}
			if recordXMP {
				media.AppendPublication(pathHint, pub) //nolint:errcheck
				chKey := "built:" + pub.Channel
				qualKey := chKey + ":" + pub.PostID
				tsVal := publishedAt.Format(time.RFC3339)
				store.UpsertMeta(photoID, chKey, tsVal)   //nolint:errcheck
				store.UpsertMeta(photoID, qualKey, tsVal) //nolint:errcheck
				if pub.PostID != "" {
					store.UpsertMeta(photoID, chKey+":postid", pub.PostID) //nolint:errcheck
				}
				if pub.GalleryTitle != "" {
					store.UpsertMeta(photoID, chKey+":title", pub.GalleryTitle)   //nolint:errcheck
					store.UpsertMeta(photoID, qualKey+":title", pub.GalleryTitle) //nolint:errcheck
				}
			}
			data, expErr := media.ExportImage(pathHint, opts)
			if expErr != nil {
				continue
			}
			base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
			outName := ch.Slug + "_" + ts + "_" + base + ext
			if fw, fwErr := zw.Create(outName); fwErr == nil {
				fw.Write(data) //nolint:errcheck
			}
		}
		zw.Close()
		tmpFile.Close()

		f, err := os.Open(tmpPath)
		if err != nil {
			http.Error(w, "open temp file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if info, statErr := os.Stat(tmpPath); statErr == nil {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		}
		fname := ch.Slug + "-export.zip"
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fname))
		io.Copy(w, f) //nolint:errcheck
	}
}

func createGalleryZip(results []buildResult, outDir, zipName string) error {
	f, err := os.Create(filepath.Join(outDir, zipName))
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for _, res := range results {
		if res.Error != "" || res.Filename == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(outDir, res.Filename))
		if err != nil {
			continue
		}
		w, err := zw.Create(res.Filename)
		if err != nil {
			continue
		}
		w.Write(data) //nolint:errcheck
	}
	return nil
}

var galleryThumbOpts = media.ExportOptions{
	Format:   "jpeg",
	Quality:  78,
	ExifMode: "strip",
	Scale: media.ScaleOptions{
		Mode:         media.ScaleModeMaxDim,
		MaxDimension: "width",
		MaxValue:     700,
	},
}

func buildOne(store *lib.Store, ch *channels.Channel, pub media.Publication, ts, outDir, thumbDir, photoID string, recordXMP bool) buildResult {
	pathHint, err := store.GetPhotoPathHint(photoID)
	if err != nil || pathHint == "" {
		return buildResult{PhotoID: photoID, Error: "photo not found"}
	}

	if recordXMP {
		if err := media.AppendPublication(pathHint, pub); err != nil {
			return buildResult{PhotoID: photoID, Error: "xmp: " + err.Error()}
		}
		metaVal := pub.PublishedAt.UTC().Format(time.RFC3339)
		chKey := "built:" + pub.Channel
		qualKey := chKey + ":" + pub.PostID
		store.UpsertMeta(photoID, chKey, metaVal)   //nolint:errcheck
		store.UpsertMeta(photoID, qualKey, metaVal) //nolint:errcheck
		if pub.Account != "" {
			store.UpsertMeta(photoID, chKey+":account", pub.Account)   //nolint:errcheck
			store.UpsertMeta(photoID, qualKey+":account", pub.Account) //nolint:errcheck
		}
		if pub.PostID != "" {
			store.UpsertMeta(photoID, chKey+":postid", pub.PostID) //nolint:errcheck
		}
		if pub.GalleryTitle != "" {
			store.UpsertMeta(photoID, chKey+":title", pub.GalleryTitle)   //nolint:errcheck
			store.UpsertMeta(photoID, qualKey+":title", pub.GalleryTitle) //nolint:errcheck
		}
	}

	exported, err := media.ExportImage(pathHint, ch.ExportOptions())
	if err != nil {
		return buildResult{PhotoID: photoID, Error: "export: " + err.Error()}
	}

	ext := "." + ch.Format
	if ch.Format == "jpeg" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
	outName := ch.Slug + "_" + ts + "_" + base + ext
	outPath := filepath.Join(outDir, outName)

	if err := os.WriteFile(outPath, exported, 0o644); err != nil {
		return buildResult{PhotoID: photoID, Error: "write export: " + err.Error()}
	}

	res := buildResult{PhotoID: photoID, OutputPath: outPath, Filename: outName}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(exported)); err == nil {
		res.Width = cfg.Width
		res.Height = cfg.Height
	}

	if thumbDir != "" {
		if thumb, err := media.ExportImage(pathHint, galleryThumbOpts); err == nil {
			thumbName := "thumbs/" + outName
			if err := os.WriteFile(filepath.Join(thumbDir, outName), thumb, 0o644); err == nil {
				res.ThumbFilename = thumbName
			}
		}
	}

	return res
}

// scanAlbumPhotos reconstructs a GalleryItem list from the files on disk.
// Used when rebuilding albums that were built before photo metadata was stored in site.json.
func scanAlbumPhotos(albumDir string) []GalleryItem {
	entries, err := os.ReadDir(albumDir)
	if err != nil {
		return nil
	}
	skip := map[string]bool{"index.html": true, "photos.zip": true, "cover.jpg": true}
	var items []GalleryItem
	for _, e := range entries {
		if e.IsDir() || skip[e.Name()] {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}
		thumbName := "thumbs/" + e.Name()
		if _, statErr := os.Stat(filepath.Join(albumDir, thumbName)); statErr != nil {
			thumbName = e.Name() // no thumb — fall back to full-res
		}
		items = append(items, GalleryItem{Filename: e.Name(), ThumbFilename: thumbName})
	}
	return items
}

// albumTarget is where a draft's photos are written: the album's stable ID, its
// output folder, and — when appending to an album that already exists — that
// album's current state.
type albumTarget struct {
	postID              string
	slug                string // human-readable folder name; site mode only
	outDir              string
	existingPhotos      []SitePhoto
	existingTitle       string
	existingPublishedAt time.Time
	unlisted            bool
}

// resolveAlbumTarget decides whether a draft appends to an existing album or
// starts a new one, loading the existing album's state in the former case. A
// non-nil error carries the HTTP status the request should fail with.
//
// Unlisted is fixed at album creation: on add-to-existing it comes from the
// stored album, never from the draft, so appending photos can't silently
// un-hide an album whose link has already been shared.
func resolveAlbumTarget(draft *channels.Draft, channelDir string, sites *siteStore, publishedAt time.Time, galleryMode, siteMode bool) (albumTarget, int, error) {
	t := albumTarget{outDir: channelDir}

	if draft.Target.PostID == "" {
		t.postID = newPostID()
		t.unlisted = draft.Target.Unlisted
		switch {
		case galleryMode:
			t.outDir = filepath.Join(channelDir, t.postID)
		case siteMode:
			existingAlbums, _ := sites.List()
			t.slug = computeSlug(draft.Target.Title, publishedAt, existingAlbums, draft.Target.Unlisted)
			t.outDir = filepath.Join(channelDir, "site", "albums", t.slug)
		}
		return t, 0, nil
	}

	t.postID = draft.Target.PostID
	switch {
	case galleryMode:
		t.outDir = filepath.Join(channelDir, t.postID)
		gs, err := loadGalleryState(filepath.Join(t.outDir, "gallery.json"))
		if err != nil || gs == nil {
			return t, http.StatusBadRequest, fmt.Errorf("gallery not found: %s", t.postID)
		}
		t.existingPhotos, t.existingTitle, t.existingPublishedAt = gs.Photos, gs.Title, gs.PublishedAt
		t.unlisted = gs.Unlisted
	case siteMode:
		siteAlbums, err := sites.List()
		if err != nil {
			return t, http.StatusInternalServerError, fmt.Errorf("read site state: %w", err)
		}
		for i := range siteAlbums {
			if siteAlbums[i].PostID != t.postID {
				continue
			}
			t.existingPhotos, t.existingTitle = siteAlbums[i].Photos, siteAlbums[i].Title
			t.existingPublishedAt, t.unlisted = siteAlbums[i].PublishedAt, siteAlbums[i].Unlisted
			t.slug = albumFolderName(siteAlbums[i])
			break
		}
		if t.existingTitle == "" {
			return t, http.StatusBadRequest, fmt.Errorf("album not found: %s", t.postID)
		}
		t.outDir = filepath.Join(channelDir, "site", "albums", t.slug)
	}

	if _, err := os.Stat(t.outDir); os.IsNotExist(err) {
		return t, http.StatusBadRequest, fmt.Errorf("gallery folder not found: %s", t.postID)
	}
	return t, 0, nil
}
