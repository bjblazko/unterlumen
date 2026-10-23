// Package apipeople serves People: installing face recognition, the
// groups of faces it found and the people their owner named (ADR-0051).
package apipeople

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"huepattl.de/unterlumen/internal/birthdays"
	"huepattl.de/unterlumen/internal/jobs"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/toolinstall"
)

// At most this many groups are shown at once, the largest first; naming
// them brings up the next. A face seen once is a group of one, last.
const maxGroups = 60

// Handle registers the People routes. bdays keeps the birthdays; nil
// without a folder for them, which leaves birthdays and ages out.
func Handle(mux *http.ServeMux, mgr *lib.Manager, reg *jobs.Registry, bdays *birthdays.Store) {
	mux.HandleFunc("GET /api/faces", recognitionStatus(mgr))
	mux.HandleFunc("POST /api/faces/install", installRecognition(mgr, reg))
	mux.HandleFunc("GET /api/library/person-names", personNames(mgr))
	mux.HandleFunc("GET /api/people", everyone(mgr))
	mux.HandleFunc("GET /api/people/someone", someone(mgr))
	mux.HandleFunc("GET /api/people/with", companions(mgr))
	mux.HandleFunc("GET /api/people/maybe", maybe(mgr))
	mux.HandleFunc("GET /api/people/hidden", hiddenEverywhere(mgr))
	mux.HandleFunc("GET /api/people/to-name", toName(mgr))
	if bdays != nil {
		mux.HandleFunc("GET /api/people/birthdays", listBirthdays(bdays))
		mux.HandleFunc("PUT /api/people/birthdays", setBirthday(bdays))
	}
	mux.HandleFunc("GET /api/library/{id}/people", overview(mgr))
	mux.HandleFunc("GET /api/library/{id}/people/hidden", hiddenFaces(mgr))
	mux.HandleFunc("GET /api/library/{id}/people/groups/{grp}", groupFaces(mgr))
	mux.HandleFunc("GET /api/library/{id}/people/groups/{grp}/suggestion", groupSuggestion(mgr))
	mux.HandleFunc("GET /api/library/{id}/people/{pid}", personFaces(mgr))
	mux.HandleFunc("PATCH /api/library/{id}/people/{pid}", renamePerson(mgr))
	mux.HandleFunc("DELETE /api/library/{id}/people/{pid}", forgetPerson(mgr))
	mux.HandleFunc("PUT /api/library/{id}/people/{pid}/cover", chooseCover(mgr))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}/faces", photoFaces(mgr, bdays))
	mux.HandleFunc("POST /api/library/{id}/photo/{photoID}/faces", markFace(mgr))
	mux.HandleFunc("GET /api/library/{id}/face/{fid}", serveFace(mgr))
	mux.HandleFunc("POST /api/library/{id}/faces/name", nameFaces(mgr))
	mux.HandleFunc("POST /api/library/{id}/faces/unname", changeFaces(mgr, (*lib.Store).UnnameFaces))
	mux.HandleFunc("POST /api/library/{id}/faces/hide", changeFaces(mgr, (*lib.Store).HideFaces))
	mux.HandleFunc("POST /api/library/{id}/faces/not", notThem(mgr))
	mux.HandleFunc("POST /api/library/{id}/faces/show", changeFaces(mgr, func(s *lib.Store, ids []int64) (int, error) { return 0, s.ShowAgain(ids) }))
}

// personNames lists the people named in the libraries ids (comma-separated;
// all when empty), for the library filter's Person chip.
func personNames(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ids []string
		for _, id := range strings.Split(r.URL.Query().Get("ids"), ",") {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
		names, err := mgr.PersonNames(ids)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, names)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func recognitionStatus(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, struct {
			Installed bool                        `json:"installed"`
			Plan      toolinstall.RecognitionPlan `json:"plan"`
		}{mgr.FacesInstalled(), toolinstall.PlanRecognition()})
	}
}

// installRecognition downloads ONNX Runtime and the models. Like installing
// the helper programs it is a job in the status line that goes on when the
// page is left; the request waits for it. Every library is looked at once
// it is done.
func installRecognition(mgr *lib.Manager, reg *jobs.Registry) http.HandlerFunc {
	var busy sync.Mutex
	return func(w http.ResponseWriter, r *http.Request) {
		if !busy.TryLock() {
			http.Error(w, "Face recognition is being installed already.", http.StatusConflict)
			return
		}
		job := reg.Start("tools", "Installing face recognition", "people")
		done := make(chan error, 1)
		go func() {
			defer busy.Unlock()
			err := toolinstall.InstallRecognition(context.Background(), job, mgr.RecognitionDir())
			job.Finish(err)
			done <- err
			if err == nil {
				mgr.FindAllFacesMissing()
			}
		}()
		select {
		case err := <-done:
			if err != nil {
				http.Error(w, "Face recognition was not installed: "+err.Error(), http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
		}
	}
}

func openStore(mgr *lib.Manager, w http.ResponseWriter, r *http.Request) (*lib.Store, bool) {
	store, err := mgr.OpenStore(r.PathValue("id"))
	if err != nil {
		http.Error(w, "The library was not found.", http.StatusNotFound)
		return nil, false
	}
	return store, true
}

func pathInt(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		http.Error(w, name+" must be a number", http.StatusBadRequest)
		return 0, false
	}
	return v, true
}

// overview is what People shows of a library: who is named, which groups
// wait for a name, how many faces are hidden, and how many photos are
// still to be looked at.
func overview(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		if !ok {
			return
		}
		people, err := store.People()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		groups, err := store.FaceGroups(1, maxGroups)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		groupCount, err := store.GroupCount()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		pending, err := store.PhotosNeedingFaces()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		hidden, err := store.HiddenCount()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, struct {
			People     []lib.Person    `json:"people"`
			Groups     []lib.FaceGroup `json:"groups"`
			GroupCount int             `json:"groupCount"`
			Hidden     int             `json:"hidden"`
			Pending    int             `json:"pending"`
			Finding    bool            `json:"finding"`
		}{people, groups, groupCount, hidden, len(pending), mgr.FindingFaces(r.PathValue("id"))})
	}
}

func groupFaces(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		grp, ok2 := pathInt(w, r, "grp")
		if !ok || !ok2 {
			return
		}
		refs, err := store.GroupFaces(grp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, refs)
	}
}

// hiddenFaces lists the faces hidden in the library, to show again.
func hiddenFaces(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		if !ok {
			return
		}
		faces, err := store.HiddenFaces()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, faces)
	}
}

// groupSuggestion answers who a group looks like: { suggestion } or {}.
func groupSuggestion(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		if !ok {
			return
		}
		grp, ok := pathInt(w, r, "grp")
		if !ok {
			return
		}
		p, err := store.GroupSuggestion(grp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]*lib.Person{"suggestion": p})
	}
}

func personFaces(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		pid, ok2 := pathInt(w, r, "pid")
		if !ok || !ok2 {
			return
		}
		refs, err := store.PersonFaces(pid)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, refs)
	}
}

// changeResult says how many photos keep their names in this library only,
// because another program keeps the face regions of their sidecars.
type changeResult struct {
	Skipped int `json:"skipped"`
}

func renamePerson(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		pid, ok2 := pathInt(w, r, "pid")
		if !ok || !ok2 {
			return
		}
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
			http.Error(w, "A name is needed.", http.StatusBadRequest)
			return
		}
		skipped, err := store.RenamePerson(pid, body.Name)
		writeChange(w, skipped, err)
	}
}

// forgetPerson takes the person's name away from all their faces.
func forgetPerson(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		pid, ok2 := pathInt(w, r, "pid")
		if !ok || !ok2 {
			return
		}
		skipped, err := store.ForgetPerson(pid)
		writeChange(w, skipped, err)
	}
}

// chooseCover makes the face in the body the one that shows the person.
func chooseCover(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		pid, ok2 := pathInt(w, r, "pid")
		if !ok || !ok2 {
			return
		}
		var body struct {
			Face int64 `json:"face"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Face == 0 {
			http.Error(w, "A face is needed.", http.StatusBadRequest)
			return
		}
		switch err := store.ChooseCover(pid, body.Face); {
		case errors.Is(err, lib.ErrNotTheirFace):
			http.Error(w, "This face is not one of theirs any more. Reload the page.", http.StatusConflict)
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// photoFaces answers the faces of one photo and the names to choose from,
// those of all libraries, for the info panel.
func photoFaces(mgr *lib.Manager, bdays *birthdays.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		if !ok {
			return
		}
		found, err := store.PhotoFaces(r.PathValue("photoID"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// The names of every library: someone named in one is offered in
		// all, so a name is not typed a second, slightly different way.
		names, err := mgr.PersonNames(nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		addAges(store, bdays, r.PathValue("photoID"), found)
		writeJSON(w, map[string]any{"faces": found, "names": names})
	}
}

// faceRequest is the body of a change to faces.
type faceRequest struct {
	Faces  []int64 `json:"faces"`
	Name   string  `json:"name"`
	Person int64   `json:"person"` // for a No: who the faces are not
}

func readFaceRequest(w http.ResponseWriter, r *http.Request) (faceRequest, bool) {
	var body faceRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Faces) == 0 {
		http.Error(w, "Faces are needed.", http.StatusBadRequest)
		return body, false
	}
	return body, true
}

func nameFaces(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		if !ok {
			return
		}
		body, ok := readFaceRequest(w, r)
		if !ok {
			return
		}
		if strings.TrimSpace(body.Name) == "" {
			http.Error(w, "A name is needed.", http.StatusBadRequest)
			return
		}
		skipped, err := store.NameFaces(body.Faces, body.Name)
		writeChange(w, skipped, err)
	}
}

// notThem records that the faces are not the person, so they are not
// suggested as them again.
func notThem(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		if !ok {
			return
		}
		body, ok := readFaceRequest(w, r)
		if !ok {
			return
		}
		if body.Person == 0 {
			http.Error(w, "A person is needed.", http.StatusBadRequest)
			return
		}
		if err := store.NotThem(body.Faces, body.Person); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// markFace marks a face the detector missed: a box in fractions of the
// upright photo and the name of who it is.
func markFace(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		if !ok {
			return
		}
		var box media.FaceRegion
		if err := json.NewDecoder(r.Body).Decode(&box); err != nil || strings.TrimSpace(box.Name) == "" {
			http.Error(w, "A frame and a name are needed.", http.StatusBadRequest)
			return
		}
		skipped, err := store.MarkFace(r.PathValue("photoID"), box)
		switch {
		case errors.Is(err, lib.ErrFaceAlreadyThere):
			http.Error(w, "There is a face there already; name it in the list.", http.StatusConflict)
		case errors.Is(err, lib.ErrBadFaceBox):
			http.Error(w, "The frame must lie inside the photo.", http.StatusBadRequest)
		default:
			writeChange(w, skipped, err)
		}
	}
}

// changeFaces serves a change that needs only the faces: unnaming, hiding.
func changeFaces(mgr *lib.Manager, change func(*lib.Store, []int64) (int, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, ok := openStore(mgr, w, r)
		if !ok {
			return
		}
		body, ok := readFaceRequest(w, r)
		if !ok {
			return
		}
		skipped, err := change(store, body.Faces)
		writeChange(w, skipped, err)
	}
}

// writeChange answers a change to names: how many photos keep them in this
// library only, because another program keeps their sidecars' face regions,
// or why a sidecar could not be written — its photo's faces then stay as
// they were, since the sidecar is written first (ADR-0052).
func writeChange(w http.ResponseWriter, skipped int, err error) {
	if err != nil {
		http.Error(w, "A photo's sidecar could not be written, so its faces stay as they were: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, changeResult{Skipped: skipped})
}
