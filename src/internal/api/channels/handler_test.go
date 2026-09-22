package apichannels

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/channels"
)

// The settings form has no deploy fields, so a full-replace save used to blank
// out the deploy status the Published tab and channel list show.
func TestUpdateChannelPreservesDeployStatus(t *testing.T) {
	dir := t.TempDir()
	store := channels.NewStore(dir, dir)

	deployedAt := time.Date(2026, 9, 20, 16, 41, 19, 0, time.UTC)
	if err := store.Save(&channels.Channel{
		Slug: "fotoshare", Name: "Fotoshare", Format: "jpeg", Quality: 90,
		GalleryExport:  true,
		LastDeployedAt: deployedAt,
		LastDeployOK:   true,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	body := `{"name":"Fotoshare","format":"jpeg","quality":90,"galleryExport":true}`
	req := httptest.NewRequest("PUT", "/api/channels/fotoshare", strings.NewReader(body))
	req.SetPathValue("slug", "fotoshare")
	rec := httptest.NewRecorder()
	updateChannel(store)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	got, err := store.Get("fotoshare")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.LastDeployedAt.Equal(deployedAt) {
		t.Errorf("LastDeployedAt = %v, want %v", got.LastDeployedAt, deployedAt)
	}
	if !got.LastDeployOK {
		t.Error("LastDeployOK was reset by a settings save")
	}
}

// Base URL is shared with site channels but must survive a save in gallery
// mode too — it is what turns "No URL configured" into a real share link.
func TestUpdateChannelKeepsSiteURLForGalleryChannel(t *testing.T) {
	dir := t.TempDir()
	store := channels.NewStore(dir, dir)
	if err := store.Save(&channels.Channel{Slug: "fotoshare", Name: "Fotoshare", GalleryExport: true}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	body := `{"name":"Fotoshare","galleryExport":true,"siteURL":"https://fotos.example.com"}`
	req := httptest.NewRequest("PUT", "/api/channels/fotoshare", strings.NewReader(body))
	req.SetPathValue("slug", "fotoshare")
	rec := httptest.NewRecorder()
	updateChannel(store)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	got, err := store.Get("fotoshare")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SiteURL != "https://fotos.example.com" {
		t.Errorf("SiteURL = %q, want it stored for a gallery channel", got.SiteURL)
	}
}
