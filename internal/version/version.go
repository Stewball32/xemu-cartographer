// Package version exposes build-time version information.
// Values are overridden via -ldflags at build time (see Taskfile.yml).
// See docs/decisions/0001-version-from-git-tag.md for the rationale.
package version

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)
