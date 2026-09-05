package gui

import (
	"math"
	"testing"
)

// TestNumericInputStepTriangleBounds locks the named bounds of the step
// triangle (issue #335 §2): it sits 4pt below the field text, never
// below the ladder's bottom rung (N6, tiny) and never above the text it
// decorates. The old widget-local floor was 8 — below every named rung.
func TestNumericInputStepTriangleBounds(t *testing.T) {
	// The text sizes rendered in the generated layout, triangle included.
	textSizes := func(cfg NumericInputCfg) map[float32]bool {
		w := &Window{}
		v := NumericInput(cfg)
		layout := generateViewLayout(v, w)
		sizes := map[float32]bool{}
		var walk func(l Layout)
		walk = func(l Layout) {
			if l.Shape != nil && l.Shape.TC != nil {
				sizes[l.Shape.TC.TextStyle.Size] = true
			}
			for _, c := range l.Children {
				walk(c)
			}
		}
		walk(layout)
		return sizes
	}

	// The lowest size anywhere in the layout, which is the triangle
	// whenever it is stepped below the field text.
	minSize := func(sizes map[float32]bool) float32 {
		var lo float32
		set := false
		for s := range sizes {
			if !set || s < lo {
				lo, set = s, true
			}
		}
		return lo
	}

	// A small field text (12) steps to 8 — the old widget-local floor,
	// and below every named rung. The named floor lifts it to N6 (10),
	// a value nothing else in this layout renders, so the assertion is
	// not satisfied by the field text itself.
	small := DefaultTextStyle
	small.Size = 12
	smallSizes := textSizes(NumericInputCfg{
		ID:        "ni-small",
		TextStyle: small,
		StepCfg:   NumericStepCfg{ShowButtons: true, Step: 1},
	})
	if got := minSize(smallSizes); got != guiTheme.N6.Size {
		t.Errorf("small layout floors at %v, want N6 (%v); sizes = %v",
			got, guiTheme.N6.Size, smallSizes)
	}

	// A default field steps clear of both bounds.
	defSizes := textSizes(NumericInputCfg{
		ID:      "ni-def",
		StepCfg: NumericStepCfg{ShowButtons: true, Step: 1},
	})
	if got := minSize(defSizes); got != DefaultTextStyle.Size-4 {
		t.Errorf("default layout triangle is %v, want %v; sizes = %v",
			got, DefaultTextStyle.Size-4, defSizes)
	}

	// The ceiling: a field text below the bottom rung (8 against N6's
	// 10) would have the floor lift the triangle *above* the text it
	// decorates. The clamp keeps it at the text size. This is the bound
	// a theme with a large SizeTextTiny crosses at ordinary text sizes.
	belowFloor := DefaultTextStyle
	belowFloor.Size = guiTheme.N6.Size - 2
	belowSizes := textSizes(NumericInputCfg{
		ID:        "ni-below",
		TextStyle: belowFloor,
		StepCfg:   NumericStepCfg{ShowButtons: true, Step: 1},
	})
	if got := minSize(belowSizes); got != belowFloor.Size {
		t.Errorf("triangle is %v, want the field text %v; sizes = %v",
			got, belowFloor.Size, belowSizes)
	}
	if belowSizes[guiTheme.N6.Size] {
		t.Errorf("floor lifted the triangle to N6 (%v) above the field text %v; sizes = %v",
			guiTheme.N6.Size, belowFloor.Size, belowSizes)
	}
}

func TestNumericInputIDPassthrough(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:      "ni1",
		StepCfg: NumericStepCfg{ShowButtons: true, Step: 1},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "ni1" {
		t.Errorf("ID: got %s", layout.Shape.ID)
	}
}

func TestNumericInputDisabledFlag(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:       "ni2",
		Disabled: true,
		StepCfg:  NumericStepCfg{ShowButtons: true, Step: 1},
	})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

func TestNumericInputStepButtonCount(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:      "ni3",
		StepCfg: NumericStepCfg{ShowButtons: true, Step: 1},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Errorf("children: got %d, want 2", len(layout.Children))
	}
}

func TestNumericInputPlaceholder(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:          "ni4",
		Placeholder: "Enter...",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
}

// TestNumericInputReadOnlyForwardsToInput checks that a read-only
// NumericInput (no steppers) forwards ReadOnly to its inner Input,
// which announces AccessStateReadOnly.
func TestNumericInputReadOnlyForwardsToInput(t *testing.T) {
	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:       "ni-ro-plain",
		ReadOnly: true,
		Text:     "5",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YState != AccessStateReadOnly {
		t.Errorf("A11YState=%d, want ReadOnly (%d)",
			layout.Shape.A11YState, AccessStateReadOnly)
	}
}

// TestNumericInputBorderFollowsTheme covers issue #300: the numeric
// input used to resolve its fallback border/radius against a private
// literal that no theme could reach. It must take them from the
// theme's input style, and an explicit cfg value must still win.
func TestNumericInputBorderFollowsTheme(t *testing.T) {
	restoreTheme(t)
	th := ThemeDark.withInputStyle(InputStyle{SizeBorder: 7, Radius: 9})
	SetTheme(th)

	w := &Window{}
	v := NumericInput(NumericInputCfg{
		ID:      "ni-theme-border",
		StepCfg: NumericStepCfg{ShowButtons: true, Step: 1},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 7 {
		t.Errorf("size_border = %v, want 7 (input style)", layout.Shape.SizeBorder)
	}
	if layout.Shape.Radius != 9 {
		t.Errorf("radius = %v, want 9 (input style)", layout.Shape.Radius)
	}

	v = NumericInput(NumericInputCfg{
		ID:         "ni-theme-border-override",
		StepCfg:    NumericStepCfg{ShowButtons: true, Step: 1},
		SizeBorder: SomeF(3),
	})
	layout = generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 3 {
		t.Errorf("size_border = %v, want 3 (cfg wins)", layout.Shape.SizeBorder)
	}
}

// TestNumericInputReadOnlyStepButtonsGated covers #82: a read-only
// numeric input must not increment via its step buttons. The buttons
// are visually disabled, and numericInputApplyStep (the choke point)
// blocks the mutation even when the handler is invoked directly. The
// editable control below proves the probe observes stepping (remove the
// ReadOnly gate in numericInputApplyStep and the read-only assertion
// fails).
func TestNumericInputReadOnlyStepButtonsGated(t *testing.T) {
	step := func(readOnly bool) bool {
		committed := false
		w := &Window{}
		v := NumericInput(NumericInputCfg{
			ID:       "ni-step",
			ReadOnly: readOnly,
			Text:     "5",
			StepCfg:  NumericStepCfg{ShowButtons: true, Step: 1},
			OnValueCommit: func(_ Opt[float64], _ string, ctx EventCtx) {
				committed = true
			},
		})
		layout := generateViewLayout(v, w)
		up := findShapeByID(&layout, ScopeID("ni-step", "step_up"))
		if up == nil {
			t.Fatal("step-up button not found")
		}
		if readOnly && !up.Shape.Disabled {
			t.Error("read-only step-up button should be Disabled")
		}
		if up.Shape.events == nil || up.Shape.events.OnClick == nil {
			t.Fatal("step-up button missing OnClick")
		}
		// Invoke directly, bypassing the dispatch-level Disabled gate,
		// to exercise the choke point.
		up.Shape.events.OnClick(EventCtx{up, &Event{}, w})
		return committed
	}

	if !step(false) {
		t.Error("editable numeric input did not step the value")
	}
	if step(true) {
		t.Error("read-only numeric input stepped the value")
	}
}

// numericStepProbe builds a NumericInput and returns its handlers plus a
// recorder for the committed value. It drives the factories directly:
// the inner Input reaches them through makeInputOnKeyDown's
// unhandled-key delegation, which TestNumericInputWiresStepHandlers
// covers separately.
func numericStepProbe(
	t *testing.T, cfg NumericInputCfg,
) (onKey, onWheel func(EventCtx), got *string, w *Window, ly *Layout) {
	t.Helper()
	committed := ""
	got = &committed
	cfg.OnValueCommit = func(_ Opt[float64], text string, _ EventCtx) {
		committed = text
	}
	applyNumericInputDefaults(&cfg)
	locale := numericLocaleNormalize(cfg.Locale)
	stepCfg := numericStepCfgNormalize(cfg.StepCfg)

	w = &Window{}
	layout := generateViewLayout(NumericInput(cfg), w)
	ly = &layout
	return numericInputOnKeyDown(cfg, locale, stepCfg),
		numericInputOnWheel(cfg, locale, stepCfg), got, w, ly
}

func TestNumericInputArrowKeysStepByDefault(t *testing.T) {
	// StepCfg carries no Keyboard flag at all: stepping is on because
	// it is the spinbox convention, which is the whole point of #503.
	onKey, _, got, w, ly := numericStepProbe(t, NumericInputCfg{
		ID:      "ni-arrows",
		Text:    "5",
		StepCfg: NumericStepCfg{Step: 1},
	})
	if onKey == nil {
		t.Fatal("arrow stepping should be on by default")
	}

	up := &Event{KeyCode: KeyUp}
	onKey(EventCtx{ly, up, w})
	if *got != "6" {
		t.Errorf("Up stepped to %q, want \"6\"", *got)
	}
	if !up.IsHandled {
		t.Error("a key that stepped must be consumed")
	}

	down := &Event{KeyCode: KeyDown}
	onKey(EventCtx{ly, down, w})
	if *got != "4" {
		t.Errorf("Down stepped to %q, want \"4\"", *got)
	}
}

func TestNumericInputNonArrowKeyBubbles(t *testing.T) {
	// The acceptance criterion from #503: an unhandled arrow still
	// reaches an enclosing list, so consume only on the stepping paths.
	onKey, _, got, w, ly := numericStepProbe(t, NumericInputCfg{
		ID:      "ni-bubble",
		Text:    "5",
		StepCfg: NumericStepCfg{Step: 1},
	})
	e := &Event{KeyCode: KeyLeft}
	onKey(EventCtx{ly, e, w})
	if e.IsHandled {
		t.Error("a key that did not step must not be consumed")
	}
	if *got != "" {
		t.Errorf("KeyLeft committed %q, want no commit", *got)
	}
}

func TestNumericInputKeyboardDisabledOptsOut(t *testing.T) {
	onKey, _, _, _, _ := numericStepProbe(t, NumericInputCfg{
		ID:      "ni-nokeys",
		Text:    "5",
		StepCfg: NumericStepCfg{Step: 1, KeyboardDisabled: true},
	})
	if onKey != nil {
		t.Error("KeyboardDisabled should leave no key handler")
	}
}

func TestNumericInputReadOnlyDeclinesArrows(t *testing.T) {
	onKey, _, got, w, ly := numericStepProbe(t, NumericInputCfg{
		ID:       "ni-ro-keys",
		Text:     "5",
		ReadOnly: true,
		StepCfg:  NumericStepCfg{Step: 1},
	})
	e := &Event{KeyCode: KeyUp}
	onKey(EventCtx{ly, e, w})
	if *got != "" {
		t.Errorf("read-only field stepped to %q, want no commit", *got)
	}
	if e.IsHandled {
		t.Error("read-only field must decline the key, not swallow it")
	}
}

func TestNumericInputWheelIsOptIn(t *testing.T) {
	_, off, _, _, _ := numericStepProbe(t, NumericInputCfg{
		ID:      "ni-nowheel",
		Text:    "5",
		StepCfg: NumericStepCfg{Step: 1},
	})
	if off != nil {
		t.Fatal("wheel stepping must stay opt-in")
	}

	_, on, got, w, ly := numericStepProbe(t, NumericInputCfg{
		ID:      "ni-wheel",
		Text:    "5",
		StepCfg: NumericStepCfg{Step: 1, MouseWheel: true},
	})
	if on == nil {
		t.Fatal("MouseWheel should install a wheel handler")
	}
	// Only the sign of ScrollY is portable: lines for a wheel, points
	// for a trackpad. A large delta is still one step.
	e := &Event{ScrollY: 12}
	on(EventCtx{ly, e, w})
	if *got != "6" {
		t.Errorf("wheel up stepped to %q, want \"6\"", *got)
	}
	if !e.IsHandled {
		t.Error("a wheel event that stepped must be consumed")
	}

	zero := &Event{ScrollY: 0}
	on(EventCtx{ly, zero, w})
	if zero.IsHandled {
		t.Error("a zero-delta wheel event must not be consumed")
	}
}

func TestNumericInputWiresStepHandlers(t *testing.T) {
	// Proves the handlers actually reach the field's shape, which the
	// factory tests above cannot show.
	w := &Window{}
	committed := ""
	v := NumericInput(NumericInputCfg{
		ID:      "ni-wired",
		Text:    "5",
		StepCfg: NumericStepCfg{Step: 1, MouseWheel: true},
		OnValueCommit: func(_ Opt[float64], text string, _ EventCtx) {
			committed = text
		},
	})
	layout := generateViewLayout(v, w)
	layoutArrange(&layout, w)

	field := findShapeByID(&layout, "ni-wired")
	if field == nil {
		t.Fatal("numeric field not found")
	}
	if field.Shape.events == nil {
		t.Fatal("field shape carries no events")
	}
	if field.Shape.events.OnMouseScroll == nil {
		t.Error("field shape carries no OnMouseScroll")
	}

	// Asserting OnKeyDown is non-nil would prove nothing -- Input always
	// installs one. What has to hold is that an arrow reaches the step
	// path THROUGH it, by makeInputOnKeyDown's unhandled-key delegation.
	// Stepping is focus-gated there, so focus the field first.
	w.SetFocus(field.Shape.idKey())
	e := &Event{KeyCode: KeyUp}
	field.Shape.events.OnKeyDown(EventCtx{field, e, w})
	if committed != "6" {
		t.Errorf("arrow through the real handler committed %q, want \"6\"",
			committed)
	}
}

func TestNumericInputNaNInfStepFallsBackToOne(t *testing.T) {
	// NaN fails every comparison, so a naive <= 0 check passes it
	// through and the field commits "NaN" on the next arrow key.
	for _, step := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		got := numericStepCfgNormalize(NumericStepCfg{Step: step})
		if got.Step != 1 {
			t.Errorf("Step %v normalized to %v, want 1", step, got.Step)
		}
	}
	for _, m := range []float64{math.NaN(), math.Inf(1)} {
		got := numericStepCfgNormalize(NumericStepCfg{
			Step: 1, ShiftMultiplier: m, AltMultiplier: m,
		})
		if got.ShiftMultiplier != 10.0 {
			t.Errorf("Shift %v normalized to %v, want 10", m, got.ShiftMultiplier)
		}
		if got.AltMultiplier != 0.1 {
			t.Errorf("Alt %v normalized to %v, want 0.1", m, got.AltMultiplier)
		}
	}

	// End to end: a NaN step still steps by one through the key path.
	onKey, _, got, w, ly := numericStepProbe(t, NumericInputCfg{
		ID:      "ni-nan",
		Text:    "5",
		StepCfg: NumericStepCfg{Step: math.NaN()},
	})
	e := &Event{KeyCode: KeyUp}
	onKey(EventCtx{ly, e, w})
	if *got != "6" {
		t.Errorf("NaN step committed %q, want \"6\"", *got)
	}
}

func TestNumericInputStepHandlersNilEventNoPanic(t *testing.T) {
	onKey, onWheel, got, w, ly := numericStepProbe(t, NumericInputCfg{
		ID:      "ni-nilev",
		Text:    "5",
		StepCfg: NumericStepCfg{Step: 1, MouseWheel: true},
	})
	// Handlers run on dispatch events, which are never nil, but a
	// synthesized call must decline rather than panic.
	onKey(EventCtx{ly, nil, w})
	onWheel(EventCtx{ly, nil, w})
	if *got != "" {
		t.Errorf("nil event committed %q, want no commit", *got)
	}

	// The shared apply path takes the raw click event, which a
	// programmatic click may leave nil; modifiers default to none.
	committed := ""
	cfg := NumericInputCfg{ID: "ni-nilapply", Text: "5"}
	cfg.OnValueCommit = func(_ Opt[float64], text string, _ EventCtx) {
		committed = text
	}
	applyNumericInputDefaults(&cfg)
	numericInputApplyStep(nil, cfg,
		numericLocaleNormalize(cfg.Locale),
		numericStepCfgNormalize(cfg.StepCfg), 1, nil, w)
	if committed != "6" {
		t.Errorf("nil-event apply committed %q, want \"6\"", committed)
	}
}

func TestNumericInputWheelReadOnlyNoHandler(t *testing.T) {
	_, on, _, _, _ := numericStepProbe(t, NumericInputCfg{
		ID:       "ni-rowheel",
		Text:     "5",
		ReadOnly: true,
		StepCfg:  NumericStepCfg{Step: 1, MouseWheel: true},
	})
	if on != nil {
		t.Error("read-only field with MouseWheel should install no handler")
	}
}

func TestNumericInputArrowShiftStepsTenfold(t *testing.T) {
	onKey, _, got, w, ly := numericStepProbe(t, NumericInputCfg{
		ID:      "ni-shift",
		Text:    "5",
		StepCfg: NumericStepCfg{Step: 1},
	})
	e := &Event{KeyCode: KeyUp, Modifiers: ModShift}
	onKey(EventCtx{ly, e, w})
	if *got != "15" {
		t.Errorf("Shift+Up committed %q, want \"15\"", *got)
	}
	if !e.IsHandled {
		t.Error("a Shift step that moved must be consumed")
	}
}
