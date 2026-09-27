package apilibrary

import (
	"net/http"

	lib "huepattl.de/unterlumen/internal/library"
)

// geoResp is the map's data. Points are arrays [photoID, lat, lon, taken, filename]
// rather than objects, which keeps tens of thousands of them small.
type geoResp struct {
	Libraries []geoLibrary `json:"libraries"`
}

type geoLibrary struct {
	ID     string  `json:"id"`
	Points [][]any `json:"points"`
}

func libraryGeo(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		libs, err := mgr.GeoPoints()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, toGeoResp(libs))
	}
}

func toGeoResp(libs []lib.LibraryGeo) geoResp {
	resp := geoResp{Libraries: make([]geoLibrary, 0, len(libs))}
	for _, l := range libs {
		points := make([][]any, len(l.Points))
		for i, p := range l.Points {
			points[i] = []any{p.ID, p.Lat, p.Lon, p.Taken, p.Filename}
		}
		resp.Libraries = append(resp.Libraries, geoLibrary{ID: l.LibraryID, Points: points})
	}
	return resp
}
