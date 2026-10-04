package site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
)

type siteAlbumData struct {
	FolderName string // slug if set, else postID — used in URLs
	Title      string
	DateStr    string
	PhotoCount int
	CoverFile  string
	Loading    string // "eager" for above-the-fold covers, "lazy" for the rest
}

var siteTmpl = template.Must(template.New("site").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.Title}}</title>
<meta name="description" content="{{.Description}}">
{{- if .SiteURL}}
<link rel="canonical" href="{{.SiteURL}}/">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Description}}">
<meta property="og:type" content="website">
{{- end}}
<script type="application/ld+json">{{.LDJSON}}</script>
<link rel="stylesheet" href="assets/style.css">
<script src="assets/toggle.js"></script>
</head>
<body>
<header>
  <div class="site-brand">
    {{- if .Nav.LogoExists}}
    <img class="site-logo" src="{{.Nav.LogoPath}}" alt="" loading="eager">
    {{- end}}
    <h1>{{.Title}}</h1>
  </div>
  <div class="header-actions">
    {{- if or .Nav.HasAbout .Nav.HasImprint}}
    <nav class="site-nav">
      {{- if .Nav.HasAbout}}<a href="about.html">About</a>{{end}}
      {{- if .Nav.HasImprint}}<a href="legal.html">Legal</a>{{end}}
    </nav>
    {{- end}}
    <button id="theme-toggle" class="theme-btn">Dark</button>
  </div>
</header>

<main class="albums">
{{range .Albums}}  <a class="album-card" href="albums/{{.FolderName}}/index.html">
    <img class="album-cover" src="albums/{{.FolderName}}/{{.CoverFile}}" alt="{{.Title}}" loading="{{.Loading}}">
    <div class="album-title">{{.Title}}</div>
    <div class="album-meta">{{.DateStr}} &middot; {{.PhotoCount}} photo{{if ne .PhotoCount 1}}s{{end}}</div>
  </a>
{{end}}</main>

<footer>
  <span>Built with <svg width="13" height="13" viewBox="0 0 24 24" fill="var(--accent)" aria-hidden="true" style="vertical-align:-1px"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg> <a href="https://huepattl.de/products/unterlumen" target="_blank" rel="noopener">Unterlumen</a></span>
  <a href="https://github.com/bjblazko/unterlumen" target="_blank" rel="noopener" title="View on GitHub"><svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg></a>
  {{- if or .Nav.ContactEmail .Nav.ContactURL}}
  <div class="footer-contact">
    {{- if .Nav.ContactEmail}}<a href="mailto:{{.Nav.ContactEmail}}">{{.Nav.ContactEmail}}</a>{{end}}
    {{- if .Nav.ContactURL}}<a href="{{.Nav.ContactURL}}" target="_blank" rel="noopener">{{.Nav.ContactURL}}</a>{{end}}
  </div>
  {{- end}}
</footer>
</body>
</html>
`))

// GenerateIndex produces a static root index.html referencing shared assets.
// Albums are ordered newest first by PublishedAt.
func GenerateIndex(siteTitle, defaultTheme, siteURL string, albums []Album, nav NavContext) []byte {
	if siteTitle == "" {
		siteTitle = "Photo Albums"
	}
	if defaultTheme == "" {
		defaultTheme = "light"
	}
	sorted := make([]Album, 0, len(albums))
	for _, a := range albums {
		if a.Unlisted {
			continue
		}
		sorted = append(sorted, a)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PublishedAt.After(sorted[j].PublishedAt)
	})
	items := make([]siteAlbumData, len(sorted))
	for i, a := range sorted {
		loading := "lazy"
		if i < 2 {
			loading = "eager"
		}
		items[i] = siteAlbumData{
			FolderName: AlbumFolderName(a),
			Title:      a.Title,
			DateStr:    DateRangeStr(a.PublishedAt, a.UpdatedAt),
			PhotoCount: a.PhotoCount,
			CoverFile:  a.CoverFile,
			Loading:    loading,
		}
	}

	description := fmt.Sprintf("Photography collection — %d album", len(sorted))
	if len(sorted) != 1 {
		description += "s"
	}
	description += "."

	ldMap := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "CollectionPage",
		"name":        siteTitle,
		"description": description,
	}
	if siteURL != "" {
		ldMap["url"] = strings.TrimRight(siteURL, "/") + "/"
	}
	ldJSON, _ := json.Marshal(ldMap)

	cleanSiteURL := ""
	if siteURL != "" {
		cleanSiteURL = strings.TrimRight(siteURL, "/")
	}

	var buf bytes.Buffer
	siteTmpl.Execute(&buf, struct { //nolint:errcheck
		Title        string
		DefaultTheme string
		Description  string
		SiteURL      string
		LDJSON       template.JS
		Albums       []siteAlbumData
		Nav          NavContext
	}{
		Title:        siteTitle,
		DefaultTheme: defaultTheme,
		Description:  description,
		SiteURL:      cleanSiteURL,
		LDJSON:       template.JS(ldJSON),
		Albums:       items,
		Nav:          nav,
	})
	return buf.Bytes()
}

/* --- Site album gallery --- */
