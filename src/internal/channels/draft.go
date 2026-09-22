package channels

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DraftPhoto references one photo, in one library, pending export to a channel.
// Nothing is exported and no file is written for a photo until Generate runs —
// this is purely an association recorded ahead of time so photos can be
// collected into a gallery incrementally, across sessions and libraries.
type DraftPhoto struct {
	LibraryID string `json:"libraryID"`
	PhotoID   string `json:"photoID"`
}

// DraftTarget identifies where a draft's photos will land once generated:
// either an existing gallery/album (PostID set) or a brand-new one (Title set).
// Mirrors the addToExisting/new-gallery distinction already used by the
// existing build pipeline (internal/api/library/handler.go's generateDraft).
type DraftTarget struct {
	PostID   string `json:"postID,omitempty"`   // non-empty = add to an existing gallery/album
	Title    string `json:"title,omitempty"`    // new gallery/album title; ignored if PostID is set
	Unlisted bool   `json:"unlisted,omitempty"` // fixed at draft creation; noindex (site export also hides it from index/sitemap)
	Account  string `json:"account,omitempty"`
}

// Draft is one pending collection of photos for a channel, not yet generated.
type Draft struct {
	ID     string       `json:"id"`
	Target DraftTarget  `json:"target"`
	Photos []DraftPhoto `json:"photos"`
}

// DraftStore manages drafts.json, one per channel output directory
// (sibling to that channel's gallery.json/site.json statefiles).
type DraftStore struct {
	channelStore *Store
	mu           sync.Mutex
}

// NewDraftStore creates a DraftStore backed by channelStore's output directories.
func NewDraftStore(channelStore *Store) *DraftStore {
	return &DraftStore{channelStore: channelStore}
}

func (s *DraftStore) path(slug string) string {
	return filepath.Join(s.channelStore.OutputDir(slug), "drafts.json")
}

// List returns all pending drafts for a channel. Returns an empty slice
// (never nil) when the channel has no drafts.json yet.
func (s *DraftStore) List(slug string) ([]*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	if drafts == nil {
		drafts = []*Draft{}
	}
	return drafts, nil
}

// Get returns one draft by ID.
func (s *DraftStore) Get(slug, draftID string) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	for _, d := range drafts {
		if d.ID == draftID {
			return d, nil
		}
	}
	return nil, fmt.Errorf("draft %q not found", draftID)
}

// Create starts a new draft for slug with the given target and initial photos.
func (s *DraftStore) Create(slug string, target DraftTarget, photos []DraftPhoto) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	d := &Draft{ID: newDraftID(), Target: target, Photos: dedupePhotos(nil, photos)}
	drafts = append(drafts, d)
	if err := s.writeLocked(slug, drafts); err != nil {
		return nil, err
	}
	return d, nil
}

// AppendPhotos adds photos to an existing draft.
func (s *DraftStore) AppendPhotos(slug, draftID string, photos []DraftPhoto) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	for _, d := range drafts {
		if d.ID == draftID {
			d.Photos = dedupePhotos(d.Photos, photos)
			if err := s.writeLocked(slug, drafts); err != nil {
				return nil, err
			}
			return d, nil
		}
	}
	return nil, fmt.Errorf("draft %q not found", draftID)
}

// dedupePhotos appends add to existing, skipping photos already present.
// Identity is (LibraryID, PhotoID): the same content hash can legitimately
// appear under two libraries, and those are two distinct source files.
// Without this, collecting the same selection twice exports every photo twice
// into the same gallery.
func dedupePhotos(existing, add []DraftPhoto) []DraftPhoto {
	seen := make(map[DraftPhoto]struct{}, len(existing)+len(add))
	out := make([]DraftPhoto, 0, len(existing)+len(add))
	for _, src := range [][]DraftPhoto{existing, add} {
		for _, p := range src {
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	return out
}

// SetTargetPostID records which album a draft's photos were published into, so
// a draft that only partially succeeded resumes into that same album on retry
// instead of minting a second one with the same title.
func (s *DraftStore) SetTargetPostID(slug, draftID, postID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return err
	}
	for _, d := range drafts {
		if d.ID == draftID {
			d.Target.PostID = postID
			return s.writeLocked(slug, drafts)
		}
	}
	return fmt.Errorf("draft %q not found", draftID)
}

// RemovePhoto removes one photo from a draft. If the draft becomes empty it is
// deleted and (nil, nil) is returned; otherwise the updated draft is returned.
func (s *DraftStore) RemovePhoto(slug, draftID, libraryID, photoID string) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	for i, d := range drafts {
		if d.ID != draftID {
			continue
		}
		filtered := d.Photos[:0]
		for _, p := range d.Photos {
			if !(p.LibraryID == libraryID && p.PhotoID == photoID) {
				filtered = append(filtered, p)
			}
		}
		d.Photos = filtered
		if len(d.Photos) == 0 {
			drafts = append(drafts[:i], drafts[i+1:]...)
			if err := s.writeLocked(slug, drafts); err != nil {
				return nil, err
			}
			return nil, nil
		}
		if err := s.writeLocked(slug, drafts); err != nil {
			return nil, err
		}
		return d, nil
	}
	return nil, fmt.Errorf("draft %q not found", draftID)
}

// Delete removes an entire draft, discarding its pending photos.
func (s *DraftStore) Delete(slug, draftID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return err
	}
	filtered := drafts[:0]
	for _, d := range drafts {
		if d.ID != draftID {
			filtered = append(filtered, d)
		}
	}
	return s.writeLocked(slug, filtered)
}

func newDraftID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	return fmt.Sprintf("%x", b)
}

func (s *DraftStore) loadLocked(slug string) ([]*Draft, error) {
	data, err := os.ReadFile(s.path(slug))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var drafts []*Draft
	return drafts, json.Unmarshal(data, &drafts)
}

func (s *DraftStore) writeLocked(slug string, drafts []*Draft) error {
	p := s.path(slug)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(drafts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
