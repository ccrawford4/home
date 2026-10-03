// Package web embeds the built React frontend (web/dist) into the binary.
package web

import "embed"

// Dist holds the Vite build output. Run `npm run build` in this directory
// before `go build`; the Dockerfile does this automatically.
//
//go:embed all:dist
var Dist embed.FS
