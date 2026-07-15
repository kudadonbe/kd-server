package http

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed static
var staticFS embed.FS

// staticHandler serves embedded admin assets (e.g. htmx.min.js) under
// /admin/static/. It requires no authentication.
func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("http: static assets: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.StripPrefix("/admin/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		fileServer.ServeHTTP(w, r)
	}))
}
