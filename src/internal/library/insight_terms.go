package library

import (
	"strconv"
	"strings"
)

// What a Statistics view calls a phone, a photo at risk of shake, a portrait,
// a cropped photo, a flash photo and its scene's light — each defined here
// once, as SQL, for the count and for the search parameter its click sets, so
// a click shows exactly the photos counted. Each condition is a set of photo
// IDs, never a subquery per photo (ADR-0045).

// phoneMakes are the makers whose cameras are phones (lower case, as Make
// says it); Sony and LG make both and are left out.
var phoneMakes = []string{"apple", "samsung", "google", "huawei", "xiaomi", "oneplus", "oppo", "motorola", "htc",
	"nokia", "realme", "vivo", "honor", "fairphone", "nothing", "zte", "meizu", "asus"}

func phoneMakeList() string {
	q := make([]string, len(phoneMakes))
	for i, m := range phoneMakes {
		q[i] = "'" + m + "'"
	}
	return strings.Join(q, ",")
}

// phoneSet is the photos taken with a phone; cameraSet those with a Make
// that is not one.
func phoneSet() string {
	return `SELECT photo_id FROM exif_index WHERE field='Make' AND LOWER(` + textOf("value") + `) IN (` + phoneMakeList() + `)`
}

func cameraSet() string {
	return `SELECT photo_id FROM exif_index WHERE field='Make' AND LOWER(` + textOf("value") + `) NOT IN (` + phoneMakeList() + `)`
}

// focal35 is the 35 mm equivalent focal length, or the focal length itself
// where the camera does not say (joined as f35 and f).
const focal35 = "COALESCE(f35.numeric_value, f.numeric_value)"

const focalJoins = ` LEFT JOIN exif_index f35 ON f35.photo_id = t.photo_id AND f35.field = 'FocalLengthIn35mmFilm'
	LEFT JOIN exif_index f ON f.photo_id = t.photo_id AND f.field = 'FocalLength'`

// slowSQL holds for an exposure (t) longer than one over the 35 mm focal
// length: the old rule for a sharp photo held in the hand.
const slowSQL = "t.numeric_value > 1.0 / " + focal35

// slowSet is the photos at risk of shake.
const slowSet = `SELECT t.photo_id FROM exif_index t` + focalJoins + `
	WHERE t.field = 'ExposureTime' AND ` + focal35 + ` > 0 AND ` + slowSQL

// evSQL is a photo's exposure value at ISO 100 — how bright its scene was —
// from aperture (n), time (t) and ISO (i): log2(N²/t) − log2(ISO/100).
const evSQL = "(log2(n.numeric_value * n.numeric_value / t.numeric_value) - log2(i.numeric_value / 100.0))"

const evFrom = ` FROM exif_index t
	JOIN exif_index n ON n.photo_id = t.photo_id AND n.field = 'FNumber' AND n.numeric_value > 0
	JOIN exif_index i ON i.photo_id = t.photo_id AND i.field = 'ISOSpeedRatings' AND i.numeric_value > 0
	WHERE t.field = 'ExposureTime' AND t.numeric_value > 0`

// evSet is the photos whose EV100, rounded, lies in [lo, hi].
func evSet(lo, hi int) (string, []any) {
	return `SELECT t.photo_id` + evFrom + ` AND ROUND(` + evSQL + `) BETWEEN ? AND ?`, []any{lo, hi}
}

// flashSet is the photos for which the flash fired (bit 0 of Flash).
const flashSet = `SELECT photo_id FROM exif_index WHERE field = 'Flash' AND (CAST(TRIM(value, '"') AS INTEGER) & 1) = 1`

// biasSQL is the exposure compensation of v ("-1/3") in stops.
func biasSQL(v string) string {
	t := "TRIM(" + v + ", '\"')"
	return "(CAST(SUBSTR(" + t + ", 1, INSTR(" + t + ", '/') - 1) AS REAL) / NULLIF(CAST(SUBSTR(" + t + ", INSTR(" + t + ", '/') + 1) AS REAL), 0))"
}

// Frame: width and height from the photo's JSON, turned by Orientation 5–8.
const (
	widthSQL   = "CAST(json_extract(p.exif_json, '$.width') AS REAL)"
	heightSQL  = "CAST(json_extract(p.exif_json, '$.height') AS REAL)"
	turnedSQL  = "COALESCE(CAST(TRIM(o.value, '\"') AS INTEGER) BETWEEN 5 AND 8, 0)"
	sizedSQL   = widthSQL + " > 0 AND " + heightSQL + " > 0"
	orientJoin = " LEFT JOIN exif_index o ON o.photo_id = p.id AND o.field = 'Orientation'"
)

// orientSQL is "portrait", "landscape" or "square" as the photo is seen.
const orientSQL = `CASE WHEN ABS(` + widthSQL + ` - ` + heightSQL + `) <= 0.02 * ` + widthSQL + ` THEN 'square'
	WHEN (` + widthSQL + ` < ` + heightSQL + `) <> ` + turnedSQL + ` THEN 'portrait' ELSE 'landscape' END`

// orientSet is the photos of one orientation.
const orientSet = `SELECT p.id FROM photos p` + orientJoin + ` WHERE p.status = 'ok' AND ` + sizedSQL + ` AND ` + orientSQL + ` = ?`

// ratioSQL is the long side over the short, at most 3, in steps of 0.05.
const ratioSQL = `MIN(3.0, ROUND(MAX(` + widthSQL + `, ` + heightSQL + `) / MIN(` + widthSQL + `, ` + heightSQL + `) * 20) / 20.0)`

// ratioSet is the photos whose ratio lies in [lo, hi].
const ratioSet = `SELECT p.id FROM photos p WHERE p.status = 'ok' AND ` + sizedSQL + ` AND ` + ratioSQL + ` BETWEEN ? AND ?`

// croppedFrom joins each photo to the largest frame of its camera in the
// library; a photo under nine tenths of it was cropped (or shot smaller).
const croppedFrom = ` FROM photos p
	JOIN exif_index m ON m.photo_id = p.id AND m.field = 'Model'
	JOIN (SELECT m2.value AS model, MAX(CAST(json_extract(p2.exif_json, '$.width') AS REAL) * CAST(json_extract(p2.exif_json, '$.height') AS REAL)) AS px
	      FROM photos p2 JOIN exif_index m2 ON m2.photo_id = p2.id AND m2.field = 'Model'
	      WHERE p2.status = 'ok' GROUP BY m2.value) mx ON mx.model = m.value
	WHERE p.status = 'ok' AND ` + sizedSQL

const croppedSQL = "(" + widthSQL + " * " + heightSQL + " < 0.9 * mx.px)"

const croppedSet = `SELECT p.id` + croppedFrom + ` AND ` + croppedSQL

// areaSet is the photos taken within a box of latitude and longitude, its
// lower bounds held, its upper ones not.
const areaSet = `SELECT p.id FROM photos p WHERE p.status = 'ok' AND json_valid(p.exif_json)
	AND CAST(json_extract(p.exif_json, '$.latitude') AS REAL) >= ? AND CAST(json_extract(p.exif_json, '$.latitude') AS REAL) < ?
	AND CAST(json_extract(p.exif_json, '$.longitude') AS REAL) >= ? AND CAST(json_extract(p.exif_json, '$.longitude') AS REAL) < ?`

// noLensSet is the photos with a camera but no lens tag: the "(no lens)" cell.
const noLensSet = `SELECT photo_id FROM exif_index WHERE field = 'Model'
	EXCEPT SELECT photo_id FROM exif_index WHERE field = 'LensModel'`

// addInsightConds adds the conditions a Statistics click sets (ListPhotosOpts).
func (f *photoFilter) addInsightConds(opts ListPhotosOpts) {
	switch opts.Device {
	case "phone":
		f.cond("p.id IN (" + phoneSet() + ")")
	case "camera":
		f.cond("p.id IN (" + cameraSet() + ")")
	}
	if opts.Slow {
		f.cond("p.id IN (" + slowSet + ")")
	}
	if opts.EVMin != nil && opts.EVMax != nil {
		q, args := evSet(*opts.EVMin, *opts.EVMax)
		f.cond("p.id IN ("+q+")", args...)
	}
	if opts.Flash {
		f.cond("p.id IN (" + flashSet + ")")
	}
	if opts.Orient != "" {
		f.cond("p.id IN ("+orientSet+")", opts.Orient)
	}
	if opts.RatioMin != nil && opts.RatioMax != nil {
		f.cond("p.id IN ("+ratioSet+")", *opts.RatioMin, *opts.RatioMax)
	}
	if opts.Cropped {
		f.cond("p.id IN (" + croppedSet + ")")
	}
	if opts.TakenFrom != "" && opts.TakenTo != "" {
		f.cond("(p.date_taken IS NOT NULL AND SUBSTR(p.date_taken, 1, 19) BETWEEN ? AND ?)", opts.TakenFrom, opts.TakenTo)
	}
	if a := opts.Area; a != nil {
		f.cond("p.id IN ("+areaSet+")", a[0], a[1], a[2], a[3])
	}
	if opts.NoLens {
		f.cond("p.id IN (" + noLensSet + ")")
	}
}

// itoa for keys built in Go.
func itoa(n int) string { return strconv.Itoa(n) }
