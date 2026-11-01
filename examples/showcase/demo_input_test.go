package main

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// inputTestWindow renders the showcase at the Input page.
func inputTestWindow(t *testing.T, app *ShowcaseApp) *gui.Window {
	t.Helper()
	app.SelectedGroup = groupEntry
	app.SelectedComponent = "input"
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 1200})
	w.TestRender(mainView)
	w.TestRender(nil)
	return w
}

// A paste past the multiline field's MaxLength leaves it unchanged and
// says why; the next edit clears the notice (#1187).
func TestInputMultilinePasteRefusedShowsNotice(t *testing.T) {
	app := newShowcaseApp()
	app.InputMultiline = "keep"
	w := inputTestWindow(t, app)
	w.SetClipboardGetFn(func() string { return strings.Repeat("a", 10_000) })

	ids := w.ResolveID("input-multi")
	if len(ids) != 1 {
		t.Fatalf("ResolveID(input-multi) = %q, want one match", ids)
	}
	id := ids[0]
	if err := w.TestKey(id, gui.KeyV, gui.ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if app.InputMultiline != "keep" {
		t.Errorf("InputMultiline has %d bytes, want it left at %q",
			len(app.InputMultiline), "keep")
	}
	if !strings.Contains(app.InputPasteNotice, "10000") {
		t.Errorf("InputPasteNotice = %q, want the refused size", app.InputPasteNotice)
	}

	if err := w.TestType(id, "x"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if app.InputPasteNotice != "" {
		t.Errorf("InputPasteNotice = %q after typing, want empty", app.InputPasteNotice)
	}
}

// The Large text field takes the issue's 92,359-byte paste whole, and
// its button fills it with the sample (#1187).
func TestInputLargeTextField(t *testing.T) {
	app := newShowcaseApp()
	w := inputTestWindow(t, app)
	ids := w.ResolveID("input-large")
	if len(ids) != 1 {
		t.Fatalf("ResolveID(input-large) = %q, want one match", ids)
	}
	paste := strings.Repeat("{\"ok\":true},", 92_359/12+1)[:92_359]
	w.SetClipboardGetFn(func() string { return paste })
	if err := w.TestKey(ids[0], gui.KeyV, gui.ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if app.InputLarge != paste {
		t.Fatalf("InputLarge has %d bytes, want the whole %d-byte paste",
			len(app.InputLarge), len(paste))
	}

	w.TestRender(nil)
	fill := w.ResolveID("input-large-fill")
	if len(fill) != 1 {
		t.Fatalf("ResolveID(input-large-fill) = %q, want one match", fill)
	}
	if err := w.TestClick(fill[0]); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if len(app.InputLarge) < 100_000 || !strings.HasPrefix(app.InputLarge, "0001 ") {
		t.Fatalf("InputLarge has %d bytes after Fill, want the 100 KB sample",
			len(app.InputLarge))
	}
	w.TestRender(nil)
}
