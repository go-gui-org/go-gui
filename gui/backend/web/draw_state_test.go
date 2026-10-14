//go:build js && wasm

package web

import (
	"fmt"
	"strings"
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// ctxState is the part of a canvas 2D state the WebGL path relies on:
// the clip rects in force and the transform and filter operations
// applied since the frame began.
type ctxState struct {
	clips  []string
	xf     []string
	filter string
}

func (s ctxState) String() string {
	return fmt.Sprintf("clips=%v xf=%v filter=%q", s.clips, s.xf, s.filter)
}

// stateCtx is a fake 2D context that models save/restore, clip and
// transform state, and records fills with the state they ran under.
type stateCtx struct {
	t     *testing.T
	obj   js.Value
	cur   ctxState
	stack []ctxState
	path  string // last rect() since beginPath
	fills []string
	errs  []string
}

func newStateCtx(t *testing.T) *stateCtx {
	t.Helper()
	c := &stateCtx{t: t, obj: js.Global().Get("Object").New()}
	var funcs []js.Func
	on := func(name string, fn func(a []js.Value)) {
		f := js.FuncOf(func(_ js.Value, a []js.Value) any { fn(a); return nil })
		funcs = append(funcs, f)
		c.obj.Set(name, f)
	}
	t.Cleanup(func() {
		for _, f := range funcs {
			f.Release()
		}
	})
	copyState := func(s ctxState) ctxState {
		return ctxState{
			clips:  append([]string(nil), s.clips...),
			xf:     append([]string(nil), s.xf...),
			filter: s.filter,
		}
	}
	// filter is a property the backend sets directly, and a real
	// context saves and restores it with the rest of the state.
	on("save", func([]js.Value) {
		st := copyState(c.cur)
		st.filter = c.filterNow()
		c.stack = append(c.stack, st)
	})
	on("restore", func([]js.Value) {
		if len(c.stack) == 0 {
			c.errs = append(c.errs, "restore without save")
			return
		}
		c.cur = c.stack[len(c.stack)-1]
		c.stack = c.stack[:len(c.stack)-1]
		c.obj.Set("filter", c.cur.filter)
	})
	on("beginPath", func([]js.Value) { c.path = "" })
	on("rect", func(a []js.Value) {
		c.path = fmt.Sprintf("%g,%g %gx%g", a[0].Float(), a[1].Float(), a[2].Float(), a[3].Float())
	})
	on("clip", func([]js.Value) { c.cur.clips = append(c.cur.clips, c.path) })
	xf := func(name string) func([]js.Value) {
		return func(a []js.Value) {
			parts := make([]string, len(a))
			for i, v := range a {
				parts[i] = fmt.Sprintf("%g", v.Float())
			}
			c.cur.xf = append(c.cur.xf, name+"("+strings.Join(parts, ",")+")")
		}
	}
	on("translate", xf("translate"))
	on("rotate", xf("rotate"))
	on("transform", xf("transform"))
	on("setTransform", xf("setTransform"))
	for _, n := range []string{"moveTo", "lineTo", "closePath", "fillRect"} {
		on(n, func([]js.Value) {})
	}
	on("fill", func([]js.Value) {
		s := c.cur
		s.filter = c.filterNow()
		c.fills = append(c.fills, s.String())
	})
	return c
}

// filterNow reads the filter property the backend sets directly.
func (c *stateCtx) filterNow() string {
	if v := c.obj.Get("filter"); v.Type() == js.TypeString {
		return v.String()
	}
	return ""
}

// newStateBackend is a Canvas2D backend over a stateCtx.
func newStateBackend(t *testing.T) (*Backend, *stateCtx) {
	t.Helper()
	ctx := newStateCtx(t)
	return &Backend{ctx2d: ctx.obj, dpiScale: 1}, ctx
}

func clipCmd(x, y, w, h float32) gui.RenderCmd {
	return gui.RenderCmd{Kind: gui.RenderClip, X: x, Y: y, W: w, H: h}
}

// svgCmd is a one-triangle mesh covering (x, y)–(x+10, y+10). Its
// fill records the 2D state it ran under.
func svgCmd(x, y float32) gui.RenderCmd {
	return gui.RenderCmd{
		Kind:      gui.RenderSvg,
		Triangles: []float32{0, 0, 10, 0, 0, 10},
		Color:     gui.RGB(255, 0, 0),
		X:         x, Y: y, Scale: 1,
	}
}

func checkNoErrs(t *testing.T, ctx *stateCtx) {
	t.Helper()
	if len(ctx.errs) > 0 {
		t.Errorf("2D context misuse: %v", ctx.errs)
	}
}

// A clip inside a rotation bracket must not undo the rotation. The
// clip stack used to be replayed by restore(), and the bracket's save
// sat above the clip saves, so the first restore popped the bracket
// and the rest of it drew unrotated.
func TestClipInsideRotationKeepsTransform(t *testing.T) {
	b, ctx := newStateBackend(t)
	b.drawCmds([]gui.RenderCmd{
		clipCmd(0, 0, 1000, 1000),
		{Kind: gui.RenderRotateBegin, RotAngle: 90, RotCX: 50, RotCY: 50},
		clipCmd(0, 0, 500, 500), svgCmd(10, 10),
		{Kind: gui.RenderRotateEnd},
		svgCmd(20, 20),
	})
	if len(ctx.fills) != 2 {
		t.Fatalf("fills = %v, want 2", ctx.fills)
	}
	if f := ctx.fills[0]; !strings.Contains(f, "rotate(") || !strings.Contains(f, "clips=[0,0 500x500]") {
		t.Errorf("fill inside the bracket = %s, want rotated under clip 0,0 500x500", f)
	}
	if f := ctx.fills[1]; strings.Contains(f, "rotate(") {
		t.Errorf("fill after the bracket = %s, want unrotated", f)
	}
	checkNoErrs(t, ctx)
}

// The same for a filter bracket: its blur must survive a clip.
func TestClipInsideFilterKeepsFilter(t *testing.T) {
	b, ctx := newStateBackend(t)
	b.drawCmds([]gui.RenderCmd{
		clipCmd(0, 0, 1000, 1000),
		{Kind: gui.RenderFilterBegin, BlurRadius: 4},
		clipCmd(0, 0, 500, 500), svgCmd(10, 10),
		{Kind: gui.RenderFilterEnd},
	})
	if len(ctx.fills) != 1 {
		t.Fatalf("fills = %v, want 1", ctx.fills)
	}
	if f := ctx.fills[0]; !strings.Contains(f, "clips=[0,0 500x500]") ||
		!strings.Contains(f, `filter="blur(4px)"`) {
		t.Errorf("fill = %s, want clip 0,0 500x500 under blur(4px)", f)
	}
	if v := ctx.filterNow(); v != "" {
		t.Errorf("filter after the bracket = %q, want none", v)
	}
	checkNoErrs(t, ctx)
}

// A stencil clip begun inside a rotation is replayed under that
// rotation when a later rect clip forces a replay.
func TestStencilClipReplaysUnderItsBracket(t *testing.T) {
	b, ctx := newStateBackend(t)
	b.drawCmds([]gui.RenderCmd{
		{Kind: gui.RenderRotateBegin, RotAngle: 90, RotCX: 50, RotCY: 50},
		{Kind: gui.RenderStencilBegin, X: 5, Y: 5, W: 20, H: 20},
		clipCmd(0, 0, 500, 500), svgCmd(10, 10),
		{Kind: gui.RenderStencilEnd},
		{Kind: gui.RenderRotateEnd},
	})
	if len(ctx.fills) != 1 {
		t.Fatalf("fills = %v, want 1", ctx.fills)
	}
	if f := ctx.fills[0]; !strings.Contains(f, "clips=[0,0 500x500 5,5 20x20]") || !strings.Contains(f, "rotate(") {
		t.Errorf("fill = %s, want the rect clip, then the stencil under the rotation", f)
	}
	checkNoErrs(t, ctx)
}

// Nested brackets close in order: after the inner one the outer
// rotation is still in force, after both none is.
func TestNestedBracketsClose(t *testing.T) {
	b, ctx := newStateBackend(t)
	b.drawCmds([]gui.RenderCmd{
		{Kind: gui.RenderRotateBegin, RotAngle: 90, RotCX: 50, RotCY: 50},
		{Kind: gui.RenderRotateBegin, RotAngle: 45, RotCX: 10, RotCY: 10},
		clipCmd(0, 0, 500, 500), svgCmd(10, 10),
		{Kind: gui.RenderRotateEnd},
		svgCmd(20, 20),
		{Kind: gui.RenderRotateEnd},
		svgCmd(30, 30),
	})
	if len(ctx.fills) != 3 {
		t.Fatalf("fills = %v, want 3", ctx.fills)
	}
	if n := strings.Count(ctx.fills[0], "rotate("); n != 2 {
		t.Errorf("fill in the inner bracket = %s, want both rotations", ctx.fills[0])
	}
	if n := strings.Count(ctx.fills[1], "rotate("); n != 1 {
		t.Errorf("fill in the outer bracket = %s, want one rotation", ctx.fills[1])
	}
	if strings.Contains(ctx.fills[2], "rotate(") {
		t.Errorf("fill after both = %s, want unrotated", ctx.fills[2])
	}
	checkNoErrs(t, ctx)
}
