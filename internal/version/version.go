package version

// Version, Commit, and Date are populated by release build flags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info contains the build metadata presented by the CLI and Studio API.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Current returns the build metadata compiled into this binary.
func Current() Info {
	return Info{Version: Version, Commit: Commit, Date: Date}
}
