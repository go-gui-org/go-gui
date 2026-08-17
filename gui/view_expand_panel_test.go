package gui

import "testing"

func TestExpandPanelOpenLayout(t *testing.T) {
	v := ExpandPanel(ExpandPanelCfg{
		Head:    Text(TextCfg{Text: "Header"}),
		Content: Text(TextCfg{Text: "Body"}),
		Open:    true,
	})
	layout := generateViewLayout(v, &Window{})
	// Column with 2 children: header row + content column
	if len(layout.Children) != 2 {
		t.Fatalf("children: got %d, want 2", len(layout.Children))
	}
	body := layout.Children[1]
	if body.Shape.Disabled {
		t.Error("open panel body should not be disabled")
	}
}

func TestExpandPanelClosedLayout(t *testing.T) {
	v := ExpandPanel(ExpandPanelCfg{
		Head:    Text(TextCfg{Text: "Header"}),
		Content: Text(TextCfg{Text: "Body"}),
		Open:    false,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) != 2 {
		t.Fatalf("children: got %d, want 2", len(layout.Children))
	}
	body := layout.Children[1]
	if body.Shape.shapeType != shapeRectangle {
		t.Error("closed body should be a container")
	}
}

func TestExpandPanelA11YRole(t *testing.T) {
	v := ExpandPanel(ExpandPanelCfg{
		Head:    Text(TextCfg{Text: "H"}),
		Content: Text(TextCfg{Text: "C"}),
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.A11YRole != AccessRoleDisclosure {
		t.Errorf("role = %d, want Disclosure", layout.Shape.A11YRole)
	}
}

func TestExpandPanelA11YExpanded(t *testing.T) {
	v := ExpandPanel(ExpandPanelCfg{
		Head:    Text(TextCfg{Text: "H"}),
		Content: Text(TextCfg{Text: "C"}),
		Open:    true,
	})
	layout := generateViewLayout(v, &Window{})
	if !layout.Shape.A11YState.Has(AccessStateExpanded) {
		t.Error("open panel should have expanded state")
	}

	v2 := ExpandPanel(ExpandPanelCfg{
		Head:    Text(TextCfg{Text: "H"}),
		Content: Text(TextCfg{Text: "C"}),
		Open:    false,
	})
	layout2 := generateViewLayout(v2, &Window{})
	if layout2.Shape.A11YState.Has(AccessStateExpanded) {
		t.Error("closed panel should not have expanded state")
	}
}

func TestExpandPanelOnToggle(t *testing.T) {
	called := false
	cfg := ExpandPanelCfg{
		Head:    Text(TextCfg{Text: "H"}),
		Content: Text(TextCfg{Text: "C"}),
		OnToggle: func(ctx EventCtx) {
			called = true
		},
	}
	v := ExpandPanel(cfg)
	layout := generateViewLayout(v, &Window{})
	// Header row is first child; it has OnClick
	header := layout.Children[0]
	if header.Shape.events == nil || header.Shape.events.OnClick == nil {
		t.Fatal("header should have OnClick")
	}
	e := &Event{}
	w := &Window{}
	header.Shape.events.OnClick(EventCtx{&header, e, w})
	if !called {
		t.Error("OnToggle should be called")
	}
}

// TestExpandPanelHeaderFocusable pins the header's keyboard surface
// (issue #345): it joins the tab order ahead of the body's own
// focusables — it is the Column's first child, and traversal is
// depth-first in order — and Space/Enter reach its OnClick.
func TestExpandPanelHeaderFocusable(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ExpandPanel(ExpandPanelCfg{
		ID:      "ep",
		Head:    Text(TextCfg{Text: "H"}),
		Content: Text(TextCfg{Text: "C"}),
	}), w)
	header := layout.Children[0]
	if !header.Shape.Focusable {
		t.Error("header must join the tab order")
	}
	if header.Shape.ID != ScopeID("ep", "head") {
		t.Errorf("header ID = %q, want %q", header.Shape.ID,
			ScopeID("ep", "head"))
	}
	ev := header.Shape.events
	if ev == nil || ev.OnClick == nil {
		t.Fatal("header needs OnClick to toggle")
	}
	if !ev.clickOnSpace || !ev.clickOnEnter {
		t.Error("header must map Space and Enter to OnClick")
	}
}

// Space and Enter toggle through the real char/key-down dispatch,
// which is what makes the header a working tab stop.
func TestExpandPanelHeaderKeyboardToggle(t *testing.T) {
	toggles := 0
	w := &Window{}
	w.SetFocus("ep:head")
	layout := generateViewLayout(ExpandPanel(ExpandPanelCfg{
		ID:      "ep",
		Head:    Text(TextCfg{Text: "H"}),
		Content: Text(TextCfg{Text: "C"}),
		OnToggle: func(EventCtx) {
			toggles++
		},
	}), w)

	charHandler(&layout, &Event{CharCode: charSpace}, w)
	keydownHandler(&layout, &Event{KeyCode: KeyEnter}, w)
	if toggles != 2 {
		t.Fatalf("toggles = %d, want 2 (space + enter)", toggles)
	}
}

// A character that is not Space must keep travelling, so the header
// does not eat typing elsewhere.
func TestExpandPanelHeaderOtherCharsTravel(t *testing.T) {
	w := &Window{}
	w.SetFocus("ep:head")
	layout := generateViewLayout(ExpandPanel(ExpandPanelCfg{
		ID:      "ep",
		Head:    Text(TextCfg{Text: "H"}),
		Content: Text(TextCfg{Text: "C"}),
		OnToggle: func(EventCtx) {
			t.Error("OnToggle fired for a non-activation key")
		},
	}), w)
	e := &Event{CharCode: 'x'}
	charHandler(&layout, e, w)
	if e.IsHandled {
		t.Error("'x' must travel past the header")
	}
}

// The header sits ahead of the panel body's own focusables in tab
// order: it is the Column's first child, and traversal walks
// depth-first in order. This drives the real traversal against a
// rendered window, so a future reorder that puts a body control
// before the header fails here.
func TestExpandPanelHeaderPrecedesBodyInTabOrder(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return ExpandPanel(ExpandPanelCfg{
			ID:   "ep",
			Open: true,
			Head: Text(TextCfg{Text: "H"}),
			Content: Column(ContainerCfg{Content: []View{
				Button(ButtonCfg{
					ID:      "inner",
					Content: []View{Text(TextCfg{Text: "B"})},
				}),
			}}),
		})
	})

	var order []string
	for range 3 {
		if s, ok := w.layout.nextFocusable(w); ok {
			order = append(order, s.idKey())
			w.SetFocus(s.idKey())
		}
	}
	if len(order) < 2 ||
		order[0] != "ep:head" || order[1] != "ep:inner" {
		t.Fatalf("tab order = %v, want [ep:head ep:inner ...]", order)
	}
}

// TestExpandPanelHoverPressedColor pins the pressed-while-hovered branch
// of the expand panel header's OnHover (view_expand_panel.go): a
// press-and-hold renders the click color, release falls back to the
// hover color. The header is resolved as the panel's first child and
// hit directly.
func TestExpandPanelHoverPressedColor(t *testing.T) {
	hover := RGBA(40, 200, 40, 255)
	click := RGBA(200, 40, 40, 255)
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return ExpandPanel(ExpandPanelCfg{
			ID:         "ep",
			Head:       Text(TextCfg{Text: "Head"}),
			ColorHover: hover,
			colorClick: click,
		})
	})

	ly, ok := w.layout.FindByID("ep")
	if !ok {
		t.Fatal("no expand panel with effective ID ep")
	}
	if len(ly.Children) == 0 {
		t.Fatal("expand panel has no header child")
	}
	header := ly.Children[0]
	// The header row carries no ID; re-resolve it by position each
	// frame, because settles rebuild the tree.
	headerShape := func() *Shape {
		cur, ok := w.layout.FindByID("ep")
		if !ok {
			t.Fatal("expand panel vanished")
		}
		return cur.Children[0].Shape
	}
	x, y, err := testHitPoint(&header, "ep")
	if err != nil {
		t.Fatal(err)
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: x, MouseY: y})
	w.settle()
	if c := headerShape().Color; c != hover {
		t.Fatalf("hover: color = %+v, want %+v", c, hover)
	}
	pressAt(w, MouseLeft, x, y)
	if c := headerShape().Color; c != click {
		t.Fatalf("held: color = %+v, want %+v", c, click)
	}
	releaseAt(w, MouseLeft, x, y)
	if c := headerShape().Color; c != hover {
		t.Fatalf("released: color = %+v, want %+v", c, hover)
	}
}
