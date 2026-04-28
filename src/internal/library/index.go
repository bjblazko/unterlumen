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
	"time"

	_ "image/png"

	"huepattl.de/unterlumen/internal/media"
)

// Progress reports the state of an ongoing index operation.
type Progress struct {
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Current  string `json:"current,omitempty"`
	Finished bool   `json:"finished,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Indexer walks a source directory and populates a library store.
type Indexer struct {
	store      *Store
	libDir     string
	sourcePath string
}

// NewIndexer creates an Indexer for the given store and source path.
func NewIndexer(store *Store, libDir, sourcePath string) *Indexer {
	return &Indexer{store: store, libDir: libDir, sourcePath: sourcePath}
}

// Run walks the source directory, indexes all supported photos, and sends
// Progress events on the provided channel. The channel is closed when done.
func (idx *Indexer) Run(ctx context.Context, progress chan<- Progress) {
	defer close(progress)

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

		progress <- Progress{Done: i, Total: total, Current: filepath.Base(absPath)}

		if err := idx.indexFile(absPath); err != nil {
			// Log but continue — single-file errors should not abort the index.
			continue
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	idx.store.SetProp("last_indexed", now)

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
		return nil
	}

	// Compute SHA-256 canonical identity.
	photoID, err := hashFile(absPath)
	if err != nil {
		return err
	}

	// Rename case: photo already indexed under a different path.
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
		return nil
	}

	// New photo: extract EXIF.
	var exifJSON string
	exifFields := make(map[string]string)
	if exifData, err := media.ExtractAllEXIF(absPath); err == nil {
		for k, v := range exifData.Tags {
			exifFields[k] = v
		}
		if exifData.DateTaken != nil {
			exifFields["DateTaken"] = *exifData.DateTaken
		}
		if b, err := json.Marshal(exifData); err == nil {
			exifJSON = string(b)
		}
	}

	// Generate and store HQ thumbnail.
	thumbRel, _ := idx.ensureThumbnail(absPath, photoID) // non-fatal on error

	if err := idx.store.UpsertPhoto(photoID, absPath, filepath.Base(absPath), fileSize, time.Now().UTC(), exifJSON, thumbRel); err != nil {
		return err
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

// indexSidecar reads XMP sidecar publications for absPath and upserts them into photo_meta.
// Non-fatal: errors are silently ignored (sidecar may not exist).
func (idx *Indexer) indexSidecar(absPath, photoID string) {
	pubs, err := media.ReadSidecar(absPath)
	if err != nil || len(pubs) == 0 {
		return
	}

	type latestEntry struct {
		ts      string
		account string
		postID  string
	}
	latest := make(map[string]latestEntry)

	for _, p := range pubs {
		ts := p.PublishedAt.UTC().Format(time.RFC3339)
		if e, ok := latest[p.Channel]; !ok || ts > e.ts {
			latest[p.Channel] = latestEntry{ts: ts, account: p.Account, postID: p.PostID}
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
	}
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
