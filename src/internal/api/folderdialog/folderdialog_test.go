package apifolderdialog

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOnlyThisComputerMayOpenTheDialog(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/folder-dialog", strings.NewReader(`{}`))
	req.RemoteAddr = "192.168.178.20:51234"
	rec := httptest.NewRecorder()
	chooseFolder(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("a request from the network got %d, want 403", rec.Code)
	}
}

func TestFromThisComputer(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1:5000":   true,
		"[::1]:5000":       true,
		"192.168.1.5:5000": false,
		"not an address":   false,
	} {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.RemoteAddr = addr
		if got := fromThisComputer(r); got != want {
			t.Errorf("%s: %v, want %v", addr, got, want)
		}
	}
}
