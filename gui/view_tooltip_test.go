package gui

import (
	"testing"
	"time"
)

func TestTooltipReturnsView(t *testing.T) {
	v := Tooltip(TooltipCfg{
		ID:      "tip1",
		Content: []View{Text(TextCfg{Text: "hello"})},
	})
	if v == nil {
		t.Fatal("expected non-nil view")
	}
	w := &Window{}
	layout := generateViewLayout(v, w)
	if !layout.Shape.Float {
		t.Error("expected floating layout")
	}
}

func TestTooltipCfgDefaults(t *testing.T) {
	cfg := TooltipCfg{ID: "test"}
	applyTooltipDefaults(&cfg)

	if !cfg.Color.IsSet() {
		t.Error("expected non-zero Color")
	}
	if cfg.Delay == 0 {
		t.Error("expected non-zero delay")
	}
	// Radius, OffsetX, OffsetY, Anchor, TieOff are now Opt
	// and resolve at use sites, not in applyTooltipDefaults.
	if cfg.Radius.IsSet() {
		t.Error("Radius should not be set by defaults")
	}
	if cfg.Anchor.IsSet() {
		t.Error("Anchor should not be set by defaults")
	}
}

func TestAnimationTooltipReturnsAnimate(t *testing.T) {
	a := animationTooltip(TooltipCfg{
		ID:    "tip1",
		Delay: 100 * time.Millisecond,
	})
	if a == nil {
		t.Fatal("expected non-nil Animate")
	}
	if a.AnimID != "___tooltip___" {
		t.Errorf("expected ___tooltip___ id, got %q",
			a.AnimID)
	}
}

func TestAnimationTooltipCallback(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip.bounds = drawClip{
		X: 10, Y: 10, Width: 50, Height: 50,
	}
	w.viewState.tooltip.hoverID = "tip1"
	w.viewState.mousePosX = 20
	w.viewState.mousePosY = 20

	a := animationTooltip(TooltipCfg{
		ID:    "tip1",
		Delay: 100 * time.Millisecond,
	})
	a.Callback(a, w)

	if w.viewState.tooltip.id != "tip1" {
		t.Errorf("expected tooltip id=tip1, got %q",
			w.viewState.tooltip.id)
	}
	if w.viewState.tooltip.popupID != ScopeID("tip1", "popup") {
		t.Errorf("expected popupID=tip1:popup, got %q",
			w.viewState.tooltip.popupID)
	}
}

func TestAnimationTooltipCallbackOutside(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip.bounds = drawClip{
		X: 10, Y: 10, Width: 50, Height: 50,
	}
	w.viewState.tooltip.hoverID = "tip1"
	w.viewState.mousePosX = 100
	w.viewState.mousePosY = 100

	a := animationTooltip(TooltipCfg{
		ID:    "tip1",
		Delay: 100 * time.Millisecond,
	})
	a.Callback(a, w)

	if w.viewState.tooltip.id != "" {
		t.Error("expected empty tooltip id when outside")
	}
}

func TestAnimationTooltipStaleHover(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip.bounds = drawClip{
		X: 10, Y: 10, Width: 50, Height: 50,
	}
	w.viewState.tooltip.hoverID = "other"
	w.viewState.mousePosX = 20
	w.viewState.mousePosY = 20

	a := animationTooltip(TooltipCfg{
		ID:    "tip1",
		Delay: 100 * time.Millisecond,
	})
	a.Callback(a, w)

	if w.viewState.tooltip.id != "" {
		t.Errorf("expected empty id with stale hover, got %q",
			w.viewState.tooltip.id)
	}
}

func TestWithTooltipReturnsView(t *testing.T) {
	w := &Window{}
	v := WithTooltip(w, WithTooltipCfg{
		ID:      "tip1",
		Text:    "hello",
		Content: []View{Text(TextCfg{Text: "trigger"})},
	})
	if v == nil {
		t.Fatal("expected non-nil view")
	}
}

func TestWithTooltipShowsPopupWhenActive(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip.id = "tip1"
	w.viewState.tooltip.popupID = ScopeID("tip1", "popup")
	v := WithTooltip(w, WithTooltipCfg{
		ID:      "tip1",
		Text:    "hello",
		Content: []View{Text(TextCfg{Text: "trigger"})},
	})
	layout := generateViewLayout(v, w)
	// Trigger content + tooltip popup.
	if len(layout.Children) < 2 {
		t.Errorf("expected >=2 children, got %d",
			len(layout.Children))
	}
}

func TestWithTooltipHidesPopupWhenInactive(t *testing.T) {
	w := &Window{}
	v := WithTooltip(w, WithTooltipCfg{
		ID:      "tip1",
		Text:    "hello",
		Content: []View{Text(TextCfg{Text: "trigger"})},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 1 {
		t.Errorf("expected 1 child, got %d",
			len(layout.Children))
	}
}

func TestWithTooltipNoBorderInflation(t *testing.T) {
	w := &Window{}
	v := WithTooltip(w, WithTooltipCfg{
		ID:      "tip1",
		Text:    "hello",
		Content: []View{Text(TextCfg{Text: "trigger"})},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("expected SizeBorder=0, got %v",
			layout.Shape.SizeBorder)
	}
}

// tooltipBoxWindow renders one WithTooltip wrapper, "tip1", around a
// fixed 100×100 box at the window origin, with the pointer at (mx, my).
// setup, when not nil, seeds the tooltip state before the frame. The
// hover is decided at the end of the arrange pass (resolveTooltipHover),
// so these tests run the whole frame rather than the amend hook alone.
func tooltipBoxWindow(
	t *testing.T, mx, my float32, setup func(*tooltipState),
) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	t.Cleanup(w.WindowCleanup)
	if setup != nil {
		setup(&w.viewState.tooltip)
	}
	w.viewState.mousePosX, w.viewState.mousePosY = mx, my
	w.TestRender(func(vw *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{
				WithTooltip(vw, WithTooltipCfg{
					ID:    "tip1",
					Text:  "hello",
					Delay: 500 * time.Millisecond,
					Content: []View{Column(ContainerCfg{
						Sizing: FixedFixed, Width: 100, Height: 100,
						SizeBorder: NoBorder,
					})},
				}),
			},
		})
	})
	return w
}

func TestWithTooltipAmendStartsHover(t *testing.T) {
	w := tooltipBoxWindow(t, 50, 50, nil)

	if w.viewState.tooltip.hoverID != "tip1" {
		t.Errorf("expected hoverID=tip1, got %q",
			w.viewState.tooltip.hoverID)
	}
	if w.viewState.tooltip.hoverStart.IsZero() {
		t.Error("expected non-zero hoverStart")
	}
}

func TestWithTooltipAmendClearsOnLeave(t *testing.T) {
	w := tooltipBoxWindow(t, 200, 200, func(ts *tooltipState) {
		ts.hoverID = "tip1"
		ts.hoverStart = time.Now()
		ts.id = "tip1"
		ts.popupID = ScopeID("tip1", "popup")
	})

	if w.viewState.tooltip.hoverID != "" {
		t.Errorf("expected empty hoverID, got %q",
			w.viewState.tooltip.hoverID)
	}
	if w.viewState.tooltip.id != "" {
		t.Errorf("expected empty tooltip id, got %q",
			w.viewState.tooltip.id)
	}
	if w.viewState.tooltip.popupID != "" {
		t.Errorf("expected empty popupID, got %q",
			w.viewState.tooltip.popupID)
	}
}

// A RichText run's tooltip shares the state; a wrapper leaves it to the
// run, whether the pointer is in the wrapper's box or not.
func TestWithTooltipAmendIgnoresOther(t *testing.T) {
	for _, at := range []float32{50, 200} {
		w := tooltipBoxWindow(t, at, at, func(ts *tooltipState) {
			ts.hoverID = "other"
			ts.text = "other"
		})

		if w.viewState.tooltip.hoverID != "other" {
			t.Errorf("pointer at %v: expected hoverID=other, got %q",
				at, w.viewState.tooltip.hoverID)
		}
	}
}

func TestWithTooltipAmendSetsIDAfterDelay(t *testing.T) {
	w := tooltipBoxWindow(t, 50, 50, func(ts *tooltipState) {
		ts.hoverID = "tip1"
		ts.hoverStart = time.Now().Add(-time.Second)
	})

	if w.viewState.tooltip.id != "tip1" {
		t.Errorf("expected tooltip id=tip1, got %q",
			w.viewState.tooltip.id)
	}
	if w.viewState.tooltip.popupID != ScopeID("tip1", "popup") {
		t.Errorf("expected popupID=tip1:popup, got %q",
			w.viewState.tooltip.popupID)
	}
}

func TestWithTooltipDefaultsID(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip.id = "hello"
	w.viewState.tooltip.popupID = "hello_popup"
	v := WithTooltip(w, WithTooltipCfg{
		Text:    "hello",
		Content: []View{Text(TextCfg{Text: "trigger"})},
	})
	layout := generateViewLayout(v, w)
	// ID defaults to Text; tooltip should show.
	if len(layout.Children) < 2 {
		t.Errorf("expected >=2 children, got %d",
			len(layout.Children))
	}
}

func TestTooltipExplicitZero(t *testing.T) {
	v := Tooltip(TooltipCfg{
		ID:      "tip1",
		Radius:  NoRadius,
		Content: []View{Text(TextCfg{Text: "hello"})},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if layout.Shape.Radius != 0 {
		t.Errorf("expected Radius=0, got %v",
			layout.Shape.Radius)
	}
}

func TestTooltipExplicitTopLeft(t *testing.T) {
	v := Tooltip(TooltipCfg{
		ID:      "tip1",
		Anchor:  Some(FloatTopLeft),
		Content: []View{Text(TextCfg{Text: "hello"})},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if layout.Shape.FloatAnchor != FloatTopLeft {
		t.Errorf("expected FloatTopLeft, got %d",
			layout.Shape.FloatAnchor)
	}
}

// tooltipPanel names one panel and the tooltip inside it. An empty
// tipID exercises the fallback to the label text as the leaf.
type tooltipPanel struct {
	panelID string
	tipID   string
	text    string
}

// tooltipsInPanels renders one tooltip wrapper inside each ID-bearing
// panel, with the mouse parked inside the first panel's trigger so that
// wrapper's AmendLayout runs hovered and every later one runs outside.
// The trigger is a fixed-size rectangle so the hit test does not depend
// on text measurement, which is a nil-measurer approximation in tests.
func tooltipsInPanels(t *testing.T, panels ...tooltipPanel) *Window {
	w := NewTestWindow(t, WindowCfg{})
	t.Cleanup(w.WindowCleanup)
	w.viewState.mousePosX = 60
	w.viewState.mousePosY = 60
	w.SetView(func(vw *Window) View {
		views := make([]View, 0, len(panels))
		for _, p := range panels {
			views = append(views, Column(ContainerCfg{
				ID: p.panelID,
				Content: []View{
					// Called while the panel's Content slice is built,
					// which is the eager-factory position issue #528 is
					// about.
					WithTooltip(vw, WithTooltipCfg{
						ID:   p.tipID,
						Text: p.text,
						Content: []View{
							Rectangle(RectangleCfg{
								Width:  100,
								Height: 50,
								Sizing: FixedFixed,
							}),
						},
					}),
				},
			}))
		}
		return Column(ContainerCfg{Sizing: FillFill, Content: views})
	})
	return w
}

// A tooltip built inside an ID-bearing panel must key its hover state on
// the identity its own shape resolves to, not on the scope of whatever
// was being built when the factory ran (issue #528).
func TestWithTooltipScopesHoverStateToPanel(t *testing.T) {
	w := tooltipsInPanels(t, tooltipPanel{"panel", "tip1", "hello"})
	w.TestRender(nil)

	if got := w.viewState.tooltip.hoverID; got != "panel:tip1" {
		t.Fatalf("hover key = %q, want %q", got, "panel:tip1")
	}
}

// The same label in two panels is two tooltips, in one window. Pre-fix
// both wrappers keyed hover on the bare "hello": the second wrapper saw
// its own tipID in ts.hoverID while the mouse was outside it and cleared
// the first wrapper's hover. The observable result is that hovering a
// tooltip did nothing whenever a same-labelled twin existed elsewhere.
func TestWithTooltipSameTextInTwoPanelsIsTwoKeys(t *testing.T) {
	w := tooltipsInPanels(t,
		tooltipPanel{panelID: "a", text: "hello"},
		tooltipPanel{panelID: "b", text: "hello"},
	)
	w.TestRender(nil)

	if got := w.viewState.tooltip.hoverID; got != "a:hello" {
		t.Fatalf("hover key = %q, want %q", got, "a:hello")
	}
}

// The popup's own shape resolves under the panel too, so the identity
// and the state key are one string.
func TestWithTooltipPopupIDResolvesUnderPanel(t *testing.T) {
	w := tooltipsInPanels(t, tooltipPanel{"panel", "tip1", "hello"})
	w.viewState.tooltip.id = "panel:tip1"
	w.viewState.tooltip.popupID = ScopeID("panel:tip1", "popup")
	root := w.TestRender(nil)

	if _, ok := root.FindByID("panel:tip1:popup"); !ok {
		t.Fatalf("FindByID(%q) = false", "panel:tip1:popup")
	}
}

// The dev-mode gate that reports an eagerly resolved key must stay quiet
// for the fixed widget.
func TestWithTooltipUnderPanelIsQuiet(t *testing.T) {
	w := tooltipsInPanels(t, tooltipPanel{"panel", "tip1", "hello"})
	if found := w.TestFindings(DebugAll); len(found) != 0 {
		t.Fatalf("findings = %v, want none", found)
	}
}
