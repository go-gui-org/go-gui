package gui

import "testing"

// These tests cover the thin convenience constructors (issue #325).
// Each asserts that the helper forwards exactly the caller's arguments
// to the Cfg form — no new behavior, no surprise defaults beyond the
// documented padding.

func TestTextButtonStructure(t *testing.T) {
	w := newTestWindow()
	v := TextButton("tb", "Click Me", noop)
	layout := generateViewLayout(v, w)

	if len(layout.Children) != 1 {
		t.Fatalf("got %d children, want 1", len(layout.Children))
	}
	if got := layout.Children[0].Shape.TC.Text; got != "Click Me" {
		t.Fatalf("button label = %q, want %q", got, "Click Me")
	}
	if got := layout.Shape.Padding; got != NewPadding(8, 16, 8, 16) {
		t.Fatalf("padding = %v, want NewPadding(8, 16, 8, 16)", got)
	}
}

func TestTextButtonClickFires(t *testing.T) {
	type state struct{ clicks int }
	w := NewTestWindow(WindowCfg{State: &state{}})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				TextButton("tb", "Click Me", func(ctx EventCtx) {
					State[state](ctx.Window).clicks++
				}),
			},
		})
	})

	if err := w.TestClick("tb"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if got := State[state](w).clicks; got != 1 {
		t.Fatalf("clicks = %d, want 1", got)
	}
}

func TestLabelZeroStyleDefaults(t *testing.T) {
	w := newTestWindow()
	layout := generateViewLayout(Label("hi", TextStyle{}), w)
	if layout.Shape.TC.Text != "hi" {
		t.Fatalf("text = %q, want %q", layout.Shape.TC.Text, "hi")
	}
	if got := *layout.Shape.TC.TextStyle; got != DefaultTextStyle {
		t.Fatalf("style = %v, want DefaultTextStyle", got)
	}
}

func TestLabelKeepsExplicitStyle(t *testing.T) {
	w := newTestWindow()
	style := TextStyle{Size: 24}
	layout := generateViewLayout(Label("hi", style), w)
	if got := *layout.Shape.TC.TextStyle; got.Size != 24 {
		t.Fatalf("style size = %v, want 24", got.Size)
	}
}

func TestLabeledToggleForwards(t *testing.T) {
	w := newTestWindow()

	sel := generateViewLayout(LabeledToggle("t1", "Accept", true, noop), w)
	if sel.Shape.A11YRole != AccessRoleCheckbox {
		t.Fatalf("role = %v, want checkbox", sel.Shape.A11YRole)
	}
	if sel.Shape.A11YState != AccessStateChecked {
		t.Fatalf("state = %v, want checked", sel.Shape.A11YState)
	}
	if len(sel.Children) != 2 {
		t.Fatalf("got %d children, want 2 (box + label)", len(sel.Children))
	}
	// The label sits inside the shared trailingLabel wrapper, which
	// carries the gap between control and name (issue #335).
	if got := sel.Children[1].Children[0].Shape.TC.Text; got != "Accept" {
		t.Fatalf("label = %q, want %q", got, "Accept")
	}

	unsel := generateViewLayout(LabeledToggle("t2", "Accept", false, noop), w)
	if unsel.Shape.A11YState != AccessStateNone {
		t.Fatalf("state = %v, want none", unsel.Shape.A11YState)
	}
}

func TestLabeledToggleClickFires(t *testing.T) {
	type state struct{ hits int }
	w := NewTestWindow(WindowCfg{State: &state{}})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				LabeledToggle("t1", "Accept", false, func(ctx EventCtx) {
					State[state](ctx.Window).hits++
				}),
			},
		})
	})

	if err := w.TestClick("t1"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if got := State[state](w).hits; got != 1 {
		t.Fatalf("hits = %d, want 1", got)
	}
}

func TestLabeledSwitchForwards(t *testing.T) {
	w := newTestWindow()

	sel := generateViewLayout(LabeledSwitch("s1", "Auto", true, noop), w)
	if sel.Shape.A11YRole != AccessRoleSwitchToggle {
		t.Fatalf("role = %v, want switch-toggle", sel.Shape.A11YRole)
	}
	if sel.Shape.A11YState != AccessStateChecked {
		t.Fatalf("state = %v, want checked", sel.Shape.A11YState)
	}
	if len(sel.Children) != 2 {
		t.Fatalf("got %d children, want 2 (pill + label)", len(sel.Children))
	}
	if got := sel.Children[1].Children[0].Shape.TC.Text; got != "Auto" {
		t.Fatalf("label = %q, want %q", got, "Auto")
	}

	unsel := generateViewLayout(LabeledSwitch("s2", "Auto", false, noop), w)
	if unsel.Shape.A11YState != AccessStateNone {
		t.Fatalf("state = %v, want none", unsel.Shape.A11YState)
	}
}

func TestLabeledSwitchClickFires(t *testing.T) {
	type state struct{ hits int }
	w := NewTestWindow(WindowCfg{State: &state{}})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				LabeledSwitch("s1", "Auto", true, func(ctx EventCtx) {
					State[state](ctx.Window).hits++
				}),
			},
		})
	})

	if err := w.TestClick("s1"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if got := State[state](w).hits; got != 1 {
		t.Fatalf("hits = %d, want 1", got)
	}
}

func TestSimpleWindowForwards(t *testing.T) {
	type appState struct{ n int }

	var installed func(*Window) View
	w := SimpleWindow("T", 400, 300, &appState{n: 7}, func(w *Window) {
		installed = func(w *Window) View { return Label("root", TextStyle{}) }
		w.UpdateView(installed)
	})

	if w.Config.Title != "T" || w.Config.Width != 400 || w.Config.Height != 300 {
		t.Fatalf("config = %+v, want Title=T 400x300", w.Config)
	}
	if got := State[appState](w).n; got != 7 {
		t.Fatalf("state = %d, want 7", got)
	}
	if w.Config.OnInit == nil {
		t.Fatal("OnInit not wired")
	}

	// Backends invoke OnInit after NewWindow; simulate that and render.
	// composeLayout may wrap the root, so locate the text shape by content.
	w.Config.OnInit(w)
	layout := w.TestRender(nil)
	if _, ok := layout.FindLayout(func(l Layout) bool {
		return l.Shape.TC != nil && l.Shape.TC.Text == "root"
	}); !ok {
		t.Fatalf("rendered view missing text %q", "root")
	}
}
