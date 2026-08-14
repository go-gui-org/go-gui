package gui

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

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
	mouseButtonHeld          MouseButton
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
