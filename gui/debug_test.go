package gui

import (
	"math"
	"strings"
	"testing"
)

// captureDebug turns every category on, redirects findings to a buffer,
// and restores both on cleanup. Returns the buffer.
func captureDebug(t *testing.T) *strings.Builder {
	return captureDebugMask(t, DebugAll)
}

// captureDebugMask turns the given categories on, redirects findings to
// a buffer, and restores both on cleanup. Returns the buffer.
func captureDebugMask(t *testing.T, mask DebugCategory) *strings.Builder {
	t.Helper()
	var buf strings.Builder
	prevOut := debugOut
	prevMask := debugMask.Load()
	debugOut = &buf
	DebugCategories(mask)
	t.Cleanup(func() {
		debugOut = prevOut
		DebugCategories(DebugCategory(prevMask))
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

// The point of the mask: identity checks run without the unconsumed
// noise and vice versa. Each category reports only its own findings.
func TestDebugCategoriesGateIndependently(t *testing.T) {
	buf := captureDebug(t)
	DebugCategories(DebugDuplicates)
	w := &Window{}
	tree := debugTree(&Shape{Focusable: true}, &Shape{ID: "dup"}, &Shape{ID: "dup"})

	w.debugAudit(&tree)
	got := buf.String()
	if !strings.Contains(got, "duplicate ID") || strings.Contains(got, "focusable shape") {
		t.Fatalf("want only the duplicate finding, got %q", got)
	}

	DebugCategories(DebugMissingIDs)
	buf.Reset()
	w.debugAudit(&tree)
	got = buf.String()
	if !strings.Contains(got, "focusable shape at 0") || strings.Contains(got, "duplicate ID") {
		t.Fatalf("want only the missing-ID finding, got %q", got)
	}
}

// A category that is off must never warn, and must not remember either:
// turning it on later reports the frame in front of it, like cycling
// the whole gate.
func TestDebugCategoriesToggleResetsWarnOnce(t *testing.T) {
	buf := captureDebug(t)
	DebugCategories(DebugMissingIDs)
	w := &Window{}
	tree := debugTree(&Shape{Focusable: true})

	w.debugAudit(&tree)
	first := buf.String()
	if first == "" {
		t.Fatal("want a finding with the category on")
	}

	DebugCategories(0)
	buf.Reset()
	DebugCategories(DebugMissingIDs)
	w.debugAudit(&tree)
	// Re-reported, not the stale silence: the finding appears exactly
	// once again in a buffer that was empty before the re-enable.
	if n := strings.Count(buf.String(), "focusable shape"); n != 1 {
		t.Fatalf("want the finding re-reported once, got %d occurrences (%q)",
			n, buf.String())
	}
}

// The tree walk exists for the frame audit categories only, so a mask
// of dispatch-side categories must skip it entirely.
func TestDebugCategoriesAuditSkippedWhenOnlyUnconsumed(t *testing.T) {
	buf := captureDebug(t)
	DebugCategories(DebugUnconsumed)
	w := &Window{}
	tree := debugTree(&Shape{Focusable: true}, &Shape{ID: "dup"}, &Shape{ID: "dup"})

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Fatalf("audit must stay silent under a dispatch-only mask, got %q", got)
	}
}

// Every internal check belongs to exactly one category, and DebugAll
// covers them all, so no check can fall through a mask as unmaskable.
func TestCheckCategoryMapping(t *testing.T) {
	tests := []struct {
		check debugCheck
		want  DebugCategory
	}{
		{debugCheckDupID, DebugDuplicates},
		{debugCheckFocusNoID, DebugMissingIDs},
		{debugCheckScrollNoID, DebugMissingIDs},
		{debugCheckMouseLeaveNoID, DebugMissingIDs},
		{debugCheckUnconsumed, DebugUnconsumed},
		{debugCheckListBoxNoHeight, DebugListBoxNoHeight},
		{debugCheckUnscopedID, DebugUnscopedIDs},
		{debugCheckGradientResampled, DebugGradientResampled},
	}
	for _, tc := range tests {
		if got := checkCategory(tc.check); got != tc.want {
			t.Errorf("checkCategory(%d) = %v, want %v", tc.check, got, tc.want)
		}
	}
	// DebugAll covers every category Debug(true) turns on.
	// DebugUnscopedIDs is opt-in and deliberately outside it: it reports
	// a design property, not a defect.
	if DebugAll != DebugDuplicates|DebugMissingIDs|DebugUnconsumed|DebugListBoxNoHeight|DebugGradientResampled {
		t.Fatal("DebugAll must cover every category Debug(true) enables")
	}
	if DebugAll&DebugUnscopedIDs != 0 {
		t.Fatal("DebugUnscopedIDs must stay opt-in, outside DebugAll")
	}
}

// A gradient resample is reported once per rect and only while its
// category is on, and the counts in the message are the degraded and
// original stop counts.
func TestDebugGradientResampledWarnOnce(t *testing.T) {
	buf := captureDebugMask(t, DebugGradientResampled)
	w := &Window{}

	w.DebugGradientResampled(10, 20, 5, 7)
	w.DebugGradientResampled(10, 20, 5, 7)
	got := buf.String()
	if !strings.Contains(got, "gradient at (10, 20) has 7 stops; resampled to 5") {
		t.Fatalf("want resample finding with both counts, got %q", got)
	}
	if n := strings.Count(got, "resampled"); n != 1 {
		t.Fatalf("warn-once: want 1 finding, got %d", n)
	}

	// A different rect is a distinct finding.
	w.DebugGradientResampled(100, 200, 5, 9)
	got = buf.String()
	if n := strings.Count(got, "resampled"); n != 2 {
		t.Fatalf("want a second finding for a new rect, got %d", n)
	}
}

// The gradient check is dispatch-side: it must stay silent under a
// mask that does not include it, and not remember the miss either.
func TestDebugGradientResampledCategoryGate(t *testing.T) {
	buf := captureDebug(t)
	DebugCategories(DebugMissingIDs)
	w := &Window{}

	w.DebugGradientResampled(0, 0, 5, 7)
	if got := buf.String(); got != "" {
		t.Fatalf("category off must be silent, got %q", got)
	}

	DebugCategories(0)
	buf.Reset()
	DebugCategories(DebugGradientResampled)
	w.DebugGradientResampled(0, 0, 5, 7)
	if n := strings.Count(buf.String(), "resampled"); n != 1 {
		t.Fatalf("re-enabled category must re-report, got %d", n)
	}
}

// NaN coords would never match a warn-once key, so the finding must
// fold to (0, 0) and dedupe against the (0, 0) finding.
func TestDebugGradientResampledNaNFolds(t *testing.T) {
	buf := captureDebugMask(t, DebugGradientResampled)
	w := &Window{}
	nan := float32(math.NaN())

	w.DebugGradientResampled(nan, nan, 5, 7)
	w.DebugGradientResampled(0, 0, 5, 7)
	if n := strings.Count(buf.String(), "resampled"); n != 1 {
		t.Fatalf("NaN coords must fold to (0, 0) and dedupe, got %d findings", n)
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

// A leaf repeated inside an ID-bearing ancestor is no longer a
// collision: the framework scopes it, so the inner bar is "dup:dup",
// a different identity from the button's "dup".
func TestTestDuplicateIDsScopesNestedLeaf(t *testing.T) {
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

	if got := w.TestDuplicateIDs(); len(got) != 0 {
		t.Fatalf("want no findings — the nested leaf scopes to "+
			"\"dup:dup\" — got %q", got)
	}
}

// Two leaves under one ID-less parent share a scope, so they still
// collide. Scoping joins explicit ancestor IDs and nothing else.
func TestTestDuplicateIDsReportsSameScopeCollision(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.UpdateView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				ProgressBar(ProgressBarCfg{ID: "dup", Percent: 50}),
				ProgressBar(ProgressBarCfg{ID: "dup", Percent: 60}),
			},
		})
	})

	got := w.TestDuplicateIDs()
	if len(got) != 1 || !strings.Contains(got[0], `duplicate ID "dup"`) {
		t.Fatalf("want one duplicate-ID finding for the sibling bars, "+
			"got %q", got)
	}
}

// A joined identity and an absolute one can spell the same string.
// There is no escaping, so this stays a duplicate — reported loudly
// rather than silently sharing a slot.
func TestTestDuplicateIDsReportsJoinedVsAbsolute(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.UpdateView(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Button(ButtonCfg{
					ID: "a",
					Content: []View{
						// Joins to "a:b".
						ProgressBar(ProgressBarCfg{ID: "b", Percent: 50}),
					},
				}),
				// Absolute: already "a:b", no further join.
				ProgressBar(ProgressBarCfg{ID: "a:b", Percent: 60}),
			},
		})
	})

	got := w.TestDuplicateIDs()
	if len(got) != 1 || !strings.Contains(got[0], `duplicate ID "a:b"`) {
		t.Fatalf("want one duplicate-ID finding for \"a:b\", got %q", got)
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
	prevOn := DebugEnabled()

	if got := w.TestDuplicateIDs(); len(got) != 0 {
		t.Fatalf("want no findings, got %q", got)
	}

	if DebugEnabled() != prevOn {
		t.Errorf("debug gate left at %v, want %v", DebugEnabled(), prevOn)
	}
	if _, ok := w.debug.warned[sentinel]; !ok {
		t.Error("warn-once memory not restored")
	}
	if w.debug.collect != nil {
		t.Error("collect sink not cleared")
	}
}

// DebugUnscopedIDs is the reusability advisory: a state-keyed widget
// with no ID-bearing ancestor still owns a window-global name.
func TestDebugUnscopedIDsReportsGlobalLeaf(t *testing.T) {
	buf := captureDebug(t)
	DebugCategories(DebugUnscopedIDs)
	w := &Window{}
	tree := debugTree(&Shape{ID: "save", Focusable: true})
	resolveShapeIDs(&tree, w)

	w.debugAudit(&tree)
	if got := buf.String(); !strings.Contains(got, `ID "save"`) ||
		!strings.Contains(got, "no ID-bearing ancestor") {
		t.Fatalf("want the unscoped-ID finding, got %q", got)
	}

	// Scoped: the same widget under a panel resolves to "panel:save"
	// and is no longer competing globally.
	scoped := Layout{Shape: &Shape{}}
	scoped.Children = append(scoped.Children, Layout{
		Shape: &Shape{ID: "panel"},
		Children: []Layout{
			{Shape: &Shape{ID: "save", Focusable: true}},
		},
	})
	resolveShapeIDs(&scoped, w)
	buf.Reset()
	w.debug.warned = nil
	w.debugAudit(&scoped)
	if got := buf.String(); got != "" {
		t.Fatalf("want no finding for a scoped ID, got %q", got)
	}
}

// The advisory is opt-in: Debug(true) must not turn it on, or every
// small app would report most of its widgets.
func TestDebugAllExcludesUnscopedIDs(t *testing.T) {
	buf := captureDebug(t)
	Debug(true)
	w := &Window{}
	tree := debugTree(&Shape{ID: "save", Focusable: true})
	resolveShapeIDs(&tree, w)

	w.debugAudit(&tree)
	if got := buf.String(); strings.Contains(got, "no ID-bearing ancestor") {
		t.Fatalf("Debug(true) reported the advisory: %q", got)
	}
}
