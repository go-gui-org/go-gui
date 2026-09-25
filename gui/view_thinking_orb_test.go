package gui

import (
	"math"
	"testing"
	"time"
)

func TestThinkingOrbDefaultLayout(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
		ID: "orb1",
	}), w)
	if layout.Shape.Width != 64 {
		t.Errorf("default width = %v, want 64", layout.Shape.Width)
	}
	if layout.Shape.Height != 64 {
		t.Errorf("default height = %v, want 64", layout.Shape.Height)
	}
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(layout.Children))
	}
	cv := layout.Children[0]
	if cv.Shape.shapeType != shapeDrawCanvas {
		t.Errorf("child shapeType = %d, want DrawCanvas", cv.Shape.shapeType)
	}
	if cv.Shape.events == nil || cv.Shape.events.OnDraw == nil {
		t.Error("canvas OnDraw not set")
	}
}

func TestThinkingOrbSmallSize(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
		ID:   "orb-small",
		Size: ThinkingOrbSmall,
	}), w)
	if layout.Shape.Width != 20 {
		t.Errorf("small width = %v, want 20", layout.Shape.Width)
	}
}

func TestThinkingOrbInvalidDesignClamps(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
		ID:     "orb-bad",
		Design: ThinkingOrbDesign(99),
	}), w)
	if layout.Shape.Width != 64 {
		t.Errorf("width = %v, want 64", layout.Shape.Width)
	}
}

func TestThinkingOrbWidthFallback(t *testing.T) {
	// Unset, negative, and non-finite widths floor onto the
	// tuned box side; a set width survives. The sanitize helper
	// itself is covered by TestMathSpinnerPositive.
	for _, width := range []float32{0, -5, float32(math.NaN()),
		float32(math.Inf(1))} {
		w := &Window{}
		layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
			ID:    "orb-width",
			Width: width,
		}), w)
		if layout.Shape.Width != 64 {
			t.Errorf("width %v laid out %v, want 64",
				width, layout.Shape.Width)
		}
	}
	w := &Window{}
	layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
		ID:    "orb-width-set",
		Width: 100,
	}), w)
	if layout.Shape.Width != 100 {
		t.Errorf("width 100 laid out %v, want 100",
			layout.Shape.Width)
	}
}

func TestThinkingOrbDurationClamps(t *testing.T) {
	if thinkingOrbDuration(1e9) != 50*time.Millisecond {
		t.Errorf("huge speed duration = %v, want 50ms",
			thinkingOrbDuration(1e9))
	}
	if thinkingOrbDuration(1e-9) != time.Hour {
		t.Errorf("tiny speed duration = %v, want 1h",
			thinkingOrbDuration(1e-9))
	}
}

func TestThinkingOrbInkMirror(t *testing.T) {
	// Near-black ink on light reads near 0; mirrored on dark it
	// reads near 255.
	if got := orbInkByte(0.1, false); got > 30 {
		t.Errorf("light ink = %d, want near 26", got)
	}
	if got := orbInkByte(0.1, true); got < 225 {
		t.Errorf("dark ink = %d, want near 230", got)
	}
	if orbAlphaByte(0) != 0 || orbAlphaByte(1) != 255 {
		t.Error("alpha ends must map exactly")
	}
}

func TestThinkingOrbPausedRegistersNoAnimation(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: new(int)})
	w.TestRender(func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{
			ID:     "orb-paused",
			Design: ThinkingOrbSearching,
			Paused: true,
		})
	})
	if w.HasAnimation(ScopeID("orb-paused", "orb")) {
		t.Error("paused orb must not register an animation")
	}
}

func TestThinkingOrbLiveRegistersAnimation(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: new(int)})
	w.TestRender(func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{
			ID:     "orb-live",
			Design: ThinkingOrbWorking,
		})
	})
	if !w.HasAnimation(ScopeID("orb-live", "orb")) {
		t.Error("live orb must register a view-bound animation")
	}
	found := w.TestDuplicateIDs()
	if len(found) != 0 {
		t.Errorf("duplicate effective IDs: %v", found)
	}
}

func TestThinkingOrbScopedIDsStayUnique(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: new(int)})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID:     "panel-a",
					Sizing: FitFit,
					Content: []View{
						ThinkingOrb(ThinkingOrbCfg{
							ID:     "orb",
							Design: ThinkingOrbSearching,
						}),
					},
				}),
				Column(ContainerCfg{
					ID:     "panel-b",
					Sizing: FitFit,
					Content: []View{
						ThinkingOrb(ThinkingOrbCfg{
							ID:     "orb",
							Design: ThinkingOrbComposing,
						}),
					},
				}),
			},
		})
	})
	if found := w.TestDuplicateIDs(); len(found) != 0 {
		t.Errorf("duplicate effective IDs: %v", found)
	}
}

func TestThinkingOrbLabelStructure(t *testing.T) {
	w := NewTestWindow(WindowCfg{State: new(int)})
	root := w.TestRender(func(*Window) View {
		return ThinkingOrbLabel(ThinkingOrbLabelCfg{
			ID:     "orb-label",
			Text:   "Searching the web…",
			Design: ThinkingOrbSearching,
		})
	})
	outer, ok := root.FindByID("orb-label")
	if !ok {
		t.Fatal("label outer not found")
	}
	if len(outer.Children) != 2 {
		t.Fatalf("label children = %d, want orb + text",
			len(outer.Children))
	}
	if found := w.TestDuplicateIDs(); len(found) != 0 {
		t.Errorf("duplicate effective IDs: %v", found)
	}
}
