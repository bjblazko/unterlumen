package publish

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
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
	"huepattl.de/unterlumen/internal/site"
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
		store, err := mgr.OpenStore(r.PathValue("id"))
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		tmpPath, err := writeDownloadZip(store, ch, body.PhotoIDs, body.RecordXMP != nil && *body.RecordXMP)
		if tmpPath != "" {
			defer os.Remove(tmpPath)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		serveZipFile(w, tmpPath, ch.Slug+"-export.zip")
	}
}

// writeDownloadZip exports the photos into a ZIP in a temporary file and
// returns its path, which the caller removes. Photos that are unknown or
// cannot be exported are left out.
func writeDownloadZip(store *lib.Store, ch *channels.Channel, photoIDs []string, recordXMP bool) (string, error) {
	tmpFile, err := os.CreateTemp("", "unterlumen-channel-zip-*.zip")
	if err != nil {
		return "", errors.New("create temp file: " + err.Error())
	}
	publishedAt := time.Now().UTC()
	ex := downloadExport{store: store, ch: ch, opts: ch.ExportOptions(), ts: publishedAt.Format("20060102T150405Z"), ext: exportExt(ch)}
	if recordXMP {
		ex.record = &media.Publication{Channel: ch.Slug, PostID: newPostID(), PublishedAt: publishedAt}
	}
	zw := zip.NewWriter(tmpFile)
	for _, photoID := range photoIDs {
		ex.add(zw, photoID)
	}
	zw.Close()
	tmpFile.Close()
	return tmpFile.Name(), nil
}

// downloadExport adds photos, exported with a destination's settings, to a
// download ZIP.
type downloadExport struct {
	store  *lib.Store
	ch     *channels.Channel
	opts   media.ExportOptions
	ts     string             // the export time in file names
	ext    string             // the exported files' extension
	record *media.Publication // recorded on each photo, or nil
}

func (ex downloadExport) add(zw *zip.Writer, photoID string) {
	pathHint, err := ex.store.GetPhotoPathHint(photoID)
	if err != nil || pathHint == "" {
		return
	}
	data, err := media.ExportImage(pathHint, ex.opts)
	if err != nil {
		return
	}
	base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
	fw, err := zw.Create(ex.ch.Slug + "_" + ex.ts + "_" + base + ex.ext)
	if err != nil {
		return
	}
	if _, err := fw.Write(data); err == nil && ex.record != nil {
		recordPublication(ex.store, photoID, pathHint, *ex.record) //nolint:errcheck // the file is in the ZIP either way
	}
}

// serveZipFile sends a ZIP file as an attachment.
func serveZipFile(w http.ResponseWriter, path, name string) {
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "open temp file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	if info, statErr := os.Stat(path); statErr == nil {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	io.Copy(w, f) //nolint:errcheck
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

// exportExt is the extension of a channel's exported files.
func exportExt(ch *channels.Channel) string {
	if ch.Format == "jpeg" {
		return ".jpg"
	}
	return "." + ch.Format
}

func buildOne(store *lib.Store, ch *channels.Channel, pub media.Publication, ts, outDir, thumbDir, photoID string, recordXMP bool) buildResult {
	pathHint, err := store.GetPhotoPathHint(photoID)
	if err != nil || pathHint == "" {
		return buildResult{PhotoID: photoID, Error: "photo not found"}
	}

	exported, err := media.ExportImage(pathHint, ch.ExportOptions())
	if err != nil {
		return buildResult{PhotoID: photoID, Error: "export: " + err.Error()}
	}

	base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
	outName := ch.Slug + "_" + ts + "_" + base + exportExt(ch)
	outPath := filepath.Join(outDir, outName)

	if err := os.WriteFile(outPath, exported, 0o644); err != nil {
		return buildResult{PhotoID: photoID, Error: "write export: " + err.Error()}
	}
	// Recorded only once the file is there: a photo marked first and then
	// failing showed as in the gallery with no file to post.
	if recordXMP {
		if err := recordPublication(store, photoID, pathHint, pub); err != nil {
			os.Remove(outPath) //nolint:errcheck
			return buildResult{PhotoID: photoID, Error: "xmp: " + err.Error()}
		}
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

// recordPublication records an exported photo as published: in its sidecar,
// then as the destination's built: keys in the library.
func recordPublication(store *lib.Store, photoID, pathHint string, pub media.Publication) error {
	if err := media.AppendPublication(pathHint, pub); err != nil {
		return err
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
	return nil
}

// scanAlbumPhotos reconstructs a site.GalleryItem list from the files on disk.
// Used when rebuilding albums that were built before photo metadata was stored in site.json.
func scanAlbumPhotos(albumDir string) []site.GalleryItem {
	entries, err := os.ReadDir(albumDir)
	if err != nil {
		return nil
	}
	skip := map[string]bool{"index.html": true, "photos.zip": true, "cover.jpg": true}
	var items []site.GalleryItem
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
		items = append(items, site.GalleryItem{Filename: e.Name(), ThumbFilename: thumbName})
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
	existingPhotos      []site.Photo
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
func resolveAlbumTarget(draft *channels.Draft, channelDir string, sites *site.Store, publishedAt time.Time, galleryMode, siteMode bool) (albumTarget, int, error) {
	t := albumTarget{outDir: channelDir}

	if draft.Target.PostID == "" {
		t.postID = newPostID()
		t.unlisted = draft.Target.Unlisted
		switch {
		case galleryMode:
			t.outDir = filepath.Join(channelDir, t.postID)
		case siteMode:
			existingAlbums, _ := sites.List()
			t.slug = site.ComputeSlug(draft.Target.Title, publishedAt, existingAlbums, draft.Target.Unlisted)
			t.outDir = filepath.Join(channelDir, "site", "albums", t.slug)
		}
		return t, 0, nil
	}

	t.postID = draft.Target.PostID
	switch {
	case galleryMode:
		t.outDir = filepath.Join(channelDir, t.postID)
		gs, err := site.LoadGalleryState(filepath.Join(t.outDir, "gallery.json"))
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
			t.slug = site.AlbumFolderName(siteAlbums[i])
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
