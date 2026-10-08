//go:build windows && !js

package gl

import (
	"math"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

func TestWindowStyleFor(t *testing.T) {
	t.Parallel()
	base := uintptr(wsClipSiblings | wsClipChildren)
	cases := []struct {
		name string
		cfg  gui.WindowCfg
		want uintptr
	}{
		{
			name: "default",
			cfg:  gui.WindowCfg{},
			want: base | wsOverlappedWindow,
		},
		{
			name: "fixed size keeps the caption, drops the resize border",
			cfg:  gui.WindowCfg{FixedSize: true},
			want: base | wsFixed,
		},
		{
			// No Win32 equivalent, so it lands on the standard frame.
			name: "hidden titlebar degrades to default",
			cfg:  gui.WindowCfg{Decorations: gui.DecorationHiddenTitlebar},
			want: base | wsOverlappedWindow,
		},
		{
			// WS_THICKFRAME is what keeps OS resize and Aero snap.
			name: "frameless",
			cfg:  gui.WindowCfg{Decorations: gui.DecorationNone},
			want: base | wsPopup | wsThickFrame | wsMinimizeBox | wsMaximizeBox,
		},
		{
			name: "frameless and fixed",
			cfg:  gui.WindowCfg{Decorations: gui.DecorationNone, FixedSize: true},
			want: base | wsPopup,
		},
	}
	for _, c := range cases {
		if got := windowStyleFor(c.cfg); got != c.want {
			t.Errorf("%s: style = %#x, want %#x", c.name, got, c.want)
		}
	}
}

func TestWmszFor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		edge gui.WindowEdge
		want uintptr
	}{
		{gui.EdgeLeft, wmszLeft},
		{gui.EdgeRight, wmszRight},
		{gui.EdgeTop, wmszTop},
		{gui.EdgeTopLeft, wmszTopLeft},
		{gui.EdgeTopRight, wmszTopRight},
		{gui.EdgeBottom, wmszBottom},
		{gui.EdgeBottomLeft, wmszBottomLeft},
		{gui.EdgeBottomRight, wmszBottomRight},
	}
	for _, c := range cases {
		if got := wmszFor(c.edge); got != c.want {
			t.Errorf("edge %d = %d, want %d", c.edge, got, c.want)
		}
	}
	// Out of range: refuse rather than resize an arbitrary edge.
	if got := wmszFor(gui.WindowEdge(200)); got != 0 {
		t.Errorf("out-of-range edge = %d, want 0", got)
	}
}

// A decorated window must keep the default non-client handling, so both
// messages fall through to DefWindowProc.
func TestFramelessMessagesIgnoredWhenDecorated(t *testing.T) {
	t.Parallel()
	b := &Backend{}
	b.plat.w = gui.NewTestWindow(t, gui.WindowCfg{})
	if _, handled := b.handleMessage(wmNcCalcSize, 1, 0); handled {
		t.Error("WM_NCCALCSIZE handled on a decorated window")
	}
	if _, handled := b.handleMessage(wmGetMinMaxInfo, 0, 0); handled {
		t.Error("WM_GETMINMAXINFO handled on a decorated window")
	}
}

// Frameless windows swallow WM_NCCALCSIZE only when wparam asks for the
// full rect calculation.
func TestNcCalcSizeFrameless(t *testing.T) {
	t.Parallel()
	b := &Backend{}
	b.plat.w = gui.NewTestWindow(t, gui.WindowCfg{})
	b.plat.frameless = true
	res, handled := b.handleMessage(wmNcCalcSize, 1, 0)
	if !handled || res != 0 {
		t.Errorf("wparam=1: (%d, %v), want (0, true)", res, handled)
	}
	if _, handled := b.handleMessage(wmNcCalcSize, 0, 0); handled {
		t.Error("wparam=0 handled; want fall-through to DefWindowProc")
	}
}

// applySizeLimits writes only the constrained axes, so an unset axis
// keeps whatever default Windows already put in MINMAXINFO.
func TestApplySizeLimitsPartialAxes(t *testing.T) {
	t.Parallel()
	b := &Backend{}
	b.plat.w = gui.NewTestWindow(t, gui.WindowCfg{})
	b.plat.minTrack = pointL{x: 500}
	b.plat.maxTrack = pointL{y: 900}

	mmi := minMaxInfo{
		ptMinTrackSize: pointL{x: 10, y: 20},
		ptMaxTrackSize: pointL{x: 3000, y: 4000},
	}
	if !b.applySizeLimits(uintptr(unsafe.Pointer(&mmi))) {
		t.Fatal("applySizeLimits reported not handled")
	}
	if mmi.ptMinTrackSize.x != 500 {
		t.Errorf("min x = %d, want 500", mmi.ptMinTrackSize.x)
	}
	if mmi.ptMinTrackSize.y != 20 {
		t.Errorf("min y = %d, want the untouched 20", mmi.ptMinTrackSize.y)
	}
	if mmi.ptMaxTrackSize.y != 900 {
		t.Errorf("max y = %d, want 900", mmi.ptMaxTrackSize.y)
	}
	if mmi.ptMaxTrackSize.x != 3000 {
		t.Errorf("max x = %d, want the untouched 3000", mmi.ptMaxTrackSize.x)
	}
}

// A window with no limits must not claim the message, so DefWindowProc
// still supplies the defaults.
func TestApplySizeLimitsUnconstrained(t *testing.T) {
	t.Parallel()
	b := &Backend{}
	b.plat.w = gui.NewTestWindow(t, gui.WindowCfg{})
	var mmi minMaxInfo
	if b.applySizeLimits(uintptr(unsafe.Pointer(&mmi))) {
		t.Error("unconstrained window claimed WM_GETMINMAXINFO")
	}
	// A null lparam is a message we cannot answer, not a crash.
	if b.applySizeLimits(0) {
		t.Error("null lparam reported handled")
	}
}

// The track size is the outer frame, so the frame overhead is added to
// the scaled client bound. Only the delta is asserted, since the exact
// frame size depends on the host's metrics.
func TestTrackSizeForAddsFrameAndScale(t *testing.T) {
	t.Parallel()
	style := windowStyleFor(gui.WindowCfg{})
	limits := gui.SizeLimits{MinW: 400, MinH: 300}

	at96, _ := trackSizeFor(limits, style, 96, 1)
	at192, _ := trackSizeFor(limits, style, 192, 2)

	if at96.x <= 400 {
		t.Errorf("min x = %d, want more than the 400 client width", at96.x)
	}
	if at192.x <= at96.x {
		t.Errorf("192 dpi min x = %d, want more than 96 dpi %d",
			at192.x, at96.x)
	}
	// An unset axis stays zero so the caller skips it.
	if _, maxT := trackSizeFor(limits, style, 96, 1); maxT != (pointL{}) {
		t.Errorf("max track = %+v, want zero when unset", maxT)
	}
}

// With GOGUI_DEVICE_SCALE set, the client bound follows the effective
// scale, not the monitor DPI: the content is laid out at that scale, so
// a MinWidth of 400 is 800 client pixels at scale 2 on a 96 DPI monitor.
// The frame still follows the DPI, so it is the same in both calls and
// the difference is the client part alone (#984).
func TestTrackSizeForUsesEffectiveScale(t *testing.T) {
	t.Parallel()
	style := windowStyleFor(gui.WindowCfg{})
	limits := gui.SizeLimits{MinW: 400, MinH: 300, MaxW: 1000, MaxH: 700}

	min1, max1 := trackSizeFor(limits, style, 96, 1)
	min2, max2 := trackSizeFor(limits, style, 96, 2)

	if got := min2.x - min1.x; got != 400 {
		t.Errorf("min x grew by %d at scale 2, want 400", got)
	}
	if got := min2.y - min1.y; got != 300 {
		t.Errorf("min y grew by %d at scale 2, want 300", got)
	}
	if got := max2.x - max1.x; got != 1000 {
		t.Errorf("max x grew by %d at scale 2, want 1000", got)
	}
	if got := max2.y - max1.y; got != 700 {
		t.Errorf("max y grew by %d at scale 2, want 700", got)
	}
}

// A FixedSize window must open at its floor. The initial client size and
// the limit go through different code, so they must truncate the same
// way. At scale 1.3 a float64 product gives 12 for a width of 10, and the
// float32 limit gives 13 (#984).
func TestClientExtentMatchesFixedSizeFloor(t *testing.T) {
	t.Parallel()
	style := windowStyleFor(gui.WindowCfg{})
	for _, scale := range []float32{1, 1.15, 1.25, 1.3, 2, 2.2} {
		for _, w := range []int{10, 20, 333, 800, 1001} {
			cfg := gui.WindowCfg{Width: w, Height: w, FixedSize: true}
			limits := gui.WindowSizeLimits(cfg)
			base, _ := trackSizeFor(gui.SizeLimits{MinW: 1, MinH: 1}, style, 96, 1)
			minT, maxT := trackSizeFor(limits, style, 96, scale)
			// base.x - 1 is the frame overhead alone.
			frame := base.x - 1
			want := clientExtent(w, scale)
			if got := minT.x - frame; got != want {
				t.Errorf("scale %v width %d: min client %d, created %d",
					scale, w, got, want)
			}
			if got := maxT.x - frame; got != want {
				t.Errorf("scale %v width %d: max client %d, created %d",
					scale, w, got, want)
			}
		}
	}
}

// The initial size is clamped like a limit: never 0, never past what
// Windows takes.
func TestClientExtentClamps(t *testing.T) {
	t.Parallel()
	if got := clientExtent(1, 0.25); got != 1 {
		t.Errorf("clientExtent(1, 0.25) = %d, want 1", got)
	}
	if got := clientExtent(9000, 8); got != win32MaxExtent {
		t.Errorf("clientExtent(9000, 8) = %d, want %d", got, win32MaxExtent)
	}
	if got := clientExtent(10, 1.3); got != 13 {
		t.Errorf("clientExtent(10, 1.3) = %d, want 13", got)
	}
	// NaN fails the >= 1 test, so it gives 1 rather than an undefined
	// int32 conversion.
	if got := clientExtent(800, float32(math.NaN())); got != 1 {
		t.Errorf("clientExtent(800, NaN) = %d, want 1", got)
	}
}
