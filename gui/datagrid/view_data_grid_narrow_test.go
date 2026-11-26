package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// A grid in a parent narrower than its pager or CRUD toolbar must keep
// the parent's width. The bar scrolls sideways, so its last control can
// still be reached (#1368). Before the fix each bar was a plain row of
// texts and buttons that cannot shrink, so its width became the grid's
// smallest width, and the grid stuck out past the end of its parent.

const narrowParentWidth = 120

func narrowGridCases() []struct {
	name  string
	barID string
	cfg   func() DataGridCfg
} {
	cols := []GridColumnCfg{{ID: "name", Title: "Name", Width: gg.SomeF(60)}}
	return []struct {
		name  string
		barID string
		cfg   func() DataGridCfg
	}{
		{"crud toolbar", gg.ScopeID("g", "crud"), func() DataGridCfg {
			return DataGridCfg{ID: "g", Columns: cols, Rows: makeTestRows(3),
				ShowCRUDToolbar: true}
		}},
		{"local pager", gg.ScopeID("g", "pager"), func() DataGridCfg {
			return DataGridCfg{ID: "g", Columns: cols, Rows: makeTestRows(240),
				PageSize: 10}
		}},
		{"source pager, offset", gg.ScopeID("g", "src_pager"), func() DataGridCfg {
			return DataGridCfg{ID: "g", Columns: cols,
				DataSource:     NewInMemoryDataSource(makeTestRows(240)),
				PageSize:       10,
				PaginationKind: GridPaginationOffset}
		}},
		{"source pager, cursor", gg.ScopeID("g", "src_pager"), func() DataGridCfg {
			return DataGridCfg{ID: "g", Columns: cols,
				DataSource:     NewInMemoryDataSource(makeTestRows(240)),
				PageSize:       10,
				PaginationKind: GridPaginationCursor}
		}},
	}
}

func TestDataGridNarrowParentKeepsWidth(t *testing.T) {
	for _, tc := range narrowGridCases() {
		t.Run(tc.name, func(t *testing.T) {
			w := gg.NewTestWindow(t, gg.WindowCfg{Width: 400, Height: 400})
			// 8 px per byte: every bar is far wider than the parent.
			w.SetTextMeasurer(dataGridRenderTextMeasurer{})
			cfg := tc.cfg()
			w.TestRender(func(w *gg.Window) gg.View {
				return gg.Row(gg.ContainerCfg{Sizing: gg.FillFill, Padding: gg.NoPadding,
					Content: []gg.View{
						gg.Column(gg.ContainerCfg{Sizing: gg.FixedFill,
							Width: narrowParentWidth, Padding: gg.NoPadding,
							SizeBorder: gg.NoBorder,
							Content:    []gg.View{New(w, cfg)}}),
					}})
			})
			l := w.TestRender(nil)

			grid, ok := l.FindByID("g")
			if !ok {
				t.Fatal("grid not found")
			}
			if grid.Shape.Width > narrowParentWidth+0.5 {
				t.Fatalf("grid width %.1f, want at most the parent's %d",
					grid.Shape.Width, narrowParentWidth)
			}

			bar, ok := l.FindByID(tc.barID)
			if !ok {
				t.Fatalf("bar %q not found", tc.barID)
			}
			if bar.Shape.Width > narrowParentWidth+0.5 {
				t.Fatalf("bar width %.1f, want at most %d", bar.Shape.Width,
					narrowParentWidth)
			}
			if len(bar.Children) == 0 {
				t.Fatal("bar has no children")
			}

			// Scroll the bar to its end: the last control must then be
			// inside the bar, so it can be clicked.
			w.ScrollHorizontalTo(tc.barID, -100000)
			l = w.TestRender(nil)
			bar, _ = l.FindByID(tc.barID)
			last := lastInFlowChild(bar)
			if last == nil {
				t.Fatal("bar has no in-flow child")
			}
			barRight := bar.Shape.X + bar.Shape.Width
			lastRight := last.Shape.X + last.Shape.Width
			if lastRight > barRight+0.5 {
				t.Fatalf("after scrolling to the end, last control ends at %.1f, "+
					"past the bar's right edge %.1f", lastRight, barRight)
			}
			if last.Shape.X < bar.Shape.X-0.5 {
				t.Fatalf("after scrolling to the end, last control starts at %.1f, "+
					"before the bar's left edge %.1f", last.Shape.X, bar.Shape.X)
			}
		})
	}
}

// lastInFlowChild returns the last child that takes part in the row's
// layout: scrollbars are floats appended after the content.
func lastInFlowChild(l *gg.Layout) *gg.Layout {
	for i := len(l.Children) - 1; i >= 0; i-- {
		c := &l.Children[i]
		if !c.Shape.Float && c.Shape.Width > 0 {
			return c
		}
	}
	return nil
}
