package gui

import (
	"context"

	"github.com/go-gui-org/go-gui/gui/appinfo"
)

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
	// since the original close event is already drained.
	OnCloseRequest func(*Window)
	// AppInfo is the app manifest, usually appinfo.toml embedded with
	// //go:embed and read with appinfo.MustParse. It is the source of
	// the app's identity at run time, and buildapp reads the same
	// file, so the bundle ID and the runtime ID cannot differ.
	// NewWindow fills only fields left empty: Title from Name, WMClass
	// and the file-access app ID from ID. The native menubar AppName
	// also defaults to Name. A zero AppInfo changes nothing.
	AppInfo appinfo.Info
	Title   string
	// AllowedSvgRoots restricts file-based SVG loads to these paths.
	// Empty means allow any local SVG path.
	// exportaudit:keep — caller-facing config (issue #372)
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
	// WMClass sets the X11 WM_CLASS property, used for both the
	// instance and class slots. Window managers group windows and
	// match .desktop files by it. Only the X11 backend reads it;
	// empty omits the property.
	WMClass string
	Width   int
	Height  int
	// MinWidth and MinHeight set the smallest size the user can drag
	// the window to, in the same logical pixels as Width and Height.
	// Zero means no floor. Honored by the macOS, Windows and X11
	// backends; ignored elsewhere. FixedSize wins over both.
	// exportaudit:keep — caller-facing config (issue #494)
	MinWidth int
	// exportaudit:keep — caller-facing config (issue #494)
	MinHeight int
	// MaxWidth and MaxHeight set the largest size the user can drag
	// the window to, in logical pixels. Zero means no ceiling. A
	// ceiling below its floor is raised to the floor. On Windows and
	// macOS the ceiling also caps the maximize button, not only the
	// drag. FixedSize wins over both, pinning the ceiling too.
	// exportaudit:keep — caller-facing config (issue #494)
	MaxWidth int
	// exportaudit:keep — caller-facing config (issue #494)
	MaxHeight int
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
	// exportaudit:keep — caller-facing config (issue #372)
	MaxImageDownloads int
	// HistoryBytes caps time-travel snapshot memory. Evicts
	// oldest entries when exceeded. Zero or negative selects
	// a default (64 MiB). Only consulted when DebugTimeTravel
	// is true.
	// exportaudit:keep — caller-facing config (issue #372)
	HistoryBytes int
	// BgColor is the color every frame is cleared with, behind all
	// rendered content. Unset takes the theme's background.
	//
	// The alpha channel is how much of the window is see-through, but
	// it only reaches the compositor on a Transparent window: on an
	// ordinary one the alpha is discarded and any value looks fully
	// opaque. Read every frame, so an app may assign it at runtime;
	// Transparent, by contrast, is fixed when the window is made.
	// exportaudit:keep — caller-facing config
	BgColor Color
	// Transparent makes the window's alpha channel reach the
	// compositor, so whatever is behind the window shows through
	// wherever the rendered content is not opaque.
	//
	// How see-through the window is comes from BgColor's alpha, not
	// from this flag: RGBA(0, 0, 0, 128) is a half-visible ground,
	// RGBA(0, 0, 0, 0) a fully clear one. An unset BgColor is treated
	// as fully transparent rather than taking the opaque theme
	// background, so the flag on its own is enough to see through.
	// Widgets keep whatever alpha their own colors carry — a solid
	// button stays solid.
	//
	// The flag stays separate from that alpha deliberately. It asks
	// the platform for a capability and can be refused, which BgColor
	// cannot, and it is answered once at creation, while BgColor is
	// read every frame.
	//
	// Creation-time only: the X11 visual and the Win32 pixel format
	// are fixed when the window is made, so there is no runtime
	// setter. This is plain transparency, not blur —
	// Window.SetWindowVibrancy is the separate macOS-only blur API.
	//
	// Honored by the macOS, Windows and X11 backends; ignored
	// elsewhere. On X11 it needs a running compositing manager. With
	// no compositor the window renders black, which gui.Debug
	// reports.
	Transparent bool
	// FixedSize disables user-driven window resizing when supported
	// by the active backend.
	FixedSize bool
	// Decorations selects the native window frame. The zero value
	// keeps the platform's standard title bar and border. A
	// DecorationNone window has nothing the user can grab, so it
	// needs Window.StartWindowDrag to stay movable. Honored by the
	// macOS, Windows and X11 backends; ignored elsewhere.
	Decorations WindowDecoration
	// VSyncOff presents frames without waiting for the display's
	// vertical refresh. The zero value keeps vsync on, which is right
	// for almost every app: frames never outrun the display and the
	// GPU stays quiet.
	//
	// It exists for renderer benchmarks. With vsync on, a benchmark
	// that redraws every frame reads the display refresh rate (60,
	// 120, 240 Hz) until a frame takes longer than one refresh, so it
	// cannot report raw throughput.
	//
	// It does not make an idle window draw. The loop still sleeps
	// while no refresh is pending, and animations still tick every
	// 16 ms. Frame rate climbs past the refresh rate only when the
	// app asks for a new frame every frame (InvalidateLayout in the
	// view function). Such an app then uses a full CPU core.
	//
	// Creation-time only. Honored by the Windows (WGL) and X11 (EGL)
	// backends; on X11 a driver setting such as vblank_mode or
	// __GL_SYNC_TO_VBLANK can still force vsync on. macOS, iOS,
	// Android and web pace frames with the display and ignore it:
	// on macOS the window compositor hands back a drawable once per
	// refresh, whatever the layer's display sync says. gui.Debug
	// reports when the flag was not honored.
	// exportaudit:keep — caller-facing config (issue #907)
	VSyncOff bool
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

// SimpleWindow is the thin form of NewWindow for the common case:
// title, size, state, and an OnInit that wires the root view. Any
// caller needing the rest of WindowCfg (OnCloseRequest, ImageFetcher,
// FixedSize, ...) uses NewWindow directly.
func SimpleWindow(title string, w, h int, state any, onInit func(*Window)) *Window {
	return NewWindow(WindowCfg{
		Title:  title,
		Width:  w,
		Height: h,
		State:  state,
		OnInit: onInit,
	})
}

// NewWindow creates a Window from the given configuration.
func NewWindow(cfg WindowCfg) *Window {
	// The manifest fills only what the caller left empty, so an app
	// can still title a window "Document 1 - Falcon".
	if cfg.Title == "" {
		cfg.Title = cfg.AppInfo.Name
	}
	// WM_CLASS matches the .desktop file buildapp names after the ID,
	// so the window groups under the installed launcher.
	if cfg.WMClass == "" {
		cfg.WMClass = cfg.AppInfo.ID
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Window{
		state:         cfg.State,
		windowWidth:   cfg.Width,
		windowHeight:  cfg.Height,
		windowOpacity: 1,
		focused:       true,
		OnEvent:       cfg.OnEvent,
		Config:        cfg,
		scratch:       newScratchPools(),
		ctx:           ctx,
		cancelCtx:     cancel,
		// mouseButtonHeld's zero value is MouseLeft (0), but the
		// resting state is "no button": hover synthesis must not
		// report a press no one made.
		viewState: ViewState{mouseButtonHeld: MouseInvalid},
		windowAnimation: windowAnimation{
			animationStop:     make(chan struct{}),
			animationDone:     make(chan struct{}),
			animationResumeCh: make(chan struct{}, 1),
		},
	}
	// A new window paints on its first frame. Seeded here, not in the
	// literal above: an atomic takes no literal (see window.go).
	w.markLayoutRefresh(refreshInitial)
	// No lock: w is not shared yet. SetFileAccessAppID still replaces it.
	w.fileAccess.appID = cfg.AppInfo.ID
	if cfg.DebugTimeTravel {
		w.enableHistory(cfg.HistoryBytes)
	}
	// Tracked so a later package-level SetTheme can repaint the windows
	// that follow the app default. Dropped again in WindowCleanup.
	registerWindow(w)
	return w
}

// FrameBackground is the color a backend clears the window with each
// frame. An unset BgColor normally takes the theme background, which is
// opaque; a Transparent window instead falls back to fully transparent,
// so the flag on its own is enough to see through the window.
//
// Backends call this after FrameFn on the same thread, so the installed
// theme happens to be right; w.Theme() makes it right by construction
// instead of by timing.
func (w *Window) FrameBackground() Color {
	if bg := w.Config.BgColor; bg != (Color{}) {
		return bg
	}
	if w.Config.Transparent {
		return ColorTransparent
	}
	// While a theme fade runs, w.Theme() is already the target; the
	// clear color must travel with the drawn colors (issue #753).
	if f := w.themeFade; f != nil && f.active {
		return f.scratch.ColorBackground
	}
	return w.Theme().ColorBackground
}
