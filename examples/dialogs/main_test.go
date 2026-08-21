package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The library keys its dialog overlay by this reserved ID
// (gui.reservedDialogID, unexported). Mirrored here so the test can find
// the overlay in the tree.
const dialogShapeID = "___dialog_reserved_do_not_use___"

// Clicking the button should put a dialog on screen. Asserting on the
// rendered tree rather than on a bool is the point: the dialog is an
// overlay built by the library, so this also proves the click reached
// the button through whatever is drawn over it.
func TestMessageButtonOpensDialog(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)

	w := gui.NewTestWindow(gui.WindowCfg{State: &App{}, Width: 640, Height: 550})
	root := w.TestRender(mainView)
	if _, ok := root.FindByID(dialogShapeID); ok {
		t.Fatal("dialog present before any click")
	}

	if err := w.TestClick("dlg_message"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}

	root = w.TestRender(nil)
	if _, ok := root.FindByID(dialogShapeID); !ok {
		t.Fatal("no dialog in the tree after clicking DialogMessage")
	}
}
