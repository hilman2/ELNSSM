// Package buildinfo holds version metadata that is injected at build time
// via ldflags. Importing this package gives commands access to a single
// source of truth for the running binary's version, commit and build date.
package buildinfo

// Set at build time via ldflags.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)
