package gui

import (
	"strings"
	"testing"
)

// captureDebug turns the gate on, redirects findings to a buffer, and
// restores both on cleanup. Returns the buffer.
func captureDebug(t *testing.T) *strings.Builder {
	t.Helper()
	var buf strings.Builder
	prevOut := debugOut
	prevOn := debugEnabled.Load()
	debugOut = &buf
	Debug(true)
	t.Cleanup(func() {
		debugOut = prevOut
		Debug(prevOn)
	})
	return &buf
}

// debugTree builds a two-level layout from the given shapes: root
// with each shape as a direct child.
func debugTree(shapes ...*Shape) Layout {
	root := Layout{Shape: &Shape{}}
	for _, s := range shapes {
		root.Children = append(root.Children, Layout{Shape: s})
	}
	return root
}

func TestDebugAuditFocusableWithoutID(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	tree := debugTree(&Shape{Focusable: true})

	w.debugAudit(&tree)

	got := buf.String()
	if !strings.Contains(got, "focusable shape at 0 has no ID") {
		t.Fatalf("want focusable-no-ID finding, got %q", got)
	}
}

// A focusable shape excluded from tab traversal is not a defect:
// FocusSkip is how Text/RTF keep click-focus without a tab stop.
func TestDebugAuditFocusSkipAndDisabledAreQuiet(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	tree := debugTree(
		&Shape{Focusable: true, FocusSkip: true},
		&Shape{Focusable: true, Disabled: true},
	)

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Fatalf("want no findings, got %q", got)
	}
}

func TestDebugAuditScrollableWithoutID(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	tree := debugTree(&Shape{Scrollable: true})

	w.debugAudit(&tree)

	got := buf.String()
	if !strings.Contains(got, "scrollable shape at 0 has no ID") {
		t.Fatalf("want scrollable-no-ID finding, got %q", got)
	}
}

// The duplicate-ID check covers every shape, not only focusable ones,
// because ID is also the key for scroll offsets and per-widget state.
func TestDebugAuditDuplicateIDCoversNonFocusable(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	tree := debugTree(&Shape{ID: "dup"}, &Shape{ID: "dup"})

	w.debugAudit(&tree)

	got := buf.String()
	if !strings.Contains(got, `duplicate ID "dup" at 1, first claimed at 0`) {
		t.Fatalf("want duplicate-ID finding naming both sites, got %q", got)
	}
}

// Distinct IDs on a nested tree must stay quiet, and the path must
// reflect the full index chain.
func TestDebugAuditNestedPath(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	inner := Layout{Shape: &Shape{ID: "outer"}}
	inner.Children = []Layout{
		{Shape: &Shape{ID: "ok"}},
		{Shape: &Shape{Focusable: true}},
	}
	tree := Layout{Shape: &Shape{}, Children: []Layout{inner}}

	w.debugAudit(&tree)

	if got := buf.String(); !strings.Contains(got, "focusable shape at 0/1 has no ID") {
		t.Fatalf("want path 0/1, got %q", got)
	}
}

// Findings are per-frame checks, so without dedupe one broken button
// emits at the frame rate. Warn once per (check, subject) per window.
func TestDebugAuditWarnsOncePerWindow(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	tree := debugTree(&Shape{Focusable: true})

	w.debugAudit(&tree)
	first := buf.String()
	w.debugAudit(&tree)

	if buf.String() != first {
		t.Fatalf("second audit re-reported: %q", buf.String())
	}
	// A second window has its own memory.
	other := &Window{}
	other.debugAudit(&tree)
	if buf.String() == first {
		t.Fatal("want a fresh window to report independently")
	}
}

// Re-enabling the gate must report the current state rather than
// staying silent on everything it already warned about.
func TestDebugToggleResetsWarnOnce(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	tree := debugTree(&Shape{Focusable: true})

	w.debugAudit(&tree)
	first := buf.String()
	if first == "" {
		t.Fatal("want a finding on the first audit")
	}

	Debug(false)
	Debug(true)
	w.debugAudit(&tree)

	if buf.String() == first {
		t.Fatal("want the finding re-reported after the gate is cycled")
	}
}

func TestDebugAuditNoopWhenOff(t *testing.T) {
	buf := captureDebug(t)
	Debug(false)
	w := &Window{}
	tree := debugTree(&Shape{Focusable: true}, &Shape{ID: "d"}, &Shape{ID: "d"})

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Fatalf("gate off must be silent, got %q", got)
	}
	if DebugEnabled() {
		t.Fatal("DebugEnabled must report false")
	}
}

func TestDebugPath(t *testing.T) {
	tests := []struct {
		want string
		path []int
	}{
		{want: "root"},
		{path: []int{0}, want: "0"},
		{path: []int{0, 3, 1}, want: "0/3/1"},
	}
	for _, tc := range tests {
		if got := debugPath(tc.path); got != tc.want {
			t.Errorf("debugPath(%v) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestEnvTruthy(t *testing.T) {
	tests := []struct {
		val  string
		want bool
	}{
		{val: "1", want: true},
		{val: "true", want: true},
		{val: " 1 ", want: true},
		{val: "0", want: false},
		{val: "", want: false},
		{val: "yes", want: false},
	}
	for _, tc := range tests {
		t.Setenv("GOGUI_DEBUG_TEST", tc.val)
		if got := envTruthy("GOGUI_DEBUG_TEST"); got != tc.want {
			t.Errorf("envTruthy(%q) = %v, want %v", tc.val, got, tc.want)
		}
	}
	if envTruthy("GOGUI_DEBUG_TEST_UNSET") {
		t.Error("unset variable must be false")
	}
}

// State panics with both types named, so the message says what the
// window actually holds rather than only what was asked for.
func TestStatePanicNamesBothTypes(t *testing.T) {
	type held struct{}
	type want struct{}
	w := &Window{state: &held{}}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("want a panic on a state type mismatch")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("want a string panic value, got %T", r)
		}
		if !strings.Contains(msg, "held") || !strings.Contains(msg, "want") {
			t.Fatalf("want both type names in %q", msg)
		}
	}()
	State[want](w)
}

// An OnMouseLeave is tracked through a map keyed by ID, and that guard
// has no Focusable precondition — so this defect exists on shapes the
// focus check passes over, and needs its own finding.
func TestDebugAuditMouseLeaveWithoutID(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	tree := debugTree(&Shape{
		events: &eventHandlers{OnMouseLeave: func(EventCtx) {}},
	})

	w.debugAudit(&tree)

	got := buf.String()
	if !strings.Contains(got, "has an OnMouseLeave but no ID") {
		t.Fatalf("want mouseleave-no-ID finding, got %q", got)
	}
	// The shape is not focusable, so the focus check must stay quiet:
	// two findings for one shape would misdescribe the defect.
	if strings.Contains(got, "focusable shape") {
		t.Errorf("focus check fired on a non-focusable shape: %q", got)
	}
}

// The decorative opt-out covers focus, not leave tracking. A
// FocusDisabled control with an OnMouseLeave is still broken, and this
// is the case neither the focus check nor the requireFocusID guard
// catches.
func TestDebugAuditMouseLeaveOnFocusDisabledShape(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	// FocusDisabled has already been resolved into Focusable: false by
	// the time a shape exists, which is why the focus check misses it.
	tree := debugTree(&Shape{
		Focusable: false,
		events:    &eventHandlers{OnMouseLeave: func(EventCtx) {}},
	})

	w.debugAudit(&tree)

	if got := buf.String(); !strings.Contains(got, "has an OnMouseLeave but no ID") {
		t.Fatalf("want mouseleave-no-ID finding, got %q", got)
	}
}

func TestDebugAuditMouseLeaveWithIDIsQuiet(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	tree := debugTree(&Shape{
		ID:     "panel",
		events: &eventHandlers{OnMouseLeave: func(EventCtx) {}},
	})

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Errorf("want silence for an ID'd shape, got %q", got)
	}
}

// Disabled shapes never reach the leave-tracking code, so reporting
// them would be a false positive.
func TestDebugAuditMouseLeaveDisabledIsQuiet(t *testing.T) {
	buf := captureDebug(t)
	w := &Window{}
	tree := debugTree(&Shape{
		Disabled: true,
		events:   &eventHandlers{OnMouseLeave: func(EventCtx) {}},
	})

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Errorf("want silence for a disabled shape, got %q", got)
	}
}

// Two Inputs and a multi-paragraph Markdown are the two widgets that
// used to report duplicates against themselves: Input repeated its ID
// on its inner text shape, and every markdown paragraph claimed the
// widget's ID. Both now reference their owner instead of claiming its
// identity, so a window built only from them is silent.
func TestTestDuplicateIDsCompositeWidgetsAreQuiet(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.UpdateView(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Input(InputCfg{ID: "one", Text: "hello"}),
				Input(InputCfg{ID: "two", Mode: InputMultiline}),
				w.Markdown(MarkdownCfg{
					ID:     "doc",
					Source: "First paragraph.\n\nSecond paragraph.\n\nThird.",
				}),
			},
		})
	})

	if got := w.TestDuplicateIDs(); len(got) != 0 {
		t.Fatalf("want no findings, got %q", got)
	}
}

// A genuine collision — a widget nested inside another with the same
// ID — must still be reported. This is the case the "descendants may
// reuse an ancestor's ID" shortcut would have hidden.
func TestTestDuplicateIDsReportsNestedCollision(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.UpdateView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Button(ButtonCfg{
					ID: "dup",
					Content: []View{
						ProgressBar(ProgressBarCfg{ID: "dup", Percent: 50}),
					},
				}),
			},
		})
	})

	got := w.TestDuplicateIDs()
	if len(got) != 1 || !strings.Contains(got[0], `duplicate ID "dup"`) {
		t.Fatalf("want one duplicate-ID finding for the nested bar, got %q", got)
	}
}

// The sweep borrows process-global state — the debug gate — and a
// window's warn-once memory. Both must come back exactly as they were,
// or a test that calls it silently changes what every later frame in
// the process reports.
func TestTestDuplicateIDsRestoresDebugState(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.UpdateView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{Button(ButtonCfg{ID: "ok"})},
		})
	})

	// A finding already remembered by this window: the sweep must not
	// inherit it, and must not drop it either.
	sentinel := debugWarnKey{check: debugCheckDupID, subject: "sentinel"}
	w.debug.warned = map[debugWarnKey]struct{}{sentinel: {}}
	prevOn := debugEnabled.Load()

	if got := w.TestDuplicateIDs(); len(got) != 0 {
		t.Fatalf("want no findings, got %q", got)
	}

	if debugEnabled.Load() != prevOn {
		t.Errorf("debug gate left at %v, want %v",
			debugEnabled.Load(), prevOn)
	}
	if _, ok := w.debug.warned[sentinel]; !ok {
		t.Error("warn-once memory not restored")
	}
	if w.debug.collect != nil {
		t.Error("collect sink not cleared")
	}
}
