// Package buildinfo holds version metadata injected at build time via
// -ldflags (see Makefile).
package buildinfo

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)
