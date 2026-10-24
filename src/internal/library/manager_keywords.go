package library

import (
	"context"
	"fmt"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/things"
)

// SetKeywordLanguage sets the language the library writes things in as
// keywords into the sidecars (ADR-0056), and rewrites those it wrote in
// another — a pass over the sidecars, without models, in the status line.
// A shared library's marker carries the language to the other
// installation, whose index keeps things by their IDs and needs no change.
func (m *Manager) SetKeywordLanguage(id, code string) (*Library, error) {
	if _, ok := things.Languages[code]; !ok {
		return nil, fmt.Errorf("there are no keywords in %q", code)
	}
	l, err := m.GetLibrary(id)
	if err != nil {
		return nil, err
	}
	if l.KeywordLanguage == code {
		return l, nil
	}
	if err := m.setKeywordLanguage(id, code); err != nil {
		return nil, err
	}
	l.KeywordLanguage = code
	if ownMarker(l) != nil {
		if err := writeMarker(l.SourcePath, markerOf(l)); err != nil {
			return nil, fmt.Errorf("the shared language could not be written: %w", err)
		}
		l.Shared = true
	}
	go m.rewriteKeywords(id)
	return l, nil
}

func (m *Manager) setKeywordLanguage(id, code string) error {
	store, err := m.OpenStore(id)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.SetProp(keywordLanguageProp, code)
}

// rewriteKeywords writes the keywords of every photo whose sidecar holds
// things in the library's language, as a scan: one at a time per library.
// A scan that runs is waited for, once.
func (m *Manager) rewriteKeywords(id string) {
	b, started := m.StartScan(id, "Writing keywords in")
	if !started {
		return
	}
	defer m.EndScan(id)
	defer b.Close()
	store, err := m.OpenStore(id)
	if err != nil {
		b.Send(Progress{Error: err.Error(), Finished: true})
		return
	}
	defer store.Close()
	paths, err := store.sidecarsWithThings()
	if err != nil {
		b.Send(Progress{Error: err.Error(), Finished: true})
		return
	}
	lex := store.lexicon()
	var failed error
	for i, p := range paths {
		if _, err := media.RewriteThingKeywords(p, lex); err != nil && failed == nil {
			failed = err
		}
		b.Send(Progress{Done: i + 1, Total: len(paths)})
	}
	if failed != nil {
		b.Send(Progress{Done: len(paths), Total: len(paths), Finished: true,
			Error: fmt.Sprintf("some sidecars could not be written: %v", failed)})
		return
	}
	b.Send(Progress{Done: len(paths), Total: len(paths), Finished: true})
}

// sidecarsWithThings lists the files of the photos whose sidecar holds
// their things record.
func (s *Store) sidecarsWithThings() ([]string, error) {
	rows, err := s.db.QueryContext(context.Background(), `SELECT p.path_hint FROM photos p
		JOIN photo_things_found f ON f.photo_id = p.id WHERE f.written = 1 AND p.status = 'ok'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// markerOf is what a shared library's marker says of it.
func markerOf(l *Library) Marker {
	return Marker{ID: l.ID, Name: l.Name, Description: l.Description, KeywordLanguage: l.KeywordLanguage}
}
