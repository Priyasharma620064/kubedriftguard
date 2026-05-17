package version

// Build information set at compile time via ldflags.
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)
