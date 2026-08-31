//go:build windows && !js

package gl

import (
	"testing"

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
	b.plat.w = gui.NewTestWindow(gui.WindowCfg{})
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
	b.plat.w = gui.NewTestWindow(gui.WindowCfg{})
	b.plat.frameless = true
	res, handled := b.handleMessage(wmNcCalcSize, 1, 0)
	if !handled || res != 0 {
		t.Errorf("wparam=1: (%d, %v), want (0, true)", res, handled)
	}
	if _, handled := b.handleMessage(wmNcCalcSize, 0, 0); handled {
		t.Error("wparam=0 handled; want fall-through to DefWindowProc")
	}
}
