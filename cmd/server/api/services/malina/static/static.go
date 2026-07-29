// Package static provides the embedded Malina administration frontend.
package static

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed site
var files embed.FS

// Handler returns the embedded frontend file server.
func Handler() http.Handler {
	site, err := fs.Sub(files, "site")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(site))
}
