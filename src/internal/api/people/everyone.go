package apipeople

import (
	"net/http"
	"strings"

	lib "huepattl.de/unterlumen/internal/library"
)

// everyone lists the people named in any library, put together by name.
func everyone(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		people, err := mgr.Everyone()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, people)
	}
}

// companions lists who is most often with the person of ?name=.
func companions(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			http.Error(w, "A name is needed.", http.StatusBadRequest)
			return
		}
		with, err := mgr.CompanionsOf(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, with)
	}
}

// someone shows the person of ?name= in every library, and who in the
// other libraries may be them.
func someone(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			http.Error(w, "A name is needed.", http.StatusBadRequest)
			return
		}
		found, err := mgr.SomeoneNamed(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, found)
	}
}
