package publish

import (
	"crypto/rand"
	"fmt"
	"strings"

	lib "huepattl.de/unterlumen/internal/library"
)

func newPostID() string {
	b := make([]byte, 12)
	rand.Read(b) //nolint:errcheck
	return fmt.Sprintf("%x", b)
}

// Publication meta keys are "built:<slug>" (channel marker), "built:<slug>:<postID>"
// (one album), and either of those plus a reserved suffix. The legacy
// "published:" prefix predates the rename and is still cleaned up alongside.
const (
	BuildPrefix     = "built:"
	legacyPubPrefix = "published:"
)

// reservedMetaSuffix reports whether a key segment is a field name rather than
// an album ID — "built:ch:title" is the channel's latest album title,
// "built:ch:9f2a…" is membership in album 9f2a….
func reservedMetaSuffix(s string) bool {
	return s == "title" || s == "account" || s == "postid"
}

// albumPostIDsForChannel lists every album a photo belongs to in one channel.
// The unqualified ":postid" key only ever names the most recently published
// album, so it can't stand in for this.
func albumPostIDsForChannel(entries []lib.MetaEntry, slug string) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range entries {
		for _, prefix := range []string{BuildPrefix, legacyPubPrefix} {
			rest, ok := strings.CutPrefix(e.Key, prefix+slug+":")
			if !ok || strings.Contains(rest, ":") || reservedMetaSuffix(rest) || seen[rest] {
				continue
			}
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out
}

// deleteAlbumKeys removes one album's membership keys under both prefixes.
func deleteAlbumKeys(s *lib.Store, photoID, slug, albumPostID string) {
	for _, prefix := range []string{BuildPrefix, legacyPubPrefix} {
		for _, suffix := range []string{"", ":account", ":title"} {
			s.DeleteMeta(photoID, prefix+slug+":"+albumPostID+suffix) //nolint:errcheck
		}
	}
}

// deleteChannelPublicationKeys removes a photo's channel marker and every
// album key it holds for that channel, under both prefixes.
func deleteChannelPublicationKeys(s *lib.Store, photoID, slug string) {
	if entries, err := s.GetMeta(photoID); err == nil {
		for _, albumPostID := range albumPostIDsForChannel(entries, slug) {
			deleteAlbumKeys(s, photoID, slug, albumPostID)
		}
	}
	for _, prefix := range []string{BuildPrefix, legacyPubPrefix} {
		for _, suffix := range []string{"", ":account", ":title", ":postid"} {
			s.DeleteMeta(photoID, prefix+slug+suffix) //nolint:errcheck
		}
	}
}

// clearPendingMarkers drops one draft's pending marker for a photo. The
// unqualified per-channel marker goes only once no other draft of that channel
// still holds the photo — a photo can be collected into several albums of one
// channel, and publishing one of them must not clear the others' pending state.
func clearPendingMarkers(s *lib.Store, photoID, slug, draftID string) {
	s.DeleteMeta(photoID, "pending:"+slug+":"+draftID) //nolint:errcheck
	entries, err := s.GetMeta(photoID)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "pending:"+slug+":") {
			return
		}
	}
	s.DeleteMeta(photoID, "pending:"+slug) //nolint:errcheck
}
