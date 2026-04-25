package channels

import "huepattl.de/unterlumen/internal/media"

// Account is one named account or destination within a channel (e.g. two Mastodon logins).
type Account struct {
	ID     string            `json:"id"`
	Label  string            `json:"label"`
	Config map[string]string `json:"config,omitempty"` // handler-specific: tokens, URLs, paths…
}

// Channel defines a publish target with its export settings and optional handler config.
type Channel struct {
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	Handler       string            `json:"handler,omitempty"`       // "" = default (export only); future: "mastodon", "scp", …
	HandlerConfig map[string]string `json:"handlerConfig,omitempty"` // free-form config for the handler
	Accounts      []Account         `json:"accounts,omitempty"`      // named sub-accounts; empty = single anonymous destination
	Format        string            `json:"format"`                  // "jpeg", "png", "webp"
	Quality       int               `json:"quality"`                 // 1–100
	Scale         media.ScaleOptions `json:"scale"`
	ExifMode      string            `json:"exifMode"`      // "strip", "keep", "keep_no_gps"
	GalleryExport bool              `json:"galleryExport,omitempty"` // generate index.html gallery on publish
}

// ExportOptions returns the media.ExportOptions for this channel.
func (c *Channel) ExportOptions() media.ExportOptions {
	return media.ExportOptions{
		Format:   c.Format,
		Quality:  c.Quality,
		Scale:    c.Scale,
		ExifMode: c.ExifMode,
	}
}

// AccountByID returns the account with the given ID, or nil.
func (c *Channel) AccountByID(id string) *Account {
	for i := range c.Accounts {
		if c.Accounts[i].ID == id {
			return &c.Accounts[i]
		}
	}
	return nil
}
