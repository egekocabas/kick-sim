package assets

import "embed"

// Files contains the versioned event contracts, built-in scenarios, and
// compatibility provenance shipped with the executable.
//
//go:embed events scenarios suites compatibility
var Files embed.FS
