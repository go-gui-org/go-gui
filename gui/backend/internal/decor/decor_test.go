//go:build linux && !android && (amd64 || arm64)

package decor

import (
	"os"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
	"golang.org/x/sys/unix"
)

type fakeHandler struct {
	configured, closed, committed int
	panicOn                       string
}

func (f *fakeHandler) Configure(Configuration) {
	f.configured++
	if f.panicOn == "configure" {
		panic("boom")
	}
}
func (f *fakeHandler) Close()  { f.closed++ }
func (f *fakeHandler) Commit() { f.committed++ }

func TestCallbacksRouteByID(t *testing.T) {
	a, b := &fakeHandler{}, &fakeHandler{}
	frames[101], frames[102] = a, b
	defer func() { delete(frames, 101); delete(frames, 102) }()

	onConfigure(0, 0, 101)
	onClose(0, 102)
	onCommit(0, 101)
	onCommit(0, 999) // unknown frame: ignored
	if a.configured != 1 || a.committed != 1 || a.closed != 0 || b.closed != 1 || b.configured != 0 {
		t.Fatalf("a %+v, b %+v", a, b)
	}
}

func TestCallbackPanicHeldForRethrow(t *testing.T) {
	h := &fakeHandler{panicOn: "configure"}
	frames[103] = h
	defer delete(frames, 103)
	// The callback returns normally, as it must into C...
	onConfigure(0, 0, 103)
	// ...and the panic comes out of Rethrow.
	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("Rethrow raised %v, want boom", r)
		}
		Rethrow() // cleared: no second panic
	}()
	Rethrow()
	t.Fatal("Rethrow did not panic")
}

func TestOnErrorMessage(t *testing.T) {
	var gotCode int
	var gotMsg string
	old := ErrorFunc
	ErrorFunc = func(code int, msg string) { gotCode, gotMsg = code, msg }
	defer func() { ErrorFunc = old }()
	msg := []byte("compositor incompatible\x00")
	onError(0, 1, uintptr(unsafe.Pointer(&msg[0])))
	if gotCode != 1 || gotMsg != "compositor incompatible" {
		t.Fatalf("got %d %q", gotCode, gotMsg)
	}
	onError(0, 2, 0)
	if gotMsg != "" {
		t.Fatalf("NULL message gave %q", gotMsg)
	}
}

func TestCString(t *testing.T) {
	for in, want := range map[string]string{"": "\x00", "abc": "abc\x00", "a\x00b": "a\x00"} {
		if got := string(cString(in)); got != want {
			t.Errorf("cString(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoad(t *testing.T) {
	if err := Load(); err != nil {
		t.Skip(err)
	}
	if ctxIface[0] == 0 || frameIface[0] == 0 || frameIface[3] == 0 {
		t.Fatal("callbacks not installed")
	}
	for i := 4; i < len(frameIface); i++ {
		if frameIface[i] != 0 {
			t.Fatalf("reserved slot %d is not NULL", i)
		}
	}
}

func TestDispatchErr(t *testing.T) {
	for _, c := range []struct {
		r    int32
		fail bool
	}{
		{0, false},
		{3, false},                     // events dispatched
		{-int32(syscall.EINTR), false}, // poll interrupted (#1194)
		{-int32(syscall.EPIPE), true},  // flush to a closed socket
		{-int32(syscall.EPROTO), true}, // protocol error
		{-1, true},                     // a dispatch_pending failure in the count
	} {
		if err := dispatchErr(c.r); (err != nil) != c.fail {
			t.Errorf("dispatchErr(%d) = %v, want failure %v", c.r, err, c.fail)
		}
	}
}

// TestDispatchInterrupted is #1194: every plugin's dispatch ends in a
// poll(), which a signal fails with EINTR. Go sends SIGURG to preempt
// goroutines, so a busy app gets one now and then. It needs a compositor:
// it skips without one, unless GOGUI_REQUIRE_WAYLAND=1
// (scripts/wayland/test.sh).
func TestDispatchInterrupted(t *testing.T) {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		if os.Getenv("GOGUI_REQUIRE_WAYLAND") == "1" {
			t.Fatal("GOGUI_REQUIRE_WAYLAND=1 but WAYLAND_DISPLAY is unset")
		}
		t.Skip("no WAYLAND_DISPLAY")
	}
	if err := Load(); err != nil {
		t.Skip(err)
	}
	// The signals go to the thread that calls Dispatch.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	conn, err := wl.Connect("")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	c, err := New(conn.Display.Ptr())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Unref()

	pid, tid := unix.Getpid(), unix.Gettid()
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				_ = unix.Tgkill(pid, tid, unix.SIGURG)
			}
		}
	})
	defer func() {
		close(stop)
		wg.Wait()
	}()
	// The backend's loop calls Dispatch(0). There the poll does not wait,
	// and a signal fails it only if it lands during the kernel's one
	// check, so a test of that would be a race. Dispatch(50) is on
	// purpose: the poll waits on an idle connection, and the signals
	// interrupt every call.
	for i := range 100 {
		if err = c.Dispatch(50); err != nil {
			t.Fatalf("Dispatch #%d: %v", i, err)
		}
	}
}
