package site

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateRobotsTxt writes a robots.txt to the site root.
// If siteURL is non-empty, a Sitemap line is included.
func GenerateRobotsTxt(siteDir, siteURL string) error {
	var b strings.Builder
	b.WriteString("User-agent: *\nAllow: /\n")
	if siteURL != "" {
		fmt.Fprintf(&b, "Sitemap: %s/sitemap.xml\n", strings.TrimRight(siteURL, "/"))
	}
	return os.WriteFile(filepath.Join(siteDir, "robots.txt"), []byte(b.String()), 0o644)
}

// GenerateSitemap writes a sitemap.xml to the site root.
// Only called when siteURL is non-empty; sitemap requires absolute URLs.
func GenerateSitemap(siteDir string, albums []Album, siteURL string) error {
	base := strings.TrimRight(siteURL, "/")
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")
	fmt.Fprintf(&b, "  <url><loc>%s/</loc></url>\n", base)
	for _, a := range albums {
		if a.Unlisted {
			continue
		}
		fmt.Fprintf(&b, "  <url><loc>%s/albums/%s/</loc></url>\n", base, AlbumFolderName(a))
	}
	b.WriteString("</urlset>\n")
	return os.WriteFile(filepath.Join(siteDir, "sitemap.xml"), []byte(b.String()), 0o644)
}
