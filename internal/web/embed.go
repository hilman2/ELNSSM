// Package web embeds the static assets of the ELNSSM web GUI and
// exposes a single HTTP handler that serves them with SPA routing.
package web

import "embed"

// Assets holds the embedded web GUI static files.
// During development, the dist/ directory may not exist yet.
// Build the frontend first: cd web && npm run build
//
//go:embed all:dist
var Assets embed.FS
