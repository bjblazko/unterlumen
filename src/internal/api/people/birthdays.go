package apipeople

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/birthdays"
	lib "huepattl.de/unterlumen/internal/library"
)

func listBirthdays(bdays *birthdays.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		all, err := bdays.All()
		if err != nil {
			http.Error(w, "The birthdays could not be read: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, all)
	}
}

// setBirthday gives the person of the body's name a birthday; an empty one
// takes it away.
func setBirthday(bdays *birthdays.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name     string `json:"name"`
			Birthday string `json:"birthday"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			http.Error(w, "A name is needed.", http.StatusBadRequest)
			return
		}
		switch err := bdays.Set(body.Name, body.Birthday); {
		case errors.Is(err, birthdays.ErrBadDate):
			http.Error(w, "A birthday is a day that has been, like 2015-05-12.", http.StatusBadRequest)
		case err != nil:
			http.Error(w, "The birthday could not be saved: "+err.Error(), http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// addAges says how old each named face was when the photo was taken, where
// the person's birthday and the photo's date are known.
func addAges(store *lib.Store, bdays *birthdays.Store, photoID string, found []lib.PhotoFace) {
	if bdays == nil {
		return
	}
	all, err := bdays.All()
	if err != nil || len(all) == 0 {
		return
	}
	taken, err := store.DateTaken(photoID)
	if err != nil || len(taken) < 10 {
		return
	}
	day, err := time.Parse("2006-01-02", taken[:10])
	if err != nil {
		return
	}
	for i := range found {
		if birth, ok := birthdays.Of(all, found[i].Name); ok && found[i].Name != "" && !day.Before(birth) {
			age := birthdays.AgeAt(birth, day)
			found[i].Age = &age
		}
	}
}
