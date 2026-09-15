//go:build windows && !js

package gl

import (
	"runtime"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Reproduce the event-loop order: Windows selects a non-client cursor,
// then the frame loop refreshes the application's client cursor. The
// latter must not overwrite the resize arrow selected by Windows.
func TestFrameCursorPreservesNativeResizeCursor(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	getCursor := user32.NewProc("GetCursor")
	previous, _, _ := getCursor.Call()
	defer pSetCursor.Call(previous)
	arrow, _, _ := pLoadCursorW.Call(0, idcArrow)
	hand, _, _ := pLoadCursorW.Call(0, idcHand)
	resize, _, _ := pLoadCursorW.Call(0, idcSizeWE)
	if arrow == 0 || hand == 0 || resize == 0 {
		t.Fatal("could not load system cursors")
	}
	b := newCharTestBackend(t)
	b.plat.cursors[gui.CursorArrow] = arrow
	b.plat.cursors[gui.CursorPointingHand] = hand
	b.plat.curCursor = arrow
	assertCursor := func(want uintptr) {
		t.Helper()
		if got, _, _ := getCursor.Call(); got != want {
			t.Fatalf("cursor = %#x, want %#x", got, want)
		}
	}
	// Client widget cursors still update immediately after a frame.
	b.handleMessage(wmSetCursor, 0, htClient)
	b.plat.setCursor(gui.CursorPointingHand)
	assertCursor(hand)
	// Windows owns the cursor over its native resize borders.
	const nativeResizeLeft = 10 // HTLEFT
	if _, handled := b.handleMessage(wmSetCursor, 0, nativeResizeLeft); handled {
		t.Fatal("resize cursor should be delegated to Windows")
	}
	pSetCursor.Call(resize) // model DefWindowProc's native resize cursor
	b.plat.setCursor(gui.CursorArrow)
	assertCursor(resize)
	// Returning to the client uses the last requested client cursor.
	b.handleMessage(wmSetCursor, 0, htClient)
	assertCursor(arrow)
	// Leaving for another window also relinquishes cursor ownership.
	b.handleMessage(wmMouseLeave, 0, 0)
	pSetCursor.Call(resize)
	b.plat.setCursor(gui.CursorPointingHand)
	assertCursor(resize)
	// A widget drag with capture may keep updating its cursor outside.
	b.plat.capturing = true
	b.plat.setCursor(gui.CursorPointingHand)
	assertCursor(hand)
}
