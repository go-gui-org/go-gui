package gui

import (
	"testing"
	"time"
)

func TestToastAddCreatesNotification(t *testing.T) {
	w := &Window{}
	id := w.Toast(ToastCfg{Title: "Test", Body: "Body"})
	if id == 0 {
		t.Error("expected non-zero toast id")
	}
	if len(w.toasts) != 1 {
		t.Fatalf("expected 1 toast, got %d", len(w.toasts))
	}
	if w.toasts[0].phase != toastEntering {
		t.Error("expected entering phase")
	}
}

func TestToastCounterIncrements(t *testing.T) {
	w := &Window{}
	id1 := w.Toast(ToastCfg{Title: "A"})
	id2 := w.Toast(ToastCfg{Title: "B"})
	if id2 <= id1 {
		t.Errorf("expected id2 > id1, got %d <= %d", id2, id1)
	}
}

func TestToastRemove(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Title: "A"}, animFrac: 1},
		{id: 2, cfg: ToastCfg{Title: "B"}, animFrac: 1},
		{id: 3, cfg: ToastCfg{Title: "C"}, animFrac: 1},
	}
	toastRemove(w, 2)
	if len(w.toasts) != 2 {
		t.Fatalf("expected 2 toasts, got %d", len(w.toasts))
	}
	for _, toast := range w.toasts {
		if toast.id == 2 {
			t.Error("toast 2 should have been removed")
		}
	}
}

func TestToastSetHovered(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Title: "A"}},
		{id: 2, cfg: ToastCfg{Title: "B"}},
	}
	toastSetHovered(w, 2, true)
	if !w.toasts[1].hovered {
		t.Error("expected toast 2 to be hovered")
	}
	toastSetHovered(w, 2, false)
	if w.toasts[1].hovered {
		t.Error("expected toast 2 not hovered")
	}
}

func TestToastEnforceMaxVisible(t *testing.T) {
	w := &Window{}
	// Add more than max.
	for i := range 8 {
		w.toasts = append(w.toasts, toastNotification{
			id:       uint64(i + 1),
			cfg:      ToastCfg{Title: "T"},
			animFrac: 1,
			phase:    toastVisible,
		})
	}
	toastEnforceMaxVisible(w)

	exiting := 0
	for _, toast := range w.toasts {
		if toast.phase == toastExiting {
			exiting++
		}
	}
	if exiting < 3 {
		t.Errorf("expected >= 3 exiting, got %d", exiting)
	}
}

func TestToastContainerViewNilWhenEmpty(t *testing.T) {
	w := &Window{}
	v := toastContainerView(w)
	if v != nil {
		t.Error("expected nil when no toasts")
	}
}

func TestToastContainerViewReturnsView(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Title: "Hi"}, animFrac: 1},
	}
	v := toastContainerView(w)
	if v == nil {
		t.Fatal("expected non-nil view")
	}
}

func TestToastDuration(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Duration: 5 * time.Second}},
	}
	d := toastDuration(w, 1)
	if d != 5*time.Second {
		t.Errorf("expected 5s, got %v", d)
	}
}

func TestToastDurationDefault(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{}},
	}
	d := toastDuration(w, 1)
	if d != toastDefaultDelay {
		t.Errorf("expected %v, got %v", toastDefaultDelay, d)
	}
}

func TestToastStartExitGuardsDouble(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Title: "T"}, animFrac: 1,
			phase: toastExiting},
	}
	// Should not panic or add another animation.
	toastStartExit(w, 1)
	if w.toasts[0].phase != toastExiting {
		t.Error("expected phase still exiting")
	}
}

func TestToastDismissAll(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Title: "A"}, animFrac: 1,
			phase: toastVisible},
		{id: 2, cfg: ToastCfg{Title: "B"}, animFrac: 1,
			phase: toastVisible},
	}
	w.ToastDismissAll()
	for _, toast := range w.toasts {
		if toast.phase != toastExiting {
			t.Errorf("toast %d should be exiting", toast.id)
		}
	}
}

func TestToastDurationPersistent(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Duration: toastPersistent}},
	}
	d := toastDuration(w, 1)
	if d != 0 {
		t.Errorf("expected 0 for persistent, got %v", d)
	}
}

func TestToastDurationNegative(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Duration: -5 * time.Second}},
	}
	d := toastDuration(w, 1)
	if d != 0 {
		t.Errorf("expected 0 for negative duration, got %v", d)
	}
}

func TestToastHoverClearedEachFrame(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Title: "A"}, animFrac: 1,
			hovered: true},
		{id: 2, cfg: ToastCfg{Title: "B"}, animFrac: 1,
			hovered: true},
	}
	toastContainerView(w)
	for _, toast := range w.toasts {
		if toast.hovered {
			t.Errorf("toast %d hovered should be cleared", toast.id)
		}
	}
}

func TestToastOnActionCallback(t *testing.T) {
	fired := false
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{
			Title:       "T",
			ActionLabel: "Undo",
			OnAction:    func(_ *Window) { fired = true },
		}, animFrac: 1, phase: toastVisible},
	}
	// Simulate action callback.
	w.toasts[0].cfg.OnAction(w)
	if !fired {
		t.Error("expected OnAction callback to fire")
	}
}

func TestToastAnchorPositioning(t *testing.T) {
	cases := []struct {
		anchor ToastAnchor
		name   string
	}{
		{ToastTopLeft, "TopLeft"},
		{ToastTopRight, "TopRight"},
		{ToastBottomLeft, "BottomLeft"},
		{ToastBottomRight, "BottomRight"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved := DefaultToastStyle.Anchor
			DefaultToastStyle.Anchor = tc.anchor
			defer func() { DefaultToastStyle.Anchor = saved }()

			w := &Window{}
			w.toasts = []toastNotification{
				{id: 1, cfg: ToastCfg{Title: "T"},
					animFrac: 1},
			}
			v := toastContainerView(w)
			if v == nil {
				t.Fatal("expected non-nil view")
			}
		})
	}
}

func TestToastItemViewSeverityColors(t *testing.T) {
	style := DefaultToastStyle
	severities := []ToastSeverity{
		ToastInfo, ToastSuccess, ToastWarning, ToastError,
	}
	for _, sev := range severities {
		toast := &toastNotification{
			id:       1,
			cfg:      ToastCfg{Title: "T", Severity: sev},
			animFrac: 1,
		}
		v := toastItemView(toast, style)
		if v == nil {
			t.Errorf("severity %d: expected non-nil view", sev)
		}
	}
}

func TestToastA11YLabelFallback(t *testing.T) {
	// Title present → use title.
	toast := &toastNotification{
		cfg: ToastCfg{Title: "Alert", Body: "Details"},
	}
	if got := toastA11YLabel(toast); got != "Alert" {
		t.Errorf("expected 'Alert', got %q", got)
	}
	// Title empty → fall back to body.
	toast2 := &toastNotification{
		cfg: ToastCfg{Body: "Details only"},
	}
	if got := toastA11YLabel(toast2); got != "Details only" {
		t.Errorf("expected 'Details only', got %q", got)
	}
	// Both empty → empty string.
	toast3 := &toastNotification{}
	if got := toastA11YLabel(toast3); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestToastAnimID(t *testing.T) {
	got := toastAnimID("enter", 42)
	if got != "enter_toast_42" {
		t.Errorf("expected 'enter_toast_42', got %q", got)
	}
	got = toastAnimID("dismiss", 0)
	if got != "dismiss_toast_0" {
		t.Errorf("expected 'dismiss_toast_0', got %q", got)
	}
}
