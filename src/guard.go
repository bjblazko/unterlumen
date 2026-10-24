package main

import (
	"net"
	"net/http"
	"strings"
	"time"
)

// A page on another site may send requests to Unterlumen through the
// browser of someone who has it open: a form or a fetch to localhost that
// deletes, moves or publishes (cross-site request forgery). And a name of
// the attacker's that resolves to 127.0.0.1 makes their page same-origin
// with the app, so it can read the answers too (DNS rebinding). guard
// refuses both: requests that change something must come from the app's own
// pages, and an installation that listens only on this computer answers
// only to this computer's names.
func guard(h http.Handler, bind string) http.Handler {
	h = withSecurityHeaders(http.NewCrossOriginProtection().Handler(h))
	if !loopbackName(bind) {
		// Served on a network, as on a NAS behind its proxy: the names it is
		// reached by are the proxy's, not known here.
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopbackName(r.Host) {
			http.Error(w, "This installation answers only to localhost.", http.StatusMisdirectedRequest)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// loopbackName reports whether host — a name or address, with or without a
// port — means this computer.
func loopbackName(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// The pages load scripts, styles and fonts only from the app itself, and
// talk to OpenFreeMap for the map; anything a forged name or file might slip
// in cannot fetch from or send to another host. Inline scripts stay allowed:
// index.html imports MapLibre in one, and thumbnails fall back in onerror.
// No other site may frame the app (clickjacking), and nothing is sniffed
// into a type it was not sent as.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob: https://tiles.openfreemap.org; font-src 'self' data:; " +
	"connect-src 'self' https://tiles.openfreemap.org; worker-src 'self' blob:; " +
	"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"

func withSecurityHeaders(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr := w.Header()
		hdr.Set("Content-Security-Policy", contentSecurityPolicy)
		hdr.Set("X-Content-Type-Options", "nosniff")
		hdr.Set("X-Frame-Options", "SAMEORIGIN")
		hdr.Set("Referrer-Policy", "same-origin")
		h.ServeHTTP(w, r)
	})
}

// newHTTPServer is the server both ways of running use. A header has to
// arrive within ten seconds, so slow clients cannot hold connections open;
// there is no write timeout, as status and scan streams stay open for
// minutes.
func newHTTPServer(h http.Handler) *http.Server {
	return &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
}
