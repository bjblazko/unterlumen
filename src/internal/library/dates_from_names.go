package library

import (
	"log"

	"huepattl.de/unterlumen/internal/media"
)

// A photo without a date in its EXIF takes the date its name gives (see
// media.DateFromFileName). A scan does that for every photo it reads; the
// photos indexed before it did are given theirs once per library, from the
// names the index has — no file is read.

const datesFromNamesProp = "dates_from_names"

// DateAllFromNames dates every library's undated photos from their names,
// once per library. A library a scan holds is tried on the next start.
func (m *Manager) DateAllFromNames() {
	libs, err := m.ListLibraries()
	if err != nil {
		return
	}
	for _, l := range libs {
		m.dateFromNames(l)
	}
}

func (m *Manager) dateFromNames(l *Library) {
	if !m.TryLockIndex(l.ID) {
		return
	}
	defer m.UnlockIndex(l.ID)
	store, err := m.OpenStore(l.ID)
	if err != nil {
		return
	}
	defer store.Close()
	if done, _, _ := store.GetProp(datesFromNamesProp); done == "1" {
		return
	}
	n, err := store.DateUndatedFromNames()
	if err != nil {
		log.Printf("Library %q: dating photos from their names: %v", l.Name, err)
		return
	}
	if n > 0 {
		log.Printf("Library %q: %d photos dated from their names", l.Name, n)
	}
	store.SetProp(datesFromNamesProp, "1") //nolint:errcheck // tried again on the next start
}

// DateUndatedFromNames gives each photo without a date taken the date its
// name gives, and returns how many it dated.
func (s *Store) DateUndatedFromNames() (int, error) {
	rows, err := s.db.Query(`SELECT id, filename FROM photos WHERE date_taken IS NULL OR date_taken = ''`)
	if err != nil {
		return 0, err
	}
	dates := map[string]string{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return 0, err
		}
		if d := media.DateFromFileName(name); d != "" {
			dates[id] = d
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(dates) == 0 {
		return 0, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for id, d := range dates {
		if _, err := tx.Exec(`UPDATE photos SET date_taken = ?,
			exif_json = CASE WHEN json_valid(exif_json) THEN json_set(exif_json, '$.dateTaken', ?) ELSE exif_json END
			WHERE id = ?`, d, d, id); err != nil {
			return 0, err
		}
		for field, value := range map[string]string{"DateTaken": d, "DateTakenFrom": "file name"} {
			if _, err := tx.Exec(`INSERT OR REPLACE INTO exif_index(photo_id, field, value) VALUES (?, ?, ?)`, id, field, value); err != nil {
				return 0, err
			}
		}
	}
	return len(dates), tx.Commit()
}
