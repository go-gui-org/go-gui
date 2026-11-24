package datagrid

import (
	"fmt"
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
	return findShapeByText(layout, want) != nil
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
		func(q GridQueryState, ctx gg.EventCtx) {
			committed = append(committed, q.QuickFilter)
		}, 0)

	handler("a", gg.EventCtx{Layout: nil, Event: nil, Window: w})
	handler("ab", gg.EventCtx{Layout: nil, Event: nil, Window: w})

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
		func(q GridQueryState, ctx gg.EventCtx) {
			committed = append(committed, q.QuickFilter)
		}, 200*time.Millisecond)

	handler("a", gg.EventCtx{Layout: nil, Event: nil, Window: w})

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
		func(_ GridQueryState, ctx gg.EventCtx) {},
		200*time.Millisecond)

	// Two keystrokes inside the debounce window: the draft must
	// track the latest text so the rendered input echoes typing.
	handler("a", gg.EventCtx{Layout: nil, Event: nil, Window: w})
	handler("ab", gg.EventCtx{Layout: nil, Event: nil, Window: w})

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
		OnQueryChange: func(_ GridQueryState, ctx gg.EventCtx) {},
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
		OnQueryChange: func(_ GridQueryState, ctx gg.EventCtx) {},
	}
	applyDataGridDefaults(cfg)

	layout := gg.GenerateViewLayout(dataGridQuickFilterRow(cfg, w), w)
	if !findShapeText(&layout, "committed") {
		t.Fatal("committed query text not rendered")
	}
}

func findShapeByText(layout *gg.Layout, want string) *gg.Layout {
	if layout.Shape != nil && layout.Shape.TC != nil &&
		layout.Shape.TC.Text == want {
		return layout
	}
	for i := range layout.Children {
		if found := findShapeByText(&layout.Children[i], want); found != nil {
			return found
		}
	}
	return nil
}

// The placeholder takes the theme's placeholder-role alpha, not a
// grid-local literal, and keeps the caller's filter hue.
func TestQuickFilterPlaceholderUsesThemeRole(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	cfg := &DataGridCfg{
		ID:                     "g1",
		QuickFilterPlaceholder: "Search here",
		OnQueryChange:          func(_ GridQueryState, ctx gg.EventCtx) {},
	}
	applyDataGridDefaults(cfg)
	cfg.TextStyleFilter.Color = gg.RGB(10, 20, 30)
	layout := gg.GenerateViewLayout(dataGridQuickFilterRow(cfg, w), w)
	ly := findShapeByText(&layout, "Search here")
	if ly == nil || ly.Shape.TC.TextStyle == nil {
		t.Fatal("placeholder text not rendered")
	}
	got := ly.Shape.TC.TextStyle.Color
	want := gg.RGBA(10, 20, 30, gg.CurrentTheme().TextStylePlaceholder.Color.A)
	if got != want {
		t.Fatalf("placeholder color: got %v, want %v", got, want)
	}
}

// quickPins are the pin sets the quick filter is checked with. Pinning
// every column is the same as pinning none.
func quickPins() map[string]map[int]GridColumnPin {
	all := map[int]GridColumnPin{}
	for c := range pinTestCols {
		all[c] = GridColumnPinLeft
	}
	return map[string]map[int]GridColumnPin{
		"none": nil,
		"left": {0: GridColumnPinLeft},
		"both": {0: GridColumnPinLeft, 9: GridColumnPinRight},
		"all":  all,
	}
}

// The quick filter stays in place while the columns scroll, with and
// without pinned ones, and shows from the search text to the clear button.
func TestQuickFilterStaysInPlace(t *testing.T) {
	for name, pins := range quickPins() {
		for _, rtl := range []bool{false, true} {
			for _, width := range []int{520, 240} {
				t.Run(fmt.Sprintf("pins=%s/rtl=%v/width=%d", name, rtl, width), func(t *testing.T) {
					if rtl {
						pinRTL(t)
					}
					if width == 240 && name == "both" {
						t.Skip("too narrow for its pinned columns, which scroll away first")
					}
					w := pinWindow(t, width, pins, func(c *DataGridCfg) { c.ShowQuickFilter = true })
					l := w.TestRender(nil)
					ids := []string{"g:quick_filter", "g:filter_clear"}
					before := map[string]float32{}
					for _, id := range ids {
						before[id] = pinFind(t, l, id).X
					}
					for _, ask := range []float32{0, -80, -100000} {
						l = pinScroll(w, ask)
						off, _, _ := w.TestScrollOffset(pinScrollID)
						if ask != 0 && off >= 0 {
							t.Fatalf("offset = %.0f, want < 0", off)
						}
						for _, id := range ids {
							s := pinFind(t, l, id)
							if s.X != before[id] {
								t.Errorf("offset %.0f: %s moved from %.1f to %.1f", off, id, before[id], s.X)
							}
							for _, x := range []float32{s.X + 1, s.X + s.Width - 1} {
								if !s.PointInShape(x, s.Y+s.Height/2) {
									t.Errorf("offset %.0f: %s is cut off at x=%.0f", off, id, x)
								}
							}
						}
					}
				})
			}
		}
	}
}

// A wheel over the quick filter scrolls the rows as one over a row does.
func TestQuickFilterWheelScrollsRows(t *testing.T) {
	for name, pins := range quickPins() {
		wheel := func(id string) (x, y float32) {
			w := pinWindow(t, 520, pins, func(c *DataGridCfg) { c.ShowQuickFilter = true })
			s := pinFind(t, w.TestRender(nil), id)
			w.EventFn(&gg.Event{
				Type: gg.EventMouseScroll, ScrollPrecise: true, ScrollX: -2, ScrollY: -1,
				MouseX: s.X + s.Width/2, MouseY: s.Y + s.Height/2,
			})
			w.TestRender(nil)
			x, y, err := w.TestScrollOffset(pinScrollID)
			if err != nil {
				t.Fatal(err)
			}
			return x, y
		}
		wx, wy := wheel("g:cell:1:c2")
		if wx >= 0 || wy >= 0 {
			t.Fatalf("pins %s: wheel over a cell: offset (%.1f, %.1f), want both < 0", name, wx, wy)
		}
		for _, id := range []string{"g:quick_filter", "g:filter_clear"} {
			if x, y := wheel(id); x != wx || y != wy {
				t.Errorf("pins %s: wheel over %s: offset (%.1f, %.1f), want (%.1f, %.1f)", name, id, x, y, wx, wy)
			}
		}
	}
}

// The quick filter does not change the width of a grid that fits its
// columns.
func TestQuickFilterFitWidth(t *testing.T) {
	width := func(quick bool) float32 {
		w := gg.NewTestWindow(t, gg.WindowCfg{Width: 3000, Height: 400})
		w.TestRender(func(w *gg.Window) gg.View {
			return pinGrid(w, nil, func(c *DataGridCfg) {
				c.Sizing, c.Height, c.MaxHeight = gg.FitFit, 0, 300
				c.ShowQuickFilter = quick
			})
		})
		return pinFind(t, w.TestRender(nil), "g").Width
	}
	if plain, quick := width(false), width(true); !near(plain, quick, 0.1) {
		t.Errorf("grid width = %.1f with the quick filter, %.1f without", quick, plain)
	}
}
