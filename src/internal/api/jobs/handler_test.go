package jobs

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"huepattl.de/unterlumen/internal/jobs"
)

func runTracked(t *testing.T, h http.HandlerFunc) jobs.Job {
	t.Helper()
	reg := jobs.NewRegistry()
	tracked := Track(reg, "deploy", "destinations", func(*http.Request) string { return "Deploying" }, h)
	tracked(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil))
	js := reg.Snapshot()
	if len(js) != 1 || !js[0].Finished {
		t.Fatalf("want one finished job, got %+v", js)
	}
	return js[0]
}

func TestTrackSuccess(t *testing.T) {
	j := runTracked(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})
	if j.Error != "" {
		t.Fatalf("want success, got error %q", j.Error)
	}
}

func TestTrackErrorStatus(t *testing.T) {
	j := runTracked(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "channel not found", http.StatusNotFound)
	})
	if j.Error != "channel not found" {
		t.Fatalf("want the response text as reason, got %q", j.Error)
	}
}

// Deploy answers 200 with "ok": false when the push fails.
func TestTrackOKFalse(t *testing.T) {
	j := runTracked(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":false,"output":"...","error":"ssh: connect refused"}`))
	})
	if j.Error != "ssh: connect refused" {
		t.Fatalf("want the error field as reason, got %q", j.Error)
	}
}

func TestTrackPlainJSONWithoutOK(t *testing.T) {
	j := runTracked(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"rebuilt":3,"errors":[]}`))
	})
	if j.Error != "" {
		t.Fatalf("an answer without ok is a success, got %q", j.Error)
	}
}
