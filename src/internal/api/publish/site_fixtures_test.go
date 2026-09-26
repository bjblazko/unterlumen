package publish

import (
	"time"

	"huepattl.de/unterlumen/internal/site"
)

func testAlbum(postID, title string) site.Album {
	publishedAt := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	return site.Album{
		PostID:      postID,
		Slug:        site.ComputeSlug(title, publishedAt, nil, false),
		Title:       title,
		PublishedAt: publishedAt,
		PhotoCount:  1,
		CoverFile:   "cover.jpg",
		Photos:      []site.Photo{{PhotoID: "abc", Filename: "a.jpg", ThumbFilename: "thumbs/a.jpg"}},
	}
}
