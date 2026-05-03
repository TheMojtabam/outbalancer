package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var embedded embed.FS

// FS returns an http.FileSystem rooted at the embedded dist directory.
func FS() http.FileSystem {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err)
	}
	return http.FS(sub)
}

// Handler returns an http.Handler that serves the embedded SPA, falling back
// to index.html for non-asset paths so client-side routing works.
func Handler() http.Handler {
	fileServer := http.FileServer(FS())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block API paths from leaking into static
		if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			http.NotFound(w, r)
			return
		}
		// Serve assets directly; for any other path, fallback to index.html
		// so SPA hash routing or path routing is supported.
		if r.URL.Path != "/" {
			f, err := FS().Open(r.URL.Path)
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
			r2 := *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, &r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
