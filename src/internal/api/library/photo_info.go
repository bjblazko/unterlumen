package apilibrary

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	lib "huepattl.de/unterlumen/internal/library"
)

// photoInfoResp mirrors the browse /api/info response shape so the frontend
// InfoPanel can consume it without modification.
type photoInfoResp struct {
	Name     string        `json:"name"`
	Path     string        `json:"path"`
	Size     int64         `json:"size"`
	Format   string        `json:"format"`
	Modified string        `json:"modified"`
	Exif     *photoExifOut `json:"exif,omitempty"`
}

type photoExifOut struct {
	Tags          map[string]string `json:"tags,omitempty"`
	Width         int               `json:"width,omitempty"`
	Height        int               `json:"height,omitempty"`
	Latitude      *float64          `json:"latitude,omitempty"`
	Longitude     *float64          `json:"longitude,omitempty"`
	DateTaken     *string           `json:"dateTaken,omitempty"`
	DateDigitized *string           `json:"dateDigitized,omitempty"`
	DateModified  *string           `json:"dateModified,omitempty"`
}

func photoInfo(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		p, err := store.GetPhotoInfo(photoID)
		if err != nil || p == nil {
			http.Error(w, "photo not found", http.StatusNotFound)
			return
		}

		writeJSON(w, buildPhotoInfoResp(p))
	}
}

// photoExifStored mirrors media.ExifData for unmarshaling the stored exif_json blob.
type photoExifStored struct {
	Tags          map[string]string `json:"tags"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	Latitude      *float64          `json:"latitude"`
	Longitude     *float64          `json:"longitude"`
	DateTaken     *string           `json:"dateTaken"`
	DateDigitized *string           `json:"dateDigitized"`
	DateModified  *string           `json:"dateModified"`
}

func buildPhotoInfoResp(p *lib.PhotoInfo) photoInfoResp {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(p.Filename), "."))
	resp := photoInfoResp{
		Name:     p.Filename,
		Path:     p.PathHint,
		Size:     p.FileSize,
		Format:   ext,
		Modified: p.IndexedAt,
	}
	if p.ExifJSON == "" {
		return resp
	}

	var stored photoExifStored
	if err := json.Unmarshal([]byte(p.ExifJSON), &stored); err != nil {
		return resp
	}

	out := &photoExifOut{
		Tags:          stored.Tags,
		Width:         stored.Width,
		Height:        stored.Height,
		DateTaken:     stored.DateTaken,
		DateDigitized: stored.DateDigitized,
		DateModified:  stored.DateModified,
	}

	// Use pre-parsed GPS coordinates when available; fall back to tag parsing.
	if stored.Latitude != nil && stored.Longitude != nil {
		out.Latitude = stored.Latitude
		out.Longitude = stored.Longitude
	} else if stored.Tags != nil {
		if lat, ok := parseGPSCoord(stored.Tags["GPSLatitude"], stored.Tags["GPSLatitudeRef"]); ok {
			if lon, ok := parseGPSCoord(stored.Tags["GPSLongitude"], stored.Tags["GPSLongitudeRef"]); ok {
				out.Latitude = &lat
				out.Longitude = &lon
			}
		}
	}

	resp.Exif = out
	return resp
}

// parseGPSCoord converts a goexif GPS tag string to decimal degrees.
// Handles rational DMS format "[48/1, 52/1, 4746/100]" and plain decimals.
func parseGPSCoord(coord, ref string) (float64, bool) {
	coord = strings.TrimSpace(coord)
	if coord == "" {
		return 0, false
	}
	// Plain decimal (e.g. "48.879850").
	if v, err := strconv.ParseFloat(coord, 64); err == nil {
		if strings.EqualFold(strings.TrimSpace(ref), "S") || strings.EqualFold(strings.TrimSpace(ref), "W") {
			v = -v
		}
		return v, true
	}
	// Rational DMS: "[d/1, m/1, s/100]".
	coord = strings.Trim(coord, "[] ")
	parts := strings.SplitN(coord, ",", 3)
	if len(parts) != 3 {
		return 0, false
	}
	vals := make([]float64, 3)
	for i, p := range parts {
		n, d, ok := parseRat(strings.TrimSpace(p))
		if !ok || d == 0 {
			return 0, false
		}
		vals[i] = n / d
	}
	deg := vals[0] + vals[1]/60 + vals[2]/3600
	if strings.EqualFold(strings.TrimSpace(ref), "S") || strings.EqualFold(strings.TrimSpace(ref), "W") {
		deg = -deg
	}
	return deg, true
}

func parseRat(s string) (float64, float64, bool) {
	idx := strings.IndexByte(s, '/')
	if idx < 0 {
		return 0, 0, false
	}
	n, err1 := strconv.ParseFloat(s[:idx], 64)
	d, err2 := strconv.ParseFloat(s[idx+1:], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return n, d, true
}
