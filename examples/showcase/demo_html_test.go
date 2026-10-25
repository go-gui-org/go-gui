package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestDemoHTMLBuildsSourceAndPreview checks that the HTML page has the
// editor and the live preview, and that the preview starts from the
// sample snippet in the app state.
func TestDemoHTMLBuildsSourceAndPreview(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{State: newShowcaseApp()})
	layout := gui.GenerateViewLayout(demoHTML(w), w)
	for _, id := range []string{"html-source", "html-preview"} {
		if _, ok := layout.FindByID(id); !ok {
			t.Errorf("%s not found", id)
		}
	}
	if got := appState(w).HTMLSource; got != htmlDemoSource {
		t.Errorf("HTMLSource = %q, want the sample snippet", got)
	}
}
