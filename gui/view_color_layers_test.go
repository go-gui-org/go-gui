package gui

import (
	"strings"
	"testing"
)

// Color widgets and the popup layers (#1244). The knobs, the channel
// slider's track and the swatch's color were Float children. The knobs,
// at Z 1, were drawn over every popup and tooltip; the track and the
// color over every Z-0 popup earlier in the tree; and all of them
// escaped the clip of a scroll view. They now stay in the widget's own
// layer. Like the group-box title tests (#1238), these read the emitted
// render commands: what is drawn after what, and under which clip.

// colorLayersValue converts to a Color and back without rounding, so a
// ColorPicker's swatch shows exactly colorLayersValue.Color().
var colorLayersValue = HSLA{H: 240, S: 1, L: 0.5, A: 1}

type colorLayersWidget struct {
	name string
	knob bool // draws a knob
	view func() View
}

// colorLayersWidgets returns every color widget that drew a part of
// itself as a float. Each has the ID "w".
func colorLayersWidgets() []colorLayersWidget {
	v := colorLayersValue
	return []colorLayersWidget{
		{"picker", true, func() View {
			return ColorPicker(ColorPickerCfg{ID: "w", Color: v.Color()})
		}},
		{"plane", true, func() View {
			return ColorPlane(ColorPlaneCfg{ID: "w", Value: v, Size: 120})
		}},
		{"wheel", true, func() View {
			return ColorWheel(ColorWheelCfg{ID: "w", Value: v, Size: 120})
		}},
		{"hue slider", true, func() View {
			return ColorChannelSlider(ColorChannelSliderCfg{ID: "w",
				Channel: ChannelHue, Value: v})
		}},
		{"vertical alpha slider", true, func() View {
			return ColorChannelSlider(ColorChannelSliderCfg{ID: "w",
				Channel: ChannelAlpha, Value: v, Vertical: true, Height: 120})
		}},
		{"swatch", false, func() View {
			return ColorSwatch(ColorSwatchCfg{ID: "w", Color: v.Color(),
				Width: 60, Height: 40})
		}},
	}
}

// isColorPart matches what a color widget draws of itself: a knob (a
// circle), a generated image (track, plane, wheel, checker) or the
// swatch's color.
func isColorPart(c RenderCmd) bool {
	switch c.Kind {
	case RenderCircle:
		return true
	case RenderImage:
		for _, p := range []string{"gui/cstrip/", "gui/cplane/", "gui/cwheel/", "gui/checker/"} {
			if strings.Contains(c.Resource, p) {
				return true
			}
		}
	case RenderRect:
		return c.Fill && c.Color == colorLayersValue.Color()
	}
	return false
}

// colorParts returns the indexes of the color part commands, and fails
// the test when none, or no knob where one is due, was drawn.
func colorParts(t *testing.T, cmds []RenderCmd, knob bool) []int {
	t.Helper()
	var parts []int
	knobs := 0
	for i := range cmds {
		if isColorPart(cmds[i]) {
			parts = append(parts, i)
			if cmds[i].Kind == RenderCircle {
				knobs++
			}
		}
	}
	if len(parts) == 0 {
		t.Fatal("setup: the color widget drew nothing")
	}
	if knob && knobs == 0 {
		t.Fatal("setup: no knob drawn")
	}
	return parts
}

// openSelect opens the Select whose leaf ID is "sel" and renders.
func openSelect(t *testing.T, w *Window) {
	t.Helper()
	ids := w.ResolveID("sel")
	if len(ids) != 1 {
		t.Fatalf("setup: Select ids = %v, want one", ids)
	}
	w.SetFocus(ids[0])
	StateMap[string, bool](w, nsSelect, capModerate).Set(ids[0], true)
	w.TestRender(nil)
}

// Every part of a color widget is drawn before the last thing an open
// popup draws, so the popup covers it. The popups: a Select list above
// the widget, the same inside a dialog, a tooltip whose owner comes
// before the widget in the tree (it drops over it) and one whose owner
// comes after it (it flips up over it, the case found by hand in
// #1243's test app).
func TestColorPartsDrawUnderPopups(t *testing.T) {
	selectAbove := func(widget View) View {
		return Column(ContainerCfg{Content: []View{
			Select(SelectCfg{ID: "sel", Items: []string{"one", "two", "three"}}),
			widget,
		}})
	}
	tooltip := func(vw *Window) View {
		return WithTooltip(vw, WithTooltipCfg{
			ID: "tip", Text: "tooltip text",
			Content: []View{Rectangle(RectangleCfg{
				Width: 100, Height: 30, Sizing: FixedFixed,
			})},
		})
	}
	popups := []struct {
		name string
		// show renders widget with the popup open and returns the text
		// the popup draws last.
		show func(t *testing.T, w *Window, widget func() View) string
	}{
		{"select", func(t *testing.T, w *Window, widget func() View) string {
			w.TestRender(func(*Window) View { return selectAbove(widget()) })
			openSelect(t, w)
			return "three"
		}},
		{"select in float panel", func(t *testing.T, w *Window, widget func() View) string {
			w.TestRender(func(*Window) View {
				return Column(ContainerCfg{Sizing: FillFill, Content: []View{
					Column(ContainerCfg{
						Float:       true,
						FloatAnchor: FloatTopLeft,
						FloatTieOff: FloatTopLeft,
						Content:     []View{selectAbove(widget())},
					}),
				}})
			})
			openSelect(t, w)
			return "three"
		}},
		{"select in dialog", func(t *testing.T, w *Window, widget func() View) string {
			w.TestRender(func(*Window) View { return Column(ContainerCfg{}) })
			w.Dialog(DialogCfg{
				DialogType: DialogCustom,
				CustomView: func(*Window) View { return selectAbove(widget()) },
			})
			w.TestRender(nil)
			openSelect(t, w)
			return "three"
		}},
		{"tooltip before", func(t *testing.T, w *Window, widget func() View) string {
			w.viewState.tooltip.id = "tip"
			w.viewState.tooltip.popupID = ScopeID("tip", "popup")
			w.TestRender(func(vw *Window) View {
				return Column(ContainerCfg{Content: []View{tooltip(vw), widget()}})
			})
			return "tooltip text"
		}},
		{"tooltip after", func(t *testing.T, w *Window, widget func() View) string {
			w.viewState.tooltip.id = "tip"
			w.viewState.tooltip.popupID = ScopeID("tip", "popup")
			w.TestRender(func(vw *Window) View {
				return Column(ContainerCfg{Content: []View{widget(), tooltip(vw)}})
			})
			return "tooltip text"
		}},
	}
	for _, p := range popups {
		for _, wc := range colorLayersWidgets() {
			t.Run(p.name+"/"+wc.name, func(t *testing.T) {
				w := NewTestWindow(t, WindowCfg{Width: 600, Height: 700})
				text := p.show(t, w, wc.view)
				cmds := w.renderers
				last := -1
				for i := range cmds {
					if isText(text)(cmds[i]) {
						last = i
					}
				}
				if last < 0 {
					t.Fatalf("setup: the popup did not draw %q", text)
				}
				for _, i := range colorParts(t, cmds, wc.knob) {
					if i > last {
						t.Errorf("%v drawn at %d, after the popup's %q at %d",
							cmds[i].Kind, i, text, last)
					}
				}
			})
		}
	}
}

// colorLayersViewportGap is the empty space above the widget inside the
// scroll viewport, so the widget starts below the viewport at rest.
const colorLayersViewportGap = 150

// scrolledColorWidget puts widget in a 100 px scroll viewport below a
// 100 px header, scrolls the viewport by offset, and returns the
// viewport's content box: what the widget must be drawn inside.
func scrolledColorWidget(t *testing.T, widget func() View, offset float32) (w *Window, top, bottom float32) {
	t.Helper()
	w = NewTestWindow(t, WindowCfg{Width: 600, Height: 700})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Rectangle(RectangleCfg{Width: 300, Height: 100, Sizing: FixedFixed}),
			Column(ContainerCfg{
				ID: "scroll", Scrollable: true,
				Width: 500, Height: 100, Sizing: FixedFixed,
				Padding: PaddingNone, Spacing: NoSpacing,
				Content: []View{
					Rectangle(RectangleCfg{Width: 10, Height: colorLayersViewportGap, Sizing: FixedFixed}),
					widget(),
					Rectangle(RectangleCfg{Width: 10, Height: 400, Sizing: FixedFixed}),
				},
			}),
		}})
	})
	w.ScrollVerticalTo("scroll", offset)
	w.TestRender(nil)
	sc, ok := w.layout.FindByID("scroll")
	if !ok {
		t.Fatal("setup: scroll viewport not found")
	}
	s := sc.Shape
	top = s.Y + s.PaddingTop()
	bottom = s.Y + s.Height - s.Padding.Bottom - s.SizeBorder
	return w, top, bottom
}

// A color widget cut by the edge of a scroll viewport is drawn inside
// the viewport's clip, at the top edge and at the bottom edge. The knobs
// matter most: the wheel's knob was drawn below the viewport while the
// wheel itself was clipped away.
func TestColorPartsClipToScrollViewport(t *testing.T) {
	for _, wc := range colorLayersWidgets() {
		// The widget's height at rest, read before the next render.
		w, _, _ := scrolledColorWidget(t, wc.view, 0)
		ly, ok := w.layout.FindByID(ScopeID("scroll", "w"))
		if !ok {
			t.Fatalf("%s: setup: widget not found", wc.name)
		}
		h := ly.Shape.Height
		for _, edge := range []struct {
			name   string
			offset float32
		}{
			// The widget's middle on the viewport's top edge.
			{"top", -(colorLayersViewportGap + h/2)},
			// Its middle on the bottom edge: the viewport is 100 high.
			{"bottom", -(colorLayersViewportGap + h/2 - 100)},
		} {
			t.Run(wc.name+"/"+edge.name, func(t *testing.T) {
				w, top, bottom := scrolledColorWidget(t, wc.view, edge.offset)
				cmds := w.renderers
				for _, i := range colorParts(t, cmds, false) {
					clip := clipAt(w, cmds, i)
					if clip.Y < top-0.5 || clip.Y+clip.Height > bottom+0.5 {
						t.Errorf("%v at %d drawn under clip y %v..%v, outside the viewport %v..%v",
							cmds[i].Kind, i, clip.Y, clip.Y+clip.Height, top, bottom)
					}
				}
			})
		}
	}
}

// A picker in a scroll viewport that is itself in a scrolled viewport is
// drawn inside both clips.
func TestColorPartsClipToNestedScrollViewports(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 600, Height: 700})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Rectangle(RectangleCfg{Width: 300, Height: 100, Sizing: FixedFixed}),
			Column(ContainerCfg{
				ID: "outer", Scrollable: true,
				Width: 500, Height: 300, Sizing: FixedFixed,
				Content: []View{
					Rectangle(RectangleCfg{Width: 10, Height: 80, Sizing: FixedFixed}),
					Column(ContainerCfg{
						ID: "inner", Scrollable: true,
						Width: 450, Height: 200, Sizing: FixedFixed,
						Content: []View{colorLayersWidgets()[0].view()},
					}),
					Rectangle(RectangleCfg{Width: 10, Height: 400, Sizing: FixedFixed}),
				},
			}),
		}})
	})
	w.ScrollVerticalTo("outer", -90)
	w.ScrollVerticalTo(ScopeID("outer", "inner"), -150)
	w.TestRender(nil)
	cmds := w.renderers
	for _, id := range []string{"outer", ScopeID("outer", "inner")} {
		ly, ok := w.layout.FindByID(id)
		if !ok {
			t.Fatalf("setup: %s not found", id)
		}
		s := ly.Shape
		top := s.Y + s.PaddingTop()
		bottom := s.Y + s.Height - s.Padding.Bottom - s.SizeBorder
		for _, i := range colorParts(t, cmds, false) {
			clip := clipAt(w, cmds, i)
			if clip.Y < top-0.5 || clip.Y+clip.Height > bottom+0.5 {
				t.Errorf("%v at %d drawn under clip y %v..%v, outside %s %v..%v",
					cmds[i].Kind, i, clip.Y, clip.Y+clip.Height, id, top, bottom)
			}
		}
	}
}

// inBracket reports whether command i lies inside an open begin..end
// bracket.
func inBracket(cmds []RenderCmd, i int, begin, end RenderKind) bool {
	depth := 0
	for j := range i {
		switch cmds[j].Kind {
		case begin:
			depth++
		case end:
			depth--
		}
	}
	return depth > 0
}

// A color widget in a turned or scaled container, or in a RotatedBox,
// is drawn inside that container's transform: its knobs, tracks and
// swatch turn with it. As floats they were drawn outside it, unturned
// and beside the widget.
func TestColorPartsTurnWithTheirWidget(t *testing.T) {
	brackets := []struct {
		name       string
		wrap       func(View) View
		begin, end RenderKind
	}{
		{"transform", func(v View) View {
			return Column(ContainerCfg{Transform: Rotate(8).Scale(0.9), Content: []View{v}})
		}, RenderTransformBegin, RenderTransformEnd},
		{"quarter turn", func(v View) View {
			return RotatedBox(RotatedBoxCfg{QuarterTurns: 1, Content: v})
		}, RenderRotateBegin, RenderRotateEnd},
	}
	for _, b := range brackets {
		for _, wc := range colorLayersWidgets() {
			t.Run(b.name+"/"+wc.name, func(t *testing.T) {
				w := NewTestWindow(t, WindowCfg{Width: 600, Height: 700})
				w.TestRender(func(*Window) View {
					return Column(ContainerCfg{Content: []View{b.wrap(wc.view())}})
				})
				cmds := w.renderers
				for _, i := range colorParts(t, cmds, wc.knob) {
					if !inBracket(cmds, i, b.begin, b.end) {
						t.Errorf("%v at %d drawn outside the %s", cmds[i].Kind, i, b.name)
					}
				}
			})
		}
	}
}

// colorControlOf generates a plane or a wheel and returns the control
// inside its knob room (colorKnobRoom), the room's only child.
func colorControlOf(t *testing.T, v View, w *Window) Layout {
	t.Helper()
	room := generateViewLayout(v, w)
	if len(room.Children) != 1 {
		t.Fatalf("knob room has %d children, want the control alone",
			len(room.Children))
	}
	return room.Children[0]
}

// A plane and a wheel keep half a marker of room on every side, so a
// marker on their edge stays inside their box. The room has no identity
// and takes no focus: both stay on the control. The picker's plane has
// no room, because the picker's own padding holds its marker.
func TestColorKnobRoom(t *testing.T) {
	for _, tc := range []struct {
		name string
		view View
	}{
		{"plane", ColorPlane(ColorPlaneCfg{ID: "c", Size: 100, MarkerSize: 16})},
		{"wheel", ColorWheel(ColorWheelCfg{ID: "c", Size: 100, MarkerSize: 16})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			room := generateViewLayout(tc.view, &Window{})
			if p := room.Shape.Padding; p.Top != 8 || p.Right != 8 || p.Bottom != 8 || p.Left != 8 {
				t.Errorf("room padding = %+v, want 8 on every side", p)
			}
			if room.Shape.ID != "" || room.Shape.Focusable {
				t.Errorf("room has ID %q, focusable %v; want neither",
					room.Shape.ID, room.Shape.Focusable)
			}
			c := colorControlOf(t, tc.view, &Window{})
			if c.Shape.ID != "c" || !c.Shape.Focusable {
				t.Errorf("control has ID %q, focusable %v; want c, true",
					c.Shape.ID, c.Shape.Focusable)
			}
			// Arranged, the widget takes Size plus one MarkerSize: the
			// size the field docs promise.
			w := NewTestWindow(t, WindowCfg{Width: 400, Height: 400})
			w.TestRender(func(*Window) View {
				return Column(ContainerCfg{Content: []View{tc.view}})
			})
			ly, ok := w.layout.FindByID("c")
			if !ok || ly.Parent == nil {
				t.Fatal("setup: control or its room not found")
			}
			if s := ly.Parent.Shape; s.Width != 116 || s.Height != 116 {
				t.Errorf("arranged widget = %vx%v, want 116x116 (Size 100 + MarkerSize 16)",
					s.Width, s.Height)
			}
		})
	}
	t.Run("picker plane", func(t *testing.T) {
		w := NewTestWindow(t, WindowCfg{Width: 600, Height: 700})
		w.TestRender(func(*Window) View {
			return ColorPicker(ColorPickerCfg{ID: "cp"})
		})
		ly, ok := w.layout.FindByID(ScopeID("cp", "plane"))
		if !ok {
			t.Fatal("setup: picker plane not found")
		}
		if p := ly.Parent.Shape.Padding; p.Top != 0 || p.Left != 0 {
			t.Errorf("picker plane's parent has padding %+v: the plane got a knob room", p)
		}
	})
}

// A knob on the edge of its widget is drawn whole when the widget sits
// flush in a container that clips: one that fits the widget exactly, and
// a scroll view at rest, whose clip is its content box inside its
// padding. Without the knob room, these knobs were cut in half.
func TestColorKnobOnEdgeStaysWhole(t *testing.T) {
	plane := func(v HSLA) func() View {
		return func() View { return ColorPlane(ColorPlaneCfg{ID: "c", Value: v, Size: 100}) }
	}
	wheel := func(v HSLA) func() View {
		return func() View { return ColorWheel(ColorWheelCfg{ID: "c", Value: v, Size: 100}) }
	}
	picker := func(c Color) func() View {
		return func() View { return ColorPicker(ColorPickerCfg{ID: "c", Color: c}) }
	}
	// slider builds a hue slider of the given length, 0 for the
	// default. Hue, not alpha: a knob at alpha 0 has no fill to find.
	slider := func(vertical bool, length float32, v HSLA) func() View {
		return func() View {
			return ColorChannelSlider(ColorChannelSliderCfg{ID: "c",
				Channel: ChannelHue, Value: v, Vertical: vertical,
				Width: length, Height: length})
		}
	}
	// A vertical slider has its minimum at the bottom.
	hueMin, hueMax := HSLA{H: 0, S: 1, L: 0.5, A: 1}, HSLA{H: 359.9, S: 1, L: 0.5, A: 1}
	widgets := []struct {
		name  string
		view  func() View
		noPad bool // under a theme with no padding
	}{
		{"plane top", plane(HSLA{H: 200, S: 0.5, L: 1, A: 1}), false},
		{"plane bottom", plane(HSLA{H: 200, S: 0.5, L: 0, A: 1}), false},
		{"plane left", plane(HSLA{H: 200, S: 0, L: 0.5, A: 1}), false},
		{"plane right", plane(HSLA{H: 200, S: 1, L: 0.5, A: 1}), false},
		{"wheel top", wheel(HSLA{H: 90, S: 1, L: 0.5, A: 1}), false},
		{"wheel bottom", wheel(HSLA{H: 270, S: 1, L: 0.5, A: 1}), false},
		{"wheel left", wheel(HSLA{H: 180, S: 1, L: 0.5, A: 1}), false},
		{"wheel right", wheel(HSLA{H: 0, S: 1, L: 0.5, A: 1}), false},
		{"picker top-left", picker(White), false},
		{"picker bottom-left", picker(Black), false},
		{"picker top-left, no padding", picker(White), true},
		{"picker bottom-left, no padding", picker(Black), true},
		{"slider start", slider(false, 0, hueMin), false},
		{"slider end", slider(false, 0, hueMax), false},
		{"vertical slider bottom", slider(true, 0, hueMin), false},
		{"vertical slider top", slider(true, 0, hueMax), false},
		// Shorter than the 16 px thumb.
		{"short slider start", slider(false, 10, hueMin), false},
		{"short slider end", slider(false, 10, hueMax), false},
		{"short vertical slider bottom", slider(true, 10, hueMin), false},
		{"short vertical slider top", slider(true, 10, hueMax), false},
	}
	clippers := []struct {
		name string
		wrap func(View) View
	}{
		{"fitted clip", func(v View) View {
			return Column(ContainerCfg{
				Clip: true, Padding: PaddingNone, SizeBorder: NoBorder,
				Content: []View{v},
			})
		}},
		{"scroll view", func(v View) View {
			return Column(ContainerCfg{
				ID: "scroll", Scrollable: true,
				Width: 400, Height: 400, Sizing: FixedFixed,
				Content: []View{v},
			})
		}},
	}
	for _, cl := range clippers {
		for _, wc := range widgets {
			t.Run(cl.name+"/"+wc.name, func(t *testing.T) {
				w := NewTestWindow(t, WindowCfg{Width: 600, Height: 700})
				if wc.noPad {
					w.SetTheme(ThemeDark.WithPadding(false))
				}
				w.TestRender(func(*Window) View {
					return Column(ContainerCfg{Content: []View{cl.wrap(wc.view())}})
				})
				cmds := w.renderers
				knobs := 0
				for i := range cmds {
					c := cmds[i]
					if c.Kind != RenderCircle {
						continue
					}
					knobs++
					clip := clipAt(w, cmds, i)
					if c.X-c.Radius < clip.X-0.5 || c.Y-c.Radius < clip.Y-0.5 ||
						c.X+c.Radius > clip.X+clip.Width+0.5 ||
						c.Y+c.Radius > clip.Y+clip.Height+0.5 {
						t.Errorf("knob at %v,%v r %v cut by clip %+v", c.X, c.Y, c.Radius, clip)
					}
				}
				if knobs == 0 {
					t.Fatal("setup: no knob drawn")
				}
			})
		}
	}
}

// A channel slider is never shorter than its thumb. Shorter, the thumb's
// travel (length minus a thumb) was negative: the thumb stuck out past
// the control and moved backward as the value grew.
func TestColorChannelSliderNotShorterThanThumb(t *testing.T) {
	// thumbAt renders the slider and returns the control's length and
	// the thumb's centre on the swept axis, read before the next render.
	thumbAt := func(t *testing.T, vertical bool, length float32, v HSLA) (ctl, centre float32) {
		t.Helper()
		w := NewTestWindow(t, WindowCfg{Width: 400, Height: 400})
		w.TestRender(func(*Window) View {
			return ColorChannelSlider(ColorChannelSliderCfg{ID: "s",
				Channel: ChannelHue, Value: v, Vertical: vertical,
				Width: length, Height: length, ThumbSize: 16})
		})
		ly, ok := w.layout.FindByID("s")
		if !ok || len(ly.Children) == 0 {
			t.Fatal("setup: slider or its thumb not found")
		}
		s, th := ly.Shape, ly.Children[len(ly.Children)-1].Shape
		if vertical {
			return s.Height, th.Y + th.Height/2
		}
		return s.Width, th.X + th.Width/2
	}
	lo, hi := HSLA{H: 0, S: 1, L: 0.5, A: 1}, HSLA{H: 359.9, S: 1, L: 0.5, A: 1}
	for _, vertical := range []bool{false, true} {
		for _, length := range []float32{4, 10, 16, 17, 40} {
			ctl, a := thumbAt(t, vertical, length, lo)
			_, b := thumbAt(t, vertical, length, hi)
			if want := f32Max(length, 16); ctl != want {
				t.Errorf("vertical %v, length %v: control is %v long, want %v",
					vertical, length, ctl, want)
			}
			// The minimum is at the start: left, or the bottom when
			// vertical, where screen y is largest.
			if (!vertical && b < a) || (vertical && b > a) {
				t.Errorf("vertical %v, length %v: thumb moved backward from %v to %v",
					vertical, length, a, b)
			}
		}
	}
}
