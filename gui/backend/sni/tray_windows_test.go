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

// wmMouseMove is a tray event version 4 sends that must fire nothing.
const wmMouseMove = 0x0200

// trayLParam packs lParam the way NOTIFYICON_VERSION_4 does: the event
// in the low word and the icon ID in the high word.
func trayLParam(event, iconID uint16) uintptr {
	return uintptr(iconID)<<16 | uintptr(event)
}

// trayWParam packs wParam the way NOTIFYICON_VERSION_4 does: the anchor
// X in the low word and Y in the high word, each a signed 16-bit value.
func trayWParam(x, y int16) uintptr {
	return uintptr(uint16(y))<<16 | uintptr(uint16(x))
}

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
		r, _, err := procPostMessageW.Call(tr.hwnd, wmAppTray, 0, trayLParam(ninSelect, 1))
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

// The version 4 layout puts the event and icon ID in lParam and the
// anchor position in wParam. Coordinates are signed: a monitor to the
// left of or above the primary one gives negative values.
func TestDecodeTrayCallback(t *testing.T) {
	tests := []struct {
		name         string
		wParam       uintptr
		lParam       uintptr
		event        uint16
		iconID       uint16
		wantX, wantY int32
	}{
		{"select", trayWParam(100, 200), trayLParam(ninSelect, 1), ninSelect, 1, 100, 200},
		{"context menu", trayWParam(1919, 1079), trayLParam(wmContextMenu, 7), wmContextMenu, 7, 1919, 1079},
		{"negative position", trayWParam(-1280, -4), trayLParam(ninKeySelect, 2), ninKeySelect, 2, -1280, -4},
		{"max icon ID", trayWParam(0, 0), trayLParam(wmMouseMove, 0xFFFF), wmMouseMove, 0xFFFF, 0, 0},
		// On 64-bit Windows the upper 32 bits can hold anything; the
		// decode reads only the low 32.
		{"upper bits ignored", 0xDEAD_0000_0000 | trayWParam(5, 6), 0xBEEF_0000_0000 | trayLParam(ninSelect, 3), ninSelect, 3, 5, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, iconID, x, y := decodeTrayCallback(tt.wParam, tt.lParam)
			if event != tt.event || iconID != tt.iconID || x != tt.wantX || y != tt.wantY {
				t.Errorf("got event %#x icon %d at (%d,%d), want event %#x icon %d at (%d,%d)",
					event, iconID, x, y, tt.event, tt.iconID, tt.wantX, tt.wantY)
			}
		})
	}
}

// menuCall is one context menu request seen by the test hook.
type menuCall struct {
	hMenu uintptr
	x, y  int32
}

// A real tray window receives version 4 callbacks (#617). Select and
// key select fire the default action. The context menu event opens the
// entry's menu at the position the shell sent, not at the mouse
// cursor, so a menu opened from the keyboard appears at the icon. Raw
// mouse events, which version 4 also sends, and events for an unknown
// icon fire nothing.
func TestTrayVersion4Callbacks(t *testing.T) {
	tr := newRealTray(t)
	fired := make(chan string, 8)
	menus := make(chan menuCall, 8)
	const fakeMenu = 0x1234
	tr.mu.Lock()
	tr.entries = map[int]*entry{
		1: {id: 1, hMenu: fakeMenu, actionCb: func(id string) { fired <- id }},
	}
	// The hook stands in for TrackPopupMenu, which would block the tray
	// thread in a modal menu loop.
	tr.trackMenu = func(hMenu uintptr, x, y int32) {
		menus <- menuCall{hMenu, x, y}
	}
	tr.mu.Unlock()

	post := func(wParam, lParam uintptr) {
		t.Helper()
		if r, _, err := procPostMessageW.Call(tr.hwnd, wmAppTray, wParam, lParam); r == 0 {
			t.Fatalf("PostMessageW: %v", err)
		}
	}
	// Events that must do nothing go first. The select posted after
	// them is a marker: messages are handled in order, so once the
	// marker's callback runs, the earlier events have been handled.
	post(trayWParam(10, 10), trayLParam(wmMouseMove, 1))
	post(trayWParam(10, 10), trayLParam(ninSelect, 9)) // unknown icon
	post(trayWParam(10, 10), trayLParam(ninSelect, 1))
	post(trayWParam(10, 10), trayLParam(ninKeySelect, 1))
	post(trayWParam(-300, 40), trayLParam(wmContextMenu, 1))

	for i, name := range []string{"select", "key select"} {
		select {
		case id := <-fired:
			if id != "" {
				t.Errorf("%s: action ID %q, want empty", name, id)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("callback %d (%s) did not run", i, name)
		}
	}
	select {
	case m := <-menus:
		if m != (menuCall{fakeMenu, -300, 40}) {
			t.Errorf("menu: got %+v, want %+v", m, menuCall{fakeMenu, -300, 40})
		}
	case <-time.After(2 * time.Second):
		t.Fatal("context menu did not open")
	}
	select {
	case id := <-fired:
		t.Errorf("extra callback ran with %q", id)
	case m := <-menus:
		t.Errorf("extra menu opened: %+v", m)
	default:
	}
}
