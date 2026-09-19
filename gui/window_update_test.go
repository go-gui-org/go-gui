package gui

import (
	"testing"
)

// A panic in the root view used to skip the genDepth decrement in
// updateLocked. The leaked depth stuck across later good frames, and
// after maxEventDepth panicked frames the tree became all placeholders
// (issue #689).
func TestRootViewPanicRestoresGenDepth(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 200, Height: 100})
	boom := false
	w.SetView(func(w *Window) View {
		if boom {
			panic("view boom")
		}
		return Column(ContainerCfg{
			Content: []View{Text(TextCfg{Text: "ok"})},
		})
	})
	w.Update()
	if w.viewState.genDepth != 0 {
		t.Fatalf("after good frame genDepth=%d, want 0", w.viewState.genDepth)
	}
	if w.viewState.idScope != "" {
		t.Fatalf("after good frame idScope=%q, want empty", w.viewState.idScope)
	}
	if len(w.layout.Children) == 0 {
		t.Fatal("good frame should have laid out children")
	}

	boom = true
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected view panic")
			}
		}()
		w.Update()
	}()
	if w.viewState.genDepth != 0 {
		t.Fatalf("after panic genDepth=%d, want 0", w.viewState.genDepth)
	}

	boom = false
	w.Update()
	if w.viewState.genDepth != 0 {
		t.Fatalf("after recovery frame genDepth=%d, want 0", w.viewState.genDepth)
	}
	if w.viewState.idScope != "" {
		t.Fatalf("after recovery frame idScope=%q, want empty", w.viewState.idScope)
	}
	if len(w.layout.Children) == 0 {
		t.Fatal("recovery frame should have laid out children")
	}
}

// A view function may query the previous frame's arranged tree (for
// example ScrollOverflowY to size a scrollbar reservation). The layer
// pool put in updateLocked must leave w.layout intact until the new
// tree replaces it, on a normal frame and after a panicked one.
func TestViewSeesPreviousFrameLayout(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 200, Height: 100})
	boom := false
	frame := 0
	var found []bool
	w.SetView(func(w *Window) View {
		frame++
		if frame > 1 {
			// Record whether last frame's leaf is still reachable.
			_, ok := w.layout.findByID("lbl")
			found = append(found, ok)
		}
		if boom {
			panic("view boom")
		}
		return Column(ContainerCfg{
			Content: []View{Text(TextCfg{ID: "lbl", Text: "a"})},
		})
	})
	w.Update() // frame 1: builds the tree
	w.Update() // frame 2: normal frame, queries frame 1

	boom = true
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected view panic")
			}
		}()
		w.Update() // frame 3: queries frame 2, then panics
	}()

	boom = false
	w.Update() // frame 4: queries the tree the panic left behind

	want := []bool{true, true, true}
	if len(found) != len(want) {
		t.Fatalf("view ran %d query frames, want %d", len(found), len(want))
	}
	for i, ok := range found {
		if !ok {
			t.Errorf("frame %d: previous layout not findable from view", i+2)
		}
	}
}
