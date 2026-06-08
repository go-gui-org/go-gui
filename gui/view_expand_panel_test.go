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
		OnToggle: func(_ *Window) {
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
	header.Shape.events.OnClick(&header, e, w)
	if !called {
		t.Error("OnToggle should be called")
	}
}
