//go:build linux && !android && (amd64 || arm64)

package decor

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// libdecor entry points, resolved by Load.
var (
	fnNew                    uintptr
	fnUnref                  uintptr
	fnDispatch               uintptr
	fnDecorate               uintptr
	fnFrameUnref             uintptr
	fnFrameSetTitle          uintptr
	fnFrameSetAppID          uintptr
	fnFrameSetMinContentSize uintptr
	fnFrameSetMaxContentSize uintptr
	fnFrameMove              uintptr
	fnFrameResize            uintptr
	fnFrameCommit            uintptr
	fnFrameMap               uintptr
	fnStateNew               uintptr
	fnStateFree              uintptr
	fnConfigContentSize      uintptr
	fnConfigWindowState      uintptr

	loadOnce sync.Once
	loadErr  error
)

// Window state bits the backend reads (enum libdecor_window_state).
// StateActive marks a focused window. StateSuspended (libdecor 0.2, over
// xdg_toplevel v6) marks one the compositor shows nowhere: minimized, on
// another workspace, or behind a locked screen.
const (
	StateActive    = 1 << 0
	StateSuspended = 1 << 7
)

// Resize edges (enum libdecor_resize_edge). Not the xdg_toplevel values.
const (
	EdgeNone = iota
	EdgeTop
	EdgeBottom
	EdgeLeft
	EdgeTopLeft
	EdgeBottomLeft
	EdgeRight
	EdgeTopRight
	EdgeBottomRight
)

// Load opens libdecor. It runs once; later calls return the first result.
func Load() error {
	loadOnce.Do(func() { loadErr = load() })
	return loadErr
}

func load() error {
	lib, err := purego.Dlopen("libdecor-0.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return fmt.Errorf("dlopen libdecor-0.so.0: %w", err)
	}
	for _, s := range []struct {
		fn   *uintptr
		name string
	}{
		{&fnNew, "libdecor_new"},
		{&fnUnref, "libdecor_unref"},
		{&fnDispatch, "libdecor_dispatch"},
		{&fnDecorate, "libdecor_decorate"},
		{&fnFrameUnref, "libdecor_frame_unref"},
		{&fnFrameSetTitle, "libdecor_frame_set_title"},
		{&fnFrameSetAppID, "libdecor_frame_set_app_id"},
		{&fnFrameSetMinContentSize, "libdecor_frame_set_min_content_size"},
		{&fnFrameSetMaxContentSize, "libdecor_frame_set_max_content_size"},
		{&fnFrameMove, "libdecor_frame_move"},
		{&fnFrameResize, "libdecor_frame_resize"},
		{&fnFrameCommit, "libdecor_frame_commit"},
		{&fnFrameMap, "libdecor_frame_map"},
		{&fnStateNew, "libdecor_state_new"},
		{&fnStateFree, "libdecor_state_free"},
		{&fnConfigContentSize, "libdecor_configuration_get_content_size"},
		{&fnConfigWindowState, "libdecor_configuration_get_window_state"},
	} {
		addr, symErr := purego.Dlsym(lib, s.name)
		if symErr != nil || addr == 0 {
			return fmt.Errorf("libdecor has no %s: %v", s.name, symErr)
		}
		*s.fn = addr
	}
	// The callbacks are made once: purego never frees one, and there
	// are only so many.
	cbError = purego.NewCallback(onError)
	frameIface[0] = purego.NewCallback(onConfigure)
	frameIface[1] = purego.NewCallback(onClose)
	frameIface[2] = purego.NewCallback(onCommit)
	frameIface[3] = purego.NewCallback(onDismissPopup)
	ctxIface[0] = cbError
	return nil
}

// The interface structs libdecor keeps pointers to: a few callbacks, then
// ten reserved slots, which must be NULL. Package globals, so their
// address never changes.
var (
	ctxIface   [11]uintptr // struct libdecor_interface
	frameIface [14]uintptr // struct libdecor_frame_interface
	cbError    uintptr
)

// Handler gets a frame's events.
type Handler interface {
	// Configure is a new configuration. The handler must answer it with
	// Frame.Commit, from inside the call.
	Configure(c Configuration)
	// Close is the user closing the window.
	Close()
	// Commit asks for the main surface to be committed: the frame
	// changed and is shown only with it.
	Commit()
}

// frames maps a frame's user data (an id, not a pointer: libdecor stores
// it in C memory) to its Handler.
var (
	frames  = map[uintptr]Handler{}
	frameID uintptr
)

// panicked holds a panic raised by a Handler. A Go panic must not unwind
// through libdecor's C frames, so the callback recovers it and Rethrow
// raises it again once the C code has returned.
var panicked any

// Rethrow raises a panic a Handler raised inside a C call. The backend
// calls it after a dispatch that may run libdecor callbacks.
func Rethrow() {
	if p := panicked; p != nil {
		panicked = nil
		panic(p)
	}
}

func guard() {
	if r := recover(); r != nil && panicked == nil {
		panicked = r
	}
}

// ErrorFunc gets libdecor's error events. The default does nothing; the
// backend logs them.
var ErrorFunc = func(code int, msg string) {}

func onError(_ uintptr, code uintptr, msg uintptr) {
	defer guard()
	ErrorFunc(int(int32(code)), goString(msg))
}

func onConfigure(frame, config, user uintptr) {
	defer guard()
	if h := frames[user]; h != nil {
		h.Configure(Configuration{ptr: config, frame: frame})
	}
}

func onClose(_, user uintptr) {
	defer guard()
	if h := frames[user]; h != nil {
		h.Close()
	}
}

func onCommit(_, user uintptr) {
	defer guard()
	if h := frames[user]; h != nil {
		h.Commit()
	}
}

// onDismissPopup is for clients with popups holding a grab; the backend
// makes none.
func onDismissPopup(_, _, _ uintptr) {}

// Context is a libdecor instance on one Wayland connection.
type Context struct{ ptr uintptr }

// New makes a libdecor instance on display, a wl_display*. It loads the
// best plugin it finds (GTK, or cairo), which may connect to the
// compositor on its own.
func New(display uintptr) (*Context, error) {
	if err := Load(); err != nil {
		return nil, err
	}
	r, _, _ := purego.SyscallN(fnNew, display, uintptr(unsafe.Pointer(&ctxIface)))
	Rethrow()
	if r == 0 {
		return nil, errors.New("libdecor_new failed")
	}
	return &Context{ptr: r}, nil
}

// Unref frees the instance. Every frame must be gone first.
func (c *Context) Unref() {
	if c == nil || c.ptr == 0 {
		return
	}
	purego.SyscallN(fnUnref, c.ptr)
	c.ptr = 0
}

// Dispatch runs the plugin's own work (the GTK plugin's main context)
// and dispatches the connection's events, waiting at most timeoutMs. 0
// never blocks.
func (c *Context) Dispatch(timeoutMs int) error {
	r, _, _ := purego.SyscallN(fnDispatch, c.ptr, uintptr(int32(timeoutMs)))
	Rethrow()
	if int32(r) < 0 {
		return fmt.Errorf("libdecor_dispatch: %d", int32(r))
	}
	return nil
}

// Frame is a decorated toplevel.
type Frame struct {
	ptr uintptr
	id  uintptr
}

// Decorate makes surface (a wl_surface*) a decorated toplevel whose
// events go to h. Show it with Map once its title and limits are set.
func (c *Context) Decorate(surface uintptr, h Handler) (*Frame, error) {
	frameID++
	id := frameID
	frames[id] = h
	r, _, _ := purego.SyscallN(fnDecorate, c.ptr, surface, uintptr(unsafe.Pointer(&frameIface)), id)
	Rethrow()
	if r == 0 {
		delete(frames, id)
		return nil, errors.New("libdecor_decorate failed")
	}
	return &Frame{ptr: r, id: id}, nil
}

// Unref destroys the frame and its xdg objects. Safe to call twice.
func (f *Frame) Unref() {
	if f == nil || f.ptr == 0 {
		return
	}
	purego.SyscallN(fnFrameUnref, f.ptr)
	delete(frames, f.id)
	f.ptr = 0
}

// SetTitle sets the title the frame shows.
func (f *Frame) SetTitle(title string) {
	b := cString(title)
	purego.SyscallN(fnFrameSetTitle, f.ptr, uintptr(unsafe.Pointer(&b[0])))
	runtime.KeepAlive(b)
}

// SetAppID sets the application id (the .desktop file name).
func (f *Frame) SetAppID(id string) {
	b := cString(id)
	purego.SyscallN(fnFrameSetAppID, f.ptr, uintptr(unsafe.Pointer(&b[0])))
	runtime.KeepAlive(b)
}

// SetMinContentSize bounds the content size from below; 0 is no bound.
func (f *Frame) SetMinContentSize(w, h int32) {
	purego.SyscallN(fnFrameSetMinContentSize, f.ptr, uintptr(w), uintptr(h))
}

// SetMaxContentSize bounds the content size from above; 0 is no bound.
func (f *Frame) SetMaxContentSize(w, h int32) {
	purego.SyscallN(fnFrameSetMaxContentSize, f.ptr, uintptr(w), uintptr(h))
}

// Move starts an interactive move. seat is a wl_seat*; serial names the
// button press that began it.
func (f *Frame) Move(seat uintptr, serial uint32) {
	purego.SyscallN(fnFrameMove, f.ptr, seat, uintptr(serial))
}

// Resize starts an interactive resize from edge (EdgeTop...).
func (f *Frame) Resize(seat uintptr, serial uint32, edge uint32) {
	purego.SyscallN(fnFrameResize, f.ptr, seat, uintptr(serial), uintptr(edge))
}

// Map shows the frame. It commits the surface, which asks for the first
// configure.
func (f *Frame) Map() {
	purego.SyscallN(fnFrameMap, f.ptr)
	Rethrow()
}

// Commit applies a content size: in reply to a configuration (c non-nil),
// or on the client's own resize of a floating window (c nil).
func (f *Frame) Commit(w, h int32, c *Configuration) {
	st, _, _ := purego.SyscallN(fnStateNew, uintptr(w), uintptr(h))
	if st == 0 {
		return
	}
	var cfg uintptr
	if c != nil {
		cfg = c.ptr
	}
	purego.SyscallN(fnFrameCommit, f.ptr, st, cfg)
	purego.SyscallN(fnStateFree, st)
	Rethrow()
}

// Configuration is a configure event. Valid only inside
// Handler.Configure.
type Configuration struct{ ptr, frame uintptr }

// ContentSize is the size the compositor asks for. ok is false when it
// leaves the size to the client.
func (c Configuration) ContentSize() (w, h int32, ok bool) {
	var cw, ch int32
	r, _, _ := purego.SyscallN(fnConfigContentSize, c.ptr, c.frame,
		uintptr(unsafe.Pointer(&cw)), uintptr(unsafe.Pointer(&ch)))
	if byte(r) == 0 || cw <= 0 || ch <= 0 {
		return 0, 0, false
	}
	return cw, ch, true
}

// WindowState is the window state (StateActive...). ok is false when the
// configuration has none and the last one still holds.
func (c Configuration) WindowState() (state uint32, ok bool) {
	var s uint32
	r, _, _ := purego.SyscallN(fnConfigWindowState, c.ptr, uintptr(unsafe.Pointer(&s)))
	return s, byte(r) != 0
}

// cString is s with a NUL, with any NUL inside cut off.
func cString(s string) []byte {
	for i := 0; i < len(s); i++ {
		if s[i] == 0 {
			s = s[:i]
			break
		}
	}
	b := make([]byte, len(s)+1)
	copy(b, s)
	return b
}

// goString copies a NUL-terminated C string; NULL gives "".
func goString(u uintptr) string {
	if u == 0 {
		return ""
	}
	p := *(*unsafe.Pointer)(unsafe.Pointer(&u))
	n := 0
	for *(*byte)(unsafe.Add(p, n)) != 0 && n < 4096 {
		n++
	}
	return string(unsafe.Slice((*byte)(p), n))
}
