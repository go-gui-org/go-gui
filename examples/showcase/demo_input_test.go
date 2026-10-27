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

// A paste too large for the multiline field leaves it unchanged and
// says why; the next edit clears the notice (#1187).
func TestInputMultilinePasteRefusedShowsNotice(t *testing.T) {
	app := newShowcaseApp()
	app.InputMultiline = "keep"
	w := inputTestWindow(t, app)
	w.SetClipboardGetFn(func() string { return strings.Repeat("a", 92_359) })

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
	if !strings.Contains(app.InputPasteNotice, "92359") {
		t.Errorf("InputPasteNotice = %q, want the refused size", app.InputPasteNotice)
	}

	if err := w.TestType(id, "x"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if app.InputPasteNotice != "" {
		t.Errorf("InputPasteNotice = %q after typing, want empty", app.InputPasteNotice)
	}
}
