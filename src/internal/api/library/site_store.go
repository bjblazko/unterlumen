package apilibrary

import (
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/channels"
)

// siteStore is a website channel's album list: the shared register is the
// truth, site.json in this machine's output directory is a cache of it that
// gets refreshed after every change.
//
// Callers say what happened to one album — Upsert("this one changed") or
// Remove("this one is gone") — and never hand over a list, so a machine cannot
// delete albums it merely does not know about.
type siteStore struct {
	reg       *albumRegister
	cachePath string
}

func newSiteStore(chStore *channels.Store, slug string) *siteStore {
	return &siteStore{
		reg:       newAlbumRegister(chStore.AlbumRegisterDir(slug)),
		cachePath: filepath.Join(SiteDir(chStore.OutputDir(slug)), "site.json"),
	}
}

// List returns every album in the register. A register that has never been
// used (its directory does not exist yet) adopts the albums of a site.json
// written before the register existed, once. After that the local cache is
// never read as a source, because it may hold an album the other installation
// has since deleted — which is also why "no albums left" does not count as
// "never used".
func (s *siteStore) List() ([]SiteAlbum, error) {
	albums, err := s.reg.List()
	if err != nil || len(albums) > 0 {
		return albums, err
	}
	if _, statErr := os.Stat(s.reg.dir); statErr == nil {
		return albums, nil
	}
	legacy, err := loadSiteState(s.cachePath)
	if err != nil || len(legacy) == 0 {
		return nil, nil
	}
	for _, a := range legacy {
		if err := s.reg.Upsert(a); err != nil {
			return nil, err
		}
	}
	return s.reg.List()
}

// Upsert records that one album was written or changed.
func (s *siteStore) Upsert(a SiteAlbum) error {
	if _, err := s.List(); err != nil { // adopt legacy albums before the register stops being empty
		return err
	}
	if err := s.reg.Upsert(a); err != nil {
		return err
	}
	return s.refreshCache()
}

// Remove records that one album is gone.
func (s *siteStore) Remove(postID string) error {
	if _, err := s.List(); err != nil {
		return err
	}
	if err := s.reg.Remove(postID); err != nil {
		return err
	}
	return s.refreshCache()
}

// refreshCache rewrites the local site.json from the register. The register
// already holds the change, so a cache that cannot be written is not an error
// the caller can do anything about; the next change rewrites it.
func (s *siteStore) refreshCache() error {
	albums, err := s.reg.List()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.cachePath), 0o755); err != nil {
		return nil
	}
	saveSiteState(s.cachePath, albums) //nolint:errcheck
	return nil
}
