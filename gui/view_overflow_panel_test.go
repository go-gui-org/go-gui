package gui

import "testing"

func TestOverflowPanelLayout(t *testing.T) {
	w := &Window{}
	cfg := OverflowPanelCfg{
		ID:      "op",
		IDFocus: 200,
		Items: []OverflowItem{
			{ID: "a", View: Text(TextCfg{Text: "A"})},
			{ID: "b", View: Text(TextCfg{Text: "B"})},
			{ID: "c", View: Text(TextCfg{Text: "C"})},
		},
	}
	view := OverflowPanel(w, cfg)
	layout := generateViewLayout(view, w)

	if layout.Shape == nil {
		t.Fatal("nil shape")
	}
	if layout.Shape.ID != "op" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
	if !layout.Shape.Overflow {
		t.Error("should have Overflow=true")
	}
}

func TestOverflowPanelTrigger(t *testing.T) {
	w := &Window{}
	cfg := OverflowPanelCfg{
		ID:      "op",
		IDFocus: 200,
		Items: []OverflowItem{
			{ID: "a", View: Text(TextCfg{Text: "A"})},
		},
		Trigger: []View{
			Text(TextCfg{Text: "..."}),
		},
	}
	view := OverflowPanel(w, cfg)
	layout := generateViewLayout(view, w)

	// Should have item + trigger button = at least 2 children.
	if len(layout.Children) < 2 {
		t.Errorf("children = %d, want >= 2",
			len(layout.Children))
	}
}

func TestOverflowPanelClosed(t *testing.T) {
	w := &Window{}
	cfg := OverflowPanelCfg{
		ID:      "op",
		IDFocus: 200,
		Items: []OverflowItem{
			{ID: "a", View: Text(TextCfg{Text: "A"})},
		},
	}
	view := OverflowPanel(w, cfg)
	layout := generateViewLayout(view, w)

	// When closed, no floating menu child.
	for _, child := range layout.Children {
		if child.Shape != nil && child.Shape.Float {
			t.Error("should not have floating menu when closed")
		}
	}
}
