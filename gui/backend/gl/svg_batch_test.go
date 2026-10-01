//go:build !js && !darwin && !android && !cgo

package gl

import (
	"bytes"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// SVG triangle batching (#895). Consecutive RenderSvg commands share one
// vertex upload and one draw; any other command kind draws the queued run
// first. Like the glyph batch tests, these need a real GL context: CI runs
// them through scripts/gl-render-test.sh and they skip elsewhere without a
// display.

// svgSquare is a RenderSvg command covering the physical-pixel square at
// (x, y) with side n, in color c. Scale undoes the backend's DPI multiply,
// so the square lands on exact pixels whatever the display scale.
func svgSquare(b *Backend, x, y, n float32, c gui.Color) gui.RenderCmd {
	return gui.RenderCmd{
		Kind: gui.RenderSvg,
		Triangles: []float32{
			x, y, x + n, y, x + n, y + n,
			x, y, x + n, y + n, x, y + n,
		},
		Color: c,
		Scale: 1 / b.dpiScale,
	}
}

// drawStream feeds cmds through the per-command dispatch renderersDraw
// uses and draws whatever run is still queued at the end, as
// renderersDraw does. unbatched flushes after every command instead,
// which is the pre-#895 call sequence: one upload and draw per command.
func drawStream(b *Backend, cmds []gui.RenderCmd, unbatched bool) {
	for i := range cmds {
		b.drawCmd(nil, &cmds[i])
		if unbatched {
			b.flushSvg()
		}
	}
	b.flushSvg()
}

// TestSvgBatchFlushCounts pins the success criterion of #895 as draw
// counts: a run of RenderSvg commands is one draw whatever their colors,
// any other command kind ends the run, and a run past the vertex cap
// splits rather than growing without bound.
func TestSvgBatchFlushCounts(t *testing.T) {
	b := newBatchProbeBackend(t)
	b.plat.makeCurrent()

	count := func(cmds []gui.RenderCmd) int {
		before := b.svgFlushes
		drawStream(b, cmds, false)
		return b.svgFlushes - before
	}

	// The ThinkingOrb shape: one small mesh per dot, each a different
	// color because each dot has its own alpha.
	var dots []gui.RenderCmd
	for i := range 40 {
		dots = append(dots, svgSquare(b, float32(i), 0, 1,
			gui.RGBA(255, 255, 255, uint8(i*6))))
	}
	if got := count(dots); got != 1 {
		t.Errorf("40 consecutive SVG meshes took %d draws, want 1", got)
	}

	w := float32(b.physW) / b.dpiScale
	h := float32(b.physH) / b.dpiScale
	if got := count([]gui.RenderCmd{
		svgSquare(b, 0, 0, 4, gui.RGB(255, 0, 0)),
		svgSquare(b, 4, 0, 4, gui.RGB(0, 255, 0)),
		{Kind: gui.RenderClip, W: w, H: h},
		svgSquare(b, 8, 0, 4, gui.RGB(0, 0, 255)),
	}); got != 2 {
		t.Errorf("svg,svg,clip,svg took %d draws, want 2", got)
	}

	if got := count([]gui.RenderCmd{
		{Kind: gui.RenderRect, Fill: true, W: 4, H: 4, Color: gui.White},
	}); got != 0 {
		t.Errorf("a stream without SVG took %d SVG draws, want 0", got)
	}

	// Two meshes that each fit under the cap but not together: the
	// first draws on its own before the second is queued.
	half := maxSvgBatchVerts/2 + 3 // vertices per mesh, a multiple of 3
	big := make([]float32, half*2)
	if got := count([]gui.RenderCmd{
		{Kind: gui.RenderSvg, Triangles: big, Color: gui.White, Scale: 1},
		{Kind: gui.RenderSvg, Triangles: big, Color: gui.White, Scale: 1},
	}); got != 2 {
		t.Errorf("two meshes past the batch cap took %d draws, want 2", got)
	}
}

// svgBatchCmds is an SVG-heavy frame whose correctness depends on draw
// order: overlapping meshes of different colors, a rect drawn between two
// of them, and a clip that cuts through a later run. A batch that drew
// out of order, or leaked across the rect or the scissor change, shows up
// as a pixel difference.
func svgBatchCmds(b *Backend) []gui.RenderCmd {
	w := float32(b.physW) / b.dpiScale
	h := float32(b.physH) / b.dpiScale
	s := b.dpiScale
	return []gui.RenderCmd{
		svgSquare(b, 0, 0, 16, gui.RGB(255, 0, 0)),
		svgSquare(b, 8, 0, 16, gui.RGBA(0, 255, 0, 128)),
		// Logical units: drawRect multiplies by the DPI scale.
		{Kind: gui.RenderRect, Fill: true, X: 12 / s, Y: 0, W: 8 / s, H: 8 / s,
			Color: gui.RGB(0, 0, 255)},
		svgSquare(b, 16, 4, 8, gui.RGB(255, 255, 0)),
		{Kind: gui.RenderClip, X: 0, Y: 0, W: 40 / s, H: h},
		svgSquare(b, 32, 0, 16, gui.RGB(255, 0, 255)),
		svgSquare(b, 36, 4, 16, gui.RGB(0, 255, 255)),
		{Kind: gui.RenderClip, X: 0, Y: 0, W: w, H: h},
		svgSquare(b, 0, 32, 8, gui.White),
	}
}

// TestSvgBatchMatchesUnbatched renders the same frame batched and one
// draw per command, and requires identical pixels. It also samples the
// spots where draw order decides the color, so a frame that rendered
// nothing, or rendered wrong both ways, cannot pass.
func TestSvgBatchMatchesUnbatched(t *testing.T) {
	b := newBatchProbeBackend(t)
	rt := newReadbackTarget(t, b)
	cmds := svgBatchCmds(b)

	batched := rt.render(b, func() { drawStream(b, cmds, false) })
	unbatched := rt.render(b, func() { drawStream(b, cmds, true) })
	if !bytes.Equal(batched, unbatched) {
		t.Fatalf("batched SVG frame differs from the unbatched one")
	}

	black := [4]byte{0, 0, 0, 255}
	checks := []struct {
		name string
		x, y int
		want [4]byte
	}{
		{"rect over the queued run", 14, 2, [4]byte{0, 0, 255, 255}},
		{"mesh after the rect over it", 18, 6, [4]byte{255, 255, 0, 255}},
		{"later mesh over earlier in one run", 37, 6, [4]byte{0, 255, 255, 255}},
		{"clip cuts the run", 46, 6, black},
		{"run after the clip restore", 4, 34, [4]byte{255, 255, 255, 255}},
	}
	for _, c := range checks {
		if got := pixelAt(b, batched, c.x, c.y); got != c.want {
			t.Errorf("%s: pixel (%d,%d) = %v, want %v",
				c.name, c.x, c.y, got, c.want)
		}
	}
}
