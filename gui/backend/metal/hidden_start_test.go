//go:build darwin && cgo && !ios

package metal

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Issue #1270: a window the app hid before the backend started is
// created hidden; any other window is created shown.
func TestStartHidden(t *testing.T) {
	w := gui.NewTestWindow(t, gui.WindowCfg{})
	if startHidden(w) {
		t.Error("startHidden = true for a window that was not hidden")
	}
	w.Hide()
	if !startHidden(w) {
		t.Error("startHidden = false after Hide")
	}
}
