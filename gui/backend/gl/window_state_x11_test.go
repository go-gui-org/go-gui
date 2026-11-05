//go:build linux && !js && !android

package gl

import (
	"encoding/binary"
	"testing"

	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

func TestNetWMStateHas(t *testing.T) {
	t.Parallel()
	var value []byte
	value = binary.LittleEndian.AppendUint32(value, 11)
	value = binary.LittleEndian.AppendUint32(value, 22)

	for _, c := range []struct {
		value []byte
		atom  xproto.Atom
		want  bool
	}{
		{value, 11, true},
		{value, 22, true},
		{value, 33, false},
		{nil, 11, false},
		{value[:3], 11, false}, // a short value holds no atom
	} {
		if got := netWMStateHas(c.value, c.atom); got != c.want {
			t.Errorf("netWMStateHas(%v, %d) = %v, want %v",
				c.value, c.atom, got, c.want)
		}
	}
}

// A window manager refuses to fullscreen a window whose hints forbid
// the size, so the ceiling goes while fullscreen and the floor stays.
func TestHintLimitsDropCeilingInFullscreen(t *testing.T) {
	t.Parallel()
	p := &platformState{limits: gui.SizeLimits{
		MinW: 10, MinH: 20, MaxW: 300, MaxH: 400,
	}}
	if got := p.hintLimits(); got != p.limits {
		t.Errorf("windowed limits = %+v, want %+v", got, p.limits)
	}
	p.fullscreen = true
	want := gui.SizeLimits{MinW: 10, MinH: 20}
	if got := p.hintLimits(); got != want {
		t.Errorf("fullscreen limits = %+v, want %+v", got, want)
	}
}

func TestFrameExtentsOf(t *testing.T) {
	t.Parallel()
	var value []byte
	for _, v := range []uint32{1, 2, 20, 4} { // left, right, top, bottom
		value = binary.LittleEndian.AppendUint32(value, v)
	}
	if left, top := frameExtentsOf(value); left != 1 || top != 20 {
		t.Errorf("extents = %d, %d, want 1, 20", left, top)
	}
	if left, top := frameExtentsOf(nil); left != 0 || top != 0 {
		t.Errorf("no property = %d, %d, want 0, 0", left, top)
	}
}

// Minimized is either report: a window manager that unmaps, or mutter's
// state property on a window it keeps mapped.
func TestIconified(t *testing.T) {
	t.Parallel()
	if (&platformState{}).iconified() {
		t.Error("iconified = true for a shown window")
	}
	if !(&platformState{unmapped: true}).iconified() {
		t.Error("iconified = false for an unmapped window")
	}
	if !(&platformState{stateHidden: true}).iconified() {
		t.Error("iconified = false for a hidden-state window")
	}
}

// With no connection and no backend, every call must be a no-op.
func TestWindowStateNoConnection(t *testing.T) {
	t.Parallel()
	for _, n := range []*nativePlatform{{b: &Backend{}}, {}} {
		n.MinimizeWindow()
		n.MaximizeWindow()
		n.RestoreWindow()
		n.SetWindowFullscreen(true)
		n.ResizeWindow(10, 10)
		n.MoveWindow(1, 1)
		if _, _, ok := n.WindowPosition(); ok {
			t.Error("WindowPosition ok = true with no connection")
		}
	}
}
