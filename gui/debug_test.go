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
// FocusSkip is how Text/Rtf keep click-focus without a tab stop.
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
