//go:build windows

package sni

import (
	"errors"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// A failed window init must stay failed. sync.Once runs the init only
// once, so the error has to live on the Tray; a per-call local would
// make every later call report success with no window behind it.
// Not parallel: it swaps the package-level trayInitWindow seam.
func TestEnsureWindowKeepsInitError(t *testing.T) {
	orig := trayInitWindow
	t.Cleanup(func() { trayInitWindow = orig })

	want := errors.New("init failed")
	calls := 0
	trayInitWindow = func(*Tray) error {
		calls++
		return want
	}

	var tr Tray
	for i := range 3 {
		if err := tr.ensureWindow(); !errors.Is(err, want) {
			t.Fatalf("call %d: got %v, want %v", i, err, want)
		}
	}
	if calls != 1 {
		t.Errorf("init calls: got %d, want 1", calls)
	}

	// Create must refuse, not call Shell_NotifyIconW against hwnd 0.
	id, err := tr.Create(gui.SystemTrayCfg{}, nil)
	if !errors.Is(err, want) {
		t.Errorf("Create err: got %v, want %v", err, want)
	}
	if id != 0 {
		t.Errorf("Create id: got %d, want 0", id)
	}
}
