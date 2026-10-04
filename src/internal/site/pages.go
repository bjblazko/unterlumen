package site

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/channels"
)

var siteAboutTmpl = template.Must(template.New("siteabout").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>About | {{.SiteTitle}}</title>
<meta name="description" content="About {{.SiteTitle}}">
<link rel="stylesheet" href="assets/style.css">
<script src="assets/toggle.js"></script>
</head>
<body>
<header>
  <div class="site-masthead">
    <div class="site-brand">
      {{- if .Nav.LogoExists}}
      <img class="site-logo" src="{{.Nav.LogoPath}}" alt="" loading="eager">
      {{- end}}
      <a class="site-name" href="index.html">{{.Nav.SiteName}}</a>
    </div>
    <div class="header-actions">
      <button id="theme-toggle" class="theme-btn">Dark</button>
    </div>
  </div>
  <div class="page-title">
    <a class="site-back" href="index.html" title="Back to albums"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="vertical-align:-4px"><polyline points="15 18 9 12 15 6"/></svg></a>About
  </div>
</header>

<main>
  <div class="about-layout">
    {{- if .AvatarExists}}
    <img class="about-photo" src="assets/avatar.jpg" alt="Portrait">
    {{- end}}
    <article class="prose">{{.Content}}</article>
  </div>
</main>

<footer>
  <span>Built with <svg width="13" height="13" viewBox="0 0 24 24" fill="var(--accent)" aria-hidden="true" style="vertical-align:-1px"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg> <a href="https://huepattl.de/products/unterlumen" target="_blank" rel="noopener">Unterlumen</a></span>
  {{- if .Nav.HasImprint}}<a href="legal.html" style="color:var(--text-muted);text-decoration:none;font-size:.75rem">Legal</a>{{end}}
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

// GenerateAboutPage produces about.html at the site root.
// Does nothing if SiteAbout is empty.
func GenerateAboutPage(siteDir string, ch *channels.Channel, avatarExists bool, nav NavContext) error {
	if ch.SiteAbout == "" {
		return nil
	}
	defaultTheme := ch.SiteTheme
	if defaultTheme == "" {
		defaultTheme = "light"
	}
	var buf bytes.Buffer
	if err := siteAboutTmpl.Execute(&buf, struct {
		SiteTitle    string
		DefaultTheme string
		Content      template.HTML
		AvatarExists bool
		Nav          NavContext
	}{
		SiteTitle:    ch.SiteTitle,
		DefaultTheme: defaultTheme,
		Content:      markdownToHTML(ch.SiteAbout),
		AvatarExists: avatarExists,
		Nav:          nav,
	}); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(siteDir, "about.html"), buf.Bytes(), 0o644)
}

/* --- Imprint / legal page --- */

var siteImprintTmpl = template.Must(template.New("siteimprint").Parse(`<!DOCTYPE html>
<html lang="en" data-default-theme="{{.DefaultTheme}}">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Legal | {{.SiteTitle}}</title>
<meta name="description" content="Legal notice for {{.SiteTitle}}">
<link rel="stylesheet" href="assets/style.css">
<script src="assets/toggle.js"></script>
</head>
<body>
<header>
  <div class="site-masthead">
    <div class="site-brand">
      {{- if .Nav.LogoExists}}
      <img class="site-logo" src="{{.Nav.LogoPath}}" alt="" loading="eager">
      {{- end}}
      <a class="site-name" href="index.html">{{.Nav.SiteName}}</a>
    </div>
    <div class="header-actions">
      <button id="theme-toggle" class="theme-btn">Dark</button>
    </div>
  </div>
  <div class="page-title">
    <a class="site-back" href="index.html" title="Back to albums"><svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" style="vertical-align:-4px"><polyline points="15 18 9 12 15 6"/></svg></a>Legal Notice
  </div>
</header>

<main>
  <article class="prose">{{.Content}}</article>
</main>

<footer>
  <span>Built with <svg width="13" height="13" viewBox="0 0 24 24" fill="var(--accent)" aria-hidden="true" style="vertical-align:-1px"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg> <a href="https://huepattl.de/products/unterlumen" target="_blank" rel="noopener">Unterlumen</a></span>
  {{- if .Nav.HasAbout}}<a href="about.html" style="color:var(--text-muted);text-decoration:none;font-size:.75rem">About</a>{{end}}
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

// GenerateImprintPage produces legal.html at the site root.
// Does nothing if SiteImprint is empty.
func GenerateImprintPage(siteDir string, ch *channels.Channel, nav NavContext) error {
	if ch.SiteImprint == "" {
		return nil
	}
	defaultTheme := ch.SiteTheme
	if defaultTheme == "" {
		defaultTheme = "light"
	}
	var buf bytes.Buffer
	if err := siteImprintTmpl.Execute(&buf, struct {
		SiteTitle    string
		DefaultTheme string
		Content      template.HTML
		Nav          NavContext
	}{
		SiteTitle:    ch.SiteTitle,
		DefaultTheme: defaultTheme,
		Content:      markdownToHTML(ch.SiteImprint),
		Nav:          nav,
	}); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(siteDir, "legal.html"), buf.Bytes(), 0o644)
}
