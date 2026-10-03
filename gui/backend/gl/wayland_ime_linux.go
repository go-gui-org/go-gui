//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"strings"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// Input methods on Wayland (#919 phase 6): text-input-v3.
//
// On X11 the backend talks to IBus over D-Bus. On Wayland the compositor
// sits between the app and the input method (IBus, fcitx5 through
// input-method-v2), so the app speaks text-input-v3 to the compositor
// instead, and the D-Bus client stays off.
//
//   - The text input enters a surface like the keyboard does. While gui
//     has an editable text widget focused (IMEStart) and the text input is
//     on that window, it is enabled, with the caret rectangle.
//   - The input method's results arrive as preedit_string and
//     commit_string, applied together at done. They become the same gui
//     events the IBus path emits: EventIMEComposition for the preedit and
//     one EventChar carrying the whole commit.
//
// While enabled, the compositor hands key presses to the input method; the
// keys it does not use still arrive on wl_keyboard.

// wlIME is the seat's text input. Main thread only.
type wlIME struct {
	ti wl.ZwpTextInputV3
	// focus is the window the text input entered; enabled says it was
	// enabled there.
	focus   *Backend
	enabled bool

	// Pending state from preedit_string and commit_string, applied at
	// done. The protocol resets it after every done: a done with no
	// preedit_string means no preedit.
	preedit      string
	begin, end   int32
	commit       string
	showPreedit  bool // a preedit is showing in focus
	events       []gui.Event
	commitSerial uint32 // commit requests sent, for done's serial
}

// attachTextInput gets the seat's text input once the manager is bound
// (see attachSelection for why it runs twice).
func (s *wlSeat) attachTextInput() {
	m := &s.ime
	if m.ti.Valid() || !s.d.textInputMgr.Valid() {
		return
	}
	m.ti = s.d.textInputMgr.GetTextInput(s.seat)
	m.ti.SetHandlers(wl.ZwpTextInputV3Handlers{
		Enter: func(surface wl.Surface) {
			m.focus = s.d.wins[surface.Ptr()]
			if b := m.focus; b != nil && b.plat.wl.imeOn {
				m.enable(b.plat.wl)
			}
		},
		Leave: func(wl.Surface) {
			// The compositor ignores our requests until the next
			// enter; a preedit on screen goes with the focus.
			m.clearPreedit()
			m.focus, m.enabled = nil, false
			m.reset()
		},
		PreeditString: func(text string, begin, end int32) {
			m.preedit, m.begin, m.end = strings.Clone(text), begin, end
		},
		CommitString: func(text string) { m.commit = strings.Clone(text) },
		// No surrounding text is sent, so there is none to delete.
		Done: func(uint32) {
			b := m.focus
			m.events = m.done(m.events[:0])
			if b == nil {
				return
			}
			for i := range m.events {
				b.emit(m.events[i])
			}
		},
	})
}

// destroyIME frees the text input.
func (s *wlSeat) destroyIME() {
	if s.ime.ti.Valid() {
		s.ime.ti.Destroy()
	}
	s.ime = wlIME{}
}

// forgetIME drops b, a window being destroyed.
func (m *wlIME) forget(b *Backend) {
	if m.focus == b {
		m.focus, m.enabled, m.showPreedit = nil, false, false
		m.reset()
	}
}

func (m *wlIME) reset() { m.preedit, m.begin, m.end, m.commit = "", 0, 0, "" }

// sendCommit ends a batch of requests; done events count them.
func (m *wlIME) sendCommit() {
	m.ti.Commit()
	m.commitSerial++
}

// enable turns the text input on for ww, which it is on.
func (m *wlIME) enable(ww *wlWindow) {
	m.ti.Enable()
	m.ti.SetContentType(wl.ZwpTextInputV3ContentHintNone, wl.ZwpTextInputV3ContentPurposeNormal)
	if ww.imeHaveRect {
		r := ww.imeRect
		m.ti.SetCursorRectangle(r[0], r[1], r[2], r[3])
	}
	m.sendCommit()
	m.enabled = true
}

// disable turns the text input off. No event is emitted: gui clears its
// composition on the focus change that stops the IME (see IMEStop on X11).
func (m *wlIME) disable() {
	if m.enabled {
		m.ti.Disable()
		m.sendCommit()
	}
	m.enabled, m.showPreedit = false, false
	m.reset()
}

// clearPreedit ends a preedit on screen with an empty composition.
func (m *wlIME) clearPreedit() {
	if m.showPreedit && m.focus != nil {
		m.focus.emit(gui.Event{Type: gui.EventIMEComposition})
	}
	m.showPreedit = false
}

// done applies the pending state, appending the gui events it makes to
// out, in the order text-input-v3 gives: the old preedit goes, the commit
// is inserted, the new preedit is shown.
func (m *wlIME) done(out []gui.Event) []gui.Event {
	commit, preedit := imeSanitizeText(m.commit), imeSanitizeText(m.preedit)
	begin, end := m.begin, m.end
	m.reset()
	if commit != "" {
		if m.showPreedit {
			out = append(out, gui.Event{Type: gui.EventIMEComposition})
			m.showPreedit = false
		}
		first, _ := utf8.DecodeRuneInString(commit)
		out = append(out, gui.Event{Type: gui.EventChar, CharCode: uint32(first), IMEText: commit})
	}
	switch {
	case preedit != "":
		start, n := wlPreeditRange(preedit, begin, end)
		out = append(out, gui.Event{
			Type:      gui.EventIMEComposition,
			IMEText:   preedit,
			IMEStart:  start,
			IMELength: n,
		})
		m.showPreedit = true
	case m.showPreedit:
		out = append(out, gui.Event{Type: gui.EventIMEComposition})
		m.showPreedit = false
	}
	return out
}

// wlPreeditRange turns text-input-v3's cursor, byte offsets into the
// preedit (-1, -1 for a hidden cursor), into gui's clause range in
// characters. Offsets outside the text or inside a character are clamped
// to a character boundary.
func wlPreeditRange(text string, begin, end int32) (start, length int32) {
	if begin < 0 || end < 0 {
		return int32(utf8.RuneCountInString(text)), 0
	}
	runeAt := func(off int32) int32 {
		off = min(off, int32(len(text)))
		for off > 0 && off < int32(len(text)) && !utf8.RuneStart(text[off]) {
			off--
		}
		return int32(utf8.RuneCountInString(text[:off]))
	}
	b, e := runeAt(begin), runeAt(end)
	if e < b {
		b, e = e, b
	}
	return b, e - b
}

// --- gui hooks (nativePlatform.IMEStart, IMEStop, IMESetRect) ---

// imeStart: an editable text widget took focus in ww.
func (ww *wlWindow) imeStart() {
	ww.imeOn = true
	if s := ww.d.seat; s != nil && s.ime.ti.Valid() && s.ime.focus == ww.b && !s.ime.enabled {
		s.ime.enable(ww)
	}
}

// imeStop: no editable text has focus in ww.
func (ww *wlWindow) imeStop() {
	ww.imeOn, ww.imeHaveRect = false, false
	if s := ww.d.seat; s != nil && s.ime.focus == ww.b {
		s.ime.disable()
	}
}

// imeSetRect reports the caret, in logical pixels relative to the window,
// which is the surface-local unit text-input-v3 takes. gui reports it
// every frame while composing, so an unchanged rect sends nothing.
func (ww *wlWindow) imeSetRect(x, y, w, h int32) {
	r := [4]int32{x, y, max(w, 1), max(h, 1)}
	if ww.imeHaveRect && ww.imeRect == r {
		return
	}
	ww.imeRect, ww.imeHaveRect = r, true
	if s := ww.d.seat; s != nil && s.ime.enabled && s.ime.focus == ww.b {
		s.ime.ti.SetCursorRectangle(r[0], r[1], r[2], r[3])
		s.ime.sendCommit()
	}
}
