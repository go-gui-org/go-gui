package datagrid

import (
	"testing"
	"time"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- Quick filter debounce draft ---

func quickFilterDraft(w *gg.Window, gridID string) (string, bool) {
	return gg.StateMap[string, string](w, nsDgQuickDraft, capModerate).Get(gridID)
}

// findShapeText walks a layout tree for a text shape whose text
// matches want. Returns true when found.
func findShapeText(layout *gg.Layout, want string) bool {
	if layout.Shape != nil && layout.Shape.TC != nil &&
		layout.Shape.TC.Text == want {
		return true
	}
	for i := range layout.Children {
		if findShapeText(&layout.Children[i], want) {
			return true
		}
	}
	return false
}

func TestQuickFilterDebounceDefaultInMemory(t *testing.T) {
	cfg := &DataGridCfg{ID: "g1"}
	applyDataGridDefaults(cfg)
	if cfg.QuickFilterDebounce != 0 {
		t.Fatalf("QuickFilterDebounce = %v without DataSource, want 0",
			cfg.QuickFilterDebounce)
	}
}

func TestQuickFilterDebounceDefaultWithSource(t *testing.T) {
	cfg := &DataGridCfg{
		ID:         "g1",
		DataSource: NewInMemoryDataSource(nil),
	}
	applyDataGridDefaults(cfg)
	if cfg.QuickFilterDebounce != 200*time.Millisecond {
		t.Fatalf("QuickFilterDebounce = %v with DataSource, want 200ms",
			cfg.QuickFilterDebounce)
	}
}

func TestQuickFilterDebounceNegativeOptsOut(t *testing.T) {
	cfg := &DataGridCfg{
		ID:                  "g1",
		DataSource:          NewInMemoryDataSource(nil),
		QuickFilterDebounce: -1,
	}
	applyDataGridDefaults(cfg)
	if cfg.QuickFilterDebounce != -1 {
		t.Fatalf("QuickFilterDebounce = %v, want -1 preserved",
			cfg.QuickFilterDebounce)
	}
}

func TestQuickFilterNoDebounceCommitsImmediately(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	var committed []string
	handler := dataGridQuickFilterOnTextChanged("g1", "g1:quick_filter",
		GridQueryState{},
		func(q GridQueryState, _ *gg.Event, _ *gg.Window) {
			committed = append(committed, q.QuickFilter)
		}, 0)

	handler(nil, "a", w)
	handler(nil, "ab", w)

	if len(committed) != 2 || committed[1] != "ab" {
		t.Fatalf("committed = %v, want [a ab]", committed)
	}
	if _, ok := quickFilterDraft(w, "g1"); ok {
		t.Fatal("draft set on undebounced path, want none")
	}
}

func TestQuickFilterDebounceParksDraft(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	var committed []string
	handler := dataGridQuickFilterOnTextChanged("g1", "g1:quick_filter",
		GridQueryState{},
		func(q GridQueryState, _ *gg.Event, _ *gg.Window) {
			committed = append(committed, q.QuickFilter)
		}, 200*time.Millisecond)

	handler(nil, "a", w)

	if len(committed) != 0 {
		t.Fatalf("committed = %v before debounce, want none", committed)
	}
	draft, ok := quickFilterDraft(w, "g1")
	if !ok || draft != "a" {
		t.Fatalf("draft = %q, %t, want %q, true", draft, ok, "a")
	}
	if !w.HasAnimation("g1:quick_filter:debounce") {
		t.Fatal("debounce animation not registered")
	}
}

func TestQuickFilterDebounceRetypeReplacesDraft(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	handler := dataGridQuickFilterOnTextChanged("g1", "g1:quick_filter",
		GridQueryState{},
		func(GridQueryState, *gg.Event, *gg.Window) {},
		200*time.Millisecond)

	// Two keystrokes inside the debounce window: the draft must
	// track the latest text so the rendered input echoes typing.
	handler(nil, "a", w)
	handler(nil, "ab", w)

	draft, ok := quickFilterDraft(w, "g1")
	if !ok || draft != "ab" {
		t.Fatalf("draft = %q, %t, want %q, true", draft, ok, "ab")
	}
}

func TestQuickFilterRowRendersPendingDraft(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	cfg := &DataGridCfg{
		ID:            "g1",
		Query:         GridQueryState{QuickFilter: "stale"},
		OnQueryChange: func(GridQueryState, *gg.Event, *gg.Window) {},
	}
	applyDataGridDefaults(cfg)

	gg.StateMap[string, string](w, nsDgQuickDraft, capModerate).
		Set("g1", "fresh")

	layout := gg.GenerateViewLayout(dataGridQuickFilterRow(cfg, w), w)
	if !findShapeText(&layout, "fresh") {
		t.Fatal("pending draft text not rendered")
	}
	if findShapeText(&layout, "stale") {
		t.Fatal("stale committed text rendered while draft pending")
	}
}

func TestQuickFilterRowRendersCommittedWithoutDraft(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	cfg := &DataGridCfg{
		ID:            "g1",
		Query:         GridQueryState{QuickFilter: "committed"},
		OnQueryChange: func(GridQueryState, *gg.Event, *gg.Window) {},
	}
	applyDataGridDefaults(cfg)

	layout := gg.GenerateViewLayout(dataGridQuickFilterRow(cfg, w), w)
	if !findShapeText(&layout, "committed") {
		t.Fatal("committed query text not rendered")
	}
}
