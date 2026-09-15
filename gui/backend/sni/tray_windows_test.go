//go:build windows

package sni

import (
	"errors"
	"testing"
	"time"

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

// wmLButtonUp is the tray event for a left click.
const wmLButtonUp = 0x0202

// newRealTray builds a tray window through the real Win32 path, from a
// goroutine that is not locked to any thread. That is the case #616
// reports: the caller's thread has no message loop.
func newRealTray(t *testing.T) *Tray {
	t.Helper()
	tr := &Tray{}
	errc := make(chan error, 1)
	go func() { errc <- tr.ensureWindow() }()
	if err := <-errc; err != nil {
		t.Fatalf("ensureWindow: %v", err)
	}
	return tr
}

// addTestEntry puts an entry with id 1 on the tray. Its callback sends
// the action ID to the returned channel.
func addTestEntry(tr *Tray) <-chan string {
	fired := make(chan string, 1)
	tr.mu.Lock()
	tr.entries = map[int]*entry{
		1: {id: 1, actionCb: func(id string) { fired <- id }},
	}
	tr.mu.Unlock()
	return fired
}

// A tray click must reach the callback when the tray was made from a
// goroutine that no loop pumps. Win32 gives a window's messages only to
// the thread that made the window. Before #616 the window was made on
// the caller's thread and the loop read a different thread, so clicks
// and menu picks never arrived and no error was reported.
//
// Two trays run at once: each must get its own messages. A window class
// names one window procedure, and that procedure is bound to one Tray,
// so a class shared by both trays would send tray 2's clicks to tray 1.
func TestTrayDeliversClicksFromAnyGoroutine(t *testing.T) {
	trays := []*Tray{newRealTray(t), newRealTray(t)}
	fired := []<-chan string{addTestEntry(trays[0]), addTestEntry(trays[1])}

	for i, tr := range trays {
		r, _, err := procPostMessageW.Call(tr.hwnd, wmAppTray, 1, wmLButtonUp)
		if r == 0 {
			t.Fatalf("tray %d: PostMessageW: %v", i, err)
		}
		select {
		case <-fired[i]:
		case <-time.After(2 * time.Second):
			t.Fatalf("tray %d: click callback did not run", i)
		}
	}
	// No click may land on the other tray.
	for i, ch := range fired {
		select {
		case <-ch:
			t.Errorf("tray %d: callback ran twice", i)
		default:
		}
	}
}
