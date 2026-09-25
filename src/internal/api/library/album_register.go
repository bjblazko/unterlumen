package apilibrary

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// albumRegister is a website channel's list of albums, kept one file per album
// (<postID>.json) in the shared channel directory. Its API is deliberately
// Upsert and Remove and never "save the list": a machine that wrote the albums
// it happens to know about would delete the other installation's albums.
//
// Nothing in a record may be a machine-local path — an album names its folder
// by slug and its files relative to the site directory. Which site directory
// that is belongs to channels.Store.OutputDir, not to the register.
type albumRegister struct{ dir string }

func newAlbumRegister(dir string) *albumRegister { return &albumRegister{dir: dir} }

func (r *albumRegister) file(postID string) (string, error) {
	if postID == "" || postID != filepath.Base(postID) || strings.ContainsAny(postID, `/\`) || postID == "." || postID == ".." {
		return "", fmt.Errorf("album register: invalid postID %q", postID)
	}
	return filepath.Join(r.dir, postID+".json"), nil
}

// Upsert writes one album, replacing its previous record if any.
func (r *albumRegister) Upsert(a SiteAlbum) error {
	path, err := r.file(a.PostID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return err
	}
	// Write-then-rename so a second installation never reads half a file.
	tmp, err := os.CreateTemp(r.dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Remove deletes one album's record. An album that is not there is not an error.
func (r *albumRegister) Remove(postID string) error {
	path, err := r.file(postID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// List returns every registered album, oldest first, so callers get a stable order.
func (r *albumRegister) List() ([]SiteAlbum, error) {
	entries, err := os.ReadDir(r.dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var albums []SiteAlbum
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var a SiteAlbum
		if err := json.Unmarshal(data, &a); err != nil {
			return nil, fmt.Errorf("album register: %s: %w", e.Name(), err)
		}
		albums = append(albums, a)
	}
	sort.SliceStable(albums, func(i, j int) bool {
		if !albums[i].PublishedAt.Equal(albums[j].PublishedAt) {
			return albums[i].PublishedAt.Before(albums[j].PublishedAt)
		}
		return albums[i].PostID < albums[j].PostID
	})
	return albums, nil
}
