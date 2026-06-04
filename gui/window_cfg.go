package gui

import "context"

// WindowCfg configures a new Window.
type WindowCfg struct {
	State any
	// ImageFetcher, if non-nil, is the default fetcher for remote
	// images in this window. DrawContext.ImageWithFetcher can
	// override this per call (e.g. to pair each map tile layer with
	// its own source-specific User-Agent).
	ImageFetcher ImageFetcher
	OnInit       func(*Window)
	OnEvent      func(*Event, *Window)
	// OnCloseRequest runs when the OS reports a window-close (title
	// bar button, Cmd-W, etc.) before the window is destroyed. If
	// nil, the backend proceeds with destroy as before. If set, the
	// callback owns the decision: call Window.Close() to proceed, or
	// do nothing to cancel. Use for save/discard/cancel prompts.
	// Re-clicking the close control is required to retry after a veto
	// since the original SDL event is already drained.
	OnCloseRequest func(*Window)
	Title          string
	// AllowedSvgRoots restricts file-based SVG loads to these paths.
	// Empty means allow any local SVG path.
	AllowedSvgRoots []string
	// AllowedImageRoots restricts file-based image loads to these
	// paths. Empty means allow any local image path. To render
	// remote http/https images (fetched via ResolveImageSrc), the
	// allowlist must include the download cache directory:
	// filepath.Join(os.TempDir(), "gui_cache", "images").
	AllowedImageRoots []string
	// IconPNG is optional PNG-encoded icon data for the window.
	// The backend sets this as the window icon when supported.
	IconPNG []byte
	Width   int
	Height  int
	// MaxImageBytes caps source image file size for decoded image
	// loads. Zero or negative selects backend defaults.
	MaxImageBytes int64
	// MaxImagePixels caps decoded image dimensions (width*height).
	// Zero or negative selects backend defaults.
	MaxImagePixels int64
	// MaxImageDownloads caps concurrent in-flight image downloads
	// across the process. Zero or negative selects a default of 6,
	// matching OSM's guidance for well-behaved map clients. The
	// first Window whose config is consulted fixes the limit for
	// the process lifetime — later Windows cannot resize it.
	MaxImageDownloads int
	// HistoryBytes caps time-travel snapshot memory. Evicts
	// oldest entries when exceeded. Zero or negative selects
	// a default (64 MiB). Only consulted when DebugTimeTravel
	// is true.
	HistoryBytes int
	BgColor      Color
	// FixedSize disables user-driven window resizing when supported
	// by the active backend.
	FixedSize bool
	// Timings enables per-frame pipeline timing instrumentation.
	Timings bool
	// DebugTimeTravel enables time-travel snapshot capture and
	// auto-spawns a scrubber window alongside the app window.
	// Requires multi-window mode (App + App.OpenWindow) and a
	// user state that implements the Snapshotter interface.
	// Leave off in release builds — the nil-history hot path
	// short-circuits with zero cost when disabled.
	DebugTimeTravel bool
}

// NewWindow creates a Window from the given configuration.
func NewWindow(cfg WindowCfg) *Window {
	ctx, cancel := context.WithCancel(context.Background())
	w := &Window{
		state:         cfg.State,
		windowWidth:   cfg.Width,
		windowHeight:  cfg.Height,
		focused:       true,
		refreshLayout: true,
		OnEvent:       cfg.OnEvent,
		Config:        cfg,
		scratch:       newScratchPools(),
		ctx:           ctx,
		cancelCtx:     cancel,
		windowAnimation: windowAnimation{
			animationStop:     make(chan struct{}),
			animationDone:     make(chan struct{}),
			animationResumeCh: make(chan struct{}, 1),
		},
	}
	if cfg.DebugTimeTravel {
		w.EnableHistory(cfg.HistoryBytes)
	}
	return w
}
