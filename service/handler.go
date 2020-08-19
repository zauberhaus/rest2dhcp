package service

import (
	"net/http"
)

// RedirectHandler redirects temporary to the given path
func RedirectHandler(path string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, path, http.StatusTemporaryRedirect)
	})
}
