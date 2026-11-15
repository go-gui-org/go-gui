//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The page hidden and shown again reaches the app as one
// EventVisibilityChanged each, with Window.IsOccluded following
// document.hidden (issue #1170). A document without the property, and a
// repeat of the same state, send nothing.
func TestSyncPageHidden(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{})
	events := 0
	w.OnEvent = func(e *gui.Event, _ *gui.Window) {
		if e.Type == gui.EventVisibilityChanged {
			events++
		}
	}
	doc := js.Global().Get("Object").New()

	for _, step := range []struct {
		name         string
		hidden       any // nil leaves the property out
		wantOccluded bool
		wantEvents   int
	}{
		{"no hidden property", nil, false, 0},
		{"visible page", false, false, 0},
		{"tab hidden", true, true, 1},
		{"still hidden", true, true, 1},
		{"tab shown", false, false, 2},
	} {
		if step.hidden != nil {
			doc.Set("hidden", step.hidden)
		}
		syncPageHidden(w, doc)
		if w.IsOccluded() != step.wantOccluded || events != step.wantEvents {
			t.Errorf("%s: occluded=%v events=%d, want %v %d", step.name,
				w.IsOccluded(), events, step.wantOccluded, step.wantEvents)
		}
	}
}
