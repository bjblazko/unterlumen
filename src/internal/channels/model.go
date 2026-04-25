package channels

import "huepattl.de/unterlumen/internal/media"

// Channel defines a publish target with its export settings.
type Channel struct {
	Slug     string           `json:"slug"`
	Name     string           `json:"name"`
	Format   string           `json:"format"`   // "jpeg", "png", "webp"
	Quality  int              `json:"quality"`  // 1–100
	Scale    media.ScaleOptions `json:"scale"`
	ExifMode string           `json:"exifMode"` // "strip", "keep", "keep_no_gps"
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
