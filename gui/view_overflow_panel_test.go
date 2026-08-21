package gui

import (
	"strconv"
	"testing"
)

func TestOverflowPanelLayout(t *testing.T) {
	w := &Window{}
	cfg := OverflowPanelCfg{
		ID:        "op",
		Focusable: true,
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
		ID:        "op",
		Focusable: true,
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
		ID:        "op",
		Focusable: true,
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

// TestOverflowPanelOpensUnderIDScope pins the trigger click. The panel
// keys its overflow count on its effective ID, so under an ID-bearing
// ancestor a factory-time key ("op") and the key layoutOverflow writes
// from the shape ("outer:op") must not diverge — when they did, the
// count read back as absent, the panel believed every item fit, and the
// trigger toggled a flag that opened nothing.
func TestOverflowPanelOpensUnderIDScope(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 200, Height: 120})
	items := make([]OverflowItem, 8)
	for i := range items {
		label := "Item " + strconv.Itoa(i+1)
		items[i] = OverflowItem{
			ID: label,
			View: Button(ButtonCfg{
				ID:      "btn" + strconv.Itoa(i),
				Sizing:  FixedFit,
				Width:   60,
				Content: []View{Text(TextCfg{Text: label})},
			}),
			Text:   label,
			Action: func(_ *MenuItemCfg, _ EventCtx) {},
		}
	}
	w.viewGenerator = func(win *Window) View {
		return Column(ContainerCfg{
			ID:     "outer",
			Sizing: FillFill,
			Content: []View{OverflowPanel(win, OverflowPanelCfg{
				ID:        "op",
				Focusable: true,
				Items:     items,
			})},
		})
	}
	w.refreshLayout = true
	w.FrameFn()
	// Second frame: the first one writes the overflow count, and the
	// panel reads it on the next generation.
	w.FrameFn()

	if vc, ok := w.overflow().Get("outer:op"); !ok || vc >= len(items) {
		t.Fatalf("overflow count = %d, ok=%v; want a scoped count < %d",
			vc, ok, len(items))
	}

	trig, ok := w.layout.FindByID("outer:op:trigger")
	if !ok {
		t.Fatal("trigger not found at the scoped effective ID")
	}
	w.EventFn(&Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: trig.Shape.X + trig.Shape.Width/2,
		MouseY: trig.Shape.Y + trig.Shape.Height/2,
	})
	w.refreshLayout = true
	w.FrameFn()

	if _, found := w.layout.FindByID("outer:op:menu"); !found {
		t.Fatal("trigger click did not open the overflow menu")
	}

	// The trigger toggles: a second click on the same scoped key must
	// close the menu again. A key that diverged would leave the flag
	// set and the menu stuck open.
	w.EventFn(&Event{
		Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: trig.Shape.X + trig.Shape.Width/2,
		MouseY: trig.Shape.Y + trig.Shape.Height/2,
	})
	w.refreshLayout = true
	w.FrameFn()

	if _, found := w.layout.FindByID("outer:op:menu"); found {
		t.Error("second trigger click did not close the overflow menu")
	}
}
