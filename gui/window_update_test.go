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

// Putting w.layout.Children back into layerLayouts before the view
// runs used to leave the live header pointing at the pooled array.
// A panic then skipped the replacement, so the next frame's put/take
// truncated that array under the still-live tree (issue #689).
func TestRootViewPanicDoesNotAliasLayerPool(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 200, Height: 100})
	boom := false
	w.SetView(func(w *Window) View {
		if boom {
			panic("view boom")
		}
		return Column(ContainerCfg{
			Content: []View{
				Text(TextCfg{Text: "a"}),
				Text(TextCfg{Text: "b"}),
			},
		})
	})
	w.Update()
	if len(w.layout.Children) == 0 {
		t.Fatal("good frame should have children")
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

	if w.layout.Children != nil {
		t.Fatal("after panic, live Children must not still point at the pooled array")
	}

	boom = false
	w.Update()
	if len(w.layout.Children) == 0 {
		t.Fatal("recovery frame should have children")
	}
	if w.viewState.genDepth != 0 {
		t.Fatalf("recovery genDepth=%d, want 0", w.viewState.genDepth)
	}
}
