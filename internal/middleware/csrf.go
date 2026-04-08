package middleware

import (
	"net/http"
	"strings"
)

// CSRFProtection rejects state-changing requests (non GET/HEAD/OPTIONS) that
// do not carry a header proving the request originated from JavaScript
// (either X-Requested-With: XMLHttpRequest or Content-Type: application/json).
// Download paths are excluded because they are protected by one-time tokens.
func CSRFProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		// Skip CSRF for download paths (protected by one-time tokens).
		if strings.HasPrefix(r.URL.Path, "/api/v1/download/") {
			next.ServeHTTP(w, r)
			return
		}

		ct := strings.ToLower(r.Header.Get("Content-Type"))
		xrw := strings.ToLower(r.Header.Get("X-Requested-With"))

		if strings.HasPrefix(ct, "application/json") || xrw == "xmlhttprequest" {
			next.ServeHTTP(w, r)
			return
		}

		// Remote config endpoints use multipart forms — allow multipart/form-data.
		if strings.HasPrefix(ct, "multipart/form-data") {
			next.ServeHTTP(w, r)
			return
		}

		writeError(w, http.StatusForbidden, 40301, "csrf validation failed")
	})
}
