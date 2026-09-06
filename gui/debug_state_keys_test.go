package gui

import (
	"strings"
	"testing"
)

// stateKeyTree builds a tree whose single leaf carries the given
// effective ID, which is what generation would have stamped.
func stateKeyTree(effID string) Layout {
	s := &Shape{ID: "name"}
	s.effID = effID
	return debugTree(s)
}

// The defect: state written under the bare leaf while the shape it
// belongs to resolved under a scope.
func TestDebugStateKeysUnresolvedLeaf(t *testing.T) {
	buf := captureDebugMask(t, DebugAll|DebugUnresolvedKeys)
	w := &Window{}
	StateMap[string, bool](w, "test", capModerate).Set("name", true)
	tree := stateKeyTree("settings:name")

	w.debugAudit(&tree)

	got := buf.String()
	if !strings.Contains(got, `state key "name"`) ||
		!strings.Contains(got, `"settings:name"`) {
		t.Fatalf("want unresolved-key finding naming both, got %q", got)
	}
}

// The correct spelling: the key matches what generation stamped.
func TestDebugStateKeysResolvedIsQuiet(t *testing.T) {
	buf := captureDebugMask(t, DebugAll|DebugUnresolvedKeys)
	w := &Window{}
	StateMap[string, bool](w, "test", capModerate).Set("settings:name", true)
	tree := stateKeyTree("settings:name")

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Fatalf("want no findings, got %q", got)
	}
}

// A top-level widget has an empty scope, so its leaf *is* its
// identity. Keying state on it is correct and must stay quiet.
func TestDebugStateKeysUnscopedWidgetIsQuiet(t *testing.T) {
	buf := captureDebugMask(t, DebugAll|DebugUnresolvedKeys)
	w := &Window{}
	StateMap[string, bool](w, "test", capModerate).Set("name", true)
	tree := stateKeyTree("name")

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Fatalf("want no findings, got %q", got)
	}
}

// A cache keyed by something that is not a widget ID names no shape in
// the window, so it is not a finding.
func TestDebugStateKeysUnrelatedKeyIsQuiet(t *testing.T) {
	buf := captureDebugMask(t, DebugAll|DebugUnresolvedKeys)
	w := &Window{}
	StateMap[string, bool](w, "svg", capModerate).Set("icon.svg", true)
	tree := stateKeyTree("settings:name")

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Fatalf("want no findings, got %q", got)
	}
}

// The hot maps are cached fields rather than registry entries and are
// the ones most often keyed by a widget ID, so they are scanned too.
func TestDebugStateKeysScansHotMaps(t *testing.T) {
	buf := captureDebugMask(t, DebugAll|DebugUnresolvedKeys)
	w := &Window{}
	w.scrollY().Set("name", 12)
	tree := stateKeyTree("panel:name")

	w.debugAudit(&tree)

	if got := buf.String(); !strings.Contains(got, `namespace "scrollY"`) {
		t.Fatalf("want scrollY finding, got %q", got)
	}
}

// A map with non-string keys answers nil and is skipped rather than
// panicking on the type assertion.
func TestDebugStateKeysIgnoresNonStringKeys(t *testing.T) {
	buf := captureDebugMask(t, DebugAll|DebugUnresolvedKeys)
	w := &Window{}
	StateMap[int, bool](w, "ints", capModerate).Set(7, true)
	tree := stateKeyTree("settings:name")

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Fatalf("want no findings, got %q", got)
	}
}

// The category gates independently of the rest.
func TestDebugStateKeysGatedByCategory(t *testing.T) {
	buf := captureDebugMask(t, DebugDuplicates)
	w := &Window{}
	StateMap[string, bool](w, "test", capModerate).Set("name", true)
	tree := stateKeyTree("settings:name")

	w.debugAudit(&tree)

	if got := buf.String(); got != "" {
		t.Fatalf("want no findings with category off, got %q", got)
	}
}
