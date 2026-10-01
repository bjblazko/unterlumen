package library

import (
	"database/sql"
	"path/filepath"
	"time"

	"huepattl.de/unterlumen/internal/appearance"
)

// AppearanceTask is a photo whose appearance is still to be measured, and
// the thumbnail to measure it from.
type AppearanceTask struct {
	ID        string
	ThumbPath string // relative to the library's directory
}

// PhotosNeedingAppearance lists the photos with a thumbnail whose appearance
// is missing or was measured by an older appearance.Version.
func (s *Store) PhotosNeedingAppearance() ([]AppearanceTask, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.thumb_path FROM photos p
		LEFT JOIN photo_appearance a ON a.photo_id = p.id
		WHERE p.status = 'ok' AND p.thumb_path IS NOT NULL AND p.thumb_path != ''
		  AND (a.photo_id IS NULL OR a.version < ?)`, appearance.Version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []AppearanceTask
	for rows.Next() {
		var t AppearanceTask
		if err := rows.Scan(&t.ID, &t.ThumbPath); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// MeasuredAppearance is one photo's measurements, ready to store.
type MeasuredAppearance struct {
	ID     string
	Result appearance.Result
}

// SaveAppearances stores the measurements of several photos in one
// transaction, replacing what was stored for them before. A photo deleted
// since it was measured is passed over: the pass runs beside scans.
func (s *Store) SaveAppearances(items []MeasuredAppearance) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	now := time.Now().UTC()
	for _, it := range items {
		if err := saveAppearance(tx, it.ID, it.Result, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func saveAppearance(tx *sql.Tx, id string, r appearance.Result, now time.Time) error {
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM photos WHERE id = ?`, id).Scan(&exists); err != nil || exists == 0 {
		return err
	}
	var tint any
	if r.Mono == appearance.Tinted {
		tint = r.TintHue
	}
	t, x := r.Tone, r.Texture
	if _, err := tx.Exec(`INSERT OR REPLACE INTO photo_appearance (
		photo_id, version, analysed_at, mono_class, tint_hue,
		avg_l, avg_c, avg_h, colourfulness, warmth,
		lum_mean, lum_median, lum_p05, lum_p95, contrast, tone_key, clip_high, clip_low, lum_hist,
		entropy, sharpness, edge_density, centroid_x, centroid_y, dhash
	) VALUES (?,?,?,?,?, ?,?,?,?,?, ?,?,?,?,?,?,?,?,?, ?,?,?,?,?,?)`,
		id, appearance.Version, now, string(r.Mono), tint,
		r.Average.L, r.Average.Chroma(), r.Average.Hue(), r.Colourfulness, r.Warmth,
		t.Mean, t.Median, t.P05, t.P95, t.Contrast, string(t.Key), t.ClipHigh, t.ClipLow, t.Histogram[:],
		x.Entropy, x.Sharpness, x.EdgeDensity, x.CentroidX, x.CentroidY, int64(r.Hash),
	); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM photo_palette WHERE photo_id = ?`, id); err != nil {
		return err
	}
	for rank, sw := range r.Palette {
		var bin any
		if b := sw.HueBin(); b >= 0 {
			bin = b
		}
		if _, err := tx.Exec(`INSERT INTO photo_palette (photo_id, rank, l, c, h, hue_bin, share) VALUES (?,?,?,?,?,?,?)`,
			id, rank, sw.Colour.L, sw.Colour.Chroma(), sw.Colour.Hue(), bin, sw.Share); err != nil {
			return err
		}
	}
	return nil
}

// ClearAppearance forgets the appearance of the given photos, so the next
// pass measures them again.
func (s *Store) ClearAppearance(ids ...string) error {
	for _, id := range ids {
		for _, q := range []string{
			`DELETE FROM photo_palette    WHERE photo_id = ?`,
			`DELETE FROM photo_appearance WHERE photo_id = ?`,
		} {
			if _, err := s.db.Exec(q, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// ClearAppearanceInFolder forgets the appearance of every photo inside the
// absolute folder, or of the whole library when folder is empty.
func (s *Store) ClearAppearanceInFolder(folder string) error {
	where, args := `1=1`, []any{}
	if folder != "" {
		where = `path_hint LIKE ? ESCAPE '\'`
		args = append(args, escapeLikePattern(folder)+string(filepath.Separator)+"%")
	}
	for _, table := range []string{"photo_palette", "photo_appearance"} {
		if _, err := s.db.Exec(`DELETE FROM `+table+` WHERE photo_id IN (SELECT id FROM photos WHERE `+where+`)`, args...); err != nil {
			return err
		}
	}
	return nil
}
