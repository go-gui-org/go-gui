//go:build android

package android

import (
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
	r, _ := utf8.DecodeRuneInString(text)
	evt := gui.Event{
		Type:     gui.EventChar,
		CharCode: uint32(r),
		IMEText:  text,
	}
	androidWindow.EventFn(&evt)
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
