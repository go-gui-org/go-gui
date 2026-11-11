package gui

import (
	"testing"
	"time"
)

// Tests for #1243: a WithTooltip wrapper opens its tooltip only when
// the topmost shape under the pointer belongs to it, not whenever the
// pointer is inside its box.

// tipMove moves the pointer and runs the frames the move asks for.
func tipMove(w *Window, x, y float32) {
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	w.settle()
}

// tipShowNow ends the hover delay of the hovered wrapper and runs a
// frame, which opens its tooltip.
func tipShowNow(t *testing.T, w *Window, popupID string) {
	t.Helper()
	w.viewState.tooltip.hoverStart = time.Now().Add(-time.Hour)
	w.TestRender(nil)
	if _, ok := w.layout.FindByID(popupID); !ok {
		t.Fatalf("control: tooltip %q not shown", popupID)
	}
}

// tipCenterView is the reproduction from the issue: a tooltip button in
// the middle of the window.
func tipCenterView(w *Window) View {
	return Column(ContainerCfg{
		Sizing: FillFill, HAlign: HAlignCenter, VAlign: VAlignMiddle,
		Content: []View{
			WithTooltip(w, WithTooltipCfg{ID: "tip", Text: "tooltip",
				Content: []View{Button(ButtonCfg{ID: "btn",
					Content: []View{Text(TextCfg{Text: "Under the dialog"})}})}}),
		},
	})
}

func newTipWindow(t *testing.T, view func(*Window) View) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	t.Cleanup(w.WindowCleanup)
	w.TestRender(view)
	return w
}

// A modal dialog keeps the tooltip of a widget behind it shut, as it
// keeps that widget's hover style off.
func TestWithTooltipShutUnderDialog(t *testing.T) {
	w := newTipWindow(t, tipCenterView)
	x, y := hitPoint(t, w, "btn")
	tipMove(w, x, y)
	if got := w.viewState.tooltip.hoverID; got != "tip" {
		t.Fatalf("control: hoverID = %q, want tip", got)
	}
	tipMove(w, 1, 1)

	w.Dialog(DialogCfg{Title: "Modal",
		Body: "A message dialog that covers the middle of the window."})
	w.TestRender(nil)
	tipMove(w, x, y)
	if got := w.viewState.tooltip.hoverID; got != "" {
		t.Fatalf("pointer on the dialog: hoverID = %q, want none", got)
	}
}

// A tooltip that is showing when a dialog opens over its widget goes in
// that frame.
func TestWithTooltipShownGoesWhenDialogOpens(t *testing.T) {
	w := newTipWindow(t, tipCenterView)
	x, y := hitPoint(t, w, "btn")
	tipMove(w, x, y)
	tipShowNow(t, w, "tip:popup")

	w.Dialog(DialogCfg{Title: "Modal", Body: "Covers the tooltip button."})
	w.TestRender(nil)
	if got := w.viewState.tooltip.id; got != "" {
		t.Fatalf("tooltip id = %q, want none", got)
	}
	if _, ok := w.layout.FindByID("tip:popup"); ok {
		t.Fatal("tooltip still drawn in the frame the dialog opened")
	}
}

// tipCoverView is a 200×60 tooltip wrapper at the window origin, with a
// handler-less float over its left half when *cover is set. Nothing in
// it carries an ID, so the hover target stays "" whatever the pointer
// is on and asks for no extra layout pass by itself.
func tipCoverView(cover *bool) func(*Window) View {
	return func(w *Window) View {
		content := []View{
			WithTooltip(w, WithTooltipCfg{ID: "tip", Text: "tooltip",
				Content: []View{Column(ContainerCfg{
					Sizing: FixedFixed, Width: 200, Height: 60,
					SizeBorder: NoBorder,
				})}}),
		}
		if *cover {
			content = append(content, Column(ContainerCfg{
				Float:       true,
				FloatAnchor: FloatTopLeft,
				FloatTieOff: FloatTopLeft,
				Sizing:      FixedFixed, Width: 100, Height: 60,
				Color:      RGB(40, 40, 40),
				SizeBorder: NoBorder,
			}))
		}
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: content,
		})
	}
}

// Any layer over the wrapper counts, including one that handles no
// events, and only where it covers the wrapper.
func TestWithTooltipShutUnderFloat(t *testing.T) {
	cover := false
	w := newTipWindow(t, tipCoverView(&cover))
	tipMove(w, 50, 30)
	if got := w.viewState.tooltip.hoverID; got != "tip" {
		t.Fatalf("control: hoverID = %q, want tip", got)
	}
	tipMove(w, 300, 200)

	cover = true
	w.TestRender(nil)
	tipMove(w, 50, 30)
	if got := w.viewState.tooltip.hoverID; got != "" {
		t.Fatalf("pointer on the float: hoverID = %q, want none", got)
	}
	tipMove(w, 150, 30)
	if got := w.viewState.tooltip.hoverID; got != "tip" {
		t.Fatalf("pointer beside the float: hoverID = %q, want tip", got)
	}
}

// A float that opens over a showing tooltip's widget, with the pointer
// still, removes the tooltip in that frame. Nothing else in the frame
// asks for the second pass here: the hover target does not change.
func TestWithTooltipShownGoesWhenFloatCovers(t *testing.T) {
	cover := false
	w := newTipWindow(t, tipCoverView(&cover))
	tipMove(w, 50, 30)
	tipShowNow(t, w, "tip:popup")

	cover = true
	w.TestRender(nil)
	if got := w.viewState.tooltip.id; got != "" {
		t.Fatalf("tooltip id = %q, want none", got)
	}
	if _, ok := w.layout.FindByID("tip:popup"); ok {
		t.Fatal("tooltip still drawn in the frame the float opened")
	}
}

// The case from the issue seen in a running app: the pointer on an open
// Select list entry that lies over a tooltip button.
func TestWithTooltipShutUnderSelectList(t *testing.T) {
	w := newTipWindow(t, func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{
				Select(SelectCfg{ID: "sel", MinWidth: 300, Items: []string{
					"one", "two", "three", "four", "five", "six"}}),
				WithTooltip(w, WithTooltipCfg{ID: "tip", Text: "tooltip",
					Content: []View{Button(ButtonCfg{ID: "btn",
						Content: []View{Text(TextCfg{Text: "Under the list"})}})}}),
			},
		})
	})
	x, y := hitPoint(t, w, "btn")
	tipMove(w, x, y)
	if got := w.viewState.tooltip.hoverID; got != "tip" {
		t.Fatalf("control: hoverID = %q, want tip", got)
	}

	if err := w.TestClick("sel"); err != nil {
		t.Fatalf("TestClick(sel): %v", err)
	}
	list, ok := w.layout.FindByID(ScopeID("sel", "dropdown"))
	if !ok {
		t.Fatal("dropdown not open")
	}
	if !list.Shape.PointInShape(x, y) {
		t.Fatalf("setup: the list does not cover the button at %v,%v", x, y)
	}
	tipMove(w, x, y)
	if got := w.viewState.tooltip.hoverID; got != "" {
		t.Fatalf("pointer on the list: hoverID = %q, want none", got)
	}
}

// The part of a wrapper a clip hides is not under the pointer: a
// tooltip button half scrolled out of a viewport opens only from the
// half that shows.
func TestWithTooltipShutWhereClipped(t *testing.T) {
	w := newTipWindow(t, func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{
				Column(ContainerCfg{
					ID: "scroll", Scrollable: true,
					Sizing: FixedFixed, Width: 300, Height: 100,
					Padding: PaddingNone, SizeBorder: NoBorder,
					Spacing: SpacingPx(0),
					Content: []View{
						Column(ContainerCfg{Sizing: FixedFixed,
							Width: 200, Height: 40, SizeBorder: NoBorder}),
						WithTooltip(w, WithTooltipCfg{ID: "tip", Text: "tooltip",
							Content: []View{Column(ContainerCfg{
								ID:     "box",
								Sizing: FixedFixed, Width: 200, Height: 60,
								SizeBorder: NoBorder,
							})}}),
					},
				}),
			},
		})
	})
	box := mustShape(t, w, "scroll:box")
	shown := box.shapeClip.Y + box.shapeClip.Height
	if shown >= box.Y+box.Height-5 {
		t.Fatalf("setup: box %v..%v not clipped (shows to %v)",
			box.Y, box.Y+box.Height, shown)
	}
	x := box.X + 50
	tipMove(w, x, shown+5)
	if got := w.viewState.tooltip.hoverID; got != "" {
		t.Fatalf("pointer on the clipped part: hoverID = %q, want none", got)
	}
	// Clear of the scrollbar strip along the viewport's bottom edge.
	tipMove(w, x, shown-20)
	if got := w.viewState.tooltip.hoverID; got != "scroll:tip" {
		t.Fatalf("pointer on the shown part: hoverID = %q, want scroll:tip", got)
	}
}

// The tooltip hangs 3 px over the bottom of its wrapper. The pointer on
// that strip is still on the wrapper: the tooltip stays, rather than
// closing and reopening each delay.
func TestWithTooltipOwnPopupKeepsHover(t *testing.T) {
	w := newTipWindow(t, tipCenterView)
	x, y := hitPoint(t, w, "btn")
	tipMove(w, x, y)
	tipShowNow(t, w, "tip:popup")

	btn, _ := w.layout.FindByID("btn")
	wrap := btn.Parent.Shape
	popup, _ := w.layout.FindByID("tip:popup")
	py := popup.Shape.Y + 1
	if py >= wrap.Y+wrap.Height || !popup.Shape.PointInShape(x, py) {
		t.Fatalf("setup: popup top %v does not overlap wrapper bottom %v",
			popup.Shape.Y, wrap.Y+wrap.Height)
	}
	tipMove(w, x, py)
	if got := w.viewState.tooltip.id; got != "tip" {
		t.Fatalf("pointer on the popup's overlap: tooltip id = %q, want tip", got)
	}
	if _, ok := w.layout.FindByID("tip:popup"); !ok {
		t.Fatal("tooltip gone with the pointer on its overlap")
	}
}

// A float with an identity of its own inside the wrapper's content is
// still a layer over the wrapper: a dropdown that a wrapped Select opens
// over itself does not open the wrapper's tooltip.
func TestWithTooltipShutUnderOwnContentFloat(t *testing.T) {
	w := newTipWindow(t, func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{
				WithTooltip(w, WithTooltipCfg{ID: "tip", Text: "tooltip",
					Content: []View{Column(ContainerCfg{
						Sizing: FixedFixed, Width: 200, Height: 60,
						SizeBorder: NoBorder,
						Content: []View{Column(ContainerCfg{
							ID:          "cover",
							Float:       true,
							FloatAnchor: FloatTopLeft,
							FloatTieOff: FloatTopLeft,
							Sizing:      FixedFixed, Width: 100, Height: 60,
							Color:      RGB(40, 40, 40),
							SizeBorder: NoBorder,
						})},
					})}}),
			},
		})
	})
	cover := mustShape(t, w, "cover")
	x, y := cover.X+50, cover.Y+30
	tipMove(w, x, y)
	if got := w.viewState.tooltip.hoverID; got != "" {
		t.Fatalf("pointer on the content's float: hoverID = %q, want none", got)
	}
	tipMove(w, x+100, y)
	if got := w.viewState.tooltip.hoverID; got != "tip" {
		t.Fatalf("pointer beside the float: hoverID = %q, want tip", got)
	}
}

// tipFloatOnPath reports whether the climb from the topmost shape at
// (x, y) passes a float, so a test knows it is testing one.
func tipFloatOnPath(w *Window, x, y float32) bool {
	for p := layoutAtPoint(w.layout.Children, x, y, w); p != nil; p = p.Parent {
		if p.Shape != nil && p.Shape.Float {
			return true
		}
	}
	return false
}

// A widget may draw part of itself as an ID-less float over itself: the
// color of a ColorSwatch, the track of a channel slider. That float is
// the widget, as it is for a click (liftedFromTarget), so the tooltip
// of a wrapper around the widget opens over it.
func TestWithTooltipOpensOverOwnDecorationFloat(t *testing.T) {
	cases := []struct {
		name    string
		content View
	}{
		{"swatch", ColorSwatch(ColorSwatchCfg{ID: "w",
			Color: RGB(200, 80, 40), Width: 80, Height: 40})},
		{"slider", ColorChannelSlider(ColorChannelSliderCfg{ID: "w",
			Channel: ChannelHue, Value: HSLA{H: 300, S: 1, L: 0.5, A: 1}})},
		{"plain", Column(ContainerCfg{ID: "w",
			Sizing: FixedFixed, Width: 200, Height: 60, SizeBorder: NoBorder,
			Content: []View{Column(ContainerCfg{
				Float:       true,
				FloatAnchor: FloatTopLeft,
				FloatTieOff: FloatTopLeft,
				Sizing:      FixedFixed, Width: 200, Height: 60,
				Color:      RGB(40, 40, 40),
				SizeBorder: NoBorder,
			})},
		})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newTipWindow(t, func(w *Window) View {
				return Column(ContainerCfg{
					Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
					Content: []View{
						WithTooltip(w, WithTooltipCfg{ID: "tip", Text: "tooltip",
							Content: []View{tc.content}}),
					},
				})
			})
			s := mustShape(t, w, "w")
			// A quarter in: clear of the slider's thumb in the middle.
			x, y := s.X+s.Width/4, s.Y+s.Height/2
			if !tipFloatOnPath(w, x, y) {
				t.Fatalf("setup: no float under the pointer at %v,%v", x, y)
			}
			tipMove(w, x, y)
			if got := w.viewState.tooltip.hoverID; got != "tip" {
				t.Fatalf("pointer on the widget's own float: hoverID = %q, want tip", got)
			}
		})
	}
}

// A float whose root has no ID but whose content does, like a menu
// panel and its items, is a layer of its own where the pointer is on
// that content.
func TestWithTooltipShutUnderIDLessPanelItem(t *testing.T) {
	w := newTipWindow(t, func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{
				WithTooltip(w, WithTooltipCfg{ID: "tip", Text: "tooltip",
					Content: []View{Column(ContainerCfg{
						Sizing: FixedFixed, Width: 200, Height: 80,
						SizeBorder: NoBorder,
						Content: []View{Column(ContainerCfg{
							Float:       true,
							FloatAnchor: FloatTopLeft,
							FloatTieOff: FloatTopLeft,
							Sizing:      FixedFixed, Width: 200, Height: 80,
							Padding:    PaddingNone,
							SizeBorder: NoBorder,
							Content: []View{Column(ContainerCfg{ID: "item",
								Sizing: FixedFixed, Width: 200, Height: 40,
								SizeBorder: NoBorder})},
						})},
					})}}),
			},
		})
	})
	x, y := hitPoint(t, w, "item")
	if !tipFloatOnPath(w, x, y) {
		t.Fatalf("setup: no float under the pointer at %v,%v", x, y)
	}
	tipMove(w, x, y)
	if got := w.viewState.tooltip.hoverID; got != "" {
		t.Fatalf("pointer on the panel's item: hoverID = %q, want none", got)
	}
}

// Nested wrappers: the topmost widget under the pointer that has a
// tooltip wins. The inner widget is drawn over the outer one, so on it
// its own tooltip opens, also when the pointer comes to it from the
// outer one.
func TestWithTooltipTopmostWins(t *testing.T) {
	w := newTipWindow(t, func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{
				WithTooltip(w, WithTooltipCfg{ID: "outer", Text: "outer",
					Content: []View{
						WithTooltip(w, WithTooltipCfg{ID: "inner", Text: "inner",
							Content: []View{Column(ContainerCfg{ID: "a",
								Sizing: FixedFixed, Width: 100, Height: 40,
								SizeBorder: NoBorder})}}),
						Column(ContainerCfg{ID: "b",
							Sizing: FixedFixed, Width: 100, Height: 40,
							SizeBorder: NoBorder}),
					}}),
			},
		})
	})
	bx, by := hitPoint(t, w, "b")
	tipMove(w, bx, by)
	if got := w.viewState.tooltip.hoverID; got != "outer" {
		t.Fatalf("pointer on b: hoverID = %q, want outer", got)
	}
	ax, ay := hitPoint(t, w, "a")
	tipMove(w, ax, ay)
	if got := w.viewState.tooltip.hoverID; got != "inner" {
		t.Fatalf("pointer on a: hoverID = %q, want inner", got)
	}
}

// A hovered wrapper that leaves the tree takes its hover with it, so it
// does not come back with its tooltip already open.
func TestWithTooltipHoverEndsWhenWrapperGoes(t *testing.T) {
	present := true
	w := newTipWindow(t, func(w *Window) View {
		content := []View{}
		if present {
			content = append(content, WithTooltip(w, WithTooltipCfg{
				ID: "tip", Text: "tooltip",
				Content: []View{Column(ContainerCfg{
					Sizing: FixedFixed, Width: 200, Height: 60,
					SizeBorder: NoBorder,
				})}}))
		}
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: content,
		})
	})
	tipMove(w, 50, 30)
	tipShowNow(t, w, "tip:popup")

	present = false
	w.TestRender(nil)
	if ts := &w.viewState.tooltip; ts.hoverID != "" || ts.id != "" {
		t.Fatalf("after the wrapper left: hoverID = %q, id = %q, want none",
			ts.hoverID, ts.id)
	}
}

// The candidates are frame-scoped: a pass leaves none behind holding a
// shape of its tree.
func TestWithTooltipAnchorsClearedEachPass(t *testing.T) {
	w := tooltipBoxWindow(t, 50, 50, nil)
	if n := len(w.viewState.tooltip.anchors); n != 0 {
		t.Fatalf("anchors left after the pass: %d", n)
	}
}
