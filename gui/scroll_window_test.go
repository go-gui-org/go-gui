package gui

import (
	"math"
	"testing"
)

// windowRow is a row of a scroll body with one frozen 60px child and a
// scroll window with a 600px one.
func windowRow(id string, cell View) View {
	return Row(ContainerCfg{ID: id, Sizing: FillFixed, Height: 20, MaxWidth: 660,
		Padding: NoPadding, Spacing: NoSpacing, SizeBorder: NoBorder, Content: []View{
			Rectangle(RectangleCfg{ID: "fixed", Width: 60, Height: 20, Sizing: FixedFixed}),
			Row(ContainerCfg{ID: "win", Sizing: FillFill, MaxWidth: 600, MinWidth: 80, Clip: true,
				ScrollLinkX: "body", scrollWindow: true,
				Padding: NoPadding, Spacing: NoSpacing, SizeBorder: NoBorder,
				Content: []View{cell}}),
		}})
}

// A body with a stated extent moves none of its own children, of any
// sizing and with none or a nil one beside them, and its scroll range is
// the stated one.
func TestScrollExtentKeepsChildren(t *testing.T) {
	for _, c := range []struct {
		name  string
		child func() View
	}{
		{"no child", nil},
		{"nil child", func() View { return nil }},
		{"fit", func() View { return Column(ContainerCfg{ID: "c", Sizing: FitFit}) }},
		{"fixed", func() View {
			return Rectangle(RectangleCfg{ID: "c", Width: 50, Height: 20, Sizing: FixedFixed})
		}},
		{"fixed, wider than the body", func() View {
			return Rectangle(RectangleCfg{ID: "c", Width: 900, Height: 20, Sizing: FixedFixed})
		}},
		{"fill", func() View { return Column(ContainerCfg{ID: "c", Sizing: FillFit}) }},
		{"nested body", func() View {
			return Column(ContainerCfg{ID: "c", Sizing: FillFit, Scrollable: true, ScrollExtentX: 900})
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
			w.TestRender(func(*Window) View {
				var content []View
				if c.child != nil {
					content = []View{c.child()}
				}
				return Column(ContainerCfg{ID: "body", Scrollable: true, Sizing: FillFill,
					Padding: NoPadding, SizeBorder: NoBorder, ScrollExtentX: 1000,
					Content: content})
			})
			w.ScrollHorizontalTo("body", -100000)
			l := w.TestRender(nil)
			if off, _, err := w.TestScrollOffset("body"); err != nil || off != -600 {
				t.Fatalf("offset = %.0f (err %v), want -600: extent 1000 less 400 visible", off, err)
			}
			if child, ok := l.FindByID(ScopeID("body", "c")); ok && child.Shape.X != 0 {
				t.Fatalf("child x = %.0f, want 0", child.Shape.X)
			}
		})
	}
}

// The window moves its content, not itself, and clips it sideways at its
// own edges. Vertically it inherits the clip around it, so a focus glow
// in a cell is not cut at the row.
func TestScrollWindowMovesAndClips(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "body", Scrollable: true, Sizing: FillFill,
			Padding: NoPadding, Spacing: NoSpacing, SizeBorder: NoBorder,
			ScrollExtentX: 660, ScrollExtentMin: 140, Content: []View{
				windowRow("r", Rectangle(RectangleCfg{ID: "cell", Width: 600, Height: 60, Sizing: FixedFixed})),
			}})
	})
	w.ScrollHorizontalTo("body", -100)
	l := w.TestRender(nil)
	win, ok := l.FindByID("body:r:win")
	if !ok {
		t.Fatal("window not found")
	}
	cell, _ := l.FindByID("body:r:win:cell")
	if win.Shape.X != 60 || win.Shape.Width != 340 {
		t.Fatalf("window x=%.0f w=%.0f, want 60 and 340", win.Shape.X, win.Shape.Width)
	}
	if cell.Shape.X != -40 {
		t.Fatalf("cell x = %.0f, want -40", cell.Shape.X)
	}
	// The cell is 60 high in a 20 high row. It takes the pointer only in
	// the row, as in any container, but draws past it.
	if c := cell.Shape.shapeClip; c.X != 60 || c.Width != 340 || c.Height != 20 {
		t.Fatalf("cell clip x=%.0f w=%.0f h=%.0f, want 60, 340 and 20", c.X, c.Width, c.Height)
	}
	var clips []drawClip
	for i := range w.renderers {
		if r := &w.renderers[i]; r.Kind == RenderClip && r.X == 60 {
			clips = append(clips, drawClip{X: r.X, Y: r.Y, Width: r.W, Height: r.H})
		}
	}
	if len(clips) != 1 || clips[0].Height != 300 {
		t.Fatalf("window clips = %+v, want one as high as the body", clips)
	}
}

// Without a live target, or with one that states no extent, a window
// keeps its content where it is. In the second case the target moves the
// window itself, as it does any child.
func TestScrollWindowWithoutExtent(t *testing.T) {
	for _, scrollable := range []bool{false, true} {
		w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
		w.TestRender(func(*Window) View {
			return Column(ContainerCfg{ID: "body", Scrollable: scrollable, Sizing: FillFill,
				Padding: NoPadding, SizeBorder: NoBorder, Content: []View{
					windowRow("r", Rectangle(RectangleCfg{ID: "cell", Width: 600, Height: 20, Sizing: FixedFixed})),
					Rectangle(RectangleCfg{Width: 900, Height: 20, Sizing: FixedFixed}),
				}})
		})
		w.scrollX().Set("body", -100)
		l := w.TestRender(nil)
		win, _ := l.FindByID("body:r:win")
		cell, _ := l.FindByID("body:r:win:cell")
		if cell.Shape.X != win.Shape.X {
			t.Fatalf("scrollable=%v: cell x = %.0f, window x = %.0f, want equal",
				scrollable, cell.Shape.X, win.Shape.X)
		}
	}
}

func TestScrollLead(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, c := range []struct {
		name          string
		extent, least float32
		scrollable    bool
		want          float32
	}{
		{"plain container", 0, 0, true, math.MaxFloat32},
		{"not scrollable", 1000, 0, false, math.MaxFloat32},
		{"NaN extent", nan, 0, true, math.MaxFloat32},
		{"infinite extent", inf, 0, true, math.MaxFloat32},
		{"rows fit", 1000, 100, true, 0},
		{"rows wider", 1000, 350, true, 150},
		{"NaN row", 1000, nan, true, 0},
		{"infinite row", 1000, inf, true, 0},
	} {
		s := &Shape{Width: 200, Scrollable: c.scrollable, events: &eventHandlers{
			scrollExtentX: c.extent, scrollExtentMin: c.least,
		}}
		if got := scrollLead(s); got != c.want {
			t.Errorf("%s: lead = %v, want %v", c.name, got, c.want)
		}
	}
	if got := scrollOwnX(-100, math.MaxFloat32); got != -100 {
		t.Errorf("own offset of a plain container = %v, want -100", got)
	}
	if got := scrollOwnX(-100, 30); got != -30 {
		t.Errorf("own offset with lead 30 = %v, want -30", got)
	}
}

// Under a fractional scale the sides of a window's clip land on the
// device pixels a box's sides round to, whichever way a backend rounds
// the clip: outward leaves them there.
func TestSnapClipX(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 2.625, 3} {
		for x := float32(0.1); x < 900; x += 3.37 {
			const width = 187.3
			got := snapClipX(drawClip{X: x, Width: width, Y: 5, Height: 7}, scale)
			s := float64(scale)
			left := math.Round(float64(float32(x * scale)))
			right := math.Round(float64(float32(float32(x+float32(width)) * scale)))
			gotLeft := math.Floor(float64(got.X) * s)
			gotRight := math.Ceil((float64(got.X) + float64(got.Width)) * s)
			if gotLeft != left || gotRight != right {
				t.Fatalf("scale %v x %v: device span %v..%v, want %v..%v", scale, x, gotLeft, gotRight, left, right)
			}
			if got.Y != 5 || got.Height != 7 {
				t.Fatalf("y span changed: %+v", got)
			}
		}
	}
	// Whole device pixels stay as they are.
	c := drawClip{X: 111, Width: 186, Height: 9}
	if got := snapClipX(c, 1); got != c {
		t.Errorf("snapped %+v to %+v, want no change", c, got)
	}
	for _, scale := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		if got := snapClipX(c, scale); got != c {
			t.Errorf("scale %v changed the clip to %+v", scale, got)
		}
	}
	if got := snapClipX(drawClip{X: 3.1, Width: 0.1, Height: 9}, 1); got != (drawClip{}) {
		t.Errorf("a clip thinner than a pixel = %+v, want empty", got)
	}
}

// The clip a window draws with is the snapped one. At 125% the window
// starts at device pixel 78.75, and an unsnapped clip would start at 78,
// on the border of the cell before it.
func TestScrollWindowClipSnaps(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.BackingScale = 1.25
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "body", Scrollable: true, Sizing: FillFill,
			Padding: NewPadding(0, 0, 0, 3), Spacing: NoSpacing, SizeBorder: NoBorder,
			ScrollExtentX: 660, Content: []View{
				windowRow("r", Rectangle(RectangleCfg{ID: "cell", Width: 600, Height: 20, Sizing: FixedFixed})),
			}})
	})
	l := w.TestRender(nil)
	win, _ := l.FindByID("body:r:win")
	if win.Shape.X != 63 {
		t.Fatalf("window x = %.2f, want 63", win.Shape.X)
	}
	for i := range w.renderers {
		if r := &w.renderers[i]; r.Kind == RenderClip && r.X > 60 && r.X < 70 {
			if got := math.Floor(float64(r.X) * 1.25); got != 79 {
				t.Fatalf("clip starts at device pixel %v, want 79", got)
			}
			return
		}
	}
	t.Fatal("no window clip drawn")
}
