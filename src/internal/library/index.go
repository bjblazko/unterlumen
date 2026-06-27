package library

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "image/png"

	"huepattl.de/unterlumen/internal/media"
)

// Progress reports the state of an ongoing index operation.
type Progress struct {
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Current  string `json:"current,omitempty"`
	Parent   string `json:"parent,omitempty"` // parent folder name of current file
	Finished bool   `json:"finished,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Indexer walks a source directory and populates a library store.
type Indexer struct {
	store      *Store
	libDir     string
	sourcePath string
	newPhotos  int
}

// NewIndexer creates an Indexer for the given store and source path.
func NewIndexer(store *Store, libDir, sourcePath string) *Indexer {
	return &Indexer{store: store, libDir: libDir, sourcePath: sourcePath}
}

// IndexFile indexes a single file. Safe to call concurrently with other indexers
// on the same library, but not with a concurrent full scan on the same Indexer.
func (idx *Indexer) IndexFile(absPath string) error {
	return idx.indexFile(absPath)
}

// NewPhotos returns the number of new photos indexed since this Indexer was created.
func (idx *Indexer) NewPhotos() int { return idx.newPhotos }

// Run walks the source directory, indexes all supported photos, and sends
// Progress events on the provided channel. The channel is closed when done.
func (idx *Indexer) Run(ctx context.Context, progress chan<- Progress) {
	defer close(progress)
	defer func() {
		if n, err := idx.store.CountPhotos(); err == nil {
			idx.store.SetProp("photo_count", strconv.Itoa(n)) //nolint:errcheck
		}
		if idx.newPhotos > 0 {
			now := time.Now().UTC().Format(time.RFC3339)
			idx.store.SetProp("last_new_photos", now) //nolint:errcheck
		}
	}()

	files, err := collectFiles(idx.sourcePath)
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	if err := idx.store.MarkAllMissing(); err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	total := len(files)
	for i, absPath := range files {
		select {
		case <-ctx.Done():
			return
		default:
		}

		progress <- Progress{Done: i, Total: total, Current: filepath.Base(absPath), Parent: filepath.Base(filepath.Dir(absPath))}

		if err := idx.indexFile(absPath); err != nil {
			// Log but continue — single-file errors should not abort the index.
			continue
		}
	}

	idx.store.PurgeMissingPhotos() //nolint:errcheck

	now := time.Now().UTC().Format(time.RFC3339)
	idx.store.SetProp("last_indexed", now) //nolint:errcheck

	progress <- Progress{Done: total, Total: total, Finished: true}
}

// RunScanNew walks the full source directory and indexes new or changed photos without
// removing photos for files that no longer exist on disk. Safe to call after an
// interrupted full index, or when new files were added by an external tool.
func (idx *Indexer) RunScanNew(ctx context.Context, progress chan<- Progress) {
	idx.RunScanNewInFolder(ctx, progress, "")
}

// RunScanNewInFolder is like RunScanNew but scoped to a subfolder relative to the
// library source path. An empty subfolder scans the full library.
func (idx *Indexer) RunScanNewInFolder(ctx context.Context, progress chan<- Progress, subfolder string) {
	defer close(progress)
	defer func() {
		if n, err := idx.store.CountPhotos(); err == nil {
			idx.store.SetProp("photo_count", strconv.Itoa(n)) //nolint:errcheck
		}
		if idx.newPhotos > 0 {
			now := time.Now().UTC().Format(time.RFC3339)
			idx.store.SetProp("last_new_photos", now) //nolint:errcheck
		}
	}()

	scanRoot := idx.sourcePath
	if subfolder != "" {
		candidate := filepath.Join(idx.sourcePath, filepath.Clean(subfolder))
		// Reject paths that escape the source directory.
		if candidate != idx.sourcePath && !strings.HasPrefix(candidate, idx.sourcePath+string(filepath.Separator)) {
			progress <- Progress{Error: "invalid subfolder path", Finished: true}
			return
		}
		scanRoot = candidate
	}

	files, err := collectFiles(scanRoot)
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	total := len(files)
	for i, absPath := range files {
		select {
		case <-ctx.Done():
			return
		default:
		}

		progress <- Progress{Done: i, Total: total, Current: filepath.Base(absPath), Parent: filepath.Base(filepath.Dir(absPath))}

		if err := idx.indexFile(absPath); err != nil {
			continue
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	idx.store.SetProp("last_indexed", now) //nolint:errcheck

	progress <- Progress{Done: total, Total: total, Finished: true}
}

func (idx *Indexer) indexFile(absPath string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	mtimeNs := info.ModTime().UnixNano()
	fileSize := info.Size()

	// Fast-path: if mtime and size match the cache, file is unchanged.
	cachedID, cachedMtime, cachedSize, found, err := idx.store.GetPathCache(absPath)
	if err != nil {
		return err
	}
	if found && cachedMtime == mtimeNs && cachedSize == fileSize {
		if err := idx.store.MarkPhotoPresent(cachedID, absPath, filepath.Base(absPath)); err != nil {
			return err
		}
		idx.indexSidecar(absPath, cachedID)
		// For HEIF files, attempt to generate a thumbnail if one was not produced during
		// initial indexing (e.g. because heif-convert failed on the first scan).
		if media.IsHEIF(absPath) {
			if currentThumb, _ := idx.store.GetPhotoThumbPath(cachedID); currentThumb == "" {
				if thumbRel, err := idx.ensureThumbnail(absPath, cachedID); err == nil {
					idx.store.SetPhotoThumbPath(cachedID, thumbRel) //nolint:errcheck
				}
			}
		}
		return nil
	}

	// Compute SHA-256 canonical identity.
	photoID, err := hashFile(absPath)
	if err != nil {
		return err
	}

	// Photo already indexed (rename case, or path-cache was cleared for forced re-index).
	exists, err := idx.store.PhotoExists(photoID)
	if err != nil {
		return err
	}
	if exists {
		if err := idx.store.MarkPhotoPresent(photoID, absPath, filepath.Base(absPath)); err != nil {
			return err
		}
		if err := idx.store.UpsertPathCache(absPath, photoID, mtimeNs, fileSize); err != nil {
			return err
		}
		idx.indexSidecar(absPath, photoID)
		// Regenerate missing thumbnail (e.g. after a forced re-index of a folder).
		if media.IsHEIF(absPath) {
			if currentThumb, _ := idx.store.GetPhotoThumbPath(photoID); currentThumb == "" {
				if thumbRel, err := idx.ensureThumbnail(absPath, photoID); err == nil {
					idx.store.SetPhotoThumbPath(photoID, thumbRel) //nolint:errcheck
				}
			}
		}
		return nil
	}

	// New photo: extract EXIF.
	exifJSON := "{}"
	exifFields := make(map[string]string)
	dateTaken := ""
	if exifData, err := media.ExtractAllEXIF(absPath); err == nil {
		for k, v := range exifData.Tags {
			exifFields[k] = v
		}
		if exifData.DateTaken != nil {
			dateTaken = *exifData.DateTaken
			exifFields["DateTaken"] = dateTaken
		}
		if b, err := json.Marshal(exifData); err == nil {
			exifJSON = string(b)
		}
	}

	ext := strings.TrimPrefix(filepath.Ext(strings.ToLower(filepath.Base(absPath))), ".")
	extNorm := map[string]string{"jpg": "jpeg", "hif": "heif", "heic": "heif"}
	if n, ok := extNorm[ext]; ok {
		ext = n
	}

	// Generate and store HQ thumbnail.
	thumbRel, _ := idx.ensureThumbnail(absPath, photoID) // non-fatal on error

	if err := idx.store.UpsertPhoto(photoID, absPath, filepath.Base(absPath), fileSize, time.Now().UTC(), exifJSON, thumbRel, dateTaken, ext); err != nil {
		return err
	}
	idx.newPhotos++
	numericValues := media.NormalizeExifNumbers(exifFields)
	if err := idx.store.UpsertExifIndex(photoID, exifFields, numericValues); err != nil {
		return err
	}
	if err := idx.store.UpsertPathCache(absPath, photoID, mtimeNs, fileSize); err != nil {
		return err
	}
	idx.indexSidecar(absPath, photoID)
	return nil
}

// forceReindexFile re-extracts EXIF, deletes any existing thumbnail from disk,
// regenerates the thumbnail, and updates the DB — regardless of mtime/size.
// It preserves photo_meta (publications, ratings, tags).
func (idx *Indexer) forceReindexFile(absPath string) error {
	info, err := os.Stat(absPath)
	if err != nil {
		return err
	}
	mtimeNs := info.ModTime().UnixNano()
	fileSize := info.Size()

	photoID, err := hashFile(absPath)
	if err != nil {
		return err
	}

	exifJSON := "{}"
	exifFields := make(map[string]string)
	dateTaken := ""
	if exifData, err := media.ExtractAllEXIF(absPath); err == nil {
		for k, v := range exifData.Tags {
			exifFields[k] = v
		}
		if exifData.DateTaken != nil {
			dateTaken = *exifData.DateTaken
			exifFields["DateTaken"] = dateTaken
		}
		if b, err := json.Marshal(exifData); err == nil {
			exifJSON = string(b)
		}
	}

	ext := strings.TrimPrefix(filepath.Ext(strings.ToLower(filepath.Base(absPath))), ".")
	extNorm := map[string]string{"jpg": "jpeg", "hif": "heif", "heic": "heif"}
	if n, ok := extNorm[ext]; ok {
		ext = n
	}

	// Delete existing thumbnail from disk so ensureThumbnail rebuilds it from scratch.
	absThumb := filepath.Join(idx.libDir, "thumbs", photoID[:2], photoID+".jpg")
	os.Remove(absThumb) //nolint:errcheck

	thumbRel, _ := idx.ensureThumbnail(absPath, photoID)

	exists, err := idx.store.PhotoExists(photoID)
	if err != nil {
		return err
	}
	if !exists {
		if err := idx.store.UpsertPhoto(photoID, absPath, filepath.Base(absPath), fileSize, time.Now().UTC(), exifJSON, thumbRel, dateTaken, ext); err != nil {
			return err
		}
	} else {
		if err := idx.store.UpdatePhotoExif(photoID, exifJSON, dateTaken); err != nil {
			return err
		}
		if err := idx.store.SetPhotoThumbPath(photoID, thumbRel); err != nil {
			return err
		}
		if err := idx.store.MarkPhotoPresent(photoID, absPath, filepath.Base(absPath)); err != nil {
			return err
		}
	}

	numericValues := media.NormalizeExifNumbers(exifFields)
	if err := idx.store.UpsertExifIndex(photoID, exifFields, numericValues); err != nil {
		return err
	}
	if err := idx.store.UpsertPathCache(absPath, photoID, mtimeNs, fileSize); err != nil {
		return err
	}
	idx.indexSidecar(absPath, photoID)
	return nil
}

const thumbMaxDim = 1200

func (idx *Indexer) ensureThumbnail(absPath, photoID string) (string, error) {
	prefix := photoID[:2]
	relPath := filepath.Join("thumbs", prefix, photoID+".jpg")
	absThumb := filepath.Join(idx.libDir, relPath)

	if _, err := os.Stat(absThumb); err == nil {
		return relPath, nil // already exists
	}

	if err := os.MkdirAll(filepath.Dir(absThumb), 0o700); err != nil {
		return "", err
	}

	var jpegData []byte
	if media.IsHEIF(absPath) {
		data, err := media.ExtractHEIFPreview(absPath)
		if err != nil {
			return "", err
		}
		data, err = media.ResizeJPEGBytes(data, thumbMaxDim)
		if err != nil {
			return "", err
		}
		jpegData = data
	} else {
		orientation := media.ExtractOrientation(absPath)
		data, ct, err := media.GenerateThumbnail(absPath, thumbMaxDim, orientation)
		if err != nil {
			return "", err
		}
		if ct != "image/jpeg" {
			// Re-encode non-JPEG formats as JPEG for consistent thumbnail storage.
			img, _, decErr := image.Decode(bytes.NewReader(data))
			if decErr == nil {
				var buf bytes.Buffer
				if encErr := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); encErr == nil {
					data = buf.Bytes()
				}
			}
		}
		jpegData = data
	}

	return relPath, os.WriteFile(absThumb, jpegData, 0o600)
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// indexSidecar reads XMP sidecar publications and title for absPath and upserts them into photo_meta.
// Non-fatal: errors are silently ignored (sidecar may not exist).
func (idx *Indexer) indexSidecar(absPath, photoID string) {
	pubs, _ := media.ReadSidecar(absPath)
	title, _ := media.ReadTitle(absPath)
	if len(pubs) == 0 && title == "" {
		return
	}

	if len(pubs) > 0 {
		type latestEntry struct {
			ts           string
			account      string
			postID       string
			galleryTitle string
		}
		latest := make(map[string]latestEntry)

		for _, p := range pubs {
			ts := p.PublishedAt.UTC().Format(time.RFC3339)
			if e, ok := latest[p.Channel]; !ok || ts > e.ts {
				latest[p.Channel] = latestEntry{
					ts:           ts,
					account:      p.Account,
					postID:       p.PostID,
					galleryTitle: p.GalleryTitle,
				}
			}
		}

		for ch, e := range latest {
			idx.store.UpsertMeta(photoID, "published:"+ch, e.ts) //nolint:errcheck
			if e.account != "" {
				idx.store.UpsertMeta(photoID, "published:"+ch+":account", e.account) //nolint:errcheck
			}
			if e.postID != "" {
				idx.store.UpsertMeta(photoID, "published:"+ch+":postid", e.postID) //nolint:errcheck
			}
			if e.galleryTitle != "" {
				idx.store.UpsertMeta(photoID, "published:"+ch+":title", e.galleryTitle) //nolint:errcheck
			}
		}
	}

	if title != "" {
		idx.store.UpsertMeta(photoID, "title", title) //nolint:errcheck
	}
}

// RunInFolder force-reindexes every file in subfolder (relative to sourcePath),
// regardless of mtime/size. It re-extracts EXIF, deletes any existing thumbnails
// from disk, and regenerates them from scratch — but preserves photo_meta
// (publications, ratings, tags). An empty subfolder reindexes the full source tree.
func (idx *Indexer) RunInFolder(ctx context.Context, progress chan<- Progress, subfolder string) {
	defer close(progress)
	defer func() {
		if n, err := idx.store.CountPhotos(); err == nil {
			idx.store.SetProp("photo_count", strconv.Itoa(n)) //nolint:errcheck
		}
	}()

	scanRoot := idx.sourcePath
	if subfolder != "" {
		candidate := filepath.Join(idx.sourcePath, filepath.Clean(subfolder))
		if candidate != idx.sourcePath && !strings.HasPrefix(candidate, idx.sourcePath+string(filepath.Separator)) {
			progress <- Progress{Error: "invalid subfolder path", Finished: true}
			return
		}
		scanRoot = candidate
	}

	files, err := collectFiles(scanRoot)
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	total := len(files)
	for i, absPath := range files {
		select {
		case <-ctx.Done():
			return
		default:
		}

		progress <- Progress{Done: i, Total: total, Current: filepath.Base(absPath), Parent: filepath.Base(filepath.Dir(absPath))}

		if err := idx.forceReindexFile(absPath); err != nil {
			continue
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	idx.store.SetProp("last_indexed", now) //nolint:errcheck

	progress <- Progress{Done: total, Total: total, Finished: true}
}

// resolveSubfolder validates and resolves subfolder (relative to sourcePath) to an
// absolute scan root. Returns an error message if the path escapes sourcePath.
func (idx *Indexer) resolveSubfolder(subfolder string) (string, string) {
	if subfolder == "" {
		return idx.sourcePath, ""
	}
	candidate := filepath.Join(idx.sourcePath, filepath.Clean(subfolder))
	if candidate != idx.sourcePath && !strings.HasPrefix(candidate, idx.sourcePath+string(filepath.Separator)) {
		return "", "invalid subfolder path"
	}
	return candidate, ""
}

// RunRegenerateMissingPreviewsInFolder generates thumbnails for photos inside
// subfolder that have no thumbnail in the DB or whose thumbnail file is missing
// on disk. It uses the path cache to avoid re-hashing source files.
// An empty subfolder covers the full source tree.
func (idx *Indexer) RunRegenerateMissingPreviewsInFolder(ctx context.Context, progress chan<- Progress, subfolder string) {
	defer close(progress)

	scanRoot, pathErr := idx.resolveSubfolder(subfolder)
	if pathErr != "" {
		progress <- Progress{Error: pathErr, Finished: true}
		return
	}

	files, err := collectFiles(scanRoot)
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	total := len(files)
	for i, absPath := range files {
		select {
		case <-ctx.Done():
			return
		default:
		}

		progress <- Progress{Done: i, Total: total, Current: filepath.Base(absPath), Parent: filepath.Base(filepath.Dir(absPath))}

		photoID, _, _, found, err := idx.store.GetPathCache(absPath)
		if err != nil || !found {
			continue
		}

		currentThumb, _ := idx.store.GetPhotoThumbPath(photoID)
		if currentThumb != "" {
			absThumb := filepath.Join(idx.libDir, currentThumb)
			if _, err := os.Stat(absThumb); err == nil {
				continue
			}
		}

		if thumbRel, err := idx.ensureThumbnail(absPath, photoID); err == nil {
			idx.store.SetPhotoThumbPath(photoID, thumbRel) //nolint:errcheck
		}
	}

	progress <- Progress{Done: total, Total: total, Finished: true}
}

// RunRebuildAllPreviewsInFolder deletes and regenerates every thumbnail inside
// subfolder without re-extracting EXIF. It uses the path cache to avoid re-hashing.
// An empty subfolder covers the full source tree.
func (idx *Indexer) RunRebuildAllPreviewsInFolder(ctx context.Context, progress chan<- Progress, subfolder string) {
	defer close(progress)

	scanRoot, pathErr := idx.resolveSubfolder(subfolder)
	if pathErr != "" {
		progress <- Progress{Error: pathErr, Finished: true}
		return
	}

	files, err := collectFiles(scanRoot)
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	total := len(files)
	for i, absPath := range files {
		select {
		case <-ctx.Done():
			return
		default:
		}

		progress <- Progress{Done: i, Total: total, Current: filepath.Base(absPath), Parent: filepath.Base(filepath.Dir(absPath))}

		photoID, _, _, found, err := idx.store.GetPathCache(absPath)
		if err != nil || !found {
			continue
		}

		absThumb := filepath.Join(idx.libDir, "thumbs", photoID[:2], photoID+".jpg")
		os.Remove(absThumb) //nolint:errcheck

		thumbRel, _ := idx.ensureThumbnail(absPath, photoID)
		idx.store.SetPhotoThumbPath(photoID, thumbRel) //nolint:errcheck
	}

	progress <- Progress{Done: total, Total: total, Finished: true}
}

// RunCleanupInFolder removes indexed photos inside subfolder whose source files
// no longer exist on disk. An empty subfolder cleans up the full library.
func (idx *Indexer) RunCleanupInFolder(ctx context.Context, progress chan<- Progress, subfolder string) {
	defer close(progress)
	defer func() {
		if n, err := idx.store.CountPhotos(); err == nil {
			idx.store.SetProp("photo_count", strconv.Itoa(n)) //nolint:errcheck
		}
	}()

	scanRoot := idx.sourcePath
	if subfolder != "" {
		candidate := filepath.Join(idx.sourcePath, filepath.Clean(subfolder))
		if candidate != idx.sourcePath && !strings.HasPrefix(candidate, idx.sourcePath+string(filepath.Separator)) {
			progress <- Progress{Error: "invalid subfolder path", Finished: true}
			return
		}
		scanRoot = candidate
	}

	presentPaths, err := collectFileSet(scanRoot)
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	var refs []PhotoRef
	if subfolder == "" {
		refs, err = idx.store.ListAllPhotoRefs()
	} else {
		refs, err = idx.store.ListPhotoRefsInFolder(scanRoot)
	}
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	total := len(refs)
	missing := 0
	for i, ref := range refs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		progress <- Progress{Done: i, Total: total, Current: filepath.Base(ref.PathHint)}

		if !presentPaths[ref.PathHint] {
			if err := idx.store.MarkPhotoMissing(ref.ID); err == nil {
				missing++
			}
		}
	}

	if missing > 0 {
		idx.store.PurgeMissingPhotos() //nolint:errcheck
	}

	now := time.Now().UTC().Format(time.RFC3339)
	idx.store.SetProp("last_indexed", now) //nolint:errcheck

	progress <- Progress{Done: total, Total: total, Finished: true}
}

// RunCleanup removes indexed photos whose source files no longer exist on disk.
// It does not re-hash or re-index anything — it only checks whether each photo's
// last-known path still exists. Renamed files that have not yet been synced will
// appear absent and be removed; run "Index new photos" first if you have renames.
func (idx *Indexer) RunCleanup(ctx context.Context, progress chan<- Progress) {
	defer close(progress)
	defer func() {
		if n, err := idx.store.CountPhotos(); err == nil {
			idx.store.SetProp("photo_count", strconv.Itoa(n)) //nolint:errcheck
		}
	}()

	presentPaths, err := collectFileSet(idx.sourcePath)
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	refs, err := idx.store.ListAllPhotoRefs()
	if err != nil {
		progress <- Progress{Error: err.Error(), Finished: true}
		return
	}

	total := len(refs)
	missing := 0
	for i, ref := range refs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		progress <- Progress{Done: i, Total: total, Current: filepath.Base(ref.PathHint)}

		if !presentPaths[ref.PathHint] {
			if err := idx.store.MarkPhotoMissing(ref.ID); err == nil {
				missing++
			}
		}
	}

	if missing > 0 {
		idx.store.PurgeMissingPhotos() //nolint:errcheck
	}

	now := time.Now().UTC().Format(time.RFC3339)
	idx.store.SetProp("last_indexed", now) //nolint:errcheck

	progress <- Progress{Done: total, Total: total, Finished: true}
}

func collectFileSet(root string) (map[string]bool, error) {
	files, err := collectFiles(root)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(files))
	for _, f := range files {
		set[f] = true
	}
	return set, nil
}

func collectFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if media.IsSupportedImage(d.Name()) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
