package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// virtualListDemoWindow opens the showcase on the VirtualList page and
// runs two frames, so the list has a height and a measured first page.
func virtualListDemoWindow(t *testing.T) (*gui.Window, *ShowcaseApp) {
	t.Helper()
	app := newShowcaseApp()
	app.SelectedGroup = groupData
	app.SelectedComponent = "virtual_list"
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 700})
	w.TestRender(mainView)
	w.TestRender(nil)
	if ids := w.ResolveID(virtualListDemoID); len(ids) == 0 {
		t.Fatalf("list %q not in the frame", virtualListDemoID)
	}
	return w, app
}

// virtualListDemoFrames runs frames until a jump into rows that were
// never measured has converged. Index scroll requests stay live for a
// few frames for that reason (virtualScrollMaxAge in gui).
func virtualListDemoFrames(w *gui.Window) *gui.Layout {
	var ly *gui.Layout
	for range 6 {
		ly = w.TestRender(nil)
	}
	return ly
}

// virtualListDemoRowInView fails the test unless the row of the item
// at index is built and overlaps the list's viewport.
func virtualListDemoRowInView(t *testing.T, w *gui.Window, app *ShowcaseApp, index int) {
	t.Helper()
	root := virtualListDemoFrames(w)
	listID := virtualListDemoEffID(w)
	list, ok := root.FindByID(listID)
	if !ok {
		t.Fatalf("list %q not in the layout", listID)
	}
	rowLeaf := gui.ScopeIDN(virtualListDemoID, "row", app.VLSeqs[index])
	var rowID string
	for _, id := range w.EffectiveIDs() {
		if id == rowLeaf || strings.HasSuffix(id, gui.IDSep+rowLeaf) {
			rowID = id
			break
		}
	}
	if rowID == "" {
		t.Fatalf("row %d (%s) was not built", index, rowLeaf)
	}
	row, _ := root.FindByID(rowID)
	ls, rs := list.Shape, row.Shape
	if rs.Y+rs.Height <= ls.Y || rs.Y >= ls.Y+ls.Height {
		t.Fatalf("row %d at y=[%v,%v] is outside list y=[%v,%v]",
			index, rs.Y, rs.Y+rs.Height, ls.Y, ls.Y+ls.Height)
	}
}

// virtualListDemoClick clicks a demo button by its leaf ID.
func virtualListDemoClick(t *testing.T, w *gui.Window, leaf string) {
	t.Helper()
	ids := w.ResolveID(leaf)
	if len(ids) == 0 {
		t.Fatalf("button %q not in the frame", leaf)
	}
	if err := w.TestClick(ids[0]); err != nil {
		t.Fatal(err)
	}
}

// virtualListDemoAtTop fails the test unless the list is unscrolled.
func virtualListDemoAtTop(t *testing.T, w *gui.Window) {
	t.Helper()
	// Default 0: absent entry means the list was never scrolled.
	if off := w.ScrollY().GetOr(virtualListDemoEffID(w), 0); off != 0 {
		t.Fatalf("scroll offset = %v, want 0", off)
	}
}

// A jump to a far index brings a row that was never built into view,
// and it still does after rows are inserted at the top.
func TestVirtualListDemoJumpAndInsertTop(t *testing.T) {
	w, app := virtualListDemoWindow(t)

	const far = 4_000
	w.ScrollToIndexAt(virtualListDemoEffID(w), far, 0.5)
	virtualListDemoRowInView(t, w, app, far)

	seq := app.VLSeqs[far]
	virtualListDemoClick(t, w, "vl-insert-top")
	if got := len(app.VLSeqs); got != virtualListDemoRows+10 {
		t.Fatalf("rows = %d, want %d", got, virtualListDemoRows+10)
	}
	// The same item is now 10 rows further down.
	if app.VLSeqs[far+10] != seq {
		t.Fatalf("item at %d = #%d, want #%d", far+10, app.VLSeqs[far+10], seq)
	}
	w.ScrollToIndexAt(virtualListDemoEffID(w), far+10, 0.5)
	virtualListDemoRowInView(t, w, app, far+10)
}

// The keyboard cursor is an index. An insert at the top moves it with
// its row, so the highlight stays on the same item.
func TestVirtualListDemoInsertTopKeepsCursorOnItem(t *testing.T) {
	w, app := virtualListDemoWindow(t)
	listID := virtualListDemoEffID(w)

	w.SetVirtualListFocusedIndex(listID, 3)
	virtualListDemoFrames(w)
	seq := app.VLSeqs[3]

	virtualListDemoClick(t, w, "vl-insert-top")
	virtualListDemoFrames(w)
	got := w.VirtualListFocusedIndex(listID)
	if got != 13 || app.VLSeqs[got] != seq {
		t.Fatalf("cursor = %d (#%d), want 13 (#%d)", got, app.VLSeqs[got], seq)
	}
}

// A cursor on the last row moves past the old row count. The handler
// runs before the list sees the new count, so a move clamped to the
// old count would leave the cursor 10 items above its own.
func TestVirtualListDemoInsertTopKeepsCursorOnLastItem(t *testing.T) {
	w, app := virtualListDemoWindow(t)
	listID := virtualListDemoEffID(w)

	last := len(app.VLSeqs) - 1
	w.SetVirtualListFocusedIndex(listID, last)
	virtualListDemoRowInView(t, w, app, last)
	seq := app.VLSeqs[last]

	virtualListDemoClick(t, w, "vl-insert-top")
	virtualListDemoFrames(w)
	got := w.VirtualListFocusedIndex(listID)
	if got != last+10 || app.VLSeqs[got] != seq {
		t.Fatalf("cursor = %d (#%d), want %d (#%d)", got, app.VLSeqs[got], last+10, seq)
	}
}

// A cursor whose row is not built stays where it is: moving it would
// scroll the view back to it.
func TestVirtualListDemoInsertTopLeavesFarCursor(t *testing.T) {
	w, app := virtualListDemoWindow(t)
	listID := virtualListDemoEffID(w)

	const far = 4_000
	w.ScrollToIndexAt(listID, far, 0.5)
	virtualListDemoRowInView(t, w, app, far)

	virtualListDemoClick(t, w, "vl-insert-top")
	if got := w.VirtualListFocusedIndex(listID); got != 0 {
		t.Fatalf("cursor = %d, want 0", got)
	}
	// The view stays far down the list, not back at the top. The list
	// keeps the same index under the viewport top across an insert,
	// so index far is still the one in view.
	virtualListDemoRowInView(t, w, app, far)
}

// With the pin on, an appended row is scrolled into view.
func TestVirtualListDemoPinFollowsAppend(t *testing.T) {
	w, app := virtualListDemoWindow(t)

	virtualListDemoClick(t, w, "vl-pin")
	if !app.VLPinned {
		t.Fatal("pin toggle did not set VLPinned")
	}
	virtualListDemoClick(t, w, "vl-append")
	virtualListDemoRowInView(t, w, app, len(app.VLSeqs)-1)
}

// The cursor row is filled with ColorSelect, so both of its texts take
// ColorTextOnSelect. The cursor starts on row 0.
func TestVirtualListDemoFocusedRowTextOnSelect(t *testing.T) {
	w, _ := virtualListDemoWindow(t)
	root := w.TestRender(nil)
	ids := w.ResolveID(gui.ScopeIDN(virtualListDemoID, "row", 0))
	if len(ids) == 0 {
		t.Fatal("row 0 was not built")
	}
	row, ok := root.FindByID(ids[0])
	if !ok {
		t.Fatalf("row %q not in the layout", ids[0])
	}
	want := w.Theme().ColorTextOnSelect
	panel := row.Children[0]
	for i, text := range panel.Children {
		tc := text.Shape.TC
		if tc == nil || tc.TextStyle == nil {
			t.Fatalf("panel child %d has no text style", i)
		}
		if got := tc.TextStyle.Color; got != want {
			t.Errorf("text %d color = %v, want ColorTextOnSelect %v", i, got, want)
		}
	}
}

// With the pin off, an append adds the row and leaves the view alone.
func TestVirtualListDemoAppendUnpinnedStays(t *testing.T) {
	w, app := virtualListDemoWindow(t)

	virtualListDemoClick(t, w, "vl-append")
	if got := len(app.VLSeqs); got != virtualListDemoRows+1 {
		t.Fatalf("rows = %d, want %d", got, virtualListDemoRows+1)
	}
	virtualListDemoFrames(w)
	virtualListDemoAtTop(t, w)
}

// The row-number box accepts surrounding spaces and rejects text,
// negatives and the first index past the end.
func TestVirtualListDemoJumpIndexParse(t *testing.T) {
	w, app := virtualListDemoWindow(t)
	last := len(app.VLSeqs) - 1
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{" 42 ", 42, true},
		{"0", 0, true},
		{strconv.Itoa(last), last, true},
		{strconv.Itoa(last + 1), 0, false},
		{"-1", 0, false},
		{"abc", 0, false},
		{"", 0, false},
	}
	for _, c := range cases {
		app.VLJump = c.in
		n, ok := virtualListDemoJumpIndex(w)
		if n != c.want || ok != c.ok {
			t.Errorf("%q: got (%d, %v), want (%d, %v)", c.in, n, ok, c.want, c.ok)
		}
		if ok == (app.VLJumpErr != "") {
			t.Errorf("%q: ok=%v but VLJumpErr=%q", c.in, ok, app.VLJumpErr)
		}
	}
}

// A row number out of range leaves an error and does not scroll.
func TestVirtualListDemoJumpRejectsBadIndex(t *testing.T) {
	w, app := virtualListDemoWindow(t)

	app.VLJump = "999999"
	virtualListDemoClick(t, w, "vl-center")
	if app.VLJumpErr == "" {
		t.Fatal("VLJumpErr empty, want an error for an out-of-range row")
	}
	virtualListDemoAtTop(t, w)
}
