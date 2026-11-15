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
	if got := startHidden(w); got != 0 {
		t.Errorf("startHidden = %d for a window that was not hidden, want 0", got)
	}
	w.Hide()
	if got := startHidden(w); got != 1 {
		t.Errorf("startHidden = %d after Hide, want 1", got)
	}
}
