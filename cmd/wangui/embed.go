package main

import (
	"embed"
	"io/fs"
)

// spaFS holds the Vite production build (web/dist).
// During development this may be just a placeholder; run `npm run build` in
// the web/ directory before `go build` to embed the real frontend.
//
//go:embed all:web-dist
var spaRoot embed.FS

func spaFS() fs.FS {
	sub, err := fs.Sub(spaRoot, "web-dist")
	if err != nil {
		return spaRoot
	}
	return sub
}
