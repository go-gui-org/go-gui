package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// Group-box titles (#1238). The title sits on the container's top
// border, in a band the box reserves above it. It used to be two Float
// children, half outside the box, which put it in the popup layer
// stack: any Z-0 popup earlier in the tree (a tooltip, a Select
// dropdown) was drawn under it, and it escaped every ancestor clip.
// These tests pin what is drawn, in which order and under which clip,
// from the emitted render commands. group_box_title_test.go covers the
// body inset, brackets, opacity, RTL and the eraser's limits.

const titleTestText = "Group"

// renderTitled renders view in a fresh 400×400 test window.
func renderTitled(t *testing.T, cfg WindowCfg, view func(*Window) View) *Window {
	t.Helper()
	if cfg.Width == 0 {
		cfg.Width, cfg.Height = 400, 400
	}
	w := NewTestWindow(t, cfg)
	w.TestRender(view)
	return w
}

// cmdIndex returns the index of the first command match accepts, or -1.
func cmdIndex(cmds []RenderCmd, match func(RenderCmd) bool) int {
	for i := range cmds {
		if match(cmds[i]) {
			return i
		}
	}
	return -1
}

// isText matches a plain text command drawing exactly s.
func isText(s string) func(RenderCmd) bool {
	return func(c RenderCmd) bool {
		return c.Kind == RenderText && c.Text == s
	}
}

// clipAt returns the scissor in force at command i: the last clip
// command before it, or the window when none was emitted.
func clipAt(w *Window, cmds []RenderCmd, i int) drawClip {
	for j := i - 1; j >= 0; j-- {
		if cmds[j].Kind == RenderClip {
			c := cmds[j]
			return drawClip{X: c.X, Y: c.Y, Width: c.W, Height: c.H}
		}
	}
	return w.windowRect()
}

// titleEraser returns the eraser drawn for the title text command at
// ti: the fill emitted right before it, starting groupTitlePad before
// the text and no higher than the text's top.
func titleEraser(cmds []RenderCmd, ti int) (RenderCmd, bool) {
	if ti <= 0 {
		return RenderCmd{}, false
	}
	c, text := cmds[ti-1], cmds[ti]
	if c.Kind != RenderRect || !c.Fill || c.X != text.X-groupTitlePad ||
		c.Y < text.Y {
		return RenderCmd{}, false
	}
	return c, true
}

func titledRadioGroup(bg Color) View {
	return RadioButtonGroupRow(RadioButtonGroupCfg{
		ID:      "orientation",
		Title:   titleTestText,
		TitleBG: bg,
		Options: []RadioOption{
			NewRadioOption("Portrait", "p"),
			NewRadioOption("Landscape", "l"),
		},
	})
}

// The issue's layout: a button with a tooltip, then a titled radio
// group below it. The tooltip hangs over the group's title and must be
// drawn after all of it.
func TestGroupBoxTitleDrawsUnderTooltip(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 400})
	w.viewState.tooltip.id = "tip"
	w.viewState.tooltip.popupID = ScopeID("tip", "popup")
	w.TestRender(func(vw *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			WithTooltip(vw, WithTooltipCfg{
				ID:   "tip",
				Text: "tooltip text",
				Content: []View{Rectangle(RectangleCfg{
					Width: 100, Height: 50, Sizing: FixedFixed,
				})},
			}),
			titledRadioGroup(RGB(240, 240, 240)),
		}})
	})
	cmds := w.renderers
	title := cmdIndex(cmds, isText(titleTestText))
	tip := cmdIndex(cmds, isText("tooltip text"))
	if title < 0 || tip < 0 {
		t.Fatalf("title at %d, tooltip at %d: both must be drawn", title, tip)
	}
	if title > tip {
		t.Errorf("title drawn at %d, after the tooltip at %d", title, tip)
	}
	if e, ok := titleEraser(cmds, title); !ok {
		t.Error("no eraser drawn before the title")
	} else if e.Color != RGB(240, 240, 240) {
		t.Errorf("eraser = %v, want TitleBG", e.Color)
	}
}

// The same holds for every Z-0 popup, not only tooltips: an open Select
// above a titled container drops its list over the title.
func TestGroupBoxTitleDrawsUnderSelectDropdown(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 400})
	view := func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Select(SelectCfg{ID: "sel", Items: []string{"one", "two", "three"}}),
			Column(ContainerCfg{
				Title:   titleTestText,
				Content: []View{Text(TextCfg{Text: "body"})},
			}),
		}})
	}
	w.TestRender(view)
	w.SetFocus("sel")
	StateMap[string, bool](w, nsSelect, capModerate).Set("sel", true)
	w.TestRender(nil)
	cmds := w.renderers
	title := cmdIndex(cmds, isText(titleTestText))
	item := cmdIndex(cmds, isText("three"))
	if title < 0 || item < 0 {
		t.Fatalf("title at %d, dropdown item at %d: both must be drawn",
			title, item)
	}
	if title > item {
		t.Errorf("title drawn at %d, after the dropdown item at %d",
			title, item)
	}
}

// scrolledGroupBox puts a titled container in a 150 px scroll viewport
// below a 100 px header, and scrolls the viewport by offset.
func scrolledGroupBox(t *testing.T, offset float32) (*Window, *Layout) {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 400})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Rectangle(RectangleCfg{Width: 300, Height: 100, Sizing: FixedFixed}),
			Column(ContainerCfg{
				ID: "scroll", Scrollable: true,
				Width: 300, Height: 150, Sizing: FixedFixed,
				Content: []View{
					Rectangle(RectangleCfg{Width: 200, Height: 60, Sizing: FixedFixed}),
					Column(ContainerCfg{
						Title: titleTestText,
						Content: []View{Rectangle(RectangleCfg{
							Width: 200, Height: 300, Sizing: FixedFixed,
						})},
					}),
				},
			}),
		}})
	})
	w.ScrollVerticalTo("scroll", offset)
	w.TestRender(nil)
	sc, ok := w.layout.FindByID("scroll")
	if !ok {
		t.Fatal("scroll container not found")
	}
	return w, sc
}

// A title scrolled out of the top of a viewport is not drawn over the
// header above it: it is clipped like the border it sits on. The
// offsets come from the box's place at rest: one keeps the title inside
// the viewport, one makes it straddle the viewport's content edge, and
// both must be drawn under the viewport's clip; one puts it wholly
// above the edge, where it is culled.
func TestGroupBoxTitleClipsToScrollViewport(t *testing.T) {
	_, rest := scrolledGroupBox(t, 0)
	box := rest.Children[1].Shape
	h := box.fx.groupTitle.Height
	// The offset at which the title's top meets the content edge.
	edge := rest.Shape.Y + rest.Shape.PaddingTop() - box.Y
	for _, tc := range []struct {
		name   string
		offset float32
		drawn  bool
	}{
		{"inside", edge + 2, true},
		{"straddling", edge - h/2, true},
		{"above", edge - h - 2, false},
	} {
		w, sc := scrolledGroupBox(t, tc.offset)
		cmds := w.renderers
		ti := cmdIndex(cmds, isText(titleTestText))
		if !tc.drawn {
			if ti >= 0 {
				t.Errorf("%s (offset %v): title drawn wholly above the viewport", tc.name, tc.offset)
			}
			continue
		}
		if ti < 0 {
			t.Errorf("%s (offset %v): title not drawn", tc.name, tc.offset)
			continue
		}
		top := sc.Shape.Y + sc.Shape.PaddingTop()
		if clip := clipAt(w, cmds, ti); clip.Y < top {
			t.Errorf("%s (offset %v): title drawn under clip top %v, above the viewport's content top %v",
				tc.name, tc.offset, clip.Y, top)
		}
	}
}

// The title reserves its own room inside the box: the border is drawn
// half a title below the box's top, and the title starts at the top.
// So a group box that is the first child of a scroll viewport, whose
// clip is its content box, keeps its whole title at rest. While the
// title stuck out above the box, the viewport cut off its top half.
func TestGroupBoxTitleFitsInsideTheBox(t *testing.T) {
	w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Column(ContainerCfg{
				ID: "scroll", Scrollable: true, Height: 200, Sizing: FillFixed,
				Content: []View{Column(ContainerCfg{
					ID:      "box",
					Title:   titleTestText,
					Content: []View{Text(TextCfg{Text: "body"})},
				})},
			}),
		}})
	})
	box, ok := w.layout.FindByID("scroll:box")
	if !ok {
		t.Fatal("box not found")
	}
	cmds := w.renderers
	ti := cmdIndex(cmds, isText(titleTestText))
	if ti < 0 {
		t.Fatal("title not drawn")
	}
	title := cmds[ti]
	band := box.Shape.fx.groupTitle.Height / 2
	if title.Y != box.Shape.Y {
		t.Errorf("title at y=%v, want the box's top %v", title.Y, box.Shape.Y)
	}
	if clip := clipAt(w, cmds, ti); clip.Y > title.Y {
		t.Errorf("title at y=%v drawn under clip top %v: its top is cut", title.Y, clip.Y)
	}
	border := cmdIndex(cmds, func(c RenderCmd) bool {
		return c.Kind == RenderStrokeRect && c.X == box.Shape.X
	})
	if border < 0 {
		t.Fatal("border not drawn")
	}
	if got, want := cmds[border].Y, box.Shape.Y+band; got != want {
		t.Errorf("border top = %v, want %v (half a title below the box's top)", got, want)
	}
	if got, want := cmds[border].H, box.Shape.Height-band; got != want {
		t.Errorf("border height = %v, want %v", got, want)
	}
}

// A titled container that clips its own content (a scrolling group box)
// must not clip its own title: the title lies on the border, outside
// the content box.
func TestGroupBoxTitleOnClippingContainerIsDrawn(t *testing.T) {
	w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Column(ContainerCfg{
				ID: "box", Title: titleTestText, Scrollable: true,
				Width: 300, Height: 100, Sizing: FixedFixed,
				Content: []View{Rectangle(RectangleCfg{
					Width: 200, Height: 300, Sizing: FixedFixed,
				})},
			}),
		}})
	})
	cmds := w.renderers
	ti := cmdIndex(cmds, isText(titleTestText))
	if ti < 0 {
		t.Fatal("title not drawn")
	}
	box, _ := w.layout.FindByID("box")
	clip := clipAt(w, cmds, ti)
	if clip.Y > cmds[ti].Y {
		t.Errorf("title at y=%v drawn under clip top %v (box top %v)",
			cmds[ti].Y, clip.Y, box.Shape.Y)
	}
}

// The title is drawn after the container's content, like a legend over
// its border.
func TestGroupBoxTitleDrawsAfterContent(t *testing.T) {
	w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Column(ContainerCfg{
				Title:   titleTestText,
				Content: []View{Text(TextCfg{Text: "body"})},
			}),
		}})
	})
	cmds := w.renderers
	title := cmdIndex(cmds, isText(titleTestText))
	body := cmdIndex(cmds, isText("body"))
	if title < 0 || body < 0 || title < body {
		t.Errorf("title at %d, body at %d: want the title after the body",
			title, body)
	}
}

// An unset TitleBG takes the color drawn behind the box, so the
// border does not run through the title (the issue's second report).
func TestGroupBoxTitleEraserDefault(t *testing.T) {
	panel := RGB(10, 60, 90)
	half := RGBA(255, 0, 0, 128)
	grad := &GradientDef{Stops: []GradientStop{
		{Color: RGB(0, 0, 0), Pos: 0}, {Color: RGB(255, 255, 255), Pos: 1},
	}}
	titled := func(cfg ContainerCfg) View {
		cfg.Title = titleTestText
		cfg.Content = []View{Text(TextCfg{Text: "body"})}
		return Column(cfg)
	}
	on := func(parent ContainerCfg, box ContainerCfg) func(*Window) View {
		return func(_ *Window) View {
			parent.Sizing = FillFill
			parent.Padding = PadAll(20)
			parent.Content = []View{titled(box)}
			return Column(parent)
		}
	}
	cases := []struct {
		name   string
		window WindowCfg
		view   func(*Window) View
		// want returns the expected eraser; ok false means none.
		want func(w *Window) (Color, bool)
	}{
		{
			name: "window background",
			view: on(ContainerCfg{}, ContainerCfg{}),
			want: func(w *Window) (Color, bool) { return w.FrameBackground(), true },
		},
		{
			name: "solid panel",
			view: on(ContainerCfg{Color: panel}, ContainerCfg{}),
			want: func(*Window) (Color, bool) { return panel, true },
		},
		{
			name: "translucent panel over the window",
			view: on(ContainerCfg{Color: half}, ContainerCfg{}),
			want: func(w *Window) (Color, bool) {
				return half.Over(w.FrameBackground()), true
			},
		},
		{
			name: "disabled panel",
			view: on(ContainerCfg{Color: panel, Disabled: true}, ContainerCfg{}),
			want: func(w *Window) (Color, bool) {
				return dimAlpha(panel).Over(w.FrameBackground()), true
			},
		},
		{
			name: "gradient panel: no flat color matches",
			view: on(ContainerCfg{Gradient: grad}, ContainerCfg{}),
			want: func(*Window) (Color, bool) { return Color{}, false },
		},
		{
			name:   "see-through window",
			window: WindowCfg{Transparent: true},
			view:   on(ContainerCfg{}, ContainerCfg{}),
			want:   func(*Window) (Color, bool) { return Color{}, false },
		},
		{
			name: "no border to hide",
			view: on(ContainerCfg{Color: panel}, ContainerCfg{SizeBorder: NoBorder}),
			want: func(*Window) (Color, bool) { return Color{}, false },
		},
		{
			name: "explicit TitleBG wins",
			view: on(ContainerCfg{Color: panel}, ContainerCfg{TitleBG: RGB(1, 2, 3)}),
			want: func(*Window) (Color, bool) { return RGB(1, 2, 3), true },
		},
		{
			name: "explicit ColorTransparent turns the eraser off",
			view: on(ContainerCfg{Color: panel}, ContainerCfg{TitleBG: ColorTransparent}),
			want: func(*Window) (Color, bool) { return Color{}, false },
		},
		{
			name: "disabled box keeps an explicit TitleBG undimmed",
			view: on(ContainerCfg{}, ContainerCfg{TitleBG: panel, Disabled: true}),
			want: func(*Window) (Color, bool) { return panel, true },
		},
		{
			// The border lies in the box's body, over the box's own
			// fill, so that is what the eraser must match.
			name: "box with its own fill",
			view: on(ContainerCfg{Color: panel}, ContainerCfg{Color: RGB(200, 200, 40)}),
			want: func(*Window) (Color, bool) { return RGB(200, 200, 40), true },
		},
		{
			name: "box with a translucent fill",
			view: on(ContainerCfg{Color: panel}, ContainerCfg{Color: half}),
			want: func(*Window) (Color, bool) { return half.Over(panel), true },
		},
		{
			name: "box with a gradient fill",
			view: on(ContainerCfg{Color: panel}, ContainerCfg{Gradient: grad}),
			want: func(*Window) (Color, bool) { return Color{}, false },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderTitled(t, tc.window, tc.view)
			cmds := w.renderers
			ti := cmdIndex(cmds, isText(titleTestText))
			if ti < 0 {
				t.Fatal("title not drawn")
			}
			got, drawn := titleEraser(cmds, ti)
			want, wantDrawn := tc.want(w)
			if drawn != wantDrawn {
				t.Fatalf("eraser drawn = %v (%v), want %v", drawn, got.Color, wantDrawn)
			}
			if drawn && got.Color != want {
				t.Errorf("eraser = %v, want %v", got.Color, want)
			}
		})
	}
}

// The default eraser covers the title's lower half only, from the
// border line down, where the border is drawn; the band above it shows
// what is behind the box anyway. It starts half a border width above
// the line, where the web and PDF backends draw half the stroke. An
// explicit TitleBG is a patch behind the whole label.
func TestGroupBoxTitleEraserExtent(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		var bg Color
		if explicit {
			bg = RGB(250, 220, 60)
		}
		w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
			return Column(ContainerCfg{Sizing: FillFill, Content: []View{
				Column(ContainerCfg{ID: "box", Title: titleTestText, TitleBG: bg}),
			}})
		})
		box, ok := w.layout.FindByID("box")
		if !ok {
			t.Fatal("box not found")
		}
		cmds := w.renderers
		ti := cmdIndex(cmds, isText(titleTestText))
		e, drawn := titleEraser(cmds, ti)
		if !drawn {
			t.Fatalf("explicit=%v: no eraser", explicit)
		}
		h := box.Shape.fx.groupTitle.Height
		sb := box.Shape.SizeBorder
		wantY, wantH := box.Shape.Y+h/2-sb/2, h-h/2+sb/2
		if explicit {
			wantY, wantH = box.Shape.Y, h
		}
		if e.Y != wantY || e.H != wantH {
			t.Errorf("explicit=%v: eraser y=%v h=%v, want y=%v h=%v",
				explicit, e.Y, e.H, wantY, wantH)
		}
	}
}

// A disabled group box dims its title once, with the theme's disabled
// amount, like every other text.
func TestGroupBoxTitleDisabledTextDimsOnce(t *testing.T) {
	ink := Hex(0x4a6b8a)
	for _, inherited := range []bool{false, true} {
		w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
			box := Column(ContainerCfg{
				Title: titleTestText, ColorBorder: ink, Disabled: !inherited,
			})
			return Column(ContainerCfg{
				Sizing: FillFill, Disabled: inherited, Content: []View{box},
			})
		})
		ti := cmdIndex(w.renderers, isText(titleTestText))
		if ti < 0 {
			t.Fatalf("inherited=%v: title not drawn", inherited)
		}
		want := w.Theme().disabledTextColor(ink)
		if got := w.renderers[ti].Color; got != want {
			t.Errorf("inherited=%v: title color = %v, want %v", inherited, got, want)
		}
	}
}

// A titled container inside a floating panel draws its title over the
// panel, not under it. An opaque panel is the eraser's color. Through a
// see-through panel shows whatever lies at the float's position, not
// its anchor, which the float's Parent pointer still leads to: the
// red anchor below must never become the eraser.
func TestGroupBoxTitleInsideFloatingPanel(t *testing.T) {
	anchor := RGB(200, 0, 0)
	cases := []struct {
		name   string
		panel  Color
		eraser bool
	}{
		{"opaque panel", RGB(50, 50, 80), true},
		{"translucent panel", RGBA(50, 50, 80, 128), false},
		{"unfilled panel", Color{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
				return Column(ContainerCfg{Sizing: FillFill, Content: []View{
					Column(ContainerCfg{
						Width: 50, Height: 50, Sizing: FixedFixed, Color: anchor,
						Content: []View{Column(ContainerCfg{
							Float:   true,
							Color:   tc.panel,
							Padding: PadAll(20),
							Content: []View{Column(ContainerCfg{
								Title:   titleTestText,
								Content: []View{Text(TextCfg{Text: "body"})},
							})},
						})},
					}),
				}})
			})
			cmds := w.renderers
			ti := cmdIndex(cmds, isText(titleTestText))
			if ti < 0 {
				t.Fatal("title not drawn")
			}
			if tc.panel.A > 0 {
				fill := cmdIndex(cmds, func(c RenderCmd) bool {
					return c.Kind == RenderRect && c.Fill && c.Color == tc.panel
				})
				if fill < 0 || ti < fill {
					t.Errorf("panel fill at %d, title at %d: want the title after the fill",
						fill, ti)
				}
			}
			e, ok := titleEraser(cmds, ti)
			if ok != tc.eraser {
				t.Fatalf("eraser drawn = %v (%v), want %v", ok, e.Color, tc.eraser)
			}
			if ok && e.Color != tc.panel {
				t.Errorf("eraser = %v, want the panel color %v", e.Color, tc.panel)
			}
		})
	}
}

// The title lives on the container's shape, drawn by the container, not
// among its children: the Content children are the only children. The
// label takes the bold title role (B3, visual-refresh §2.2) and the
// container's border color.
func TestGroupBoxTitleIsNoChild(t *testing.T) {
	v := Column(ContainerCfg{
		ID:          "gb",
		Title:       titleTestText,
		TitleBG:     RGB(30, 30, 30),
		ColorBorder: RGB(100, 100, 100),
		Content:     []View{Text(TextCfg{Text: "child"})},
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1 (the content alone)",
			len(layout.Children))
	}
	if c := layout.Children[0].Shape; c.TC == nil || c.TC.Text != "child" {
		t.Errorf("children[0] = %+v, want the content text", c)
	}
	fx := layout.Shape.fx
	if fx == nil || fx.groupTitle == nil {
		t.Fatal("container carries no group-box title")
	}
	label := fx.groupTitle
	if label.shapeType != shapeText || label.Float || label.TC.Text != titleTestText {
		t.Errorf("title = %+v, want a non-floating text shape %q",
			label, titleTestText)
	}
	if got := label.TC.TextStyle.Typeface; got != glyph.TypefaceBold {
		t.Errorf("title typeface = %d, want bold (B3, visual-refresh §2.2)", got)
	}
	if got := label.TC.TextStyle.Color; got != RGB(100, 100, 100) {
		t.Errorf("title color = %v, want the border color", got)
	}
	if fx.titleBG != RGB(30, 30, 30) {
		t.Errorf("titleBG = %v, want TitleBG", fx.titleBG)
	}
}

// An empty group box keeps its padding on both axes, its top padding
// grown by the title's band, as it did while its title was two Float
// children; an empty untitled container still collapses to 0 on the
// cross axis (the closed Sidebar relies on it).
func TestEmptyGroupBoxKeepsItsPadding(t *testing.T) {
	for _, row := range []bool{false, true} {
		build := Column
		if row {
			build = Row
		}
		arrange := func(title string) Layout {
			l := generateViewLayout(build(ContainerCfg{Title: title, Padding: PadAll(10)}), &Window{})
			layoutWidths(&l)
			layoutHeights(&l)
			return l
		}
		titled := arrange(titleTestText)
		s := titled.Shape
		if want := 10 + s.fx.groupTitle.Height/2; s.Padding.Top != want {
			t.Errorf("row=%v: top padding = %v, want %v", row, s.Padding.Top, want)
		}
		if s.Width != s.paddingWidth() || s.Height != s.paddingHeight() {
			t.Errorf("row=%v: empty group box = %vx%v, want its padding %vx%v",
				row, s.Width, s.Height, s.paddingWidth(), s.paddingHeight())
		}
		u := arrange("").Shape
		if row && u.Height != 0 || !row && u.Width != 0 {
			t.Errorf("row=%v: empty untitled container = %vx%v, want 0 on the cross axis",
				row, u.Width, u.Height)
		}
	}
}

// The title is not a child, and the only room it takes is the band it
// reserves above the border: a titled container lays out its content,
// and sizes itself, exactly like an untitled one whose top padding is
// half a title taller. For every kind of child, in a Column and a Row,
// fixed and fit (gui/CLAUDE.md, Container children). The top padding
// here clears half the title, so the title adds nothing below the
// border (TestGroupBoxTitleClearsFirstChild covers the other case).
func TestGroupBoxTitleLeavesChildrenAlone(t *testing.T) {
	kids := map[string][]View{
		"none": nil,
		"nil":  {nil, Text(TextCfg{Text: "after nil"})},
		"fit":  {Text(TextCfg{Text: "fit"})},
		"fixed": {Rectangle(RectangleCfg{
			Width: 40, Height: 30, Sizing: FixedFixed,
		})},
		"fill": {Rectangle(RectangleCfg{Sizing: FillFill})},
		"nested": {Column(ContainerCfg{
			Title:   "inner",
			Content: []View{Text(TextCfg{Text: "deep"})},
		})},
	}
	axes := map[string]func(ContainerCfg) View{"column": Column, "row": Row}
	sizings := map[string]Sizing{"fixed": FixedFixed, "fit": FitFit}
	for name, content := range kids {
		for axisName, build := range axes {
			for sizingName, sizing := range sizings {
				t.Run(name+"/"+axisName+"/"+sizingName, func(t *testing.T) {
					arrange := func(title string, padTop float32) *Layout {
						w := renderTitled(t, WindowCfg{}, func(_ *Window) View {
							return Column(ContainerCfg{Sizing: FillFill, Content: []View{
								build(ContainerCfg{
									ID:          "box",
									Title:       title,
									ColorBorder: RGB(90, 90, 90),
									SizeBorder:  BorderPx(1),
									Padding:     NewPadding(padTop, 10, 10, 10),
									Width:       200, Height: 120, Sizing: sizing,
									Content: content,
								}),
							}})
						})
						box, ok := w.layout.FindByID("box")
						if !ok {
							t.Fatal("box not found")
						}
						return box
					}
					titled := arrange(titleTestText, 10)
					band := titled.Shape.fx.groupTitle.Height / 2
					if band <= 0 {
						t.Fatalf("title band = %v, want > 0", band)
					}
					plain := arrange("", 10+band)
					if p, q := plain.Shape, titled.Shape; p.Width != q.Width || p.Height != q.Height {
						t.Errorf("box %vx%v, want %vx%v", q.Width, q.Height, p.Width, p.Height)
					}
					if len(titled.Children) != len(plain.Children) {
						t.Fatalf("children = %d, want %d (the title is no child)",
							len(titled.Children), len(plain.Children))
					}
					for i := range plain.Children {
						p, q := plain.Children[i].Shape, titled.Children[i].Shape
						if p.X != q.X || p.Y != q.Y || p.Width != q.Width || p.Height != q.Height {
							t.Errorf("child %d at %v,%v %vx%v, want %v,%v %vx%v",
								i, q.X, q.Y, q.Width, q.Height, p.X, p.Y, p.Width, p.Height)
						}
					}
				})
			}
		}
	}
}
