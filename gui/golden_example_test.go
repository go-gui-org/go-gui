package gui_test

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// This file proves the public golden helpers work from outside the
// gui package: it imports gui like any app or sibling repo and uses
// only exported API. It also pins the dx-cheat-sheet section quoting
// it (examples/showcase/sound_snippet_test.go).

// doc:snippet-begin golden
//
// TestGoldenExample pins a button's appearance as text. TestRender
// builds the view, and TestGolden records one frame per theme into
// testdata/save_button.{dark,light}.golden. Re-record with
// GOGUI_UPDATE_GOLDEN=1 after reading the diff.

func TestGoldenExample(t *testing.T) {
	w := gui.NewTestWindow(t, gui.WindowCfg{Width: 320, Height: 240})
	w.TestRender(func(win *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing: gui.FillFill,
			Content: []gui.View{
				gui.Button(gui.ButtonCfg{ID: "save", Label: "Save"}),
			},
		})
	})
	w.TestGolden(t, gui.GoldenCfg{Name: "save_button"})
}

// doc:snippet-end golden
