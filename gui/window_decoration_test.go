package gui

import "testing"

// The zero value has to stay the standard frame: every existing caller
// leaves WindowCfg.Decorations unset.
func TestWindowDecorationZeroValueIsDefault(t *testing.T) {
	t.Parallel()
	var cfg WindowCfg
	if cfg.Decorations != DecorationDefault {
		t.Errorf("zero Decorations = %d, want DecorationDefault", cfg.Decorations)
	}
}

// WindowEdge values are passed straight to X11 as _NET_WM_MOVERESIZE
// direction codes, so the order is protocol, not taste.
func TestWindowEdgeMatchesNetMoveResizeCodes(t *testing.T) {
	t.Parallel()
	want := []WindowEdge{
		EdgeTopLeft, EdgeTop, EdgeTopRight, EdgeRight,
		EdgeBottomRight, EdgeBottom, EdgeBottomLeft, EdgeLeft,
	}
	for i, edge := range want {
		if uint8(edge) != uint8(i) {
			t.Errorf("edge %d has value %d, want %d", i, edge, i)
		}
	}
}

func TestStartWindowDragNilPlatform(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(WindowCfg{})
	w.StartWindowDrag()
	w.StartWindowResize(EdgeBottomRight)
}

func TestStartWindowGesturesReachPlatform(t *testing.T) {
	t.Parallel()
	np := &recordingGesturePlatform{}
	w := NewTestWindow(WindowCfg{})
	w.SetNativePlatform(np)

	w.StartWindowDrag()
	w.StartWindowResize(EdgeTopLeft)

	if np.drags != 1 {
		t.Errorf("drags = %d, want 1", np.drags)
	}
	if np.resizes != 1 || np.lastEdge != EdgeTopLeft {
		t.Errorf("resizes = %d edge = %d, want 1 and EdgeTopLeft",
			np.resizes, np.lastEdge)
	}
}

type recordingGesturePlatform struct {
	noopNativePlatform
	drags    int
	resizes  int
	lastEdge WindowEdge
}

func (p *recordingGesturePlatform) StartWindowDrag() { p.drags++ }

func (p *recordingGesturePlatform) StartWindowResize(edge WindowEdge) {
	p.resizes++
	p.lastEdge = edge
}
