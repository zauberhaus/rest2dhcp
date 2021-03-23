package service

import (
	"net/http"
)

// RedirectHandler redirects temporary to the given path
func RedirectHandler(path string, code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path, code)
	})
}
