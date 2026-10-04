//go:build linux && !android && (amd64 || arm64)

package wl

import (
	"errors"
	"os"
	"testing"
	"time"
)

// These tests talk to a real compositor. They skip when there is none,
// unless GOGUI_REQUIRE_WAYLAND=1 (set by scripts/wayland/test.sh), which
// turns a missing compositor into a failure so a broken harness cannot pass
// silently.

func skipOrFail(t *testing.T, why string) {
	t.Helper()
	if os.Getenv("GOGUI_REQUIRE_WAYLAND") == "1" {
		t.Fatal(why)
	}
	t.Skip(why)
}

func connect(t *testing.T) *Conn {
	t.Helper()
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		skipOrFail(t, "no WAYLAND_DISPLAY")
	}
	c, err := Connect("")
	if err != nil {
		skipOrFail(t, err.Error())
	}
	t.Cleanup(c.Close)
	return c
}

type global struct{ name, version uint32 }

// globals lists the compositor's globals by interface name.
func globals(t *testing.T, c *Conn) (Registry, map[string]global) {
	t.Helper()
	reg := c.Display.GetRegistry()
	got := map[string]global{}
	reg.SetHandlers(RegistryHandlers{
		Global: func(name uint32, iface string, version uint32) {
			got[iface] = global{name, version}
		},
	})
	if err := c.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	return reg, got
}

// bind binds iface at the lower of want and what the compositor offers.
func bind(t *testing.T, reg Registry, gs map[string]global, iface *Interface, want uint32) Proxy {
	t.Helper()
	g, ok := gs[iface.Name()]
	if !ok {
		t.Fatalf("compositor has no %s", iface.Name())
	}
	p := reg.Bind(g.name, iface, min(want, g.version))
	if !p.Valid() {
		t.Fatalf("bind %s returned null", iface.Name())
	}
	return p
}

// TestRegistryGlobals covers the request path (get_registry: a typed new_id
// on wl_display) and the event path (global: uint, string, uint) end to end.
func TestRegistryGlobals(t *testing.T) {
	c := connect(t)
	_, gs := globals(t, c)
	for _, want := range []string{"wl_compositor", "wl_shm", "xdg_wm_base"} {
		if g, ok := gs[want]; !ok || g.version == 0 {
			t.Errorf("global %s missing (have %v)", want, gs)
		}
	}
}

func TestSyncDone(t *testing.T) {
	c := connect(t)
	cb := c.Display.Sync()
	done := false
	cb.SetHandlers(CallbackHandlers{Done: func(uint32) { done = true }})
	if err := c.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	if !done {
		t.Error("wl_callback.done never arrived")
	}
	cb.DestroyProxy()
}

// TestSurfaceLifecycle covers a generic new_id (bind), a typed new_id
// (create_surface) and a destructor request.
func TestSurfaceLifecycle(t *testing.T) {
	c := connect(t)
	reg, gs := globals(t, c)
	comp := Compositor{bind(t, reg, gs, &CompositorInterface, 4)}
	s := comp.CreateSurface()
	if !s.Valid() || s.ID() == 0 {
		t.Fatalf("create_surface gave %+v", s)
	}
	if s.Version() != comp.Version() {
		t.Errorf("surface version %d, compositor %d", s.Version(), comp.Version())
	}
	s.Destroy()
	if err := c.Roundtrip(); err != nil {
		t.Fatal(err)
	}
}

// TestToplevelConfigure maps an xdg_toplevel and waits for the first
// configure. It covers a string request (set_title), an array event
// argument (configure states) and answering a ping.
func TestToplevelConfigure(t *testing.T) {
	c := connect(t)
	reg, gs := globals(t, c)
	comp := Compositor{bind(t, reg, gs, &CompositorInterface, 4)}
	wm := XdgWmBase{bind(t, reg, gs, &XdgWmBaseInterface, 2)}
	wm.SetHandlers(XdgWmBaseHandlers{Ping: func(serial uint32) { wm.Pong(serial) }})

	surf := comp.CreateSurface()
	xs := wm.GetXdgSurface(surf)
	top := xs.GetToplevel()
	var serial uint32
	configured := false
	statesOK := true
	xs.SetHandlers(XdgSurfaceHandlers{Configure: func(s uint32) { serial, configured = s, true }})
	top.SetHandlers(XdgToplevelHandlers{Configure: func(w, h int32, states []byte) {
		// states is an array of uint32 enum values.
		if len(states)%4 != 0 {
			statesOK = false
		}
	}})
	top.SetTitle("go-gui wl test")
	top.SetAppId("org.go-gui.wltest")
	surf.Commit() // the first commit, with no buffer, asks for a configure

	deadline := time.Now().Add(5 * time.Second)
	for !configured && time.Now().Before(deadline) {
		if err := c.Dispatch(100 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	if !configured {
		t.Fatal("no xdg_surface.configure within 5s")
	}
	if !statesOK {
		t.Error("configure states array length not a multiple of 4")
	}
	xs.AckConfigure(serial)
	top.Destroy()
	xs.Destroy()
	surf.Destroy()
	if err := c.Roundtrip(); err != nil {
		t.Fatal(err)
	}
}

// TestProtocolError binds a version the compositor does not offer. The
// compositor kills the connection; Roundtrip must say so with the details.
func TestProtocolError(t *testing.T) {
	c := connect(t)
	reg, gs := globals(t, c)
	g := gs["wl_compositor"]
	reg.Bind(g.name, &CompositorInterface, g.version+50)
	err := c.Roundtrip()
	var pe *ProtocolError
	if !errors.As(err, &pe) {
		t.Fatalf("Roundtrip err = %v, want a *ProtocolError", err)
	}
	if pe.Interface == "" || pe.ID == 0 {
		t.Errorf("protocol error lacks details: %+v", pe)
	}
	if c.Err() == nil {
		t.Error("Err() nil after a protocol error")
	}
}

func TestDispatchTimeout(t *testing.T) {
	c := connect(t)
	start := time.Now()
	if err := c.Dispatch(30 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("Dispatch(30ms) took %v", d)
	}
	if err := c.Dispatch(0); err != nil {
		t.Fatal(err)
	}
}

// TestHandlerPanic: a panic in a handler comes out of Roundtrip in Go, and
// the connection still works afterwards.
func TestHandlerPanic(t *testing.T) {
	c := connect(t)
	reg := c.Display.GetRegistry()
	reg.SetHandlers(RegistryHandlers{Global: func(uint32, string, uint32) { panic("handler boom") }})
	func() {
		defer func() {
			if r := recover(); r != "handler boom" {
				t.Fatalf("recovered %v, want handler boom", r)
			}
		}()
		_ = c.Roundtrip()
		t.Fatal("Roundtrip did not panic")
	}()
	reg.SetHandlers(RegistryHandlers{})
	if err := c.Roundtrip(); err != nil {
		t.Fatalf("connection broken after a handler panic: %v", err)
	}
}

// TestDispatchIdleAllocs pins the cost of an idle Dispatch, which the
// backend calls every frame. The allocations are purego.SyscallN's own (see
// glbind/raw_other.go), not this package's: measured 4 under sway, weston
// and mutter (linux/arm64, purego v0.11.1). A rise means new per-call work.
func TestDispatchIdleAllocs(t *testing.T) {
	c := connect(t)
	allocs := testing.AllocsPerRun(50, func() { _ = c.Dispatch(0) })
	if allocs > 4 {
		t.Errorf("idle Dispatch allocates %.0f times, want at most 4", allocs)
	}
}

func TestConnectNoCompositor(t *testing.T) {
	if err := Load(); err != nil {
		skipOrFail(t, err.Error())
	}
	if c, err := Connect("gogui-no-such-display"); err == nil {
		c.Close()
		t.Fatal("Connect to a missing socket succeeded")
	}
}

// TestWake: a Wake from another goroutine ends a Dispatch that would wait
// much longer, and the wake is consumed, so the next Dispatch waits again.
func TestWake(t *testing.T) {
	c := connect(t)
	go func() {
		time.Sleep(50 * time.Millisecond)
		c.Wake()
	}()
	start := time.Now()
	if err := c.Dispatch(10 * time.Second); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("Wake did not end Dispatch (took %v)", d)
	}
	start = time.Now()
	if err := c.Dispatch(100 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < 80*time.Millisecond {
		t.Errorf("a consumed wake still cut the next wait short (%v)", d)
	}
}

// TestWakeAfterClose: Wake on a closed connection does nothing. The app
// can wake from a goroutine that outlives the window.
func TestWakeAfterClose(t *testing.T) {
	c := connect(t)
	c.Close()
	c.Wake()
}

// TestEGLWindow covers libwayland-egl: create, resize and destroy.
func TestEGLWindow(t *testing.T) {
	c := connect(t)
	reg, gs := globals(t, c)
	comp := Compositor{bind(t, reg, gs, &CompositorInterface, 4)}
	s := comp.CreateSurface()
	defer s.Destroy()
	if _, err := NewEGLWindow(s, 0, 10); err == nil {
		t.Error("a zero-width EGL window was created")
	}
	e, err := NewEGLWindow(s, 64, 48)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Valid() || e.Ptr() == 0 {
		t.Fatal("EGL window not valid")
	}
	e.Resize(128, 96)
	e.Resize(0, 0) // raised to 1x1, not ignored
	e.Destroy()
	if e.Valid() {
		t.Error("Destroy left the window valid")
	}
	e.Destroy() // a second Destroy is a no-op
	e.Resize(1, 1)
}
