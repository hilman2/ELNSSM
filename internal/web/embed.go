package web

import "embed"

// Assets holds the embedded web GUI static files.
// During development, the dist/ directory may not exist yet.
// Build the frontend first: cd web && npm run build
//
//go:embed all:dist
var Assets embed.FS
