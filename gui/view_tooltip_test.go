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
	a := AnimationTooltip(TooltipCfg{
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

	a := AnimationTooltip(TooltipCfg{
		ID:    "tip1",
		Delay: 100 * time.Millisecond,
	})
	a.Callback(a, w)

	if w.viewState.tooltip.id != "tip1" {
		t.Errorf("expected tooltip id=tip1, got %q",
			w.viewState.tooltip.id)
	}
	if w.viewState.tooltip.popupID != "tip1_popup" {
		t.Errorf("expected popupID=tip1_popup, got %q",
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

	a := AnimationTooltip(TooltipCfg{
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

	a := AnimationTooltip(TooltipCfg{
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
	w.viewState.tooltip.popupID = "tip1_popup"
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

func TestWithTooltipAmendStartsHover(t *testing.T) {
	w := &Window{}
	w.viewState.mousePosX = 50
	w.viewState.mousePosY = 50

	v := WithTooltip(w, WithTooltipCfg{
		ID:      "tip1",
		Text:    "hello",
		Content: []View{Text(TextCfg{Text: "trigger"})},
	})
	layout := generateViewLayout(v, w)
	layout.Shape.X = 0
	layout.Shape.Y = 0
	layout.Shape.Width = 100
	layout.Shape.Height = 100

	if layout.Shape.events == nil ||
		layout.Shape.events.AmendLayout == nil {
		t.Fatal("expected AmendLayout handler")
	}
	layout.Shape.events.AmendLayout(&layout, w)

	if w.viewState.tooltip.hoverID != "tip1" {
		t.Errorf("expected hoverID=tip1, got %q",
			w.viewState.tooltip.hoverID)
	}
	if w.viewState.tooltip.hoverStart.IsZero() {
		t.Error("expected non-zero hoverStart")
	}
}

func TestWithTooltipAmendClearsOnLeave(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip.hoverID = "tip1"
	w.viewState.tooltip.hoverStart = time.Now()
	w.viewState.tooltip.id = "tip1"
	w.viewState.tooltip.popupID = "tip1_popup"
	w.viewState.mousePosX = 200
	w.viewState.mousePosY = 200

	v := WithTooltip(w, WithTooltipCfg{
		ID:      "tip1",
		Text:    "hello",
		Content: []View{Text(TextCfg{Text: "trigger"})},
	})
	layout := generateViewLayout(v, w)
	layout.Shape.X = 0
	layout.Shape.Y = 0
	layout.Shape.Width = 100
	layout.Shape.Height = 100

	layout.Shape.events.AmendLayout(&layout, w)

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

func TestWithTooltipAmendIgnoresOther(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip.hoverID = "other"
	w.viewState.mousePosX = 200
	w.viewState.mousePosY = 200

	v := WithTooltip(w, WithTooltipCfg{
		ID:      "tip1",
		Text:    "hello",
		Content: []View{Text(TextCfg{Text: "trigger"})},
	})
	layout := generateViewLayout(v, w)
	layout.Shape.X = 0
	layout.Shape.Y = 0
	layout.Shape.Width = 100
	layout.Shape.Height = 100

	layout.Shape.events.AmendLayout(&layout, w)

	if w.viewState.tooltip.hoverID != "other" {
		t.Errorf("expected hoverID=other, got %q",
			w.viewState.tooltip.hoverID)
	}
}

func TestWithTooltipAmendSetsIDAfterDelay(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip.hoverID = "tip1"
	w.viewState.tooltip.hoverStart = time.Now().Add(-time.Second)
	w.viewState.mousePosX = 50
	w.viewState.mousePosY = 50

	v := WithTooltip(w, WithTooltipCfg{
		ID:      "tip1",
		Text:    "hello",
		Delay:   500 * time.Millisecond,
		Content: []View{Text(TextCfg{Text: "trigger"})},
	})
	layout := generateViewLayout(v, w)
	layout.Shape.X = 0
	layout.Shape.Y = 0
	layout.Shape.Width = 100
	layout.Shape.Height = 100

	layout.Shape.events.AmendLayout(&layout, w)

	if w.viewState.tooltip.id != "tip1" {
		t.Errorf("expected tooltip id=tip1, got %q",
			w.viewState.tooltip.id)
	}
	if w.viewState.tooltip.popupID != "tip1_popup" {
		t.Errorf("expected popupID=tip1_popup, got %q",
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
