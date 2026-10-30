package library

import (
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// A library holds photos and videos (ADR-0058). Videos are listed, shown and
// filtered, but nothing looks for faces or things in them.

// videoExtSet is the SQL list of the videos' extensions, "('avi','mov',…)".
// The values are the program's own, never a request's.
var videoExtSet = "('" + strings.Join(media.VideoExts(), "','") + "')"

// notVideo is a condition on photos aliased p that leaves out the videos.
var notVideo = "p.ext NOT IN " + videoExtSet

// Kinds a filter can narrow to.
const (
	KindPhoto = "photo"
	KindVideo = "video"
)

// CountVideos counts the library's videos, through photos_ext_idx.
func (s *Store) CountVideos() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM photos WHERE status = 'ok' AND ext IN ` + videoExtSet).Scan(&n)
	return n, err
}

// kindCond restricts the photo IDs in col to one kind, "" for any other
// value. A set collected once through photos_ext_idx, never per row; gone
// photos keep their kind, so Statistics can count them.
func kindCond(col, kind string) string {
	switch kind {
	case KindPhoto:
		return " AND " + col + " IN (SELECT id FROM photos WHERE status IN ('ok', 'missing') AND ext NOT IN " + videoExtSet + ")"
	case KindVideo:
		return " AND " + col + " IN (SELECT id FROM photos WHERE status IN ('ok', 'missing') AND ext IN " + videoExtSet + ")"
	}
	return ""
}
