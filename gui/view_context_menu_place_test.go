package gui

import (
	"strings"
	"testing"
)

// Where a context menu opens (#1193). The popup hangs from a zero-size
// anchor at the click point, so its auto-flip mirrors around the
// cursor: with room it opens below-right of it, at the bottom edge
// above it, at the right edge to its left. Before the fix the flip
// mirrored the click offset to the area's far edge, and over a tall
// area the menu landed at the top of the window.

// cmCorner names the popup corner that must sit on the cursor.
type cmCorner int

const (
	cmTopLeft     cmCorner = iota // room below and right
	cmBottomLeft                  // flipped above
	cmTopRight                    // flipped left
	cmBottomRight                 // flipped above and left
)

func cmPlaceItems() []MenuItemCfg {
	items := make([]MenuItemCfg, 8)
	for i := range items {
		items[i] = MenuItemCfg{ID: string(rune('a' + i)), Text: "Item"}
	}
	return items
}

func cmPlaceMenu(sizing Sizing, width, height float32) View {
	return ContextMenu(nil, ContextMenuCfg{
		ID: "cm", Sizing: sizing, Width: width, Height: height,
		Items: cmPlaceItems(),
	})
}

// cmPlaceColumn stacks content edge to edge, with nothing of its own
// between the children or around them.
func cmPlaceColumn(content ...View) View {
	return Column(ContainerCfg{
		Sizing: FillFill, Padding: NoPadding, Spacing: NoSpacing,
		SizeBorder: NoBorder, Content: content,
	})
}

// cmPlaceGap is a block of height h that takes no clicks.
func cmPlaceGap(sizing Sizing, h float32) View {
	return Rectangle(RectangleCfg{Sizing: sizing, Height: h})
}

func cmPlaceWindow(t *testing.T, height int, view func(*Window) View) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 800, Height: height})
	w.TestRender(view)
	return w
}

// cmOpenAt right-clicks at window point (x, y) and returns the popup.
func cmOpenAt(t *testing.T, w *Window, popupID string, x, y float32) *Shape {
	t.Helper()
	pressAt(w, MouseRight, x, y)
	releaseAt(w, MouseRight, x, y)
	return mustShape(t, w, popupID)
}

func expectPopupInWindow(t *testing.T, w *Window, p *Shape) {
	t.Helper()
	ww, wh := float32(w.windowWidth), float32(w.windowHeight)
	if p.X < 0 || p.Y < 0 || p.X+p.Width > ww || p.Y+p.Height > wh {
		t.Errorf("popup at (%v,%v %vx%v), outside the %vx%v window",
			p.X, p.Y, p.Width, p.Height, ww, wh)
	}
}

func expectPopupCorner(t *testing.T, w *Window, p *Shape, cx, cy float32, c cmCorner) {
	t.Helper()
	x, y := p.X, p.Y
	if c == cmTopRight || c == cmBottomRight {
		x += p.Width
	}
	if c == cmBottomLeft || c == cmBottomRight {
		y += p.Height
	}
	if !f32AreClose(x, cx) || !f32AreClose(y, cy) {
		t.Errorf("popup at (%v,%v %vx%v): corner %d at (%v,%v), want the cursor (%v,%v)",
			p.X, p.Y, p.Width, p.Height, c, x, y, cx, cy)
	}
	expectPopupInWindow(t, w, p)
}

func TestContextMenuPlacement(t *testing.T) {
	// The issue's layout: a 100px header above a menu area that fills
	// the rest of the 800x800 window.
	header := func(*Window) View {
		return cmPlaceColumn(cmPlaceGap(FillFixed, 100), cmPlaceMenu(FillFill, 0, 0))
	}
	full := func(*Window) View { return cmPlaceColumn(cmPlaceMenu(FillFill, 0, 0)) }
	// A short strip at the window's bottom edge: the flip used to land
	// near the cursor here only because the area is short.
	strip := func(*Window) View {
		return cmPlaceColumn(cmPlaceGap(FillFill, 0), cmPlaceMenu(FillFixed, 0, 40))
	}
	cases := []struct {
		name   string
		view   func(*Window) View
		x, y   float32
		corner cmCorner
	}{
		{"room below and right", full, 100, 100, cmTopLeft},
		{"top-left corner", full, 1, 1, cmTopLeft},
		{"bottom of a tall area", header, 300, 760, cmBottomLeft},
		{"bottom edge of the window", header, 300, 799, cmBottomLeft},
		{"right edge", full, 790, 100, cmTopRight},
		{"bottom-right corner", header, 790, 790, cmBottomRight},
		{"short area at the window bottom", strip, 300, 780, cmBottomLeft},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := cmPlaceWindow(t, 800, c.view)
			p := cmOpenAt(t, w, ScopeID("cm", "popup"), c.x, c.y)
			expectPopupCorner(t, w, p, c.x, c.y, c.corner)
		})
	}
}

// A window shorter than the menu has room on neither side. The popup
// keeps to the side it was asked for and is clamped into the window.
func TestContextMenuPlacementNoRoomEitherSide(t *testing.T) {
	w := cmPlaceWindow(t, 200, func(*Window) View {
		return cmPlaceColumn(cmPlaceMenu(FillFill, 0, 0))
	})
	p := cmOpenAt(t, w, ScopeID("cm", "popup"), 100, 100)
	if p.Height <= 100 {
		t.Fatalf("menu height %v fits beside the cursor; the case needs a taller one", p.Height)
	}
	if !f32AreClose(p.Y, 0) || !f32AreClose(p.X, 100) {
		t.Errorf("popup at (%v,%v), want (100,0): clamped to the top edge", p.X, p.Y)
	}
}

// cmScrollView puts the menu area, 3000px tall, in a scroller below a
// 100px header.
func cmScrollView(*Window) View {
	return cmPlaceColumn(
		cmPlaceGap(FillFixed, 100),
		Column(ContainerCfg{
			ID: "s", Scrollable: true, ScrollMode: ScrollVerticalOnly,
			Sizing: FillFill, Padding: NoPadding, SizeBorder: NoBorder,
			Content: []View{cmPlaceMenu(FillFixed, 0, 3000)},
		}),
	)
}

// Scrolled, the area's top is far above the window, and the flip used
// to mirror the click offset off its far edge.
func TestContextMenuPlacementScrolled(t *testing.T) {
	popupID := ScopeID("s", "cm", "popup")
	for _, c := range []struct {
		name   string
		y      float32
		corner cmCorner
	}{
		{"middle of the view", 400, cmTopLeft},
		{"bottom of the view", 760, cmBottomLeft},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := cmPlaceWindow(t, 800, cmScrollView)
			w.ScrollVerticalTo("s", -1000)
			w.TestRender(nil)
			p := cmOpenAt(t, w, popupID, 300, c.y)
			expectPopupCorner(t, w, p, 300, c.y, c.corner)
		})
	}
}

// The wheel does not close the menu. It follows the content, and when
// the click point leaves the window the popup stays at the edge nearest
// it.
func TestContextMenuPlacementScrolledWhileOpen(t *testing.T) {
	popupID := ScopeID("s", "cm", "popup")
	w := cmPlaceWindow(t, 800, cmScrollView)
	w.ScrollVerticalTo("s", -1000)
	w.TestRender(nil)
	cmOpenAt(t, w, popupID, 300, 400)

	w.ScrollVerticalTo("s", -1100)
	w.TestRender(nil)
	p := mustShape(t, w, popupID)
	expectPopupCorner(t, w, p, 300, 300, cmTopLeft)

	// The point is now 100px above the window's bottom edge: no room
	// below, so the popup flips above it.
	w.ScrollVerticalTo("s", -700)
	w.TestRender(nil)
	p = mustShape(t, w, popupID)
	expectPopupCorner(t, w, p, 300, 700, cmBottomLeft)

	// The point is now 100px below the window.
	w.ScrollVerticalTo("s", -500)
	w.TestRender(nil)
	p = mustShape(t, w, popupID)
	if !f32AreClose(p.Y+p.Height, 800) {
		t.Errorf("popup at y=%v (height %v), want its bottom on the window's bottom edge",
			p.Y, p.Height)
	}
	expectPopupInWindow(t, w, p)

	// The point is now 200px above the window.
	w.ScrollVerticalTo("s", -1600)
	w.TestRender(nil)
	p = mustShape(t, w, popupID)
	if !f32AreClose(p.Y, 0) {
		t.Errorf("popup at y=%v, want 0: on the window's top edge", p.Y)
	}
}

// A second right-click while the menu is open moves it to the new
// point, and flips there.
func TestContextMenuPlacementReopen(t *testing.T) {
	w := cmPlaceWindow(t, 800, func(*Window) View {
		return cmPlaceColumn(cmPlaceGap(FillFixed, 100), cmPlaceMenu(FillFill, 0, 0))
	})
	popupID := ScopeID("cm", "popup")
	p := cmOpenAt(t, w, popupID, 200, 200)
	expectPopupCorner(t, w, p, 200, 200, cmTopLeft)
	p = cmOpenAt(t, w, popupID, 300, 760)
	expectPopupCorner(t, w, p, 300, 760, cmBottomLeft)
}

// Under a view transform the click point maps out to where the area is
// drawn. The popup's anchor is itself a float, so the point is mapped
// once, not again for the menu that hangs from it.
func TestContextMenuPlacementTransformed(t *testing.T) {
	tr := Translate(50, 300)
	view := func(*Window) View {
		return cmPlaceColumn(Column(ContainerCfg{
			ID: "card", Padding: NoPadding, SizeBorder: NoBorder,
			TransformFn: func() Transform { return tr },
			Content:     []View{cmPlaceMenu(FixedFixed, 400, 480)},
		}))
	}
	popupID := ScopeID("card", "cm", "popup")
	for _, c := range []struct {
		name   string
		y      float32
		corner cmCorner
	}{
		{"room below", 320, cmTopLeft},
		{"bottom of the drawn area", 760, cmBottomLeft},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := cmPlaceWindow(t, 800, view)
			p := cmOpenAt(t, w, popupID, 150, c.y)
			expectPopupCorner(t, w, p, 150, c.y, c.corner)
		})
	}
}

// In a dialog the anchor and the menu are lifted out of the dialog
// layer one after the other. The menu still opens at the cursor and its
// items still take a click.
func TestContextMenuPlacementInDialog(t *testing.T) {
	picked := ""
	w := cmPlaceWindow(t, 800, func(*Window) View {
		return cmPlaceColumn()
	})
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		CustomView: func(*Window) View {
			return ContextMenu(nil, ContextMenuCfg{
				ID: "cm", Sizing: FixedFixed, Width: 200, Height: 100,
				Items:  cmPlaceItems(),
				Action: func(id string, _ EventCtx) { picked = id },
			})
		},
	})
	w.TestRender(nil)
	w.TestRender(nil)
	// The dialog scopes the menu's ID; read it back, do not spell it.
	ids := w.ResolveID("cm")
	if len(ids) != 1 {
		t.Fatalf("ResolveID(cm) = %v, want one", ids)
	}
	area := mustShape(t, w, ids[0])
	x, y := area.X+20, area.Y+20
	p := cmOpenAt(t, w, ScopeID(ids[0], "popup"), x, y)
	expectPopupCorner(t, w, p, x, y, cmTopLeft)
	if err := w.TestClick(ScopeID(ids[0], "popup", "b")); err != nil {
		t.Fatalf("TestClick(b): %v", err)
	}
	if picked != "b" {
		t.Errorf("picked = %q, want b", picked)
	}
}

// A submenu hangs off its item with the usual gap semantics. Opened from
// a popup flipped left at the right edge, it flips to the left too and
// stays in the window.
func TestContextMenuPlacementSubmenuAtRightEdge(t *testing.T) {
	w := cmPlaceWindow(t, 800, func(*Window) View {
		return cmPlaceColumn(ContextMenu(nil, ContextMenuCfg{
			ID: "cm", Sizing: FillFill,
			Items: []MenuItemCfg{
				MenuSubmenu("more", "More", []MenuItemCfg{
					{ID: "x", Text: "X"}, {ID: "y", Text: "Y"},
				}),
			},
		}))
	})
	popupID := ScopeID("cm", "popup")
	p := cmOpenAt(t, w, popupID, 790, 100)
	expectPopupCorner(t, w, p, 790, 100, cmTopRight)
	if err := w.TestKey(popupID, KeyRight, ModNone); err != nil {
		t.Fatalf("TestKey(Right): %v", err)
	}
	item := mustShape(t, w, ScopeID("cm", "popup", "more"))
	var sub *Shape
	for i := range w.layout.Children {
		if ly := &w.layout.Children[i]; ly.Parent != nil && ly.Parent.Shape == item {
			sub = ly.Shape
		}
	}
	if sub == nil {
		t.Fatal("submenu not open")
	}
	expectPopupInWindow(t, w, sub)
	if sub.X >= p.X {
		t.Errorf("submenu at x=%v, want it opened to the left of the popup at x=%v",
			sub.X, p.X)
	}
}

// The RTF link menu hangs from the same anchor. Its parent is the whole
// RTF block, which in a long document is as tall as the window: a link
// near its bottom used to open the menu at the window's top.
func TestRtfLinkMenuPlacement(t *testing.T) {
	for _, c := range []struct {
		name   string
		y      float32
		corner cmCorner
	}{
		{"room below", 100, cmTopLeft},
		{"bottom of a tall block", 740, cmBottomLeft},
	} {
		t.Run(c.name, func(t *testing.T) {
			h := newRtfSelectHarness(t, RichText{Runs: []RichTextRun{
				{Text: strings.Repeat("line\n", 37), Style: TextStyle{Size: 14}},
				{Text: "page", Link: "https://safe.example.com",
					Style: TextStyle{Size: 14}},
			}})
			block := h.shape(t).Shape
			if block.Height < 700 {
				t.Fatalf("block height %v, the case needs a tall one", block.Height)
			}
			showLinkContextMenu(h.w, "https://safe.example.com", 20, c.y,
				rtfRunsKey(block.TC.rtf.rTFRuns), "")
			h.w.TestRender(nil)
			p := mustShape(t, h.w, rtfLinkMenuFocusID)
			expectPopupCorner(t, h.w, p, block.X+20, block.Y+c.y, c.corner)
		})
	}
}

// rtfTipRuns is an abbreviation run first, where the test measurer
// can hover it, then lines that make the block tall.
func rtfTipRuns(lines int) RichText {
	return RichText{Runs: []RichTextRun{
		{Text: "abbr", Tooltip: "tip", Style: TextStyle{Size: 14}},
		{Text: strings.Repeat("\nline", lines), Style: TextStyle{Size: 14}},
	}}
}

// rtfTipHover hovers the drawn abbreviation in the RTF id, as the
// pointer would, and shows its tooltip the way the hover delay does. It
// returns the run's box as drawn and the tooltip.
func rtfTipHover(t *testing.T, w *Window, id string) (drawClip, *Shape) {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("no RTF %q", id)
	}
	var run drawClip
	for _, it := range ly.Shape.TC.rtf.rTFLayout.Items {
		if it.StartIndex == 0 {
			run = rtfRunRect(it)
		}
	}
	box := xfBoxToWindow(ly, drawClip{
		X: ly.Shape.X + run.X, Y: ly.Shape.Y + run.Y,
		Width: run.Width, Height: run.Height,
	})
	w.EventFn(&Event{Type: EventMouseMove,
		MouseX: box.X + box.Width/2, MouseY: box.Y + box.Height/2})
	w.settle()
	ts := &w.viewState.tooltip
	if ts.hoverID != "tip" {
		t.Fatalf("hover missed the run: hoverID = %q", ts.hoverID)
	}
	// The run's box as drawn is what the popup hangs from.
	if !f32AreClose(ts.floatWidth, box.Width) || !f32AreClose(ts.floatHeight, box.Height) {
		t.Errorf("anchor %vx%v, want the drawn run %vx%v",
			ts.floatWidth, ts.floatHeight, box.Width, box.Height)
	}
	rtfTooltipAnimation(ts.hoverID).Callback(nil, w)
	w.TestRender(nil)
	return box, mustShape(t, w, ts.popupID)
}

// The RTF tooltip sits 3px above the hovered run, centered on it. It
// hangs from the run's box as drawn, so with no room above it flips to
// 3px below the run. Hung from the block's corner instead, it flipped to
// the block's bottom edge, far below the word; and under a view
// transform the box is the drawn one, scaled or turned, not the run's
// layout size.
func TestRtfTooltipPlacement(t *testing.T) {
	// Scale and turn pivot on the card's center, so the transformed
	// cases take a one-line block: the drawn word stays at the top.
	for _, c := range []struct {
		name  string
		top   float32 // space above the RTF
		lines int     // lines below the abbreviation
		xf    Transform
		above bool
	}{
		{"room above", 300, 20, Transform{}, true},
		{"top of a tall block", 0, 20, Transform{}, false},
		{"scaled at the top", 0, 0, Scale(2), false},
		{"turned at the top", 0, 0, Rotate(90), false},
		{"scaled with room above", 300, 0, Scale(2), true},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{Width: 800, Height: 800})
			w.textMeasurer = rtfSelTestMeasurer{}
			w.TestRender(func(*Window) View {
				return cmPlaceColumn(cmPlaceGap(FillFixed, c.top), Column(ContainerCfg{
					ID: "card", Padding: NoPadding, SizeBorder: NoBorder,
					Transform: c.xf,
					Content:   []View{RTF(RTFCfg{ID: "rtf", RichText: rtfTipRuns(c.lines)})},
				}))
			})
			box, p := rtfTipHover(t, w, ScopeID("card", "rtf"))
			expectPopupInWindow(t, w, p)
			if c.above {
				if !f32AreClose(p.Y+p.Height, box.Y-3) {
					t.Errorf("tooltip y=%v..%v, want its bottom 3px above the word at %v",
						p.Y, p.Y+p.Height, box.Y)
				}
			} else if !f32AreClose(p.Y, box.Y+box.Height+3) {
				t.Errorf("tooltip y=%v, want it 3px below the word at %v..%v",
					p.Y, box.Y, box.Y+box.Height)
			}
			if mid := box.X + box.Width/2; !f32AreClose(p.X+p.Width/2, mid) {
				t.Errorf("tooltip centered at x=%v, want the word's center %v",
					p.X+p.Width/2, mid)
			}
		})
	}
}

// The tooltip's anchor covers its word and its layer can sit above an
// open menu that covers the word too. The anchor is inert, so a press
// on the menu item over the word still picks the item.
func TestRtfTooltipAnchorTakesNoPress(t *testing.T) {
	picked := ""
	w := NewTestWindow(t, WindowCfg{Width: 800, Height: 800})
	w.textMeasurer = rtfSelTestMeasurer{}
	w.TestRender(func(*Window) View {
		return cmPlaceColumn(
			ContextMenu(nil, ContextMenuCfg{
				ID: "cm", Sizing: FillFixed, Height: 30, Items: cmPlaceItems(),
				Action: func(id string, _ EventCtx) { picked = id },
			}),
			RTF(RTFCfg{ID: "rtf", RichText: rtfTipRuns(0)}),
		)
	})
	word := mustShape(t, w, "rtf")
	// Open the menu above-left of the word, so its items cover it.
	cmOpenAt(t, w, ScopeID("cm", "popup"), word.X+5, 5)
	box, _ := rtfTipHover(t, w, "rtf")
	x, y := box.X+box.Width/2, box.Y+box.Height/2
	target := interactionTargetAt(w.layout.Children, x, y, w)
	if !strings.HasPrefix(target, ScopeID("cm", "popup")+":") {
		t.Fatalf("target under the word = %q, want a menu item", target)
	}
	pressAt(w, MouseLeft, x, y)
	releaseAt(w, MouseLeft, x, y)
	if want := strings.TrimPrefix(target, ScopeID("cm", "popup")+":"); picked != want {
		t.Errorf("picked = %q, want %q", picked, want)
	}
}
