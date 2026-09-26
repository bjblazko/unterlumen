package publish

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"huepattl.de/unterlumen/internal/channels"
)

func galleryErrorStore(t *testing.T) *channels.Store {
	t.Helper()
	chStore := channels.NewStore(t.TempDir(), t.TempDir())
	for _, ch := range []*channels.Channel{
		{Slug: "gal", Name: "Gallery", GalleryExport: true},
		{Slug: "web", Name: "Website", SiteExport: true},
		{Slug: "plain", Name: "Plain"},
	} {
		if err := chStore.Save(ch); err != nil {
			t.Fatal(err)
		}
	}
	return chStore
}

func TestGalleryHandlersAnswerErrors(t *testing.T) {
	chStore := galleryErrorStore(t)
	cases := []struct {
		name, method, slug, postID, body string
		status                           int
		text                             string
	}{
		{"rename: unknown destination", "PATCH", "nope", "p1", `{"title":"X"}`, 404, "channel not found"},
		{"rename: not JSON", "PATCH", "gal", "p1", `{`, 400, "invalid JSON"},
		{"rename: site album cannot change unlisted", "PATCH", "web", "p1", `{"title":"X","unlisted":true}`, 400, "unlisted cannot be changed"},
		{"rename: unknown site album", "PATCH", "web", "p1", `{"title":"X"}`, 404, "gallery not found"},
		{"rename: gallery id outside the output", "PATCH", "gal", "..", `{"title":"X"}`, 400, "invalid gallery id"},
		{"rename: destination without galleries", "PATCH", "plain", "p1", `{"title":"X"}`, 400, "not configured for gallery or site export"},
		{"delete: unknown destination", "DELETE", "nope", "p1", ``, 404, "channel not found"},
		{"delete: unknown site album", "DELETE", "web", "p1", ``, 404, "gallery not found"},
		{"delete: gallery id outside the output", "DELETE", "gal", "..", ``, 400, "invalid gallery id"},
		{"delete: destination without galleries", "DELETE", "plain", "p1", ``, 400, "not configured for gallery or site export"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, "/api/channels/"+c.slug+"/galleries/x", strings.NewReader(c.body))
			req.SetPathValue("slug", c.slug)
			req.SetPathValue("postID", c.postID)
			rec := httptest.NewRecorder()
			var h http.HandlerFunc = renameGallery(chStore, nil)
			if c.method == "DELETE" {
				h = deleteGallery(chStore, nil)
			}
			h(rec, req)
			if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.text) {
				t.Errorf("answer = %d %q, want %d containing %q", rec.Code, strings.TrimSpace(rec.Body.String()), c.status, c.text)
			}
		})
	}
}
