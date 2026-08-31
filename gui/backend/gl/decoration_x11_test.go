//go:build linux && !js && !android

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The payload is what every Motif-reading window manager parses, so its
// shape is a contract, not an implementation detail.
func TestMotifHintsUndecorated(t *testing.T) {
	t.Parallel()
	got := motifHintsUndecorated()
	want := []uint32{2, 0, 0, 0, 0}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("word %d = %d, want %d", i, got[i], want[i])
		}
	}
}

// WindowEdge is passed to the WM unmapped, so drift between the two
// orderings would silently resize the wrong edge.
func TestWindowEdgeIsNetMoveResizeDirection(t *testing.T) {
	t.Parallel()
	cases := []struct {
		edge gui.WindowEdge
		want uint32
	}{
		{gui.EdgeTopLeft, 0},
		{gui.EdgeTop, 1},
		{gui.EdgeTopRight, 2},
		{gui.EdgeRight, 3},
		{gui.EdgeBottomRight, 4},
		{gui.EdgeBottom, 5},
		{gui.EdgeBottomLeft, 6},
		{gui.EdgeLeft, 7},
	}
	for _, c := range cases {
		if uint32(c.edge) != c.want {
			t.Errorf("edge %d maps to %d, want %d", c.edge, uint32(c.edge), c.want)
		}
	}
	if netMoveResizeMove != 8 {
		t.Errorf("move direction = %d, want 8", netMoveResizeMove)
	}
}

// A nil connection means the window was never created; the gesture has
// to fall through rather than panic.
func TestStartMoveResizeNoConnection(t *testing.T) {
	t.Parallel()
	var p platformState
	p.startMoveResize(netMoveResizeMove)
}

// setWindowDecorations must not touch the connection for a decorated
// window — a nil conn here would panic if it did.
func TestSetWindowDecorationsSkipsDefault(t *testing.T) {
	t.Parallel()
	setWindowDecorations(nil, 0, gui.DecorationDefault)
	setWindowDecorations(nil, 0, gui.DecorationHiddenTitlebar)
}
