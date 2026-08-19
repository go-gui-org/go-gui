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
			OnAction:    func(ctx EventCtx) { fired = true },
		}, animFrac: 1, phase: toastVisible},
	}
	// Simulate action callback.
	w.toasts[0].cfg.OnAction(EventCtx{nil, nil, w})
	if !fired {
		t.Error("expected OnAction callback to fire")
	}
}

func TestToastAnchorPositioning(t *testing.T) {
	cases := []struct {
		anchor toastAnchor
		name   string
	}{
		{toastTopLeft, "TopLeft"},
		{toastTopRight, "TopRight"},
		{toastBottomLeft, "BottomLeft"},
		{toastBottomRight, "BottomRight"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			saved := defaultToastStyle.Anchor
			defaultToastStyle.Anchor = tc.anchor
			defer func() { defaultToastStyle.Anchor = saved }()

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
	style := defaultToastStyle
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
	if got != "enter:toast:42" {
		t.Errorf("expected 'enter:toast:42', got %q", got)
	}
	got = toastAnimID("dismiss", 0)
	if got != "dismiss:toast:0" {
		t.Errorf("expected 'dismiss:toast:0', got %q", got)
	}
}

// --- dismiss flow ---

func TestToastStartDismissTimerPersistentNoAnimation(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Duration: toastPersistent}, phase: toastVisible},
	}
	toastStartDismissTimer(w, 1)
	if w.HasAnimation(toastAnimID("dismiss", 1)) {
		t.Error("persistent toast must not arm a dismiss timer")
	}
}

func TestToastStartDismissTimerArms(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Duration: 5 * time.Second}, phase: toastVisible},
	}
	toastStartDismissTimer(w, 1)

	a, ok := w.animations[toastAnimID("dismiss", 1)]
	if !ok {
		t.Fatal("dismiss timer animation not registered")
	}
	anim, ok := a.(*Animate)
	if !ok {
		t.Fatalf("dismiss timer = %T, want *Animate", a)
	}
	if anim.Delay != 5*time.Second {
		t.Errorf("delay = %v, want 5s", anim.Delay)
	}
}

func TestToastStartDismissTimerCallbackExits(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Duration: time.Second}, phase: toastVisible},
	}
	toastStartDismissTimer(w, 1)
	a := w.animations[toastAnimID("dismiss", 1)].(*Animate)

	// Fire the timer: toast is not hovered → starts exit.
	a.Callback(a, w)
	if w.toasts[0].phase != toastExiting {
		t.Error("non-hovered toast should exit when the timer fires")
	}
}

func TestToastStartDismissTimerCallbackHoveredResets(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Duration: time.Second}, phase: toastVisible, hovered: true},
	}
	toastStartDismissTimer(w, 1)
	a := w.animations[toastAnimID("dismiss", 1)].(*Animate)

	// Fire the timer while hovered: hover cleared, timer re-armed,
	// toast stays visible.
	a.Callback(a, w)
	if w.toasts[0].phase != toastVisible {
		t.Error("hovered toast must not exit when the timer fires")
	}
	if w.toasts[0].hovered {
		t.Error("hovered flag should be cleared by the timer callback")
	}
	if !w.HasAnimation(toastAnimID("dismiss", 1)) {
		t.Error("dismiss timer should be re-armed after a hovered fire")
	}
}

func TestToastStartDismissTimerHoveredResetThenExits(t *testing.T) {
	// Simulate the natural sequence: timer fires while hovered
	// (re-arms), then fires again once the pointer left — exit starts.
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Duration: time.Second}, phase: toastVisible, hovered: true},
	}
	toastStartDismissTimer(w, 1)
	a := w.animations[toastAnimID("dismiss", 1)].(*Animate)

	a.Callback(a, w) // hovered → re-arm
	second := w.animations[toastAnimID("dismiss", 1)].(*Animate)
	second.Callback(second, w) // not hovered anymore → exit
	if w.toasts[0].phase != toastExiting {
		t.Error("toast should exit on the second timer fire")
	}
}

func TestToastDismiss(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Title: "A"}, phase: toastVisible},
		{id: 2, cfg: ToastCfg{Title: "B"}, phase: toastVisible},
	}
	w.ToastDismiss(1)
	if w.toasts[0].phase != toastExiting {
		t.Error("dismissed toast should be exiting")
	}
	if w.toasts[1].phase != toastVisible {
		t.Error("other toast must be untouched")
	}
	if !w.HasAnimation(toastAnimID("exit", 1)) {
		t.Error("exit animation should be registered")
	}
}

func TestToastDismissUnknownIDNoop(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, cfg: ToastCfg{Title: "A"}, phase: toastVisible},
	}
	w.ToastDismiss(99)
	if w.toasts[0].phase != toastVisible {
		t.Error("dismissing an unknown id must not touch existing toasts")
	}
	if len(w.animations) != 0 {
		t.Errorf("unknown dismiss registered animations: %d", len(w.animations))
	}
}

func TestToastFullLifecycle(t *testing.T) {
	// Toast → enter → dismiss timer → exit → removed, driven through
	// the animation callbacks so no timers run.
	w := &Window{}
	id := w.Toast(ToastCfg{Title: "T"})

	// Enter animation registered on creation.
	enter, ok := w.animations[toastAnimID("enter", id)].(*TweenAnimation)
	if !ok {
		t.Fatal("enter animation not registered")
	}
	// Finish enter → visible + dismiss timer armed.
	enter.OnDone(w)
	if w.toasts[0].phase != toastVisible {
		t.Fatal("toast should be visible after enter completes")
	}
	dismiss := w.animations[toastAnimID("dismiss", id)].(*Animate)

	// Fire dismiss → exiting.
	dismiss.Callback(dismiss, w)
	if w.toasts[0].phase != toastExiting {
		t.Fatal("toast should be exiting after dismiss fires")
	}
	// Finish exit → removed.
	exit := w.animations[toastAnimID("exit", id)].(*TweenAnimation)
	exit.OnDone(w)
	if len(w.toasts) != 0 {
		t.Fatalf("toasts = %d, want 0 after exit completes", len(w.toasts))
	}
}
