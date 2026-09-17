package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web
var webFiles embed.FS

// uiHandler serves the operator UI from inside the binary. Nothing is fetched
// from a CDN: an instance on a laptop with no internet, or on a machine that is
// not allowed to reach one, has to work the same as any other.
func uiHandler() http.Handler {
	sub, err := fs.Sub(webFiles, "web")
	if err != nil {
		// The files are embedded at build time; a failure here means the build
		// is wrong, not the request.
		panic("vigil: embedded web assets missing: " + err.Error())
	}
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The UI holds a bearer token in localStorage and talks to the API with
		// it. A strict policy is what keeps an injected script from being able
		// to read that token and post it somewhere; 'self' only, no inline.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		files.ServeHTTP(w, r)
	})
}
