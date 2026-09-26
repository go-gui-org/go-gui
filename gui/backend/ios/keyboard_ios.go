//go:build ios

package ios

// keyboard_ios.go — text input and the soft keyboard (issue #806).
//
// GoGuiView (ios_app.m) adopts UIKeyInput. UIKit sends typed text and
// Backspace to it, and hardware key presses to pressesBegan:. These
// come to Go through the goIOS* exports below and go into the window as
// ordinary events. The ioskey package does the conversion, so that it
// runs under `go test` on any host.
//
// The other direction, the gui asking for the keyboard, goes through
// keyboardState. The gui calls IMEStart, IMEStop, ShowSoftKeyboard and
// HideSoftKeyboard while it holds the window lock, and a move between
// two fields makes all four calls in one frame. So the calls only write
// the wanted state here, and one dispatch_async block later reads the
// last state and applies it to UIKit:
//
//   - The keyboard does not close and open again between two fields.
//   - UIKit is never called with the window lock held.
//     becomeFirstResponder can post the keyboard-frame notification
//     at once, and its handler sends an event, which takes that lock.

/*
#include "ios_app.h"
*/
import "C"

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/ioskey"
)

// keyboardState is the keyboard the gui wants. active is true while an
// editable text field holds focus: the view is then first responder,
// so a hardware keyboard types into the field. hidden keeps the soft
// keyboard down while active (HideSoftKeyboard, KeyboardNone).
type keyboardState struct {
	active  bool
	hidden  bool
	secure  bool
	kbdType int
	// queued is true while an apply block waits on the main queue. More
	// requests before it runs only change the state it will read.
	queued bool
}

var (
	keyboardMu sync.Mutex
	keyboard   keyboardState
)

// updateKeyboard changes the wanted state and queues one apply block,
// if none waits already.
func updateKeyboard(change func(*keyboardState)) {
	keyboardMu.Lock()
	change(&keyboard)
	queue := !keyboard.queued
	keyboard.queued = true
	keyboardMu.Unlock()
	if queue {
		C.iosKeyboardQueueApply()
	}
}

func (n *nativePlatform) IMEStart() {
	updateKeyboard(func(k *keyboardState) { k.active = true })
}

func (n *nativePlatform) IMEStop() {
	updateKeyboard(func(k *keyboardState) { k.active = false })
}

// ShowSoftKeyboard asks for the keyboard of the given kind. KeyboardNone
// keeps it down, but the view stays first responder, so a hardware
// keyboard and an app-drawn keypad still type into the field.
func (n *nativePlatform) ShowSoftKeyboard(kind gui.KeyboardKind, secure bool) {
	updateKeyboard(func(k *keyboardState) {
		k.hidden = kind == gui.KeyboardNone
		k.secure = secure
		k.kbdType = ioskey.KeyboardType(kind)
	})
}

func (n *nativePlatform) HideSoftKeyboard() {
	updateKeyboard(func(k *keyboardState) { k.hidden = true })
}

// goIOSKeyboardTake is called by the apply block on the main thread. It
// returns the last wanted state and lets the next request queue a new
// block.
//
//export goIOSKeyboardTake
func goIOSKeyboardTake(active, hidden, kbdType, secure *C.int) {
	keyboardMu.Lock()
	k := keyboard
	keyboard.queued = false
	keyboardMu.Unlock()
	*active = cBool(k.active)
	*hidden = cBool(k.hidden)
	*kbdType = C.int(k.kbdType)
	*secure = cBool(k.secure)
}

func cBool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// goIOSInsertText receives UIKeyInput insertText:. text is UTF-8 and
// owned by the caller.
//
//export goIOSInsertText
func goIOSInsertText(text *C.char) {
	if iosWindow == nil || text == nil {
		return
	}
	if e, ok := ioskey.TextEvent(C.GoString(text)); ok {
		iosWindow.EventFn(&e)
	}
}

// goIOSDeleteBackward receives UIKeyInput deleteBackward.
//
//export goIOSDeleteBackward
func goIOSDeleteBackward() {
	if iosWindow == nil {
		return
	}
	e := ioskey.BackspaceEvent()
	iosWindow.EventFn(&e)
}

// goIOSKey receives a hardware key press (down 1) or release (down 0)
// from pressesBegan: / pressesEnded:. editing is 1 when a text field is
// active (the view is first responder). For a press it returns 1 when
// Go took the key, and 0 when the caller must pass the press on to
// UIKit, which then types it through insertText: or deleteBackward. The
// view sends a release only for a key whose press Go took.
//
//export goIOSKey
func goIOSKey(usage, flags, down, repeat, editing C.int) C.int {
	e, ok := ioskey.KeyEvent(int(usage), int(flags), down != 0, editing != 0)
	if !ok {
		return 0
	}
	if iosWindow != nil {
		e.KeyRepeat = repeat != 0
		iosWindow.EventFn(&e)
	}
	return 1
}

// goIOSSoftKeyboardInset receives the height, in points, that the soft
// keyboard covers at the bottom of the view, 0 when it is down. The gui
// side clamps the value.
//
//export goIOSSoftKeyboardInset
func goIOSSoftKeyboardInset(h C.float) {
	if iosWindow == nil {
		return
	}
	e := gui.Event{Type: gui.EventSoftKeyboard, SoftKeyboardInset: float32(h)}
	iosWindow.EventFn(&e)
}
