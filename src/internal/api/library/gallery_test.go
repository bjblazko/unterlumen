package apilibrary

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateGallery(t *testing.T) {
	items := []GalleryItem{
		{Filename: "photo1.jpg", ThumbFilename: "thumbs/photo1.jpg", Width: 1200, Height: 800},
		{Filename: "photo2.jpg", ThumbFilename: "thumbs/photo2.jpg", Width: 900, Height: 600},
	}
	html := string(GenerateGallery("Summer 2026", items, GalleryOptions{}))

	for _, want := range []string{
		"<title>Summer 2026</title>",
		"<h1>Summer 2026</h1>",
		`"full":"photo1.jpg"`,
		`"thumb":"thumbs/photo1.jpg"`,
		`img.loading = 'lazy'`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestGenerateGalleryZipLink(t *testing.T) {
	html := string(GenerateGallery("Test", nil, GalleryOptions{ZipFilename: "photos.zip"}))
	if !strings.Contains(html, `href="photos.zip"`) {
		t.Error("ZIP download link missing")
	}
	if !strings.Contains(html, "download") {
		t.Error("download attribute missing on ZIP link")
	}

	htmlNoZip := string(GenerateGallery("Test", nil, GalleryOptions{}))
	if strings.Contains(htmlNoZip, "photos.zip") {
		t.Error("ZIP link should not appear when ZipFilename is empty")
	}
}

func TestGenerateGalleryEscapesTitle(t *testing.T) {
	html := string(GenerateGallery("<script>alert(1)</script>", nil, GalleryOptions{}))
	// The title must appear escaped; the literal unescaped injection must not.
	if strings.Contains(html, "<script>alert(1)") {
		t.Error("title was not HTML-escaped")
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("escaped title not found in output")
	}
}

func TestGenerateGalleryNoDimensions(t *testing.T) {
	items := []GalleryItem{{Filename: "img.jpg"}}
	html := string(GenerateGallery("Test", items, GalleryOptions{}))
	if strings.Contains(html, `width="0"`) {
		t.Error("zero dimensions should not appear in output")
	}
}

// TestGenerateGalleryNoInlineScript verifies the page references its
// theme/lightbox script externally rather than inlining it — a strict CSP
// (script-src 'self', no 'unsafe-inline') silently blocks inline <script>
// content, which is exactly what broke the deployed site.
func TestGenerateGalleryNoInlineScript(t *testing.T) {
	html := string(GenerateGallery("Test", nil, GalleryOptions{}))
	if strings.Contains(html, "function openLightbox") {
		t.Error("index.html still inlines lightbox JS instead of referencing gallery.js")
	}
	if strings.Contains(html, "localStorage.getItem('ul-theme')") {
		t.Error("index.html still inlines the theme-init snippet instead of referencing theme-init.js")
	}
	if !strings.Contains(html, `<script src="theme-init.js"></script>`) {
		t.Error("index.html missing external theme-init.js script reference")
	}
	if !strings.Contains(html, `<script src="gallery.js"></script>`) {
		t.Error("index.html missing external gallery.js script reference")
	}
	if !strings.Contains(html, `<script type="application/json" id="ul-photos">`) {
		t.Error("index.html missing the ul-photos application/json data element gallery.js reads from")
	}
}

func TestWriteGalleryAssets(t *testing.T) {
	dir := t.TempDir()
	if err := writeGalleryAssets(dir); err != nil {
		t.Fatalf("writeGalleryAssets: %v", err)
	}
	themeInit, err := os.ReadFile(filepath.Join(dir, "theme-init.js"))
	if err != nil {
		t.Fatalf("read theme-init.js: %v", err)
	}
	if !strings.Contains(string(themeInit), "localStorage.getItem('ul-theme')") {
		t.Error("theme-init.js missing expected theme-init logic")
	}
	galleryJS, err := os.ReadFile(filepath.Join(dir, "gallery.js"))
	if err != nil {
		t.Fatalf("read gallery.js: %v", err)
	}
	if !strings.Contains(string(galleryJS), "function openLightbox") {
		t.Error("gallery.js missing expected lightbox logic")
	}
	if strings.Contains(string(galleryJS), "{{") {
		t.Error("gallery.js must be fully static — no leftover template placeholders")
	}
}
