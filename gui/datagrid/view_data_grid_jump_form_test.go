package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// Enter in a grid's jump box belongs to the grid. Since #1257, Enter in
// a single-line Input submits the Form around it unless a handler
// consumed the key, so the jump box must consume Enter even when the
// typed row is not valid and the jump does nothing.

// jumpEnterSubmits renders jump inside a Form, presses Enter in the
// Input with leaf ID inputLeaf and returns how often the form submitted.
func jumpEnterSubmits(t *testing.T, inputLeaf string, jump gg.View) int {
	t.Helper()
	submits := 0
	w := gg.NewTestWindow(t, gg.WindowCfg{})
	w.TestRender(func(*gg.Window) gg.View {
		return gg.Form(gg.FormCfg{
			ID:       "f",
			OnSubmit: func(gg.FormSubmitEvent, gg.EventCtx) { submits++ },
			Content:  []gg.View{jump},
		})
	})
	ids := w.ResolveID(inputLeaf)
	if len(ids) != 1 {
		t.Fatalf("ResolveID(%q) = %v, want one ID", inputLeaf, ids)
	}
	if err := w.TestFocus(ids[0]); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	if err := w.TestKey(ids[0], gg.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	w.TestRender(nil)
	return submits
}

func TestLocalJumpEnterDoesNotSubmitForm(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	// The context has no rows, so the jump returns early without
	// moving; the Input itself is enabled.
	jump := dataGridPagerJumpInput(cfg, "jump", "jump", "", true,
		dataGridJumpContext{gridID: "g"}, "")
	if n := jumpEnterSubmits(t, "jump", jump); n != 0 {
		t.Fatalf("Enter in the jump box submitted the form %d times", n)
	}
}

func TestSourceJumpEnterDoesNotSubmitForm(t *testing.T) {
	rows := 10
	cfg := &DataGridCfg{
		ID:                "g",
		OnSelectionChange: func(GridSelection, gg.EventCtx) {},
	}
	state := dataGridSourceState{RowCount: &rows}
	// No jump text: the row does not parse, so the jump returns early.
	views := dataGridSourcePagerJump(cfg, "", state, GridDataCapabilities{},
		GridPaginationOffset, 5, "")
	if len(views) != 2 {
		t.Fatalf("pager jump built %d views, want label and input", len(views))
	}
	if n := jumpEnterSubmits(t, "g:jump", views[1]); n != 0 {
		t.Fatalf("Enter in the jump box submitted the form %d times", n)
	}
}
