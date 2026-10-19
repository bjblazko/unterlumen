package library

import (
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/media"
)

// publicationKeys turns the publications in a photo's sidecar into the
// built: keys the index keeps for them: one key set per album the photo
// appears in, and per destination an unqualified set — the "has been built
// here" marker the destination filter matches on — with the most recent
// album's details. Collapsing a destination to its newest album, as an
// earlier version did, dropped every earlier album the sidecar recorded.
func publicationKeys(pubs []media.Publication) map[string]string {
	keys := map[string]string{}
	set := func(k, v string) {
		if v != "" {
			keys[k] = v
		}
	}
	latest := map[string]media.Publication{}
	for _, p := range pubs {
		if e, ok := latest[p.Channel]; !ok || p.PublishedAt.After(e.PublishedAt) {
			latest[p.Channel] = p
		}
		if p.PostID == "" {
			continue
		}
		key := "built:" + p.Channel + ":" + p.PostID
		set(key, stamp(p))
		set(key+":account", p.Account)
		set(key+":title", p.GalleryTitle)
	}
	for ch, p := range latest {
		key := "built:" + ch
		set(key, stamp(p))
		set(key+":account", p.Account)
		set(key+":postid", p.PostID)
		set(key+":title", p.GalleryTitle)
	}
	return keys
}

func stamp(p media.Publication) string { return p.PublishedAt.UTC().Format(time.RFC3339) }

// replaceBuiltMeta makes the photo's built: keys exactly want: a gallery the
// photo was taken out of — on this installation or the other, whose
// removal reached the sidecar — is no longer claimed by the index. Keys
// that already hold their value are left as they are.
func (s *Store) replaceBuiltMeta(photoID string, want map[string]string) error {
	have, err := s.photoBuiltMeta(photoID)
	if err != nil || (len(have) == 0 && len(want) == 0) {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	now := time.Now().UTC().Format(time.RFC3339)
	for k, v := range want {
		if old, ok := have[k]; ok && old == v {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO photo_meta(photo_id,key,value,updated_at) VALUES(?,?,?,?)
			ON CONFLICT(photo_id,key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
			photoID, k, v, now); err != nil {
			return err
		}
	}
	for k := range have {
		if _, ok := want[k]; ok {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM photo_meta WHERE photo_id = ? AND key = ?`, photoID, k); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) photoBuiltMeta(photoID string) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM photo_meta WHERE photo_id = ? AND key LIKE 'built:%'`, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	have := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if strings.HasPrefix(k, "built:") {
			have[k] = v
		}
	}
	return have, rows.Err()
}
