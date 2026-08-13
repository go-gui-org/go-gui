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

// TestExpandPanelHoverPressedColor pins the pressed-while-hovered branch
// of the expand panel header's OnHover (view_expand_panel.go): a
// press-and-hold renders the click color, release falls back to the
// hover color. The header row carries no ID, so the test resolves it as
// the panel's first child and hits it directly.
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
