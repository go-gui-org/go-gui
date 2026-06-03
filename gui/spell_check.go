package gui

import (
	"strconv"
	"time"

	"github.com/go-gui-org/go-glyph"
)

// spellCheckState caches spell check results for an input.
type spellCheckState struct {
	Text   string       // text that was checked
	Ranges []SpellRange // misspelled byte ranges
}

const spellCheckDelay = 300 * time.Millisecond

func spellCheckAnimID(idFocus uint32) string {
	return "spell-check-" + strconv.FormatUint(uint64(idFocus), 10)
}

// spellCheckTrigger schedules a debounced spell check for the given
// input. Cancels any pending timer and schedules a new one. Called
// from AmendLayout (under w.mu).
func spellCheckTrigger(idFocus uint32, text string, w *Window) {
	if w.nativePlatform == nil {
		return
	}
	sm := StateMap[uint32, spellCheckState](
		w, nsSpellCheck, capMany)
	cached, _ := sm.Get(idFocus)
	if cached.Text == text {
		return
	}

	animID := spellCheckAnimID(idFocus)
	// Cancel previous pending timer.
	delete(w.animations, animID)

	// Store pending state so subsequent AmendLayout calls see
	// the text match and skip re-scheduling. Ranges are nil
	// until the callback populates them.
	sm.Set(idFocus, spellCheckState{Text: text})

	capturedText := text
	w.animationAdd(&Animate{
		AnimID: animID,
		Delay:  spellCheckDelay,
		Callback: func(_ *Animate, w *Window) {
			if w.nativePlatform == nil {
				return
			}
			ranges := w.nativePlatform.SpellCheck(capturedText)
			sm := StateMap[uint32, spellCheckState](
				w, nsSpellCheck, capMany)
			sm.Set(idFocus, spellCheckState{
				Text:   capturedText,
				Ranges: ranges,
			})
		},
	})
}

// spellCheckClear removes cached spell state for an input.
func spellCheckClear(idFocus uint32, w *Window) {
	sm := StateMapRead[uint32, spellCheckState](w, nsSpellCheck)
	if sm != nil {
		sm.Delete(idFocus)
	}
	delete(w.animations, spellCheckAnimID(idFocus))
}

// spellCheckHasRanges returns true if completed spell check results
// exist for the given input. Used by the render path to ensure a
// glyph layout is computed for underline positioning.
func spellCheckHasRanges(idFocus uint32, w *Window) bool {
	sm := StateMapRead[uint32, spellCheckState](w, nsSpellCheck)
	if sm == nil {
		return false
	}
	state, ok := sm.Get(idFocus)
	return ok && len(state.Ranges) > 0
}

// renderSpellCheckUnderlines draws red underlines beneath
// misspelled words. Called from renderLayoutText after IME
// preedit underlines.
func renderSpellCheckUnderlines(
	shape *Shape, text string,
	baseX, baseY float32,
	gl glyph.Layout,
	w *Window,
) {
	if shape.IDFocus == 0 {
		return
	}
	sm := StateMapRead[uint32, spellCheckState](w, nsSpellCheck)
	if sm == nil {
		return
	}
	state, ok := sm.Get(shape.IDFocus)
	if !ok || len(state.Ranges) == 0 {
		return
	}
	// Only render if cached text matches current text.
	if state.Text != text {
		return
	}

	color := DefaultInputStyle.ColorSpellError
	underlineH := max(float32(1.5), shape.TC.TextStyle.Size/10)

	for _, r := range state.Ranges {
		endByte := r.StartByte + r.LenBytes
		if endByte > len(text) {
			continue
		}
		rects := gl.GetSelectionRects(r.StartByte, endByte)
		for _, rect := range rects {
			emitRenderer(RenderCmd{
				Kind:  RenderRect,
				X:     baseX + rect.X,
				Y:     baseY + rect.Y + rect.Height - underlineH,
				W:     rect.Width,
				H:     underlineH,
				Color: color,
				Fill:  true,
			}, w)
		}
	}
}
