package library

import (
	"testing"
	"time"
)

func TestPublishedPostsFromPublicationKeys(t *testing.T) {
	mgr := newTestManager(t)
	a, _ := mgr.CreateLibrary("A", "", t.TempDir())
	b, _ := mgr.CreateLibrary("B", "", t.TempDir())
	sa, _ := mgr.OpenStore(a.ID)
	sb, _ := mgr.OpenStore(b.ID)
	const ig = "built:instagram:"
	for _, s := range []*Store{sa, sb} {
		for _, id := range []string{"p1", "p2", "p3"} {
			s.UpsertPhoto(id, "/x/"+id+".jpg", id+".jpg", 0, time.Now(), "", "", "", "jpeg") //nolint:errcheck
		}
	}
	for _, s := range []*Store{sa, sb} { // the same photo in two libraries counts once
		s.UpsertMeta("p1", ig+"stream", "2026-09-27T12:00:00Z")      //nolint:errcheck
		s.UpsertMeta("p1", ig+"stream:title", "Social Media Stream") //nolint:errcheck
	}
	sa.UpsertMeta("p2", ig+"stream", "2026-09-30T12:00:00Z") //nolint:errcheck — added later
	sa.UpsertMeta("p3", ig+"single", "2026-06-26T12:00:00Z") //nolint:errcheck
	sa.UpsertMeta("p3", ig+"postid", "single")               //nolint:errcheck — the unqualified marker
	sa.UpsertMeta("p3", "built:website:album", "2025")       //nolint:errcheck — another destination

	posts, err := mgr.PublishedPosts("instagram")
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 2 {
		t.Fatalf("posts = %+v, want stream and single", posts)
	}
	stream, single := posts[0], posts[1]
	if stream.PostID != "stream" || stream.Title != "Social Media Stream" || stream.PhotoCount != 2 {
		t.Errorf("stream = %+v", stream)
	}
	if !stream.PublishedAt.Equal(time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)) || !stream.UpdatedAt.Equal(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("stream dates = %v / %v", stream.PublishedAt, stream.UpdatedAt)
	}
	if single.PostID != "single" || single.Title != "" || single.PhotoCount != 1 {
		t.Errorf("single = %+v", single)
	}
}
