package apilibrary

import (
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/channels"
)

// SiteStore is a website channel's album list: the shared register is the
// truth, site.json in this machine's output directory is a cache of it that
// gets refreshed after every change.
//
// Callers say what happened to one album — Upsert("this one changed") or
// Remove("this one is gone") — and never hand over a list, so a machine cannot
// delete albums it merely does not know about.
type SiteStore struct {
	reg       *albumRegister
	cachePath string
}

func NewSiteStore(chStore *channels.Store, slug string) *SiteStore {
	return &SiteStore{
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
func (s *SiteStore) List() ([]SiteAlbum, error) {
	albums, err := s.reg.List()
	if err != nil || len(albums) > 0 {
		return albums, err
	}
	if _, statErr := os.Stat(s.reg.dir); statErr == nil {
		return albums, nil
	}
	legacy, err := LoadSiteState(s.cachePath)
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
func (s *SiteStore) Upsert(a SiteAlbum) error {
	if _, err := s.List(); err != nil { // adopt legacy albums before the register stops being empty
		return err
	}
	if err := s.reg.Upsert(a); err != nil {
		return err
	}
	return s.refreshCache()
}

// Remove records that one album lost its entry — for instance because it ran
// out of photos — without forbidding it to come back. Use Delete for an album
// the user deleted.
func (s *SiteStore) Remove(postID string) error {
	if _, err := s.List(); err != nil {
		return err
	}
	if err := s.reg.Remove(postID); err != nil {
		return err
	}
	return s.refreshCache()
}

// Delete records that one album was deleted on purpose. Unlike Remove it
// leaves a tombstone, so the album cannot be restored from the photos.
func (s *SiteStore) Delete(postID string) error {
	if _, err := s.List(); err != nil {
		return err
	}
	if err := s.reg.Delete(postID); err != nil {
		return err
	}
	return s.refreshCache()
}

// IsDeleted reports whether an album was deleted on purpose.
func (s *SiteStore) IsDeleted(postID string) bool { return s.reg.IsDeleted(postID) }

// refreshCache rewrites the local site.json from the register. The register
// already holds the change, so a cache that cannot be written is not an error
// the caller can do anything about; the next change rewrites it.
func (s *SiteStore) refreshCache() error {
	albums, err := s.reg.List()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.cachePath), 0o755); err != nil {
		return nil
	}
	SaveSiteState(s.cachePath, albums) //nolint:errcheck
	return nil
}
