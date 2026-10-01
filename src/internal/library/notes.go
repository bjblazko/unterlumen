package library

import (
	"fmt"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// Notes are the title and the free fields a person writes about a photo. They
// live in the photo's XMP sidecar, where every installation that sees the
// photo reads them; photo_meta only copies them for search and filters
// (ADR-0048). Keys made by publishing (built:, published:, pending:) are not
// notes: they are derived from the publication records and drafts.

// notesInSidecarProp marks a library whose notes have been written out to the
// sidecars. Until then a scan only adds notes from sidecars and never removes
// one the index alone holds, so nothing is lost before the move.
const notesInSidecarProp = "notes_in_sidecar"

// IsNoteKey says key is a note: the title or a free field.
func IsNoteKey(key string) bool {
	for _, p := range []string{"built:", "published:", "pending:"} {
		if strings.HasPrefix(key, p) {
			return false
		}
	}
	return key != ""
}

// WriteNote writes a note to the sidecar beside the photo first, then to the
// index. An empty value removes it. When the sidecar cannot be written the
// index is left as it is, so the two never disagree.
func (s *Store) WriteNote(photoID, key, value string) error {
	path, err := s.GetPhotoPathHint(photoID)
	if err != nil || path == "" {
		return fmt.Errorf("the photo's file is not known; scan the library and try again")
	}
	if err := writeNote(path, key, value); err != nil {
		return fmt.Errorf("the sidecar beside the photo could not be written, so nothing was changed: %w", err)
	}
	if value == "" {
		return s.DeleteMeta(photoID, key)
	}
	return s.UpsertMeta(photoID, key, value)
}

// NotesInSidecars says the library's notes live in the sidecars, so the index
// may drop a note the sidecar no longer has.
func (s *Store) NotesInSidecars() bool {
	v, ok, _ := s.GetProp(notesInSidecarProp)
	return ok && v == "1"
}

// ApplyNotes makes the photo's notes in the index what the sidecar says.
// Without replace it only adds and updates (see notesInSidecarProp).
func (s *Store) ApplyNotes(photoID string, notes media.Notes, replace bool) error {
	want := make(map[string]string, len(notes.Fields)+1)
	for k, v := range notes.Fields {
		if IsNoteKey(k) && k != "title" && v != "" {
			want[k] = v
		}
	}
	if notes.Title != "" {
		want["title"] = notes.Title
	}
	have, err := s.GetMeta(photoID)
	if err != nil {
		return err
	}
	for _, e := range have {
		if !IsNoteKey(e.Key) {
			continue
		}
		v, keep := want[e.Key]
		switch {
		case !keep && replace:
			if err := s.DeleteMeta(photoID, e.Key); err != nil {
				return err
			}
		case keep && v == e.Value:
			delete(want, e.Key) // unchanged; no write
		}
	}
	for k, v := range want {
		if err := s.UpsertMeta(photoID, k, v); err != nil {
			return err
		}
	}
	return nil
}
