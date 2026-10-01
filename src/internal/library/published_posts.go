package library

import (
	"sort"
	"strings"
	"time"
)

// PublishedPost is one post of a destination that keeps no list of its own —
// a plain export such as Instagram — as the libraries know it from the
// photos' publication records (built:<slug>:<postID>, from the sidecars).
type PublishedPost struct {
	PostID      string
	Title       string
	PublishedAt time.Time // the first publish
	UpdatedAt   time.Time // the last one, when photos were added later; zero otherwise
	PhotoCount  int
}

// PublishedPosts returns every post of the destination slug across all
// libraries, newest first. A photo in two libraries counts once.
func (m *Manager) PublishedPosts(slug string) ([]PublishedPost, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, err
	}
	posts := map[string]*PublishedPost{}
	photos := map[string]map[string]bool{}
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		rows, err := store.builtMeta(slug)
		if err != nil {
			continue
		}
		for _, r := range rows {
			collectPostRow(posts, photos, r)
		}
	}
	out := make([]PublishedPost, 0, len(posts))
	for id, p := range posts {
		p.PhotoCount = len(photos[id])
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PublishedAt.After(out[j].PublishedAt) })
	return out, nil
}

// PublishedPost returns one post of slug, and whether it is known.
func (m *Manager) PublishedPost(slug, postID string) (PublishedPost, bool) {
	posts, err := m.PublishedPosts(slug)
	if err != nil {
		return PublishedPost{}, false
	}
	for _, p := range posts {
		if p.PostID == postID {
			return p, true
		}
	}
	return PublishedPost{}, false
}

// metaRow is one photo_meta row of a publication key, the key without its
// "built:<slug>:" prefix.
type metaRow struct {
	photoID, rest, value string
}

// builtMeta reads the album-qualified publication keys of slug.
func (s *Store) builtMeta(slug string) ([]metaRow, error) {
	prefix := "built:" + slug + ":"
	rows, err := s.db.Query(`SELECT photo_id, substr(key, ?), value FROM photo_meta
		WHERE substr(key, 1, ?) = ?`, len(prefix)+1, len(prefix), prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []metaRow
	for rows.Next() {
		var r metaRow
		if err := rows.Scan(&r.photoID, &r.rest, &r.value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// collectPostRow adds one key to the posts: "<postID>" carries the publish
// time and makes the photo a member, "<postID>:title" the title. The
// unqualified "postid", "title" and "account" describe only the newest post
// and are skipped (ADR-0027).
func collectPostRow(posts map[string]*PublishedPost, photos map[string]map[string]bool, r metaRow) {
	postID, field, _ := strings.Cut(r.rest, ":")
	switch postID {
	case "", "postid", "title", "account":
		return
	}
	p := posts[postID]
	if p == nil {
		p = &PublishedPost{PostID: postID}
		posts[postID] = p
		photos[postID] = map[string]bool{}
	}
	switch field {
	case "":
		photos[postID][r.photoID] = true
		if t, err := time.Parse(time.RFC3339, r.value); err == nil {
			p.stamp(t)
		}
	case "title":
		if r.value != "" {
			p.Title = r.value
		}
	}
}

// stamp widens the post's dates to include t.
func (p *PublishedPost) stamp(t time.Time) {
	switch {
	case p.PublishedAt.IsZero():
		p.PublishedAt = t
	case t.Before(p.PublishedAt):
		p.UpdatedAt, p.PublishedAt = latest(p.UpdatedAt, p.PublishedAt), t
	case t.After(p.PublishedAt):
		p.UpdatedAt = latest(p.UpdatedAt, t)
	}
}

func latest(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
