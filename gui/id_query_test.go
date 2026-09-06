package gui

import (
	"strings"
	"testing"
)

// scopedIDTree builds an arranged-looking tree: a panel with an ID and
// one focusable leaf under it, resolved the way layoutArrange would
// have resolved them.
func scopedIDTree() Layout {
	leaf := &Shape{ID: "nav", Focusable: true}
	leaf.effID = "detail:nav"
	panel := &Shape{ID: "detail"}
	panel.effID = "detail"
	root := Layout{Shape: &Shape{}}
	root.Children = append(root.Children, Layout{
		Shape:    panel,
		Children: []Layout{{Shape: leaf}},
	})
	return root
}

// The identities the frame stamped, in tree order, with the ID-less
// root left out.
func TestEffectiveIDsListsTheFrame(t *testing.T) {
	w := &Window{}
	w.layout = scopedIDTree()

	got := w.EffectiveIDs()
	want := []string{"detail", "detail:nav"}
	if len(got) != len(want) {
		t.Fatalf("EffectiveIDs() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("EffectiveIDs() = %v, want %v", got, want)
		}
	}
}

// A window that has never rendered has no identities to report, and
// must answer with an empty list rather than panic on a nil tree.
func TestEffectiveIDsBeforeFirstFrame(t *testing.T) {
	w := &Window{}
	if got := w.EffectiveIDs(); len(got) != 0 {
		t.Fatalf("want no identities before the first frame, got %v", got)
	}
}

// The question the API exists for: the app wrote ID "nav", and needs
// the string SetFocus takes.
func TestResolveIDAnswersTheScopedSpelling(t *testing.T) {
	w := &Window{}
	w.layout = scopedIDTree()

	got := w.ResolveID("nav")
	if len(got) != 1 || got[0] != "detail:nav" {
		t.Fatalf(`ResolveID("nav") = %v, want ["detail:nav"]`, got)
	}
}

// A widget that resolved its own ID puts the joined string on the
// shape, so there is no bare leaf on the tree to match. The trailing
// segment is what makes the lookup work anyway.
func TestResolveIDMatchesASelfResolvedWidget(t *testing.T) {
	s := &Shape{ID: "detail:grid"}
	s.effID = "detail:grid"
	w := &Window{}
	w.layout = debugTree(s)

	got := w.ResolveID("grid")
	if len(got) != 1 || got[0] != "detail:grid" {
		t.Fatalf(`ResolveID("grid") = %v, want ["detail:grid"]`, got)
	}
}

// A leaf used under two scopes has two answers, and the caller picks.
func TestResolveIDReportsEveryScope(t *testing.T) {
	a := &Shape{ID: "name"}
	a.effID = "settings:name"
	b := &Shape{ID: "name"}
	b.effID = "profile:name"
	w := &Window{}
	w.layout = debugTree(a, b)

	got := w.ResolveID("name")
	if len(got) != 2 || got[0] != "settings:name" || got[1] != "profile:name" {
		t.Fatalf("ResolveID(\"name\") = %v, want both scopes", got)
	}
}

// A name no widget carries answers with nothing rather than a guess.
func TestResolveIDUnknownLeafIsEmpty(t *testing.T) {
	w := &Window{}
	w.layout = scopedIDTree()

	if got := w.ResolveID("missing"); len(got) != 0 {
		t.Fatalf("want no answer for an unknown leaf, got %v", got)
	}
}

// The defect this reports: focus set to the unscoped spelling of a
// scoped widget. The finding must name the string that would work.
func TestUnknownFocusReportsTheScopedSpelling(t *testing.T) {
	buf := captureDebugMask(t, DebugAll)
	w := &Window{}
	w.layout = scopedIDTree()
	w.viewState.focusID = "nav"

	w.debugAudit(&w.layout)

	got := buf.String()
	if !strings.Contains(got, `focus is set to "nav"`) ||
		!strings.Contains(got, "detail:nav") {
		t.Fatalf("want a finding naming both spellings, got %q", got)
	}
}

// The correct spelling is silent.
func TestUnknownFocusCorrectSpellingIsQuiet(t *testing.T) {
	buf := captureDebugMask(t, DebugAll)
	w := &Window{}
	w.layout = scopedIDTree()
	w.viewState.focusID = "detail:nav"

	w.debugAudit(&w.layout)

	if got := buf.String(); got != "" {
		t.Fatalf("want no findings, got %q", got)
	}
}

// A shape of that name exists but cannot take focus. That is a
// different mistake from a misspelling, so it gets its own wording.
func TestUnknownFocusNamesANonFocusableShape(t *testing.T) {
	buf := captureDebugMask(t, DebugAll)
	w := &Window{}
	w.layout = scopedIDTree()
	w.viewState.focusID = "detail"

	w.debugAudit(&w.layout)

	if got := buf.String(); !strings.Contains(got, "is not focusable") {
		t.Fatalf("want the not-focusable wording, got %q", got)
	}
}

// No focus is not a finding: a window with nothing focused is the
// normal state at startup.
func TestUnknownFocusEmptyFocusIsQuiet(t *testing.T) {
	buf := captureDebugMask(t, DebugAll)
	w := &Window{}
	w.layout = scopedIDTree()

	w.debugAudit(&w.layout)

	if got := buf.String(); got != "" {
		t.Fatalf("want no findings, got %q", got)
	}
}
