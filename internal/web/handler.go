package web

import (
	"io/fs"
	"net/http"
	"strings"
)

// SPAHandler serves the embedded SPA with fallback to index.html for client-side routing.
func SPAHandler() http.Handler {
	fsys, err := fs.Sub(Assets, "dist")
	if err != nil {
		// No embedded assets (development mode), return placeholder
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>ELNSSM</title></head>
<body>
<h1>ELNSSM - Web GUI</h1>
<p>The web GUI has not been built yet. Run <code>cd web && npm install && npm run build</code> to build it.</p>
<p>API is available at <a href="/api/v1/system/status">/api/v1/system/status</a></p>
</body>
</html>`))
		})
	}

	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Try to serve the file directly
		if path != "/" && !strings.HasPrefix(path, "/api/") {
			// Check if file exists
			f, err := fsys.Open(strings.TrimPrefix(path, "/"))
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// SPA fallback: serve index.html for all non-file routes
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
