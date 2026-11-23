package gui

import (
	"strings"
	"testing"
)

// Under RTL a Column or Stack pins a child wider than itself at its right
// edge, its start, and its horizontal scroll offset is mirrored the way
// an RTL Row's always was (scrollMirrorsX), so an unscrolled container
// shows the start of its content (#1345). Before, the child was pinned at
// the left edge, and a Table or DataGrid opened at its last column.

// rtlWideColumn renders a scrolling FillFill Column holding a Row of two
// 400-wide rectangles in a 300-wide window, in direction dir. It returns
// the window and the arranged scroller, first and second rectangles.
func rtlWideColumn(t *testing.T, dir TextDirection) (w *Window, s, first, second *Layout) {
	t.Helper()
	w = NewTestWindow(t, WindowCfg{Width: 300, Height: 200})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{ID: "s", Scrollable: true, Sizing: FillFill,
			Padding: PaddingNone, SizeBorder: NoBorder, TextDir: dir, Content: []View{
				Row(ContainerCfg{Padding: PaddingNone, SizeBorder: NoBorder, Spacing: NoSpacing, Content: []View{
					Rectangle(RectangleCfg{ID: "first", Width: 400, Height: 20, Sizing: FixedFixed}),
					Rectangle(RectangleCfg{ID: "second", Width: 400, Height: 20, Sizing: FixedFixed}),
				}}),
			}})
	})
	return rtlWideColumnParts(t, w, w.TestRender(nil))
}

func rtlWideColumnParts(t *testing.T, w *Window, l *Layout) (_ *Window, s, first, second *Layout) {
	t.Helper()
	find := func(id string) *Layout {
		ids := w.ResolveID(id)
		if len(ids) != 1 {
			t.Fatalf("ResolveID(%q) = %v", id, ids)
		}
		ly, ok := l.FindByID(ids[0])
		if !ok {
			t.Fatalf("%q not found", id)
		}
		return ly
	}
	return w, find("s"), find("first"), find("second")
}

// contentBoxX returns the left and right x of a container's content box.
// RTL mirrors padding (layoutChildStartPos): the left inset is
// Padding.Right there.
func contentBoxX(s *Shape) (left, right float32) {
	if effectiveTextDir(s) == TextDirRTL {
		return s.X + s.Padding.Right + s.SizeBorder,
			s.X + s.Width - s.Padding.Left - s.SizeBorder
	}
	return s.X + s.PaddingLeft(), s.X + s.Width - s.Padding.Right - s.SizeBorder
}

func TestRTLColumnScrollOpensAtStart(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	loc := saved
	loc.TextDir = TextDirRTL
	SetLocale(loc)

	w, s, first, _ := rtlWideColumn(t, TextDirRTL)
	_, right := contentBoxX(s.Shape)
	if got := first.Shape.X + first.Shape.Width; !f32AreClose(got, right) {
		t.Fatalf("unscrolled: first item right edge = %v, want the viewport's right edge %v", got, right)
	}

	id := s.Shape.idKey()
	w.ScrollHorizontalTo(id, -100000)
	_, s, first, second := rtlWideColumnParts(t, w, w.TestRender(nil))
	x, _, _ := w.TestScrollOffset(id)
	if want := scrollMaxOffsetX(s); !f32AreClose(x, want) || x >= 0 {
		t.Fatalf("scrolled to the end: offset = %v, want %v (< 0)", x, want)
	}
	left, _ := contentBoxX(s.Shape)
	if !f32AreClose(second.Shape.X, left) {
		t.Fatalf("scrolled to the end: last item x = %v, want the viewport's left edge %v",
			second.Shape.X, left)
	}
	if first.Shape.X <= right-first.Shape.Width {
		t.Fatalf("scrolled to the end: first item x = %v, want it moved off to the right", first.Shape.X)
	}
}

// The LTR control: the wider row still starts at the left edge.
func TestLTRColumnScrollOpensAtStart(t *testing.T) {
	_, s, first, _ := rtlWideColumn(t, TextDirLTR)
	left, _ := contentBoxX(s.Shape)
	if !f32AreClose(first.Shape.X, left) {
		t.Fatalf("first item x = %v, want the viewport's left edge %v", first.Shape.X, left)
	}
}

// A Column or Stack that does not scroll pins a wider child the same
// way, so it sticks out past the end edge: the left one under RTL.
func TestRTLColumnOverflowPinsAtStart(t *testing.T) {
	for _, tc := range []struct {
		name string
		make func(ContainerCfg) View
	}{
		{"column", Column},
		{"stack", Stack},
	} {
		for _, dir := range []TextDirection{TextDirLTR, TextDirRTL} {
			w := NewTestWindow(t, WindowCfg{Width: 300, Height: 200})
			w.TestRender(func(*Window) View {
				return Row(ContainerCfg{Sizing: FillFill, Padding: PaddingNone,
					HAlign: HAlignCenter, Content: []View{
						tc.make(ContainerCfg{ID: "box", Sizing: FixedFixed, Width: 100, Height: 50,
							Padding: PadAll(5), TextDir: dir, HAlign: HAlignCenter,
							Content: []View{
								Rectangle(RectangleCfg{ID: "wide", Width: 160, Height: 20, Sizing: FixedFixed}),
							}}),
					}})
			})
			l := w.TestRender(nil)
			box, ok := l.FindByID(w.ResolveID("box")[0])
			if !ok {
				t.Fatalf("%s: box not found", tc.name)
			}
			wide, ok := l.FindByID(w.ResolveID("wide")[0])
			if !ok {
				t.Fatalf("%s: wide not found", tc.name)
			}
			left, right := contentBoxX(box.Shape)
			if dir == TextDirRTL {
				if got := wide.Shape.X + wide.Shape.Width; !f32AreClose(got, right) {
					t.Errorf("%s RTL: wide child right edge = %v, want %v", tc.name, got, right)
				}
			} else if !f32AreClose(wide.Shape.X, left) {
				t.Errorf("%s LTR: wide child x = %v, want %v", tc.name, wide.Shape.X, left)
			}
		}
	}
}

// The unscrolled thumb of an RTL column sits at the right end of its
// track, where the content's start is, and moves left as it scrolls.
func TestRTLColumnThumbStartsAtRight(t *testing.T) {
	w, sc, trackX, _ := scrollbarBothRTL(t, false)
	thumb := &trackX.Children[len(trackX.Children)-1]
	trackMid := trackX.Shape.X + trackX.Shape.Width/2
	if mid := thumb.Shape.X + thumb.Shape.Width/2; mid <= trackMid {
		t.Fatalf("unscrolled: thumb center %v, want right of the track center %v", mid, trackMid)
	}
	w.ScrollHorizontalTo(sc.Shape.idKey(), scrollMaxOffsetX(sc))
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
	sc, _ = w.layout.FindByID("scroller")
	for i := range sc.Children {
		c := &sc.Children[i]
		if c.Shape.Width > c.Shape.Height && len(c.Children) > 0 {
			thumb = &c.Children[len(c.Children)-1]
			trackMid = c.Shape.X + c.Shape.Width/2
		}
	}
	if mid := thumb.Shape.X + thumb.Shape.Width/2; mid >= trackMid {
		t.Fatalf("scrolled to the end: thumb center %v, want left of the track center %v", mid, trackMid)
	}
}

// A rightward content displacement (wheel, trackpad, keyboard, touch pan)
// reveals what is off the left of an RTL column, as it does in an RTL row.
func TestRTLColumnScrollHorizontalMirrorsDelta(t *testing.T) {
	w, sc, _, _ := scrollbarBothRTL(t, false)
	if moved := scrollHorizontal(sc, 1, w); !moved {
		t.Fatal("a rightward delta must move an RTL column toward its overflow")
	}
	if got, _ := w.scrollX().Get(sc.Shape.idKey()); got >= 0 {
		t.Fatalf("offset after a rightward delta = %v, want < 0", got)
	}
}

// A DragScroll pan moves the content with the pointer in every
// direction. An RTL container's offset is mirrored, so the pointer delta
// is too; an RTL row used to pan against the pointer.
func TestDragScrollRTLFollowsPointer(t *testing.T) {
	for _, axis := range []Axis{axisLeftToRight, axisTopToBottom} {
		for _, dir := range []TextDirection{TextDirLTR, TextDirRTL} {
			w := dragScrollTestWindow(t, ScrollBoth, OnClickChild{})
			vp := w.layout.Children[0].Shape
			vp.Axis = axis
			vp.TextDir = dir
			// Start from the middle of the range so both directions can move.
			w.scrollX().Set("view", -50)
			dragScrollDown(w, 60, 60)
			dragScrollMove(w, 90, 60)
			got := dragScrollOffsetX(w)
			// Content moved 30 right: LTR offset grows by 30, RTL shrinks.
			want := float32(-20)
			if dir == TextDirRTL {
				want = -80
			}
			if got != want {
				t.Errorf("axis %v dir %v: offset after dragging right 30 = %v, want %v",
					axis, dir, got, want)
			}
			dragScrollUp(w, 90, 60)
		}
	}
}

// A drag held near the left edge of an RTL row scrolls toward the
// overflow on the left, not back toward the start.
func TestDragReorderAutoScrollRTLNearLeft(t *testing.T) {
	w, root := rtlScrollRow(t)
	w.layout = *root
	left := root.Shape.X
	right := left + root.Shape.Width
	if !dragReorderAutoScroll(left+1, left, right, "sc", dragReorderHorizontal, w) {
		t.Fatal("near the left edge: no auto-scroll")
	}
	if got, _ := w.scrollX().Get("sc"); got >= 0 {
		t.Fatalf("near the left edge: offset = %v, want < 0 (toward the overflow)", got)
	}
}

// A Tree narrower than its rows shows each row's start under RTL: the
// right end, where the expand arrow is. It used to show the far end and
// hide the arrow off the right edge.
func TestTreeRTLShowsExpandArrow(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	loc := saved
	loc.TextDir = TextDirRTL
	SetLocale(loc)

	w := NewTestWindow(t, WindowCfg{Width: 300, Height: 200})
	w.TestRender(func(*Window) View {
		return Tree(TreeCfg{ID: "tree", Sizing: FixedFixed, Width: 80, Height: 100,
			Nodes: []TreeNodeCfg{{ID: "root",
				Text:  "a node label much wider than the tree itself",
				Nodes: []TreeNodeCfg{{ID: "child", Text: "Child"}}}}})
	})
	l := w.TestRender(nil)
	tree, ok := l.FindByID(w.ResolveID("tree")[0])
	if !ok {
		t.Fatal("tree not found")
	}
	rows := treeRows(tree)
	if len(rows) == 0 {
		t.Fatal("tree has no rows")
	}
	row := rows[0].Shape
	if row.Width <= tree.Shape.Width {
		t.Fatalf("row width %.0f does not overflow the tree's %.0f; the case needs a wider label",
			row.Width, tree.Shape.Width)
	}
	if end := row.X + row.Width; end > tree.Shape.X+tree.Shape.Width {
		t.Fatalf("row start (right edge) x=%.0f is past the tree's right edge %.0f",
			end, tree.Shape.X+tree.Shape.Width)
	}
}

// A multiline Input's field scrolls sideways only to reach ink: an
// unbreakable run paints past the right edge of its text box, in either
// direction. So the field keeps the unmirrored offset under RTL, or the
// caret follow (which reads offset 0 as the left edge) and the drag
// hit test would push the text the wrong way. Review of #1345.
func TestRTLMultilineInputFieldNotMirrored(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	loc := saved
	loc.TextDir = TextDirRTL
	SetLocale(loc)

	w := caretTestWindow(200, 200)
	text := strings.Repeat("a", 60) // no space: nowhere to break
	cfg := InputCfg{ID: "in", Text: text, Mode: InputMultiline, Scrollable: true}
	_, field := arrangeInput(t, w, cfg)
	if effectiveTextDir(field.Shape) != TextDirRTL {
		t.Fatal("the field is not RTL; the case tests nothing")
	}
	if scrollMirrorsX(field.Shape) {
		t.Fatal("scrollMirrorsX(field) = true, want false: its overflow is ink on the right")
	}

	// Caret at the end of the run: the follow scrolls toward the ink,
	// and the arranged text must then end inside the viewport.
	setInputState(w, "in", inputState{CursorPos: len(text)})
	inputScrollCursorIntoView("in", text, field, w)
	if got := w.scrollX().GetOr("in", 0); got >= 0 {
		t.Fatalf("scrollX after End = %v, want < 0", got)
	}
	_, field = arrangeInput(t, w, cfg)
	txt := inputTextShape(t, field)
	_, right := contentBoxX(field.Shape)
	if end := txt.X + txt.inkOverflowW; end > right+1 {
		t.Fatalf("text ink ends at x=%v, past the viewport's right edge %v: the caret is off screen",
			end, right)
	}
}

// Every kind of child an RTL Column can hold: what is wider than the
// column ends at its right edge, whatever the child is, and an empty or
// nil child breaks nothing.
func TestRTLColumnOverflowChildKinds(t *testing.T) {
	plain := ContainerCfg{Padding: PaddingNone, SizeBorder: NoBorder}
	box := func(sizing Sizing, content ...View) View {
		cfg := plain
		cfg.ID, cfg.Sizing, cfg.Content = "wide", sizing, content
		switch sizing {
		case FixedFixed:
			cfg.Width, cfg.Height = 400, 20
		case FillFit:
			cfg.MinWidth = 400
		}
		return Column(cfg)
	}
	wide := func() View {
		return Rectangle(RectangleCfg{Width: 400, Height: 20, Sizing: FixedFixed})
	}
	nested := func(make func(ContainerCfg) View, child View) View {
		cfg := plain
		cfg.Sizing, cfg.Content = FillFit, []View{child}
		return make(cfg)
	}
	for _, tc := range []struct {
		name    string
		content func() []View
		wide    bool
	}{
		{"none", func() []View { return nil }, false},
		{"nil", func() []View { return []View{nil} }, false},
		{"fit", func() []View { return []View{box(FitFit, wide())} }, true},
		{"fixed", func() []View { return []View{box(FixedFixed)} }, true},
		{"fill", func() []View { return []View{box(FillFit, wide())} }, true},
		{"after nil", func() []View { return []View{nil, box(FixedFixed)} }, true},
		{"in column", func() []View { return []View{nested(Column, box(FixedFixed))} }, true},
		{"in stack", func() []View { return []View{nested(Stack, box(FixedFixed))} }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{Width: 300, Height: 200})
			w.TestRender(func(*Window) View {
				cfg := plain
				cfg.ID, cfg.Scrollable, cfg.Sizing = "s", true, FillFill
				cfg.TextDir, cfg.Content = TextDirRTL, tc.content()
				return Column(cfg)
			})
			l := w.TestRender(nil)
			s, ok := l.FindByID("s")
			if !ok {
				t.Fatal("column not found")
			}
			if !tc.wide {
				if got := scrollMaxOffsetX(s); got != 0 {
					t.Fatalf("max offset = %v, want 0", got)
				}
				return
			}
			c, ok := l.FindByID(ScopeID("s", "wide"))
			if !ok {
				t.Fatal("wide child not found")
			}
			_, right := contentBoxX(s.Shape)
			if got := c.Shape.X + c.Shape.Width; c.Shape.Width < 400 || !f32AreClose(got, right) {
				t.Fatalf("child spans %v..%v, want 400 wide ending at %v", c.Shape.X, got, right)
			}
		})
	}
}
