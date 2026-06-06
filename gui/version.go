package gui

// Build-time values injected via -ldflags. Version is also used as the
// default image fetcher User-Agent.
var (
	Version = "dev"
	Commit  = "unknown"
)
