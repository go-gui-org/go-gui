package gui

import (
	"strings"
	"time"

	"github.com/go-gui-org/go-glyph"
)

// inputTextFromLayout extracts the current text from the input's
// inner layout structure (Column → Row → Text).
func inputTextFromLayout(layout *Layout) string {
	if len(layout.Children) == 0 {
		return ""
	}
	row := &layout.Children[0]
	if len(row.Children) == 0 {
		return ""
	}
	txt := &row.Children[0]
	if txt.Shape.TC == nil {
		return ""
	}
	if txt.Shape.TC.TextIsPlaceholder {
		return ""
	}
	return txt.Shape.TC.Text
}

// inputGlyphLayoutFor navigates to the inner text shape of an
// input layout and returns a glyph Layout for cursor navigation.
func inputGlyphLayoutFor(layout *Layout, w *Window) (glyph.Layout, bool) {
	return inputGlyphLayoutWithText(
		inputTextFromLayout(layout), layout, w,
	)
}

// inputGlyphLayoutWithText returns a glyph Layout using
// pre-extracted text, avoiding redundant layout traversal.
func inputGlyphLayoutWithText(
	text string, layout *Layout, w *Window,
) (glyph.Layout, bool) {
	if w.textMeasurer == nil {
		return glyph.Layout{}, false
	}
	if len(layout.Children) == 0 {
		return glyph.Layout{}, false
	}
	row := &layout.Children[0]
	if len(row.Children) == 0 {
		return glyph.Layout{}, false
	}
	txt := &row.Children[0]
	if txt.Shape == nil || txt.Shape.TC == nil {
		return glyph.Layout{}, false
	}
	style := textStyleOrDefault(txt.Shape)
	return inputGlyphLayout(text, txt.Shape, style, w)
}

// trailingLineStart returns the start of the previous visual line
// when byteIdx is at a soft-wrap boundary and CursorTrailing is set.
// Falls back to fallback if no boundary match is found.
func trailingLineStart(lines []glyph.Line, byteIdx, fallback int) int {
	for i, line := range lines {
		if i > 0 && byteIdx == line.StartIndex {
			return lines[i-1].StartIndex
		}
	}
	return fallback
}

// trailingLineEnd returns the end of the previous visual line
// when byteIdx is at a soft-wrap boundary and CursorTrailing is set.
// Falls back to fallback if no boundary match is found.
func trailingLineEnd(lines []glyph.Line, byteIdx, fallback int) int {
	for i, line := range lines {
		if i > 0 && byteIdx == line.StartIndex {
			return lines[i-1].StartIndex + lines[i-1].Length
		}
	}
	return fallback
}

// inputDeleteGrapheme deletes a grapheme cluster at cursor using
// glyph when available, falling back to rune-based inputDelete.
func inputDeleteGrapheme(
	text string, focusID string, forward bool,
	layout *Layout, w *Window,
) (string, bool) {
	gl, glOK := inputGlyphLayoutFor(layout, w)
	if !glOK {
		newText, _ := inputDelete(text, focusID, forward, w)
		return newText, newText != text
	}
	is := inputStateOrDefault(focusID, w)
	if is.SelectBeg != is.SelectEnd {
		newText, _ := inputDelete(text, focusID, forward, w)
		return newText, newText != text
	}
	pos := min(is.CursorPos, utf8RuneCount(text))
	byteIdx := runeToByteIndex(text, pos)
	var res glyph.MutationResult
	if forward {
		res = glyph.DeleteForward(text, gl, byteIdx)
	} else {
		res = glyph.DeleteBackward(text, gl, byteIdx)
	}
	if res.NewText == text {
		return text, false
	}
	newPos := byteToRuneIndex(res.NewText, res.CursorPos)
	undo := inputPushUndo(is, text)
	imap := StateMap[string, InputState](w, nsInput, capMany)
	imap.Set(focusID, InputState{
		CursorPos:    newPos,
		CursorOffset: -1,
		Undo:         undo,
	})
	return res.NewText, true
}

// passwordMask replaces each rune with a bullet character.
// Uses a stack-local buffer for short strings to avoid heap
// allocation from strings.Repeat.
func passwordMask(text string) string {
	n := utf8RuneCount(text)
	// "•" (U+2022) = 3 bytes UTF-8: 0xE2 0x80 0xA2
	const bLen = 3
	if n <= 64 {
		var buf [64 * bLen]byte
		for i := range n {
			buf[i*bLen] = 0xe2
			buf[i*bLen+1] = 0x80
			buf[i*bLen+2] = 0xa2
		}
		return string(buf[:n*bLen])
	}
	return strings.Repeat("•", n)
}

// inputDragState holds state for drag-to-select in an input.
// Replaces ~10 closure-captured locals with explicit fields;
// runes is non-nil iff the drag started from a double-click.
type inputDragState struct {
	displayText            string
	runes                  []rune // non-nil = double-click word select
	gl                     glyph.Layout
	anchorPos, anchorEnd   uint32
	txtOffX, txtOffY       float32
	focusID                string
	scrollID               string
	lastMouseX, lastMouseY float32
	scrollY0               float32
	viewTop, viewBot       float32
	maxScrollNeg           float32
}

func (d *inputDragState) computeRunePos(
	mx, my float32, w *Window,
) int {
	scrollDelta := float32(0)
	if d.scrollID != "" {
		sy := w.scrollY()
		// Default 0: absent entry means unscrolled initial position.
		sNow := sy.GetOr(d.scrollID, 0)
		scrollDelta = sNow - d.scrollY0
	}
	relX := mx - d.txtOffX
	relY := my - (d.txtOffY + scrollDelta)
	byteIdx := d.gl.GetClosestOffset(relX, relY)
	return byteToRuneIndex(d.displayText, byteIdx)
}

func (d *inputDragState) updateSelection(rp int, w *Window) {
	imap := StateMap[string, InputState](w, nsInput, capMany)
	// Default InputState{}: zero value seeds initial drag-selection state.
	is := imap.GetOr(d.focusID, InputState{})
	if d.runes != nil {
		wb, we := wordBoundsAt(d.runes, rp)
		if rp < int(d.anchorPos) {
			is.SelectBeg = d.anchorEnd
			is.SelectEnd = uint32(wb)
			is.CursorPos = wb
		} else {
			is.SelectBeg = d.anchorPos
			is.SelectEnd = uint32(we)
			is.CursorPos = we
		}
	} else {
		is.CursorPos = rp
		is.SelectBeg = d.anchorPos
		is.SelectEnd = uint32(rp)
	}
	is.CursorOffset = -1
	imap.Set(d.focusID, is)
	resetBlinkCursorVisible(w)
}

func (d *inputDragState) scrollCallback(
	_ *Animate, w *Window,
) {
	var delta float32
	if d.lastMouseY < d.viewTop {
		delta = (d.viewTop - d.lastMouseY) * 0.3
	} else if d.lastMouseY > d.viewBot {
		delta = -((d.lastMouseY - d.viewBot) * 0.3)
	} else {
		w.AnimationRemove(animIDDragScroll)
		return
	}
	sy := w.scrollY()
	// Default 0: unscrolled position when no offset recorded yet.
	cur := sy.GetOr(d.scrollID, 0)
	newScroll := f32Clamp(cur+delta, d.maxScrollNeg, 0)
	if newScroll == cur {
		return
	}
	sy.Set(d.scrollID, newScroll)
	rp := d.computeRunePos(d.lastMouseX, d.lastMouseY, w)
	d.updateSelection(rp, w)
}

// startInputDrag sets up MouseLock drag-to-select for an input.
func startInputDrag(d *inputDragState, w *Window) {
	w.MouseLock(MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			d.lastMouseX = e.MouseX
			d.lastMouseY = e.MouseY
			rp := d.computeRunePos(e.MouseX, e.MouseY, w)
			d.updateSelection(rp, w)
			if d.scrollID != "" {
				outside := e.MouseY < d.viewTop ||
					e.MouseY > d.viewBot
				if outside && !w.HasAnimation(
					animIDDragScroll) {
					w.AnimationAdd(&Animate{
						AnimID:   animIDDragScroll,
						Delay:    32 * time.Millisecond,
						Repeat:   true,
						Refresh:  AnimationRefreshLayout,
						Callback: d.scrollCallback,
					})
				} else if !outside {
					w.AnimationRemove(animIDDragScroll)
				}
			}
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			w.AnimationRemove(animIDDragScroll)
			w.MouseUnlock()
		},
	})
}
