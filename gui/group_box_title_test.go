package gui

import (
	"fmt"
	"slices"
	"testing"
)

// Group-box title rendering beyond draw order and clipping (#1238):
// the body inset, the brackets the title is drawn in, render-only
// frames, opacity, RTL, overflow, invisible ink, the eraser's limits.
// Helpers live in view_container_title_test.go.

// titledOn renders one titled box with ID "box" on a padded parent and
// returns the window and the box's arranged layout.
func titledOn(t *testing.T, parent, box ContainerCfg) (*Window, *Layout) {
	t.Helper()
	box.ID = "box"
	box.Title = titleTestText
	if box.Content == nil {
		box.Content = []View{Text(TextCfg{Text: "body"})}
	}
	parent.Sizing = FillFill
	if !parent.Padding.IsSet() {
		parent.Padding = PadAll(20)
	}
	parent.Content = []View{Column(box)}
	w := renderTitled(t, WindowCfg{}, func(_ *Window) View { return Column(parent) })
	ly, ok := w.layout.FindByID("box")
	if !ok {
		t.Fatal("box not found")
	}
	return w, ly
}

// band returns the height a titled box reserves above its border.
func band(ly *Layout) float32 { return ly.Shape.fx.groupTitle.Height / 2 }

// lastIndex returns the index of the last command match accepts, or -1.
func lastIndex(cmds []RenderCmd, match func(RenderCmd) bool) int {
	for i, cmd := range slices.Backward(cmds) {
		if match(cmd) {
			return i
		}
	}
	return -1
}

func isKind(k RenderKind) func(RenderCmd) bool {
	return func(c RenderCmd) bool { return c.Kind == k }
}

// Every way a container paints its body starts below the title's band,
// where the border is, not at the box's top: fill, shadow, gradient,
// blur and shader, and the shader's resolution is the body's.
func TestGroupBoxBodyInset(t *testing.T) {
	fill := RGB(200, 200, 40)
	grad := &GradientDef{Stops: []GradientStop{
		{Color: RGB(0, 0, 0), Pos: 0}, {Color: RGB(255, 255, 255), Pos: 1},
	}}
	cases := []struct {
		name string
		box  ContainerCfg
		kind RenderKind
	}{
		{"fill", ContainerCfg{Color: fill}, RenderRect},
		{"shadow", ContainerCfg{Color: fill, Shadow: &BoxShadow{Color: RGB(0, 0, 0), BlurRadius: 4}}, RenderShadow},
		{"gradient", ContainerCfg{Gradient: grad}, RenderGradient},
		{"blur", ContainerCfg{Color: fill, BlurRadius: 3}, RenderBlur},
		{"shader", ContainerCfg{Color: fill, Shader: &Shader{Metal: "m", GLSL: "g"}}, RenderCustomShader},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, box := titledOn(t, ContainerCfg{}, tc.box)
			b := band(box)
			i := cmdIndex(w.renderers, func(c RenderCmd) bool {
				return c.Kind == tc.kind && (c.Kind != RenderRect || c.Color == fill)
			})
			if i < 0 {
				t.Fatalf("no %v command", tc.kind)
			}
			c := w.renderers[i]
			if c.Y != box.Shape.Y+b || c.H != box.Shape.Height-b {
				t.Errorf("%v at y=%v h=%v, want the body y=%v h=%v", tc.kind,
					c.Y, c.H, box.Shape.Y+b, box.Shape.Height-b)
			}
			if tc.kind == RenderCustomShader {
				got := box.Shape.fx.shaderBuiltins[shaderBuiltinResY]
				if want := box.Shape.Height - b; got != want {
					t.Errorf("shader resolution y = %v, want the body's %v", got, want)
				}
			}
		})
	}
}

// A rounded clipping box masks its children to its body, and the title,
// half above the body, is drawn after the mask closes.
func TestGroupBoxTitleOutsideStencil(t *testing.T) {
	w, box := titledOn(t, ContainerCfg{}, ContainerCfg{
		ClipContents: true, Radius: RadiusMedium,
		Width: 200, Height: 100, Sizing: FixedFixed,
	})
	cmds := w.renderers
	begin := cmdIndex(cmds, isKind(RenderStencilBegin))
	end := lastIndex(cmds, isKind(RenderStencilEnd))
	ti := cmdIndex(cmds, isText(titleTestText))
	if begin < 0 || end < 0 || ti < 0 {
		t.Fatalf("stencil begin %d, end %d, title %d: all must be drawn", begin, end, ti)
	}
	if want := box.Shape.Y + band(box) + box.Shape.SizeBorder; cmds[begin].Y != want {
		t.Errorf("stencil top = %v, want the body's inner top %v", cmds[begin].Y, want)
	}
	if ti < end {
		t.Errorf("title at %d drawn inside the stencil (ends at %d)", ti, end)
	}
}

// The title is drawn inside its box's transform and filter brackets, so
// it turns, scales and filters with the box. In a RotatedBox, whose clip
// is its child's edges, the title stays whole.
func TestGroupBoxTitleInsideTransformAndFilter(t *testing.T) {
	cases := []struct {
		name       string
		box        ContainerCfg
		begin, end RenderKind
	}{
		{"transform", ContainerCfg{Transform: Rotate(10)}, RenderTransformBegin, RenderTransformEnd},
		{"filter", ContainerCfg{ColorFilter: ColorFilterGrayscale()}, RenderFilterBegin, RenderFilterEnd},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, _ := titledOn(t, ContainerCfg{Padding: PadAll(40)}, tc.box)
			cmds := w.renderers
			b := cmdIndex(cmds, isKind(tc.begin))
			e := lastIndex(cmds, isKind(tc.end))
			ti := cmdIndex(cmds, isText(titleTestText))
			if b < 0 || (b >= ti || ti >= e) {
				t.Errorf("begin %d, title %d, end %d: want the title inside", b, ti, e)
			}
		})
	}
	t.Run("rotated box", func(t *testing.T) {
		w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Padding: PadAll(40), Content: []View{
				RotatedBox(RotatedBoxCfg{QuarterTurns: 1, Content: Column(ContainerCfg{
					Title: titleTestText, Content: []View{Text(TextCfg{Text: "body"})},
				})}),
			}})
		})
		cmds := w.renderers
		ti := cmdIndex(cmds, isText(titleTestText))
		if ti < 0 {
			t.Fatal("title not drawn")
		}
		if clip := clipAt(w, cmds, ti); clip.Y > cmds[ti].Y {
			t.Errorf("title at y=%v cut by the RotatedBox clip at %v", cmds[ti].Y, clip.Y)
		}
	})
}

// A render-only frame reuses the arranged tree and draws the same title
// shape again: its position and opacity are taken from the box on every
// frame, assigned, not added to. Two unchanged frames draw it in place,
// and a box moved and faded between frames takes its title along.
func TestGroupBoxTitleRenderOnlyFrame(t *testing.T) {
	ink := RGB(100, 100, 100)
	w, box := titledOn(t, ContainerCfg{}, ContainerCfg{ColorBorder: ink})
	check := func(when string, wantColor Color) {
		t.Helper()
		ti := cmdIndex(w.renderers, isText(titleTestText))
		if ti < 0 {
			t.Fatalf("%s: title not drawn", when)
		}
		c := w.renderers[ti]
		wantX := box.Shape.X + groupTitleInset + groupTitlePad
		if c.X != wantX || c.Y != box.Shape.Y {
			t.Errorf("%s: title at %v,%v, want %v,%v", when, c.X, c.Y, wantX, box.Shape.Y)
		}
		if c.Color != wantColor {
			t.Errorf("%s: title color = %v, want %v", when, c.Color, wantColor)
		}
	}
	w.renderOnlyLocked()
	w.renderOnlyLocked()
	check("two unchanged frames", ink)
	box.Shape.X += 7
	box.Shape.Y += 5
	box.Shape.Opacity = 0.5
	w.renderOnlyLocked()
	check("moved and faded", ink.WithOpacity(0.5))
}

// A faded box fades its title with it, and its own faded fill is what
// the eraser matches.
func TestGroupBoxTitleFadesWithBox(t *testing.T) {
	ink := RGB(100, 100, 100)
	panel := RGB(10, 60, 90)
	fill := RGB(200, 200, 40)
	w, _ := titledOn(t, ContainerCfg{Color: panel},
		ContainerCfg{ColorBorder: ink, Color: fill, Opacity: SomeF(0.5)})
	ti := cmdIndex(w.renderers, isText(titleTestText))
	if ti < 0 {
		t.Fatal("title not drawn")
	}
	if got, want := w.renderers[ti].Color, ink.WithOpacity(0.5); got != want {
		t.Errorf("title color = %v, want %v", got, want)
	}
	e, ok := titleEraser(w.renderers, ti)
	if want := fill.WithOpacity(0.5).Over(panel); !ok || e.Color != want {
		t.Errorf("eraser = %v (drawn %v), want %v", e.Color, ok, want)
	}
}

// In a right-to-left box the title mirrors to the right edge with the
// same inset, its eraser with it.
func TestGroupBoxTitleRTL(t *testing.T) {
	w, box := titledOn(t, ContainerCfg{}, ContainerCfg{
		TextDir: TextDirRTL, Width: 300, Sizing: FixedFit,
	})
	ti := cmdIndex(w.renderers, isText(titleTestText))
	if ti < 0 {
		t.Fatal("title not drawn")
	}
	tw := box.Shape.fx.groupTitle.Width
	eraserW := tw + 2*groupTitlePad - 1
	wantEraserX := box.Shape.X + box.Shape.Width - groupTitleInset - eraserW
	if got, want := w.renderers[ti].X, wantEraserX+groupTitlePad; got != want {
		t.Errorf("title x = %v, want %v (right-aligned, inset %v)", got, want, groupTitleInset)
	}
	if e, ok := titleEraser(w.renderers, ti); !ok || e.X != wantEraserX || e.W != eraserW {
		t.Errorf("eraser x=%v w=%v (drawn %v), want x=%v w=%v", e.X, e.W, ok, wantEraserX, eraserW)
	}
}

// A box an overflowing row hid draws no title, and no eraser.
func TestGroupBoxTitleHiddenByOverflow(t *testing.T) {
	const n = 4
	w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
		items := make([]View, 0, n+1)
		for i := range n {
			items = append(items, Column(ContainerCfg{
				ID: fmt.Sprintf("g%d", i), Title: fmt.Sprintf("T%d", i),
				Width: 80, Sizing: FixedFit,
				Content: []View{Text(TextCfg{Text: "x"})},
			}))
		}
		items = append(items, Rectangle(RectangleCfg{Width: 20, Height: 20, Sizing: FixedFixed}))
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Row(ContainerCfg{ID: "ov", Overflow: true, Width: 250, Sizing: FixedFit, Content: items}),
		}})
	})
	row, ok := w.layout.FindByID("ov")
	if !ok {
		t.Fatal("row not found")
	}
	hidden := 0
	for i := range n {
		s := row.Children[i].Shape
		drawn := cmdIndex(w.renderers, isText(fmt.Sprintf("T%d", i))) >= 0
		if s.shapeType == shapeNone {
			hidden++
			if drawn {
				t.Errorf("hidden box %d drew its title", i)
			}
		} else if !drawn {
			t.Errorf("visible box %d drew no title", i)
		}
	}
	if hidden == 0 {
		t.Fatal("the row hid nothing; the test does not exercise overflow")
	}
}

// A title whose ink is invisible draws nothing: no text, and no eraser
// cutting a gap in a gradient border.
func TestGroupBoxTitleInvisibleInk(t *testing.T) {
	grad := &GradientDef{Stops: []GradientStop{
		{Color: RGB(255, 0, 0), Pos: 0}, {Color: RGB(0, 0, 255), Pos: 1},
	}}
	w, box := titledOn(t, ContainerCfg{}, ContainerCfg{
		ColorBorder: ColorTransparent, BorderGradient: grad,
	})
	if ti := cmdIndex(w.renderers, isText(titleTestText)); ti >= 0 {
		t.Errorf("invisible title drawn at %d", ti)
	}
	x := box.Shape.X + groupTitleInset
	if i := cmdIndex(w.renderers, func(c RenderCmd) bool {
		return c.Kind == RenderRect && c.Fill && c.X == x
	}); i >= 0 {
		t.Errorf("eraser drawn for an invisible title: %+v", w.renderers[i])
	}
}

// No one color matches what lies under the border in these cases, so
// an unset TitleBG draws no eraser: a filter the patch would be drawn
// inside while the fill it matches is outside it, the box's own blurred
// fill, a non-container ancestor, a shader fill. A filtered box with an
// opaque fill of its own is filtered together with its eraser, so that
// one keeps it.
func TestGroupBoxTitleEraserGivesUp(t *testing.T) {
	panel := RGB(10, 60, 90)
	fill := RGB(200, 200, 40)
	cases := []struct {
		name   string
		view   func(*Window) View
		eraser bool
	}{
		{"box filter over an outside fill", func(*Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Color: panel, Padding: PadAll(20), Content: []View{
				Column(ContainerCfg{Title: titleTestText, ColorFilter: ColorFilterGrayscale()}),
			}})
		}, false},
		{"filtered box with its own fill", func(*Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Color: panel, Padding: PadAll(20), Content: []View{
				Column(ContainerCfg{Title: titleTestText, Color: fill, ColorFilter: ColorFilterGrayscale()}),
			}})
		}, true},
		{"blurred fill", func(*Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Color: panel, Padding: PadAll(20), Content: []View{
				Column(ContainerCfg{Title: titleTestText, Color: fill, BlurRadius: 4}),
			}})
		}, false},
		{"circle ancestor", func(*Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Color: panel, Padding: PadAll(20), Content: []View{
				Circle(ContainerCfg{Color: RGB(200, 0, 0), Width: 200, Height: 200, Sizing: FixedFixed,
					Content: []View{Column(ContainerCfg{Title: titleTestText})}}),
			}})
		}, false},
		{"shader fill", func(*Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Color: panel, Padding: PadAll(20), Content: []View{
				Column(ContainerCfg{Title: titleTestText, Shader: &Shader{Metal: "m", GLSL: "g"}}),
			}})
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderTitled(t, WindowCfg{}, tc.view)
			ti := cmdIndex(w.renderers, isText(titleTestText))
			if ti < 0 {
				t.Fatal("title not drawn")
			}
			if _, ok := titleEraser(w.renderers, ti); ok != tc.eraser {
				t.Errorf("eraser drawn = %v, want %v", ok, tc.eraser)
			}
		})
	}
}

// A sibling drawn underneath is not part of the backdrop: the eraser
// takes the container's color over it. TitleBG names the right one;
// this pins what the docs say happens without it.
func TestGroupBoxTitleEraserIgnoresSiblingUnderneath(t *testing.T) {
	panel := RGB(10, 60, 90)
	w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Color: panel, Padding: PadAll(20), Content: []View{
			Stack(ContainerCfg{Width: 200, Height: 120, Sizing: FixedFixed, Content: []View{
				Rectangle(RectangleCfg{Sizing: FillFill, Color: RGB(200, 0, 0)}),
				Column(ContainerCfg{Title: titleTestText, Sizing: FillFill}),
			}}),
		}})
	})
	ti := cmdIndex(w.renderers, isText(titleTestText))
	if e, ok := titleEraser(w.renderers, ti); !ok || e.Color != panel {
		t.Errorf("eraser = %v (drawn %v), want the panel's %v", e.Color, ok, panel)
	}
}

// A box shorter than its title's band keeps its eraser inside the box
// and emits no command the render guard has to drop.
func TestGroupBoxTitleShortBox(t *testing.T) {
	w, box := titledOn(t, ContainerCfg{}, ContainerCfg{
		Color: RGB(1, 2, 3), Height: 4, Sizing: FillFixed, Content: []View{},
	})
	if w.renderGuardWarned != 0 {
		t.Errorf("render guard dropped a command: %b", w.renderGuardWarned)
	}
	ti := cmdIndex(w.renderers, isText(titleTestText))
	if e, ok := titleEraser(w.renderers, ti); ok && e.Y+e.H > box.Shape.Y+box.Shape.Height {
		t.Errorf("eraser ends at %v, below the box's bottom %v",
			e.Y+e.H, box.Shape.Y+box.Shape.Height)
	}
}

// The content starts below the title even with no top padding: the box
// reserves the title's whole height above it, so neither the title nor
// its eraser covers the first child. A top padding of half the title or
// more is kept as it is, below the band.
func TestGroupBoxTitleClearsFirstChild(t *testing.T) {
	for _, pad := range []Padding{PaddingNone, PadAll(30)} {
		_, box := titledOn(t, ContainerCfg{}, ContainerCfg{Padding: pad})
		h := box.Shape.fx.groupTitle.Height
		child := box.Children[0].Shape
		if child.Y < box.Shape.Y+h {
			t.Errorf("padding %v: first child at y=%v, inside the title (bottom %v)",
				pad.Top, child.Y, box.Shape.Y+h)
		}
		if pad.Top >= h/2 {
			if got, want := box.Shape.Padding.Top, pad.Top+h/2; got != want {
				t.Errorf("padding %v: top padding = %v, want %v", pad.Top, got, want)
			}
		}
	}
}

// A rounded clipping box hands its images the same clip radius with or
// without a title: the corner is the body's, and the title's band is
// not an inset from it. The padding clears half the title (8 px in
// tests), so the title adds nothing below the border.
func TestGroupBoxTitleKeepsClipRadius(t *testing.T) {
	radius := func(title string) float32 {
		l := generateViewLayout(Column(ContainerCfg{
			Title: title, Clip: true, Radius: RadiusPx(24), Padding: PadAll(10),
			SizeBorder: BorderPx(1), Width: 200, Height: 100, Sizing: FixedFixed,
		}), &Window{})
		return resolveClipRadius(0, l.Shape)
	}
	if plain, titled := radius(""), radius(titleTestText); titled != plain {
		t.Errorf("clip radius = %v with a title, %v without", titled, plain)
	}
}

// A title with a newline is drawn on two lines and given room for both:
// its first line sits on the border, the band above is half that line,
// and the content starts below the last line. An explicit TitleBG is a
// patch behind both lines.
func TestGroupBoxTitleMultiLine(t *testing.T) {
	const title = "Account\nSettings"
	// The tests' fallback measurer: 16 px a line.
	const line, lines = float32(16), float32(2)
	for _, pad := range []Padding{PaddingNone, PadAll(30)} {
		w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Padding: PadAll(20), Content: []View{
				Column(ContainerCfg{
					ID: "box", Title: title, Padding: pad, TitleBG: RGB(250, 220, 60),
					Content: []View{Text(TextCfg{Text: "body"})},
				}),
			}})
		})
		box, ok := w.layout.FindByID("box")
		if !ok {
			t.Fatal("box not found")
		}
		s := box.Shape
		if got := s.fx.groupTitle.Height; got != line*lines {
			t.Errorf("padding %v: title height = %v, want %v", pad.Top, got, line*lines)
		}
		if i := cmdIndex(w.renderers, isKind(RenderStrokeRect)); i < 0 ||
			w.renderers[i].Y != s.Y+line/2 {
			t.Errorf("padding %v: border not at the first line's middle %v", pad.Top, s.Y+line/2)
		}
		if child := box.Children[0].Shape; child.Y < s.Y+line*lines {
			t.Errorf("padding %v: first child at y=%v, inside the title (bottom %v)",
				pad.Top, child.Y, s.Y+line*lines)
		}
		ti := cmdIndex(w.renderers, isText(title))
		if e, drawn := titleEraser(w.renderers, ti); !drawn || e.Y != s.Y || e.H != line*lines {
			t.Errorf("padding %v: TitleBG patch y=%v h=%v (drawn %v), want y=%v h=%v",
				pad.Top, e.Y, e.H, drawn, s.Y, line*lines)
		}
	}
}

// A rounded corner wider than the title's inset pushes the title past
// its curve, onto the straight part of the top border, from the left or
// in RTL from the right; an eraser over the curve would square off the
// corner. A radius past half the body counts as the renderers clamp it,
// and a corner inside the inset leaves the title where it was.
func TestGroupBoxTitleClearsRoundedCorner(t *testing.T) {
	// A 300 x 80 box: its body is 72 high below the 8 px band.
	for _, tc := range []struct {
		radius, lead float32
	}{
		{8, groupTitleInset},
		{30, 30},
		{1000, 36},
	} {
		for _, dir := range []TextDirection{TextDirLTR, TextDirRTL} {
			w, box := titledOn(t, ContainerCfg{}, ContainerCfg{
				Radius: RadiusPx(tc.radius), Color: RGB(230, 210, 90), TextDir: dir,
				Width: 300, Height: 80, Sizing: FixedFixed,
			})
			s := box.Shape
			ti := cmdIndex(w.renderers, isText(titleTestText))
			if ti < 0 {
				t.Fatalf("radius %v: title not drawn", tc.radius)
			}
			eraserW := s.fx.groupTitle.Width + 2*groupTitlePad - 1
			wantX := s.X + tc.lead
			if dir == TextDirRTL {
				wantX = s.X + s.Width - tc.lead - eraserW
			}
			if got := w.renderers[ti].X; got != wantX+groupTitlePad {
				t.Errorf("radius %v dir %v: title x = %v, want %v",
					tc.radius, dir, got, wantX+groupTitlePad)
			}
			if e, ok := titleEraser(w.renderers, ti); !ok || e.X != wantX {
				t.Errorf("radius %v dir %v: eraser x = %v (drawn %v), want %v",
					tc.radius, dir, e.X, ok, wantX)
			}
		}
	}
}

// A border thicker than the title would lift the eraser's top, half a
// border width above the line, over the box's top edge and whatever
// lies above it. It stops at the box's top.
func TestGroupBoxTitleThickBorderEraser(t *testing.T) {
	w, box := titledOn(t, ContainerCfg{}, ContainerCfg{SizeBorder: BorderPx(30)})
	ti := cmdIndex(w.renderers, isText(titleTestText))
	e, ok := titleEraser(w.renderers, ti)
	if !ok {
		t.Fatal("no eraser")
	}
	s := box.Shape
	if e.Y != s.Y {
		t.Errorf("eraser starts at %v, want the box's top %v", e.Y, s.Y)
	}
	if bottom := s.Y + s.fx.groupTitle.Height; e.Y+e.H != bottom {
		t.Errorf("eraser ends at %v, want the title's bottom %v", e.Y+e.H, bottom)
	}
}

// A titled Circle draws its circle below the title's band, like the
// body of any container, so the title sits on the line its top touches,
// and the border rings that circle. A clipping circle's corner is the
// circle drawn. An untitled circle keeps its whole box.
func TestGroupBoxTitleCircleBody(t *testing.T) {
	const size = 160
	w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
		circle := func(id, title string) View {
			return Circle(ContainerCfg{
				ID: id, Title: title, Width: size, Height: size, Sizing: FixedFixed,
				Color: RGB(230, 210, 90), SizeBorder: BorderPx(2), ColorBorder: RGB(20, 20, 20),
				Clip: true, Padding: PadAll(20),
			})
		}
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			circle("titled", titleTestText), circle("plain", ""),
		}})
	})
	for _, tc := range []struct {
		id   string
		band float32
	}{{"titled", 8}, {"plain", 0}} {
		ly, ok := w.layout.FindByID(tc.id)
		if !ok {
			t.Fatalf("%s not found", tc.id)
		}
		s := ly.Shape
		r := (size - tc.band) / 2
		cx, cy := s.X+size/2, s.Y+tc.band+r
		var fill, ring *RenderCmd
		for i := range w.renderers {
			c := &w.renderers[i]
			if c.Kind == RenderCircle && c.Fill && c.X == cx && c.Y == cy {
				fill = c
			}
			if c.Kind == RenderStrokeRect && c.Y >= s.Y && c.Y < s.Y+size {
				ring = c
			}
		}
		if fill == nil || fill.Radius != r {
			t.Errorf("%s: no circle of radius %v at %v,%v", tc.id, r, cx, cy)
		}
		if ring == nil || ring.X != cx-r || ring.Y != cy-r || ring.W != 2*r ||
			ring.H != 2*r || ring.Radius != r {
			t.Errorf("%s: border %+v, want a ring of radius %v around %v,%v", tc.id, ring, r, cx, cy)
		}
		// The padding is 20 on every side below the band; the corner is
		// the circle's radius less that and the border.
		if got, want := resolveClipRadius(0, s), r-20-s.SizeBorder; got != want {
			t.Errorf("%s: clip radius = %v, want %v", tc.id, got, want)
		}
	}
}
