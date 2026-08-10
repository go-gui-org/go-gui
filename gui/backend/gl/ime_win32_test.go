//go:build windows && !js

package gl

import (
	"math"
	"testing"
	"unicode/utf16"

	"github.com/go-gui-org/go-gui/gui"
)

func TestIMEClauseRange(t *testing.T) {
	const conv = attrTargetConverted
	const notConv = 3 // ATTR_TARGET_NOTCONVERTED

	cases := []struct {
		name       string
		attr       []byte
		start, end int
		ok         bool
	}{
		{"nil", nil, 0, 0, false},
		{"no target run", []byte{notConv, notConv}, 0, 0, false},
		{"leading run", []byte{conv, conv, notConv, notConv}, 0, 2, true},
		{"middle run", []byte{notConv, conv, conv, notConv}, 1, 3, true},
		{"trailing run", []byte{notConv, notConv, conv}, 2, 3, true},
		{"whole string", []byte{conv, conv, conv}, 0, 3, true},
		{"single unit", []byte{conv}, 0, 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, ok := imeClauseRange(tc.attr)
			if ok != tc.ok || start != tc.start || end != tc.end {
				t.Errorf("imeClauseRange(%v) = (%d, %d, %v), want (%d, %d, %v)",
					tc.attr, start, end, ok, tc.start, tc.end, tc.ok)
			}
		})
	}
}

// TestIMEClauseRangeFirstRunWins pins the "first run" rule: an IME that
// marks two disjoint converted runs must not yield a range spanning the
// gap between them.
func TestIMEClauseRangeFirstRunWins(t *testing.T) {
	const conv = attrTargetConverted
	attr := []byte{conv, 3, conv, conv}
	start, end, ok := imeClauseRange(attr)
	if !ok || start != 0 || end != 1 {
		t.Errorf("imeClauseRange = (%d, %d, %v), want (0, 1, true)",
			start, end, ok)
	}
}

func TestIMERuneIndex(t *testing.T) {
	bmp := utf16.Encode([]rune("にほんご"))
	// U+1F600 is a surrogate pair, so "あ😀い" is 3 runes in 4 units.
	mixed := utf16.Encode([]rune("あ😀い"))

	cases := []struct {
		name string
		u    []uint16
		n    int
		want int32
	}{
		{"zero", bmp, 0, 0},
		{"bmp mid", bmp, 2, 2},
		{"bmp end", bmp, 4, 4},
		{"past end clamps", bmp, 99, 4},
		{"before pair", mixed, 1, 1},
		{"inside pair", mixed, 2, 2},
		{"after pair", mixed, 3, 2},
		{"pair end", mixed, 4, 3},
		{"empty buffer", nil, 3, 0},
		{"negative", bmp, -1, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := imeRuneIndex(tc.u, tc.n); got != tc.want {
				t.Errorf("imeRuneIndex(%q, %d) = %d, want %d",
					string(utf16.Decode(tc.u)), tc.n, got, tc.want)
			}
		})
	}
}

func TestIMEDecodeDropsReplacementChar(t *testing.T) {
	u := []uint16{'a', 0xFFFD, 'b'}
	if got := imeDecode(u); got != "ab" {
		t.Errorf("imeDecode = %q, want %q", got, "ab")
	}
	if got := imeDecode(nil); got != "" {
		t.Errorf("imeDecode(nil) = %q, want empty", got)
	}
}

// TestIMEDecodeDropsLoneSurrogate covers a malformed composition buffer:
// utf16.Decode turns an unpaired surrogate into U+FFFD, which must not
// reach a widget as a visible replacement glyph.
func TestIMEDecodeDropsLoneSurrogate(t *testing.T) {
	if got := imeDecode([]uint16{0xD800, 'a'}); got != "a" {
		t.Errorf("imeDecode(lone high surrogate) = %q, want %q", got, "a")
	}
	if got := imeDecode([]uint16{'a', 0xDC00}); got != "a" {
		t.Errorf("imeDecode(lone low surrogate) = %q, want %q", got, "a")
	}
}

func TestIMEDecodeKeepsNonBMP(t *testing.T) {
	u := utf16.Encode([]rune("😀"))
	if got := imeDecode(u); got != "😀" {
		t.Errorf("imeDecode = %q, want %q", got, "😀")
	}
}

// TestIMECompositionEventClause covers the converted-clause case: the
// second of two clauses is selected, and the reported range must be
// absolute (see gui/event.go's IMEStart/IMELength contract), not a
// cursor plus a length.
func TestIMECompositionEventClause(t *testing.T) {
	const conv = attrTargetConverted
	comp := utf16.Encode([]rune("にほんご"))
	attr := []byte{3, 3, conv, conv}

	e := imeCompositionEvent(comp, attr, 2)
	if e.Type != gui.EventIMEComposition {
		t.Fatalf("Type = %v, want EventIMEComposition", e.Type)
	}
	if e.IMEText != "にほんご" {
		t.Errorf("IMEText = %q, want %q", e.IMEText, "にほんご")
	}
	if e.IMEStart != 2 || e.IMELength != 2 {
		t.Errorf("clause = (%d, %d), want (2, 2)", e.IMEStart, e.IMELength)
	}
}

// TestIMECompositionEventNoClause covers the pre-conversion state: no
// attribute marks a clause, so IMEStart falls back to the cursor and
// IMELength stays zero.
func TestIMECompositionEventNoClause(t *testing.T) {
	comp := utf16.Encode([]rune("にほん"))
	e := imeCompositionEvent(comp, nil, 3)
	if e.IMEText != "にほん" {
		t.Errorf("IMEText = %q, want %q", e.IMEText, "にほん")
	}
	if e.IMEStart != 3 || e.IMELength != 0 {
		t.Errorf("clause = (%d, %d), want (3, 0)", e.IMEStart, e.IMELength)
	}
}

// TestIMECompositionEventNonBMPOffsets is the PR #210 regression in its
// Win32 form: IMM reports UTF-16 offsets, gui.Event wants characters, so
// a surrogate pair before the clause must not shift the range by one.
func TestIMECompositionEventNonBMPOffsets(t *testing.T) {
	const conv = attrTargetConverted
	comp := utf16.Encode([]rune("😀にほ")) // 4 UTF-16 units, 3 runes
	attr := []byte{3, 3, conv, conv}    // clause covers "にほ"

	e := imeCompositionEvent(comp, attr, 2)
	if e.IMEStart != 1 || e.IMELength != 2 {
		t.Errorf("clause = (%d, %d), want (1, 2)", e.IMEStart, e.IMELength)
	}
}

// TestIMECursorPastEndClamps guards the render path: a hostile or
// confused input method must not report an offset beyond the preedit.
func TestIMECursorPastEndClamps(t *testing.T) {
	comp := utf16.Encode([]rune("あい"))
	e := imeCompositionEvent(comp, nil, 99)
	if e.IMEStart != 2 {
		t.Errorf("IMEStart = %d, want 2", e.IMEStart)
	}
}

func TestEmitCommitSingleEventFullText(t *testing.T) {
	b := newCharTestBackend(t)
	b.plat.evt = gui.Event{Type: gui.EventInvalid}
	b.emitCommit(utf16.Encode([]rune("日本語")))

	if b.plat.evt.Type != gui.EventChar {
		t.Fatalf("Type = %v, want EventChar", b.plat.evt.Type)
	}
	if b.plat.evt.IMEText != "日本語" {
		t.Errorf("IMEText = %q, want %q", b.plat.evt.IMEText, "日本語")
	}
	if b.plat.evt.CharCode != uint32('日') {
		t.Errorf("CharCode = %#x, want %#x", b.plat.evt.CharCode, '日')
	}
	if b.plat.evt.Modifiers != 0 {
		t.Errorf("Modifiers = %v, want none", b.plat.evt.Modifiers)
	}
}

// TestEmitCommitNonBMP checks the commit path preserves astral
// characters on its own: it bypasses charInput's surrogate reassembly.
func TestEmitCommitNonBMP(t *testing.T) {
	b := newCharTestBackend(t)
	b.plat.evt = gui.Event{Type: gui.EventInvalid}
	b.emitCommit(utf16.Encode([]rune("😀")))

	if b.plat.evt.IMEText != "😀" {
		t.Errorf("IMEText = %q, want %q", b.plat.evt.IMEText, "😀")
	}
	if b.plat.evt.CharCode != 0x1F600 {
		t.Errorf("CharCode = %#x, want 0x1F600", b.plat.evt.CharCode)
	}
}

func TestEmitCommitEmptyEmitsNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		u    []uint16
	}{
		{"empty", nil},
		{"all replacement chars", []uint16{0xFFFD, 0xFFFD}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newCharTestBackend(t)
			b.plat.evt = gui.Event{Type: gui.EventInvalid}
			b.emitCommit(tc.u)
			if b.plat.evt.Type != gui.EventInvalid {
				t.Errorf("emitted %v, want no event", b.plat.evt.Type)
			}
		})
	}
}

// TestIMEMessageEndCompositionClears asserts WM_IME_ENDCOMPOSITION emits
// the empty composition that resets Window.IMEComposing.
func TestIMEMessageEndCompositionClears(t *testing.T) {
	b := newCharTestBackend(t)
	b.plat.evt = gui.Event{Type: gui.EventInvalid}

	res, handled := b.imeMessage(wmIMEEndComposition, 0, 0)
	if res != 0 || !handled {
		t.Fatalf("imeMessage = (%d, %v), want (0, true)", res, handled)
	}
	if b.plat.evt.Type != gui.EventIMEComposition || b.plat.evt.IMEText != "" {
		t.Errorf("event = {%v, %q}, want {EventIMEComposition, \"\"}",
			b.plat.evt.Type, b.plat.evt.IMEText)
	}
}

// TestIMEMessageStartAndCharAreOwned pins the two messages that must not
// reach DefWindowProc: WM_IME_STARTCOMPOSITION (which would draw the
// IME's own composition window) and WM_IME_CHAR (which DefWindowProc
// turns into the WM_CHAR that double-inserts every commit).
func TestIMEMessageStartAndCharAreOwned(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  uintptr
	}{
		{"start composition", wmIMEStartComposition},
		{"ime char", wmIMEChar},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newCharTestBackend(t)
			b.plat.evt = gui.Event{Type: gui.EventInvalid}
			res, handled := b.imeMessage(tc.msg, 'あ', 0)
			if res != 0 || !handled {
				t.Fatalf("imeMessage = (%d, %v), want (0, true)", res, handled)
			}
			if b.plat.evt.Type != gui.EventInvalid {
				t.Errorf("emitted %v, want no event", b.plat.evt.Type)
			}
		})
	}
}

func TestIMEPixelBounds(t *testing.T) {
	inf := float32(math.Inf(1))
	cases := []struct {
		name string
		in   float32
		want int32
	}{
		{"zero", 0, 0},
		{"ordinary", 120.7, 120},
		{"negative", -40.2, -40},
		{"at bound", imeCaretBound, imeCaretBound},
		{"past bound", 1e9, imeCaretBound},
		{"past negative bound", -1e9, -imeCaretBound},
		{"positive infinity", inf, imeCaretBound},
		{"negative infinity", -inf, -imeCaretBound},
		{"nan", float32(math.NaN()), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := imePixel(tc.in); got != tc.want {
				t.Errorf("imePixel(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestIMEReadersRejectEmpty covers the degenerate buffers the pure
// helpers must survive: an input method is out-of-process and can report
// anything, and these feed the render path.
func TestIMEReadersRejectEmpty(t *testing.T) {
	if s, e, ok := imeClauseRange(nil); ok || s != 0 || e != 0 {
		t.Errorf("imeClauseRange(nil) = (%d, %d, %v), want (0, 0, false)", s, e, ok)
	}
	if got := imeRuneIndex(nil, 5); got != 0 {
		t.Errorf("imeRuneIndex(nil, 5) = %d, want 0", got)
	}
	e := imeCompositionEvent(nil, nil, 0)
	if e.Type != gui.EventIMEComposition || e.IMEText != "" {
		t.Errorf("empty composition = {%v, %q}, want cleared", e.Type, e.IMEText)
	}
}

// TestIMECompositionEventAttrLongerThanText guards the case where IMM
// reports an attribute buffer inconsistent with the composition string:
// the clause range must stay inside the text rather than index past it.
func TestIMECompositionEventAttrLongerThanText(t *testing.T) {
	const conv = attrTargetConverted
	comp := utf16.Encode([]rune("あ"))
	attr := []byte{conv, conv, conv, conv}

	e := imeCompositionEvent(comp, attr, 0)
	if e.IMEStart != 0 || e.IMELength != 1 {
		t.Errorf("clause = (%d, %d), want (0, 1)", e.IMEStart, e.IMELength)
	}
	if e.IMEStart+e.IMELength > 1 {
		t.Errorf("clause runs past the preedit: %d", e.IMEStart+e.IMELength)
	}
}

// TestIMECompositionEventLengthNeverNegative pins the invariant
// gui.imeUpdate's slice arithmetic depends on.
func TestIMECompositionEventLengthNeverNegative(t *testing.T) {
	const conv = attrTargetConverted
	for _, attr := range [][]byte{
		{conv},
		{conv, conv, 3},
		{3, conv},
		{3, 3, 3},
		{},
	} {
		e := imeCompositionEvent(utf16.Encode([]rune("にほ")), attr, 0)
		if e.IMELength < 0 || e.IMEStart < 0 {
			t.Errorf("attr %v gave negative range (%d, %d)",
				attr, e.IMEStart, e.IMELength)
		}
	}
}

func TestIMEMessageDeclinesUnrelated(t *testing.T) {
	b := newCharTestBackend(t)
	if _, handled := b.imeMessage(wmChar, 'a', 0); handled {
		t.Error("imeMessage claimed WM_CHAR")
	}
}

// TestIMECompositionWithoutContextEmitsNothing pins the ImmGetContext
// failure path: no context means no composition to read, and the message
// must still be owned rather than falling through to DefWindowProc,
// which would draw the IME's own composition window.
func TestIMECompositionWithoutContextEmitsNothing(t *testing.T) {
	b := newCharTestBackend(t) // hwnd stays 0, so ImmGetContext fails
	b.plat.evt = gui.Event{Type: gui.EventInvalid}

	res, handled := b.imeMessage(wmIMEComposition, 0, gcsResultStr|gcsCompStr)
	if res != 0 || !handled {
		t.Fatalf("imeMessage = (%d, %v), want (0, true)", res, handled)
	}
	if b.plat.evt.Type != gui.EventInvalid {
		t.Errorf("emitted %v, want no event", b.plat.evt.Type)
	}
}

// TestKeyProcessKeyIsSuppressed pins the VK_PROCESSKEY gate: while the
// input method owns a keystroke, the widget must not also see it as a
// key event.
func TestKeyProcessKeyIsSuppressed(t *testing.T) {
	for _, msg := range []uintptr{
		wmKeyDown, wmSysKeyDown, wmKeyUp, wmSysKeyUp,
	} {
		b := newCharTestBackend(t)
		b.plat.evt = gui.Event{Type: gui.EventInvalid}
		res, handled := b.handleMessage(msg, vkProcessKey, 0)
		if res != 0 || !handled {
			t.Fatalf("handleMessage(%#x) = (%d, %v), want (0, true)",
				msg, res, handled)
		}
		if b.plat.evt.Type != gui.EventInvalid {
			t.Errorf("msg %#x emitted %v, want no event", msg, b.plat.evt.Type)
		}
	}
}

// TestKeyDownStillEmitsOrdinaryKeys is the counterpart: the
// VK_PROCESSKEY gate must not swallow ordinary typing.
func TestKeyDownStillEmitsOrdinaryKeys(t *testing.T) {
	b := newCharTestBackend(t)
	b.plat.evt = gui.Event{Type: gui.EventInvalid}
	if _, handled := b.handleMessage(wmKeyDown, 'A', 0); !handled {
		t.Fatal("handleMessage declined WM_KEYDOWN")
	}
	if b.plat.evt.Type != gui.EventKeyDown || b.plat.evt.KeyCode != gui.KeyA {
		t.Errorf("event = {%v, %v}, want {EventKeyDown, KeyA}",
			b.plat.evt.Type, b.plat.evt.KeyCode)
	}
}

// TestIMESetRectWithoutWindowIsNoop guards the pre-creation and
// post-destroy paths: no hwnd means no IMM context, and nothing may be
// cached that would short-circuit a later, real call.
func TestIMESetRectWithoutWindowIsNoop(t *testing.T) {
	b := newCharTestBackend(t)
	n := &nativePlatform{b: b}
	n.IMESetRect(10, 20, 2, 16)
	if b.plat.imeHaveRect {
		t.Error("cached a rect that was never applied")
	}
}

// TestIMEStopClearsCachedRect asserts the caret cache does not survive a
// focus change: the next IMEStart must re-report the rect even if the
// caret happens to land in the same place.
func TestIMEStopClearsCachedRect(t *testing.T) {
	b := newCharTestBackend(t)
	b.plat.imeRect = rectW{left: 1, top: 2, right: 3, bottom: 4}
	b.plat.imeHaveRect = true

	n := &nativePlatform{b: b}
	n.IMEStop()
	if b.plat.imeHaveRect {
		t.Error("IMEStop left the caret rect cached")
	}
}
