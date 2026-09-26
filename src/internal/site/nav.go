package site

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"huepattl.de/unterlumen/internal/channels"
)

// NavContext carries optional page-link and contact data passed to all site template generators.
type NavContext struct {
	HasAbout     bool
	HasImprint   bool
	ContactEmail string
	ContactURL   string
	LogoExists   bool
	LogoPath     string // "assets/logo.jpg" for root-level pages; "../../assets/logo.jpg" for album pages
	SiteName     string
}

// markdownToHTML converts markdown text to safe HTML using goldmark.
// HTML passthrough is enabled so advanced users can embed raw HTML.
var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(html.WithUnsafe()),
)

func markdownToHTML(src string) template.HTML {
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(src))
	}
	return template.HTML(buf.String())
}

// AvatarExistsAt reports whether site/assets/avatar.jpg exists in siteDir.
func AvatarExistsAt(siteDir string) bool {
	_, err := os.Stat(filepath.Join(siteDir, "assets", "avatar.jpg"))
	return err == nil
}

// logoExistsAt reports whether site/assets/logo.jpg exists in siteDir.
func logoExistsAt(siteDir string) bool {
	_, err := os.Stat(filepath.Join(siteDir, "assets", "logo.jpg"))
	return err == nil
}

// BuildNavContext constructs a NavContext from channel config and current site state.
// rootLevel=true uses "assets/logo.jpg" (root index, about, legal); false uses "../../assets/logo.jpg" (album pages).
func BuildNavContext(ch *channels.Channel, siteDir string, rootLevel bool) NavContext {
	logoPath := "assets/logo.jpg"
	if !rootLevel {
		logoPath = "../../assets/logo.jpg"
	}
	return NavContext{
		HasAbout:     ch.SiteAbout != "",
		HasImprint:   ch.SiteImprint != "",
		ContactEmail: ch.SiteContactEmail,
		ContactURL:   ch.SiteContactURL,
		LogoExists:   logoExistsAt(siteDir),
		LogoPath:     logoPath,
		SiteName:     ch.SiteTitle,
	}
}
