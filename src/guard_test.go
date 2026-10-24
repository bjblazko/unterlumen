package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func guardAnswer(bind, method, host string, header map[string]string) int {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(method, "http://"+host+"/api/fileops/delete", nil)
	req.Host = host
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	guard(ok, bind).ServeHTTP(rec, req)
	return rec.Code
}

func TestGuardRefusesRequestsFromOtherSites(t *testing.T) {
	cases := []struct {
		name   string
		bind   string
		method string
		host   string
		header map[string]string
		want   int
	}{
		{"the app's own page", "localhost", "POST", "localhost:8090", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://localhost:8090"}, http.StatusNoContent},
		{"a page on another site", "localhost", "POST", "localhost:8090", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, http.StatusForbidden},
		{"an older browser from another site", "localhost", "POST", "localhost:8090", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		{"curl and the tests, without a browser", "localhost", "POST", "127.0.0.1:8090", nil, http.StatusNoContent},
		{"reading from another site is the browser's to refuse", "localhost", "GET", "localhost:8090", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusNoContent},
		{"a rebinding name on this computer", "localhost", "GET", "attacker.example:8090", nil, http.StatusMisdirectedRequest},
		{"IPv6 loopback", "::1", "GET", "[::1]:8090", nil, http.StatusNoContent},
		{"served on the network, behind a proxy's name", "0.0.0.0", "GET", "fotos.example.org", nil, http.StatusNoContent},
		{"served on the network, still no forgery", "0.0.0.0", "POST", "fotos.example.org", map[string]string{"Sec-Fetch-Site": "cross-site"}, http.StatusForbidden},
	}
	for _, c := range cases {
		if got := guardAnswer(c.bind, c.method, c.host, c.header); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestGuardSetsSecurityHeaders(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	rec := httptest.NewRecorder()
	guard(ok, "localhost").ServeHTTP(rec, httptest.NewRequest("GET", "http://localhost:8090/", nil))
	for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if rec.Header().Get(h) == "" {
			t.Errorf("no %s", h)
		}
	}
}
