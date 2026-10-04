package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
)

var siteGalleryTmpl = template.Must(template.New("sitegallery").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover">
<title>{{.PageTitle}}</title>
<meta name="description" content="{{.Description}}">
{{- if .Unlisted}}
<meta name="robots" content="noindex, nofollow">
{{- end}}
{{- if .SiteURL}}
<link rel="canonical" href="{{.AlbumURL}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
{{- if .CoverURL}}
<meta property="og:image" content="{{.CoverURL}}">{{end}}
<meta property="og:type" content="website">
{{- end}}
<script type="application/ld+json">{{.LDJSON}}</script>
<link rel="stylesheet" href="../../assets/style.css">
<script src="../../assets/toggle.js"></script>
</head>
<body>
<header>
  <div class="site-masthead">
    <div class="site-brand">
      {{- if .Nav.LogoExists}}
      <img class="site-logo" src="{{.Nav.LogoPath}}" alt="" loading="eager">
      {{- end}}
      <a class="site-name" href="../../index.html">{{.Nav.SiteName}}</a>
    </div>
    <div class="header-actions">
      <button class="menu-btn" id="menu-btn" aria-label="Menu" aria-expanded="false">&#x22EF;</button>
      <div class="header-actions-inner">
        {{if .ZipFilename}}<a class="dl-btn" href="{{.ZipFilename}}" download><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg> Download all photos</a>{{end}}
        <button id="theme-toggle" class="theme-btn">Dark</button>
      </div>
    </div>
  </div>
  <div class="page-title">
    <a class="site-back" href="../../index.html" title="Back to albums"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="vertical-align:-4px"><polyline points="15 18 9 12 15 6"/></svg></a>{{.Title}}
  </div>
</header>

<main class="gallery" id="gallery">
{{range .Figures}}<figure data-index="{{.Index}}">
  <img src="{{.Thumb}}" loading="{{.Loading}}" alt="{{.Alt}}">
</figure>
{{end}}</main>

<div id="lb">
  <button id="lb-close" title="Close (Esc)">&times;</button>
  <button class="lb-nav" id="lb-prev" title="Previous (←)"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="15 18 9 12 15 6"/></svg></button>
  <img id="lb-img" src="" alt="">
  <button class="lb-nav" id="lb-next" title="Next (→)"><svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><polyline points="9 18 15 12 9 6"/></svg></button>
  <div id="lb-counter"></div>
</div>

<footer>
  <span>Built with <svg width="13" height="13" viewBox="0 0 24 24" fill="var(--accent)" aria-hidden="true" style="vertical-align:-1px"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg> <a href="https://huepattl.de/products/unterlumen" target="_blank" rel="noopener">Unterlumen</a></span>
  <a href="https://github.com/bjblazko/unterlumen" target="_blank" rel="noopener" title="View on GitHub"><svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg></a>
  {{- if or .Nav.HasAbout .Nav.HasImprint .Nav.ContactEmail .Nav.ContactURL}}
  <div class="footer-contact">
    {{- if .Nav.HasAbout}}<a href="../../about.html">About</a>{{end}}
    {{- if .Nav.HasImprint}}<a href="../../legal.html">Legal</a>{{end}}
    {{- if .Nav.ContactEmail}}<a href="mailto:{{.Nav.ContactEmail}}">{{.Nav.ContactEmail}}</a>{{end}}
    {{- if .Nav.ContactURL}}<a href="{{.Nav.ContactURL}}" target="_blank" rel="noopener">{{.Nav.ContactURL}}</a>{{end}}
  </div>
  {{- end}}
</footer>

<script type="application/json" id="ul-photos">{{.PhotosJSON}}</script>
<script src="../../assets/lightbox.js"></script>
</body>
</html>
`))

type siteGalleryPhoto struct {
	Full  string `json:"full"`
	Thumb string `json:"thumb"`
}

// GenerateAlbum produces an album index.html for site mode.
// It references shared assets via ../../assets/ instead of embedding CSS.
func GenerateAlbum(title, defaultTheme string, items []GalleryItem, opts GalleryOptions) []byte {
	if defaultTheme == "" {
		defaultTheme = "light"
	}
	total := len(items)
	photos := make([]siteGalleryPhoto, 0, total)
	figures := make([]galleryFigureData, 0, total)
	for i, item := range items {
		photos = append(photos, siteGalleryPhoto{Full: item.Filename, Thumb: item.ThumbFilename})
		loading := "lazy"
		if i < 2 {
			loading = "eager"
		}
		alt := fmt.Sprintf("%s – Photo %d of %d", title, i+1, total)
		figures = append(figures, galleryFigureData{
			Index:   i,
			Thumb:   item.ThumbFilename,
			Full:    item.Filename,
			Loading: loading,
			Alt:     alt,
		})
	}
	photosJSON, _ := json.Marshal(photos)

	dateStr := opts.DateStr
	description := fmt.Sprintf("A collection of %d photo", total)
	if total != 1 {
		description += "s"
	}
	if dateStr != "" {
		description += ", " + dateStr
	}
	description += "."

	pageTitle := title
	if opts.SiteTitle != "" {
		pageTitle = title + " | " + opts.SiteTitle
	}

	ldMap := map[string]any{
		"@context":      "https://schema.org",
		"@type":         "ImageGallery",
		"name":          title,
		"description":   description,
		"numberOfItems": total,
	}
	if !opts.PublishedAt.IsZero() {
		ldMap["datePublished"] = opts.PublishedAt.UTC().Format("2006-01-02")
	}
	albumURL := ""
	coverURL := ""
	if opts.SiteURL != "" && opts.AlbumSlug != "" {
		base := strings.TrimRight(opts.SiteURL, "/")
		albumURL = base + "/albums/" + opts.AlbumSlug + "/"
		coverURL = albumURL + "cover.jpg"
		ldMap["url"] = albumURL
		ldMap["image"] = coverURL
	}
	ldJSON, _ := json.Marshal(ldMap)

	var buf bytes.Buffer
	siteGalleryTmpl.Execute(&buf, struct { //nolint:errcheck
		Title        string
		PageTitle    string
		DefaultTheme string
		Description  string
		PhotosJSON   template.JS
		LDJSON       template.JS
		ZipFilename  string
		Figures      []galleryFigureData
		SiteURL      string
		AlbumURL     string
		CoverURL     string
		Unlisted     bool
		Nav          NavContext
	}{
		Title:        title,
		PageTitle:    pageTitle,
		DefaultTheme: defaultTheme,
		Description:  description,
		PhotosJSON:   template.JS(photosJSON),
		LDJSON:       template.JS(ldJSON),
		ZipFilename:  opts.ZipFilename,
		Figures:      figures,
		SiteURL:      opts.SiteURL,
		AlbumURL:     albumURL,
		CoverURL:     coverURL,
		Unlisted:     opts.Unlisted,
		Nav:          opts.Nav,
	})
	return buf.Bytes()
}

/* --- About page --- */
