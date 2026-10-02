package apilibrary

import (
	"net/http"
	"net/url"
	"regexp"

	lib "huepattl.de/unterlumen/internal/library"
)

// The shared filter of the Map, the Statistics and the Timeline (ADR-0050):
// month_from and month_until are months, YYYY-MM; model and lens repeat, one
// value each, as exif_index stores them.

var monthPattern = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)

// scopeFilter reads the filter from a request; anything malformed is left out.
func scopeFilter(r *http.Request) lib.Filter {
	return ScopeFilter(r)
}

// ScopeFilter is scopeFilter for other API packages (the Timeline).
func ScopeFilter(r *http.Request) lib.Filter {
	return filterFromQuery(r.URL.Query())
}

func filterFromQuery(q url.Values) lib.Filter {
	f := lib.Filter{Models: nonEmpty(q["model"]), Lenses: nonEmpty(q["lens"])}
	if v := q.Get("month_from"); monthPattern.MatchString(v) {
		f.From = v
	}
	if v := q.Get("month_until"); monthPattern.MatchString(v) {
		f.Until = v
	}
	return f
}

func nonEmpty(values []string) []string {
	var out []string
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// scopeValues answers what the shared filter offers: cameras and lenses with
// their photos — lenses of the chosen cameras only — and the months spanned.
func scopeValues(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		v, err := mgr.ScopeValues(parseIDList(q.Get("ids")), nonEmpty(q["model"]))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, v)
	}
}
