//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
	"github.com/go-gui-org/go-gui/gui/backend/internal/xkb"
)

func TestWlButton(t *testing.T) {
	for _, c := range []struct {
		code uint32
		want gui.MouseButton
		bit  uint16
		ok   bool
	}{
		{btnLeft, gui.MouseLeft, x11MaskButton1, true},
		{btnMiddle, gui.MouseMiddle, x11MaskButton2, true},
		{btnRight, gui.MouseRight, x11MaskButton3, true},
		{0x113, 0, 0, false}, // BTN_SIDE
	} {
		got, bit, ok := wlButton(c.code)
		if got != c.want || bit != c.bit || ok != c.ok {
			t.Errorf("wlButton(%#x) = %v,%#x,%v; want %v,%#x,%v",
				c.code, got, bit, ok, c.want, c.bit, c.ok)
		}
	}
}

func TestWlAxisAcc(t *testing.T) {
	const v, h = wl.PointerAxisVerticalScroll, wl.PointerAxisHorizontalScroll
	var a wlAxisAcc
	if _, _, _, ok := a.take(); ok {
		t.Error("empty frame scrolled")
	}

	// One wheel click down: value plus value120, in one frame.
	a.addValue(v, 15)
	a.addValue120(v, 120)
	sx, sy, precise, ok := a.take()
	if !ok || precise || sx != 0 || sy != -x11ScrollLines {
		t.Errorf("wheel down = %v,%v,%v,%v; want 0,%v,false,true", sx, sy, precise, ok, -x11ScrollLines)
	}
	if _, _, _, ok = a.take(); ok {
		t.Error("take did not clear the frame")
	}

	// A high-resolution wheel: half a click up.
	a.addValue(v, -7.5)
	a.addValue120(v, -60)
	if _, sy, precise, _ = a.take(); precise || sy != x11ScrollLines/2 {
		t.Errorf("half click = %v,%v; want %v,false", sy, precise, x11ScrollLines/2)
	}

	// A touchpad: both axes, no clicks, points of travel.
	a.addValue(v, 4)
	a.addValue(h, -2.5)
	if sx, sy, precise, _ = a.take(); !precise || sx != 2.5 || sy != -4 {
		t.Errorf("touchpad = %v,%v,%v; want 2.5,-4,true", sx, sy, precise)
	}

	// Mixed: a clicking vertical axis with a continuous horizontal one
	// falls back to the continuous values.
	a.addValue(v, 15)
	a.addValue120(v, 120)
	a.addValue(h, 3)
	if sx, sy, precise, _ = a.take(); !precise || sx != -3 || sy != -15 {
		t.Errorf("mixed = %v,%v,%v; want -3,-15,true", sx, sy, precise)
	}

	// An unknown axis and NaN are ignored.
	a.addValue(7, 1)
	a.addValue120(7, 120)
	a.addValue(v, nan32())
	if _, _, _, ok = a.take(); ok {
		t.Error("unknown axis or NaN scrolled")
	}
}

func nan32() float64 {
	var zero float64
	return zero / zero
}

func TestWlKeyChar(t *testing.T) {
	cyrillic := func(sym uint32) rune {
		switch sym {
		case 0x6c1:
			return 'а'
		case 0xff0d:
			return '\r'
		}
		return 0
	}
	var c compose
	for _, k := range []struct {
		name  string
		sym   uint32
		state uint16
		want  rune
	}{
		{"latin", 'a', 0, 'a'},
		{"cyrillic falls back to xkb", 0x6c1, 0, 'а'},
		{"Return is a key, not text", 0xff0d, 0, 0},
		{"shortcut types nothing", 0x6c1, xkb.MaskControl, 0},
		{"dead key starts a sequence", 0xfe51, 0, 0}, // dead_acute
		{"sequence resolves", 'e', 0, 'é'},
		{"dead key again", 0xfe51, 0, 0},
		{"shift inside a sequence is transparent", 0xffe1, xkb.MaskShift, 0},
		{"miss falls through to xkb", 0x6c1, 0, 'а'},
	} {
		if got := wlKeyChar(&c, k.sym, k.state, cyrillic); got != k.want {
			t.Errorf("%s: got %q, want %q", k.name, got, k.want)
		}
	}
}

func TestWlRepeat(t *testing.T) {
	t0 := time.Unix(1000, 0)
	r := wlRepeat{rate: 10, delay: 500}
	r.start(38, t0)
	if r.due(t0.Add(499 * time.Millisecond)) {
		t.Error("fired before the delay")
	}
	if !r.due(t0.Add(500 * time.Millisecond)) {
		t.Error("did not fire at the delay")
	}
	if r.due(t0.Add(550 * time.Millisecond)) {
		t.Error("fired before the interval")
	}
	if !r.due(t0.Add(600 * time.Millisecond)) {
		t.Error("did not fire after the interval")
	}
	// A stall of 2 s fires once, then waits a full interval: no burst.
	late := t0.Add(2600 * time.Millisecond)
	if !r.due(late) || r.due(late) {
		t.Error("stall did not fire exactly once")
	}
	if want := late.Add(100 * time.Millisecond); !r.at.Equal(want) {
		t.Errorf("next at %v, want %v", r.at, want)
	}
	r.stop()
	if r.due(late.Add(time.Hour)) {
		t.Error("fired after stop")
	}

	// Rate 0 is "no repeat", and ends a running one.
	r.start(38, t0)
	r.setInfo(0, 500)
	if r.key != 0 {
		t.Error("rate 0 left the repeat running")
	}
	r.start(38, t0)
	if r.key != 0 {
		t.Error("started with rate 0")
	}
	// Negative values from a broken compositor are clamped.
	r.setInfo(-5, -5)
	if r.rate != 0 || r.delay != 0 {
		t.Errorf("setInfo(-5,-5) = %d,%d", r.rate, r.delay)
	}
}

// TestWlTickRepeat covers the paths that end a repeat without a key
// event: no key held, focus gone, keymap gone.
func TestWlTickRepeat(t *testing.T) {
	now := time.Unix(1000, 0)
	s := &wlSeat{repeat: wlRepeat{rate: 10, delay: 500}}
	if got := s.tickRepeat(now); got != -1 {
		t.Errorf("no key held: wait %v, want -1", got)
	}
	s.repeat.start(38, now)
	if got := s.tickRepeat(now); got != -1 || s.repeat.key != 0 {
		t.Errorf("no focus: wait %v, key %d; want -1 and stopped", got, s.repeat.key)
	}
	s.kbFocus = &Backend{}
	s.repeat.start(38, now)
	if got := s.tickRepeat(now); got != -1 || s.repeat.key != 0 {
		t.Errorf("no keymap: wait %v, key %d; want -1 and stopped", got, s.repeat.key)
	}
}

func TestWlTouches(t *testing.T) {
	b1, b2 := &Backend{}, &Backend{}
	var tc wlTouches
	var out [3]gui.Event
	fx := wl.FixedFrom

	// One finger down: began, with it marked changed.
	tc.down(b1, 7, fx(10), fx(20))
	if n := tc.frame(&out); n != 1 || out[0].Type != gui.EventTouchesBegan ||
		out[0].NumTouches != 1 || !out[0].Touches[0].Changed ||
		out[0].Touches[0].Identifier != 7 || out[0].Touches[0].PosX != 10 {
		t.Fatalf("down frame: n=%d %+v", n, out[0])
	}

	// A second window's touch is ignored while the first has fingers.
	tc.down(b2, 8, fx(0), fx(0))
	if tc.n != 1 {
		t.Fatal("touch on another window was tracked")
	}

	// Second finger down and the first moving in one frame: began then
	// moved, each listing both fingers.
	tc.down(b1, 9, fx(30), fx(40))
	tc.motion(7, fx(11), fx(21))
	n := tc.frame(&out)
	if n != 2 || out[0].Type != gui.EventTouchesBegan || out[1].Type != gui.EventTouchesMoved {
		t.Fatalf("down+move frame: n=%d types %v %v", n, out[0].Type, out[1].Type)
	}
	if out[0].NumTouches != 2 || out[0].Touches[0].Changed || !out[0].Touches[1].Changed {
		t.Errorf("began marks: %+v", out[0].Touches[:2])
	}
	if out[1].NumTouches != 2 || !out[1].Touches[0].Changed || out[1].Touches[1].Changed ||
		out[1].Touches[0].PosX != 11 {
		t.Errorf("moved marks: %+v", out[1].Touches[:2])
	}

	// One lifts: ended lists only that finger.
	tc.up(7)
	if n = tc.frame(&out); n != 1 || out[0].Type != gui.EventTouchesEnded ||
		out[0].NumTouches != 1 || out[0].Touches[0].Identifier != 7 {
		t.Fatalf("up frame: n=%d %+v", n, out[0])
	}
	if tc.n != 1 || tc.target != b1 {
		t.Fatalf("after lift: n=%d", tc.n)
	}

	// Cancel ends the rest and frees the window.
	if n = tc.cancel(&out); n != 1 || out[0].Type != gui.EventTouchesCancelled ||
		out[0].NumTouches != 1 {
		t.Fatalf("cancel: n=%d %+v", n, out[0])
	}
	if tc.n != 0 || tc.target != nil {
		t.Error("cancel left fingers")
	}
	if n = tc.cancel(&out); n != 0 {
		t.Error("cancel with no fingers emitted")
	}

	// Tap: down and up in one frame give began then ended.
	tc.down(b2, 1, fx(5), fx(5))
	tc.up(1)
	if n = tc.frame(&out); n != 2 || out[0].Type != gui.EventTouchesBegan ||
		out[1].Type != gui.EventTouchesEnded || tc.target != nil {
		t.Errorf("tap frame: n=%d", n)
	}

	// Fingers past the event's capacity, duplicate ids and a nil window
	// are dropped.
	for i := range int32(len(out[0].Touches) + 2) {
		tc.down(b1, i, fx(0), fx(0))
	}
	tc.down(b1, 0, fx(0), fx(0))
	tc.down(nil, 99, fx(0), fx(0))
	if tc.n != len(tc.pts) {
		t.Errorf("tracked %d fingers, want %d", tc.n, len(tc.pts))
	}
	// Motion and up for unknown ids are ignored.
	tc.motion(1234, fx(1), fx(1))
	tc.up(1234)
}

// requireInjection skips unless the compositor is sway with wtype and
// wlrctl, the only harness compositor that can inject input.
func requireInjection(t *testing.T) {
	t.Helper()
	requireWayland(t)
	if os.Getenv("SWAYSOCK") == "" {
		t.Skip("input injection needs sway")
	}
	for _, tool := range []string{"wtype", "wlrctl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("no %s", tool)
		}
	}
}

// TestWaylandKeyboard types into a real window with wtype and checks the
// key and character events, the Shift modifier and auto-repeat.
func TestWaylandKeyboard(t *testing.T) {
	requireInjection(t)
	t.Setenv("GOGUI_WAYLAND", "1")

	var (
		mu     sync.Mutex
		events []gui.Event
	)
	inject := func(w *gui.Window, d *wlDisplay) {
		time.Sleep(500 * time.Millisecond) // let sway map and focus it
		// wtype makes a virtual keyboard per run. The leading pause lets
		// the client get its wl_keyboard before the first key; a real
		// keyboard exists long before. wtype's own keymap gives each
		// character one level, so B types B and Shift+b still types b.
		// The held a repeats (sway: 600 ms delay, 25/s).
		out, err := exec.Command("wtype", "-s", "300", "aB", "-M", "shift", "b",
			"-m", "shift", "-P", "a", "-s", "1000", "-p", "a").CombinedOutput()
		if err != nil {
			t.Errorf("wtype: %v: %s", err, out)
		}
		time.Sleep(200 * time.Millisecond)
		w.Close()
		d.conn.Wake()
	}
	w := waylandTestWindow(gui.WindowCfg{
		OnInit: func(w *gui.Window) { go inject(w, wlShared) },
		OnEvent: func(e *gui.Event, _ *gui.Window) {
			mu.Lock()
			events = append(events, *e)
			mu.Unlock()
		},
	})
	done := make(chan error, 1)
	go func() { done <- runE(w) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("Run did not return")
	}

	mu.Lock()
	defer mu.Unlock()
	var text strings.Builder
	var repeats int
	var shiftB bool
	for _, e := range events {
		switch e.Type {
		case gui.EventChar:
			text.WriteString(e.IMEText)
		case gui.EventKeyDown:
			if e.KeyRepeat {
				repeats++
			}
			shiftB = shiftB || (e.KeyCode == gui.KeyB && e.Modifiers.Has(gui.ModShift))
		}
	}
	if got := text.String(); !strings.HasPrefix(got, "aBbaa") {
		t.Errorf("typed %q, want aBb then repeated a", got)
	}
	if repeats < 2 {
		t.Errorf("%d key repeats, want several", repeats)
	}
	if !shiftB {
		t.Error("no KeyB down with Shift")
	}
}

// TestWaylandPointerTouchRouting feeds pointer and touch events through
// the seat handlers for a real window's surface and checks they come out
// as gui events on that window. The harness cannot inject them for real:
// wlrctl makes a fresh virtual pointer per command, and its events are
// sent before the client can get a wl_pointer for it.
func TestWaylandPointerTouchRouting(t *testing.T) {
	var got []gui.Event
	b := newWaylandTestBackend(t, gui.WindowCfg{
		OnEvent: func(e *gui.Event, _ *gui.Window) { got = append(got, *e) },
	})
	defer b.Destroy()
	surface := b.plat.wl.surface
	s := &wlSeat{d: b.plat.wl.d}
	fx := wl.FixedFrom

	s.pointerEnter(1, surface, fx(10), fx(20))
	s.pointerMotion(0, fx(15), fx(25))
	s.pointerButton(2, 0, btnRight, wl.PointerButtonStatePressed)
	s.pointerButton(3, 0, btnRight, wl.PointerButtonStateReleased)
	s.axis.addValue(wl.PointerAxisVerticalScroll, 15)
	s.axis.addValue120(wl.PointerAxisVerticalScroll, 120)
	s.pointerFrame()
	s.pointerLeave(4, surface)
	// After leave nothing reaches the window.
	s.pointerMotion(0, fx(1), fx(1))

	s.touchDown(5, 0, surface, 3, fx(40), fx(50))
	s.touchFrame()
	s.touches.up(3)
	s.touchFrame()

	want := []struct {
		typ  gui.EventType
		x, y float32
	}{
		{gui.EventMouseMove, 10, 20},
		{gui.EventMouseMove, 15, 25},
		{gui.EventMouseDown, 15, 25},
		{gui.EventMouseUp, 15, 25},
		{gui.EventMouseScroll, 15, 25},
		{gui.EventMouseLeave, 0, 0},
		{gui.EventTouchesBegan, 0, 0},
		{gui.EventTouchesEnded, 0, 0},
	}
	if len(got) != len(want) {
		types := make([]gui.EventType, len(got))
		for i := range got {
			types[i] = got[i].Type
		}
		t.Fatalf("got events %v, want %d", types, len(want))
	}
	for i, w := range want {
		e := got[i]
		if e.Type != w.typ || e.MouseX != w.x || e.MouseY != w.y {
			t.Errorf("event %d = %v at %v,%v; want %v at %v,%v",
				i, e.Type, e.MouseX, e.MouseY, w.typ, w.x, w.y)
		}
	}
	if got[1].MouseDX != 5 || got[1].MouseDY != 5 {
		t.Errorf("motion delta %v,%v, want 5,5", got[1].MouseDX, got[1].MouseDY)
	}
	if got[2].MouseButton != gui.MouseRight || got[2].Modifiers.Has(gui.ModRMB) {
		t.Errorf("press: %v, mods %v (a press must not report its own button)",
			got[2].MouseButton, got[2].Modifiers)
	}
	if !got[3].Modifiers.Has(gui.ModRMB) {
		t.Errorf("release mods %v, want RMB held", got[3].Modifiers)
	}
	if got[4].ScrollY != -x11ScrollLines || got[4].ScrollPrecise {
		t.Errorf("scroll %v precise %v, want %v lines", got[4].ScrollY, got[4].ScrollPrecise, -x11ScrollLines)
	}
	if tp := got[6].Touches[0]; got[6].NumTouches != 1 || tp.PosX != 40 || tp.PosY != 50 || !tp.Changed {
		t.Errorf("touch began %+v", got[6])
	}

	// Destroying the window clears every reference the seat holds to it.
	s.pointerEnter(6, surface, fx(1), fx(1))
	s.kbFocus = b
	s.touchDown(7, 0, surface, 4, fx(1), fx(1))
	s.forget(b)
	if s.ptrFocus != nil || s.kbFocus != nil || s.touches.target != nil {
		t.Error("forget left a reference")
	}
}

// TestWlKeymapFd feeds wl_keyboard.keymap a memfd, as a compositor does:
// a good keymap compiles, a size past the end of the file or a format
// other than xkb is refused instead of crashing, and the fd is closed
// every time.
func TestWlKeymapFd(t *testing.T) {
	if err := xkb.Load(); err != nil {
		t.Skip(err)
	}
	const text = `xkb_keymap {
	xkb_keycodes { include "evdev" };
	xkb_types { include "complete" };
	xkb_compat { include "complete" };
	xkb_symbols { include "pc+us" };
};` + "\x00"
	memfd := func(t *testing.T) int {
		t.Helper()
		fd, err := unix.MemfdCreate("keymap", unix.MFD_CLOEXEC)
		if err != nil {
			t.Skip(err)
		}
		if _, err = unix.Write(fd, []byte(text)); err != nil {
			t.Fatal(err)
		}
		return fd
	}
	closed := func(fd int) bool {
		_, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0)
		return err == unix.EBADF
	}
	s := &wlSeat{}

	fd := memfd(t)
	s.keyboardKeymap(wl.KeyboardKeymapFormatXkbV1, fd, uint32(len(text)))
	if s.keymap == nil {
		t.Skip("keymap did not compile: xkeyboard-config data missing")
	}
	if got := s.keymap.Sym(30 + 8); got != 'a' {
		t.Errorf("Sym(KEY_A) = %#x, want 'a'", got)
	}
	if !closed(fd) {
		t.Error("fd left open")
	}

	for name, c := range map[string]struct{ format, size uint32 }{
		"size past the file": {wl.KeyboardKeymapFormatXkbV1, uint32(len(text)) + 4096},
		"no keymap format":   {wl.KeyboardKeymapFormatNoKeymap, uint32(len(text))},
		"zero size":          {wl.KeyboardKeymapFormatXkbV1, 0},
	} {
		fd = memfd(t)
		s.keyboardKeymap(c.format, fd, c.size)
		if s.keymap != nil {
			t.Errorf("%s: keymap accepted", name)
		}
		if !closed(fd) {
			t.Errorf("%s: fd left open", name)
		}
	}
}

// TestWlClearKeyboardEndsCompose: a dead key still open when the keyboard
// goes away must not compose with the next keyboard's first key.
func TestWlClearKeyboardEndsCompose(t *testing.T) {
	b := &Backend{}
	b.plat.compose.feed(0xfe51) // dead_acute
	s := &wlSeat{kbFocus: b}
	s.repeat = wlRepeat{key: 38, rate: 25}
	s.clearKeyboard()
	if b.plat.compose.n != 0 {
		t.Error("dead key survived the keyboard")
	}
	if s.kbFocus != nil || s.repeat.key != 0 {
		t.Error("focus or repeat survived the keyboard")
	}
	if r := b.plat.compose.feed('e'); r != 'e' {
		t.Errorf("next key typed %q, want plain e", r)
	}
}

// TestWaylandSeatRemoved: when the compositor removes the bound seat, the
// display lets it go, and a wl_seat global announced later is bound.
func TestWaylandSeatRemoved(t *testing.T) {
	requireWayland(t)
	d, err := acquireWaylandDisplay()
	if err != nil {
		t.Fatal(err)
	}
	defer d.release()
	if d.seat == nil {
		t.Skip("compositor offers no seat")
	}
	name, ver := d.seatName, d.seat.seat.Version()

	d.removeGlobal(name + 1000) // another global: no effect
	if d.seat == nil {
		t.Fatal("removing another global dropped the seat")
	}
	d.removeGlobal(name)
	if d.seat != nil {
		t.Fatal("seat kept after its global was removed")
	}
	// The same global stands in for a new seat: it still exists in the
	// compositor, so binding it again is valid.
	d.addSeat(name, ver)
	if d.seat == nil || d.seatName != name {
		t.Fatal("new seat not bound after the old one was removed")
	}
	if err := d.conn.Roundtrip(); err != nil {
		t.Fatal(err)
	}
}
