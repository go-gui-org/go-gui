package gui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-gui-org/go-glyph"
)

// TextMeasurer measures text dimensions. Set by the backend
// after initialization; nil in tests (placeholder fallback).
type TextMeasurer interface {
	TextWidth(text string, style TextStyle) float32
	TextHeight(text string, style TextStyle) float32
	FontHeight(style TextStyle) float32
	FontAscent(style TextStyle) float32
	// LayoutText uses wrapWidth > 0 for wrap-enabled block width and
	// wrapWidth < 0 for width-constrained no-wrap alignment/layout.
	LayoutText(text string, style TextStyle, wrapWidth float32) (glyph.Layout, error)
}

// FontLister is an optional capability on TextMeasurer backends.
// Backends that wrap a *glyph.TextSystem opt in; others need no change.
type fontLister interface {
	ListFontFamilies() []string
}

// ListSystemFonts returns font family names from the active text
// system's catalog, sorted case-insensitively. Returns nil for a nil
// window, before backend init, on WASM stub backends, or on backends
// that do not implement FontLister. Includes RegisterAppFont families
// once those paths have been added to the text system.
func ListSystemFonts(w *Window) []string {
	if w == nil {
		return nil
	}
	if fl, ok := w.textMeasurer.(fontLister); ok {
		return fl.ListFontFamilies()
	}
	return nil
}

// windowRender holds render-walk state reset each frame.
type windowRender struct {
	// Renderers — flat draw command list, reused via [:0].
	renderers []RenderCmd
	// Clip radius propagated during render walk.
	clipRadius float32
	// Stencil depth for nested ClipContents.
	stencilDepth uint8
	// Nesting guard for filter brackets.
	inFilter bool
	// Render guard — warnings emitted once per kind (bitmask over RenderKind).
	renderGuardWarned uint32
}

// windowAnimation holds animation lifecycle state.
type windowAnimation struct {
	animMu sync.Mutex // guards animations, animViewBound
	// Active animations keyed by ID.
	animations map[string]Animation
	// View-bound animation heartbeats: animID → last-seen UnixNano.
	// Nil until first view-bound animation is registered.
	animViewBound map[string]int64
	// Animation loop lifecycle.
	animationStop      chan struct{}
	animationDone      chan struct{}
	animationResumeCh  chan struct{} // buffered(1), resumes ticker
	animationStopOnce  sync.Once
	animationStartOnce sync.Once
	animationStarted   bool
	// Per-frame pipeline timings.
	frameTimings FrameTimings
}

// windowBackend holds backend-injected dependencies. All fields
// are set once at init by the backend and nil in tests.
type windowBackend struct {
	textMeasurer   TextMeasurer
	svgParser      SvgParser
	nativePlatform NativePlatform
	clipboardSetFn func(string)
	clipboardGetFn func() string
	// primarySetFn/primaryGetFn drive the X11 PRIMARY selection — the
	// implicit, select-to-copy / middle-click-to-paste buffer that is
	// independent of CLIPBOARD. Only the X11 backend wires these; every
	// other platform leaves them nil, so GetPrimary yields "" there.
	primarySetFn func(string)
	primaryGetFn func() string
	// setTitleFn updates the OS window title. Set by backend; nil-safe.
	setTitleFn func(string)
	// wakeMainFn wakes the main thread from WaitEventTimeout.
	// Set by backend; nil-safe.
	wakeMainFn func()
}

// windowToast holds toast notification state.
type windowToast struct {
	toasts       []toastNotification
	toastCounter uint64
}

// windowInspector holds dev-tools inspector state.
type windowInspector struct {
	inspectorPropsCache map[string]inspectorNodeProps
	inspectorTreeCache  []TreeNodeCfg
	inspectorEnabled    bool
}

// Window is the main application window — the root container for all UI
// state, layout, and rendering.
//
// # Lifecycle
//
// Created via [NewWindow], which accepts a [WindowCfg] with the initial
// view generator, user state, and window properties. The window is then
// handed to a backend for the event loop:
//
//	metal.Run(w)      // Metal-only (macOS)
//	gl.Run(w)         // OpenGL-only
//
// During the event loop, the backend calls the view generator each frame
// to produce a [Layout] tree, which is sized, positioned, and rendered.
// OnInit fires once after the first frame. WindowCleanup fires on close.
//
// # Goroutine model
//
// The backend runs the event loop on the main thread (OS requirement for
// most GUI frameworks). View functions and event callbacks execute on the
// calling goroutine — typically the main thread. View functions run
// without [Window.mu] held so the animation goroutine can tick during
// slow View generation. Layout and render phases hold [Window.mu].
// Use [Window.Ctx] for async operations that should abort on window close.
//
// # Key subsystems
//
//   - [Window.State] / [State] — typed per-window user data
//   - [Window.Now] — virtual-clock-aware time (supports time-travel debug)
//   - [Window.UpdateView] — request a full rebuild next frame
//   - [Window.SetTitle] — update the OS window title
//   - [Window.Close] — request window close (safe from any goroutine)
//   - [Window.Backend] — access text measurement, clipboard, native dialogs
type Window struct {
	a11y a11y // Accessibility backend state.
	windowBackend
	windowInspector

	// File access / security-scoped bookmarks.
	fileAccess fileAccessState

	// User state — accessed via State[T](w).
	state any

	// Lifecycle context — cancelled in WindowCleanup to abort
	// in-flight async goroutines (HTTP fetches, notifications, etc.).
	ctx context.Context

	// Multi-window: parent App and platform window ID.
	app *App

	// View generator — produces the root View each frame.
	viewGenerator func(*Window) View

	// OnEvent is called for unhandled events. Nil-safe.
	OnEvent func(*Event, *Window)

	cancelCtx context.CancelFunc

	// Virtual clock — nil means live (time.Now). Non-nil means
	// Now() returns the stored instant. Set by time-travel scrub
	// so views that read w.Now() render with a past timestamp.
	virtualNow atomic.Pointer[time.Time]

	// Time-travel history. nil when disabled; hot-path checks
	// against nil to short-circuit with zero overhead. When
	// frozen is true, EventFn drops events (scrub read-only).
	history *snapshotRing

	// View state.
	viewState ViewState

	// Config is the WindowCfg passed to NewWindow. Read-only after init.
	// Backends read Title, Width, Height, and other properties from this.
	Config WindowCfg

	// Layout tree — current frame.
	layout Layout

	// Command queue — flushed at frame start.
	commands []queuedCommand

	// Command registry — registered commands for shortcut
	// dispatch, menu/button integration.
	cmdRegistry []Command

	// Scratch queue used to avoid reallocating command storage each frame.
	commandScratch []queuedCommand

	scratch scratchPools // Reusable per-frame scratch buffers.

	// Cached BoundedMap pointers for hot StateMap namespaces.
	// Bypasses StateRegistry map[string]any lookup + type assertion
	// on per-shape calls in the layout pipeline. Nil until first use;
	// lazily allocated by accessor methods. See §5 in
	// docs/specs/perf-optimizations.md.
	hoverInsideMap *BoundedMap[string, bool]
	scrollXMap     *BoundedMap[string, float32]
	scrollYMap     *BoundedMap[string, float32]
	overflowMap    *BoundedMap[string, int]

	// idJoinCache memoizes (scope, leaf) -> joined identity, so the one
	// allocation a join costs is paid once per distinct identity rather
	// than once per widget per frame. See (*Window).joinLeaf.
	idJoinCache *BoundedMap[idJoinKey, string]

	// idScopeStack is the ancestor stack resolveShapeIDs walks with,
	// kept here so its backing array is reused frame to frame rather
	// than allocated per pipeline root. See gui/id_resolve.go.
	idScopeStack []idFrame

	// scrollSmooth eases discrete mouse-wheel scrolling toward a
	// target offset. Guarded by animMu (see gui/scroll_smooth.go).
	scrollSmooth *scrollSmoothAnimation

	// scrollAnchors holds pending one-shot scroll-anchoring requests,
	// consumed by the layout pipeline. See Window.ScrollAnchor.
	scrollAnchors []scrollAnchor

	windowToast

	// Embedded concern groups.
	windowRender
	ime ime // Input Method Editor state.

	// Dialog state.
	dialogCfg DialogCfg

	// nativeDialogVisible is true while a native (OS) modal dialog is
	// showing. Native dialogs block in runModal and never touch
	// dialogCfg, so this flag lets DialogIsVisible — and the quit/close
	// dedup that relies on it — see them too. Set/cleared on the command
	// goroutine around the blocking platform call (see native_dialog.go).
	nativeDialogVisible bool

	windowAnimation

	// Window dimensions (logical pixels).
	windowWidth  int
	windowHeight int

	// Frame counter — incremented each FrameFn call, stamped
	// on events for frame-based timing (double-click detection).
	frameCount uint64

	// Cleanup guard.
	cleanupOnce sync.Once

	// Mutexes.
	mu         sync.Mutex // guards layout/renderer state
	commandsMu sync.Mutex // guards command queue

	platformID uint32
	closeReq   atomic.Bool

	// BackingScale is the device pixel ratio set by the backend each frame
	// (e.g. 2.0 on Retina/HiDPI). Zero until the first frame is rendered.
	BackingScale float32

	frozen atomic.Bool

	// Refresh flags.
	refreshLayout     bool
	refreshRenderOnly bool

	// pumping guards PumpFrame against re-entry: a nested platform
	// runloop can fire its frame timer again while the previous pump
	// is still inside FrameFn (a command callback that itself spins a
	// runloop, for instance). Main-thread only — no atomic needed.
	pumping bool

	// Window focus state — backend sets false on unfocus event.
	focused bool

	// debug is warn-once state for the dev-mode diagnostics in
	// debug.go. Untouched unless Debug is on.
	debug debugState
}

// MouseLockCfg stores callbacks for mouse event handling in a
// locked state (drag operations). When mouse is locked, these
// callbacks intercept normal mouse event processing.
type MouseLockCfg struct {
	// No auto-consume: mouse lock already bypasses hit-testing and
	// normal propagation, so handled-marking is moot here. Note the
	// coordinates stay window-absolute, unlike the shape-relative
	// coords everywhere else.
	mouseDown func(EventCtx)
	MouseMove func(EventCtx)
	MouseUp   func(EventCtx)

	// Cancel unwinds a drag that ended without a button release —
	// the platform revoked mouse capture (a system modal, a lock
	// screen, another process grabbing capture) so no MouseUp will
	// ever arrive. Synthesising one is wrong: MouseUp *commits* a
	// dock drop or a reorder, and a cancellation must not.
	//
	// It takes *Window, not EventCtx, precisely because there is no
	// event to carry — the other callbacks all read ctx.Event.
	// Optional: MouseCancel unlocks either way, so only a widget
	// with state beyond the lock itself (a pending drop, a pressed
	// flag, a drag-scroll animation) needs one.
	Cancel func(*Window)

	CursorPos int
}

// ViewState holds per-window UI state.
// exportaudit:keep — collides with the window's viewState state field
type ViewState struct {
	gesture gestureState

	mouseLock     MouseLockCfg
	registry      stateRegistry
	markdownCache *BoundedMap[int64, []markdownBlock]
	diagramCache  *BoundedDiagramCache

	// RTF layout cache — avoids re-shaping unchanged content.
	rtfLayoutCache *BoundedMap[uint64, rtfLayoutEntry]
	tooltip        tooltipState

	// Markdown caches (lazy-init: nil until first use).
	markdownTheme     string
	rtfLayoutTheme    string
	diagramRequestSeq uint64
	focusID           string

	// idScope is the effective ID of the innermost ID-bearing shape
	// currently being generated. Maintained by generateViewLayout and
	// read by (*Window).EffID; empty outside the view phase.
	idScope                  string
	mousePosX                float32
	mousePosY                float32
	mouseCursor              MouseCursor
	inputCursorOn            atomic.Bool
	menuKeyNav               bool
	externalAPIWarningLogged bool
}

// State returns a typed pointer to the user-supplied state.
//
// Panics if the window holds a different state type. That is a
// programmer error discoverable on the first frame, not a runtime
// condition worth threading through every view function.
func State[T any](w *Window) *T {
	s, ok := w.state.(*T)
	if !ok {
		var want *T
		panic(fmt.Sprintf(
			"gui: State[%T] requested but window holds %T", want, w.state))
	}
	return s
}

// SetState sets the user state for the window.
func (w *Window) setState(state any) {
	w.state = state
}

// Ctx returns the window's lifecycle context. The context is
// cancelled when WindowCleanup runs. Use for async operations
// that should abort on window destruction.
func (w *Window) Ctx() context.Context {
	if w.ctx == nil {
		return context.Background()
	}
	return w.ctx
}

// clearViewState resets all view state.
func (w *Window) clearViewState() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.clearViewStateLocked()
}

// clearViewStateLocked resets view state. Caller must hold w.mu.
func (w *Window) clearViewStateLocked() {
	w.viewState.registry.Clear()
	w.clearHotMaps()
	w.viewState.focusID = ""
}

// ClearDrawCanvasCache drops all cached tessellation data,
// forcing every DrawCanvas widget to re-render next frame.
func (w *Window) ClearDrawCanvasCache() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.viewState.registry.clearNamespace(nsDrawCanvas)
}

// Lock locks the window's mutex.
func (w *Window) Lock() {
	w.mu.Lock()
}

// Unlock unlocks the window's mutex.
func (w *Window) Unlock() {
	w.mu.Unlock()
}

// WindowSize returns cached window dimensions.
func (w *Window) WindowSize() (int, int) {
	return w.windowWidth, w.windowHeight
}

// windowRect returns the window as a drawClip.
func (w *Window) windowRect() drawClip {
	return drawClip{
		X: 0, Y: 0,
		Width:  float32(w.windowWidth),
		Height: float32(w.windowHeight),
	}
}

// PointerOverApp returns true if the mouse pointer is within
// the application window bounds.
func (w *Window) pointerOverApp(e *Event) bool {
	return e.MouseX >= 0 && e.MouseY >= 0 &&
		e.MouseX <= float32(w.windowWidth) &&
		e.MouseY <= float32(w.windowHeight)
}

// clearInputSelections zeros SelectBeg/SelectEnd for all
// input states.
func (w *Window) clearInputSelections() {
	imap := StateMapRead[string, inputState](w, nsInput)
	if imap == nil {
		return
	}
	imap.Range(func(key string, v inputState) bool {
		v.selectBeg = 0
		v.selectEnd = 0
		imap.Set(key, v)
		return true
	})
}

// inputCursorOn returns the input cursor blink state.
func (w *Window) inputCursorOn() bool {
	return w.viewState.inputCursorOn.Load()
}

// MouseIsLocked returns true if the mouse is locked (drag).
func (w *Window) mouseIsLocked() bool {
	ml := &w.viewState.mouseLock
	return ml.mouseDown != nil ||
		ml.MouseMove != nil || ml.MouseUp != nil
}

// MouseLock locks the mouse so all mouse events go to the
// handlers in MouseLockCfg.
func (w *Window) MouseLock(cfg MouseLockCfg) {
	w.viewState.mouseLock = cfg
}

// MouseUnlock returns mouse handling events to normal behavior.
func (w *Window) MouseUnlock() {
	w.viewState.mouseLock = MouseLockCfg{}
}

// MouseCancel aborts an in-flight drag: it unlocks the mouse and
// runs the lock's Cancel hook, if any. Backends call it when the
// platform takes mouse capture away without delivering a button
// release, which would otherwise leave the lock in place forever —
// every later move keeps driving the drag with no button held.
//
// No-op when the mouse is not locked. The lock is cleared before
// Cancel runs, so a hook that calls MouseUnlock itself (the
// escape-key cancel paths do) stays correct.
func (w *Window) MouseCancel() {
	if !w.mouseIsLocked() {
		return
	}
	cancel := w.viewState.mouseLock.Cancel
	w.MouseUnlock()
	if cancel != nil {
		cancel(w)
	}
	w.UpdateWindow()
}

// SetTextMeasurer sets the text measurement backend.
func (w *Window) SetTextMeasurer(tm TextMeasurer) {
	w.textMeasurer = tm
}

// TextMeasurer returns the window's text measurement backend, or nil
// if none has been set (e.g. headless tests without a backend).
func (w *Window) TextMeasurer() TextMeasurer {
	return w.textMeasurer
}

// FrameCount returns the monotonic frame counter for this window.
// Incremented once per FrameFn call. Useful for widgets that need
// to detect whether a callback is being invoked multiple times
// within the same render cycle. Must be called from the main thread;
// not safe for concurrent use.
func (w *Window) FrameCount() uint64 {
	return w.frameCount
}

// SetWakeMainFn sets the function called to wake the main event
// loop from WaitEventTimeout. The backend sets this at init time.
func (w *Window) SetWakeMainFn(fn func()) {
	w.wakeMainFn = fn
}

// TextWidth measures the rendered width of text for the supplied style.
// When no backend measurer is available, it uses the same approximation
// as text layout generation.
func (w *Window) TextWidth(text string, style TextStyle) float32 {
	if style.Size == 0 {
		style.Size = sizeTextMedium
	}
	if w == nil || w.textMeasurer == nil {
		return float32(utf8RuneCount(text)) * style.Size * 0.6
	}
	return w.textMeasurer.TextWidth(text, style)
}

// allocShape returns a pooled *Shape initialized to src. The
// pointer is valid until the next frame's view-phase pool reset.
// Falls back to a heap allocation when w has no pool (tests).
func (w *Window) allocShape(src Shape) *Shape {
	if w == nil {
		cp := src
		return &cp
	}
	return w.scratch.viewShapes.alloc(src)
}

// allocEventHandlers returns a pooled *eventHandlers initialized
// to src. Pointer valid until the next view-phase pool reset.
func (w *Window) allocEventHandlers(src eventHandlers) *eventHandlers {
	if w == nil {
		cp := src
		return &cp
	}
	return w.scratch.viewEvents.alloc(src)
}

// allocEffects returns a pooled *shapeEffects initialized to src.
// Pointer valid until the next view-phase pool reset.
func (w *Window) allocEffects(src shapeEffects) *shapeEffects {
	if w == nil {
		cp := src
		return &cp
	}
	return w.scratch.viewEffects.alloc(src)
}

// SetClipboardFn sets the function used to copy text to the clipboard.
func (w *Window) SetClipboardFn(fn func(string)) {
	w.clipboardSetFn = fn
}

// SetClipboard copies text to the system clipboard.
func (w *Window) SetClipboard(text string) {
	if w.clipboardSetFn != nil {
		w.clipboardSetFn(text)
	}
}

// SetClipboardGetFn sets the function used to read from the clipboard.
func (w *Window) SetClipboardGetFn(fn func() string) {
	w.clipboardGetFn = fn
}

// GetClipboard returns text from the system clipboard.
func (w *Window) GetClipboard() string {
	if w.clipboardGetFn != nil {
		return w.clipboardGetFn()
	}
	return ""
}

// SetPrimaryFn sets the function used to publish text as the PRIMARY
// selection.
func (w *Window) SetPrimaryFn(fn func(string)) {
	w.primarySetFn = fn
}

// SetPrimary publishes text as the X11 PRIMARY selection — the buffer filled
// by selecting text and pasted with the middle mouse button. It is entirely
// separate from the clipboard, so SetPrimary does not disturb whatever
// SetClipboard last stored. A no-op on platforms without PRIMARY (macOS,
// Windows, web, mobile).
func (w *Window) SetPrimary(text string) {
	if w.primarySetFn != nil {
		w.primarySetFn(text)
	}
}

// SetPrimaryGetFn sets the function used to read the PRIMARY selection.
func (w *Window) SetPrimaryGetFn(fn func() string) {
	w.primaryGetFn = fn
}

// GetPrimary returns the current X11 PRIMARY selection, or "" on platforms
// that have no such concept. Callers wanting a paste that works everywhere
// should fall back to GetClipboard on an empty result.
func (w *Window) GetPrimary() string {
	if w.primaryGetFn != nil {
		return w.primaryGetFn()
	}
	return ""
}

// SetTitleFn sets the function used to update the OS window title.
// Called by the backend at init.
func (w *Window) SetTitleFn(fn func(string)) {
	w.setTitleFn = fn
}

// maxTitleBytes caps SetTitle input to bound per-call allocation
// cost (the backend copies to a C string). Real window titles are
// rarely over ~100 bytes; 4 KiB is generous and forgiving.
const maxTitleBytes = 4096

// SetTitle updates the OS window title and Config.Title. No-op if
// the backend has not wired a title function (e.g. headless tests).
// Input is truncated to maxTitleBytes and stripped of embedded NUL
// bytes (which would silently cut the title in C.CString). Must be
// called from the main thread; window title updates are not
// thread-safe on macOS.
func (w *Window) SetTitle(title string) {
	title = sanitizeTitle(title)
	w.Config.Title = title
	if w.setTitleFn != nil {
		w.setTitleFn(title)
	}
}

// sanitizeTitle truncates overlong titles and strips NUL bytes.
func sanitizeTitle(title string) string {
	if len(title) > maxTitleBytes {
		// Truncate on a valid UTF-8 boundary to avoid producing
		// invalid sequences.
		cut := maxTitleBytes
		for cut > 0 && (title[cut]&0xC0) == 0x80 {
			cut--
		}
		title = title[:cut]
	}
	if strings.IndexByte(title, 0) < 0 {
		return title
	}
	// Rare path: strip NUL bytes.
	b := make([]byte, 0, len(title))
	for i := 0; i < len(title); i++ {
		if title[i] != 0 {
			b = append(b, title[i])
		}
	}
	return string(b)
}

// Renderers returns the current render command slice.
func (w *Window) Renderers() []RenderCmd {
	return w.renderers
}

// Timings returns the most recent frame's pipeline timings.
func (w *Window) Timings() FrameTimings { return w.frameTimings }

// MouseCursorState returns the current mouse cursor shape.
func (w *Window) MouseCursorState() MouseCursor {
	return w.viewState.mouseCursor
}

// SetTheme sets the active theme and updates the window.
func (w *Window) SetTheme(t Theme) {
	SetTheme(t)
	w.UpdateWindow()
}

// App returns the parent App, or nil for single-window mode.
func (w *Window) App() *App { return w.app }

// PlatformID returns the platform-native window ID (0 if not yet registered).
func (w *Window) PlatformID() uint32 { return w.platformID }

// Close requests the window be closed on the next frame.
// Safe to call from any goroutine.
func (w *Window) Close() { w.closeReq.Store(true) }

// Now returns the window's current time. When live, this is
// time.Now(). When time-travel scrub has pinned a virtual
// instant, Now returns that instant so views relying on
// clock-driven rendering (elapsed counters, "N seconds ago"
// labels) match the scrubbed snapshot. Views that need the
// scrubbed clock should call w.Now() instead of time.Now().
// Safe to call from any goroutine.
func (w *Window) Now() time.Time {
	if w == nil {
		return time.Now()
	}
	if t := w.virtualNow.Load(); t != nil {
		return *t
	}
	return time.Now()
}

// setVirtualNow pins the window's virtual clock to t. Passing
// nil clears the pin and restores live time.Now(). Intended
// for time-travel scrub internals; not part of the public API.
// Safe to call from any goroutine.
func (w *Window) setVirtualNow(t *time.Time) {
	w.virtualNow.Store(t)
}

// CloseRequested returns true if Close() was called.
func (w *Window) CloseRequested() bool { return w.closeReq.Load() }
