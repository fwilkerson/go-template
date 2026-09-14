// Package web is the application shell: the page layout, static assets and
// middleware that feature packages build on. Features register their own
// routes; main mounts them alongside this package's.
package web

//go:generate templ generate -path ..
//go:generate tailwindcss -i tailwind.css -o static/app.css --minify

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

// Routes serves the embedded static assets.
func Routes(mux *http.ServeMux) {
	mux.Handle("GET /static/", http.FileServerFS(static))
}

// Render writes a component, or a 500 when rendering fails.
func Render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
	}
}

// LogRequests logs one line per request before passing it on.
func LogRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.InfoContext(r.Context(), "request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
