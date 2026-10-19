//go:build android

package android

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
)

// imeComposition dispatches an IME preedit composition event.
func imeComposition(text string, cursor, selLen int32) {
	if androidWindow == nil {
		return
	}
	evt := gui.Event{
		Type:      gui.EventIMEComposition,
		IMEText:   text,
		IMEStart:  cursor,
		IMELength: selLen,
	}
	androidWindow.EventFn(&evt)
}

// maxIMETextRunes bounds a commit string crossing the JNI boundary,
// which arrives with no length promised. Mirrors maxIMEPreeditRunes
// in gui/ime.go; a commit is a phrase, far past any real one.
const maxIMETextRunes = 4096

// imeSanitizeText strips decoding failures and caps the length of a
// commit string, returning empty when nothing committable remains.
func imeSanitizeText(s string) string {
	if strings.ContainsRune(s, 0xFFFD) {
		s = strings.Map(func(r rune) rune {
			if r == 0xFFFD {
				return -1
			}
			return r
		}, s)
	}
	if s == "" {
		return ""
	}
	if utf8.RuneCountInString(s) > maxIMETextRunes {
		i := 0
		for range maxIMETextRunes {
			_, size := utf8.DecodeRuneInString(s[i:])
			i += size
		}
		s = s[:i]
	}
	return s
}

// imeCommit dispatches committed text as a single EventChar carrying
// the whole string in IMEText, matching macOS and web. CharCode holds
// only the first rune; consumers read IMEText for the full commit.
func imeCommit(text string) {
	if androidWindow == nil {
		return
	}
	// Drop invalid content crossing the JNI boundary rather than
	// dispatching a RuneError char, and cap what a hostile caller
	// can push through one commit.
	text = imeSanitizeText(text)
	if len(text) == 0 {
		return
	}
	// An edit key the host sends as a commit types no text: the field
	// drops control characters and acts on the key-down instead.
	if kc, ok := imeCommitKey(text); ok {
		evt := gui.Event{Type: gui.EventKeyDown, KeyCode: kc}
		androidWindow.EventFn(&evt)
		return
	}
	r, _ := utf8.DecodeRuneInString(text)
	evt := gui.Event{
		Type:     gui.EventChar,
		CharCode: uint32(r),
		IMEText:  text,
	}
	androidWindow.EventFn(&evt)
}

// imeCommitKey maps a commit that is a single edit key to that key.
// The Kotlin host's sendKeyEvent sends Backspace as "\b" and forward
// delete as "\x7f" (KEYCODE_DEL, KEYCODE_FORWARD_DEL), and Enter as
// "\n" (KEYCODE_ENTER). A soft keyboard that knows nothing of the
// field's text sends those keys instead of deleting text. A
// deleteSurroundingText request does not come here: the host passes it
// to IMEDeleteSurrounding, which carries the count.
func imeCommitKey(text string) (gui.KeyCode, bool) {
	switch text {
	case "\b":
		return gui.KeyBackspace, true
	case "\x7f":
		return gui.KeyDelete, true
	case "\n", "\r":
		return gui.KeyEnter, true
	}
	return gui.KeyInvalid, false
}

// imeDeleteSurrounding dispatches the InputConnection's requested range as
// a Backspace for the text before the selection and a Delete for the text
// after it, each carrying its count and unit (Event.IMEDeleteCount).
func imeDeleteSurrounding(before, after int64, inCodePoints bool) {
	if androidWindow == nil {
		return
	}
	evts, n := imeDeleteEvents(before, after, inCodePoints)
	for i := range n {
		androidWindow.EventFn(&evts[i])
	}
}

// imeDeleteEvents builds the key-downs for a deleteSurroundingText
// request: up to one Backspace and one Delete, none for an empty
// direction. The lengths cross JNI as Java ints widened to int64, so a
// negative one is dropped and a huge one is capped.
func imeDeleteEvents(before, after int64, inCodePoints bool) ([2]gui.Event, int) {
	var evts [2]gui.Event
	n := 0
	for _, d := range [2]struct {
		key   gui.KeyCode
		count int64
	}{{gui.KeyBackspace, before}, {gui.KeyDelete, after}} {
		if d.count <= 0 {
			continue
		}
		evts[n] = gui.Event{
			Type:                  gui.EventKeyDown,
			KeyCode:               d.key,
			IMEDeleteCount:        int32(min(d.count, math.MaxInt32)),
			IMEDeleteInCodePoints: inCodePoints,
		}
		n++
	}
	return evts, n
}

// touchEvent dispatches a raw touch event to the window's event
// pipeline. The gesture recognizer in gui/gesture.go processes
// these into gesture events and synthesized mouse events.
func touchEvent(typ gui.EventType, id uint64, x, y float32) {
	if androidWindow == nil {
		return
	}
	evt := gui.Event{
		Type:       typ,
		NumTouches: 1,
		Touches: [8]gui.TouchPoint{{
			Identifier: id,
			PosX:       x,
			PosY:       y,
			ToolType:   gui.TouchToolFinger,
			Changed:    true,
		}},
	}
	androidWindow.EventFn(&evt)
}
