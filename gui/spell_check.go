package gui

import (
	"time"

	"github.com/go-gui-org/go-glyph"
)

// spellCheckState caches spell check results for an input.
type spellCheckState struct {
	Text   string       // text that was checked
	Ranges []SpellRange // misspelled byte ranges
}

const spellCheckDelay = 300 * time.Millisecond

func spellCheckAnimID(focusID string) string {
	return "spell-check-" + focusID
}

// spellCheckTrigger schedules a debounced spell check for the given
// input. Cancels any pending timer and schedules a new one. Called
// from AmendLayout (under w.mu).
func spellCheckTrigger(focusID string, text string, w *Window) {
	if w.nativePlatform == nil {
		return
	}
	sm := StateMap[string, spellCheckState](
		w, nsSpellCheck, capMany)
	// Default zero state: absent entry means no cached spell result yet.
	cached := sm.GetOr(focusID, spellCheckState{})
	if cached.Text == text {
		return
	}

	animID := spellCheckAnimID(focusID)
	// Cancel previous pending timer. Must hold animMu — spellCheckTrigger
	// runs under w.mu from AmendLayout, but w.animations is guarded by
	// w.animMu (read by the animation goroutine).
	w.animMu.Lock()
	delete(w.animations, animID)
	w.animMu.Unlock()

	// Store pending state so subsequent AmendLayout calls see
	// the text match and skip re-scheduling. Ranges are nil
	// until the callback populates them.
	sm.Set(focusID, spellCheckState{Text: text})

	capturedText := text
	w.AnimationAdd(&Animate{
		AnimID: animID,
		Delay:  spellCheckDelay,
		Callback: func(_ *Animate, w *Window) {
			if w.nativePlatform == nil {
				return
			}
			ranges := w.nativePlatform.SpellCheck(capturedText)
			sm := StateMap[string, spellCheckState](
				w, nsSpellCheck, capMany)
			sm.Set(focusID, spellCheckState{
				Text:   capturedText,
				Ranges: ranges,
			})
		},
	})
}

// spellCheckClear removes cached spell state for an input.
// Called from AmendLayout (under w.mu). Acquires w.animMu to
// safely delete from w.animations, which the animation goroutine
// reads under w.animMu.
func spellCheckClear(focusID string, w *Window) {
	sm := StateMapRead[string, spellCheckState](w, nsSpellCheck)
	if sm != nil {
		sm.Delete(focusID)
	}
	w.animMu.Lock()
	delete(w.animations, spellCheckAnimID(focusID))
	w.animMu.Unlock()
}

// spellCheckHasRanges returns true if completed spell check results
// exist for the given input. Used by the render path to ensure a
// glyph layout is computed for underline positioning.
func spellCheckHasRanges(focusID string, w *Window) bool {
	sm := StateMapRead[string, spellCheckState](w, nsSpellCheck)
	if sm == nil {
		return false
	}
	state, ok := sm.Get(focusID)
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
	if shape.ID == "" || !shape.Focusable {
		return
	}
	sm := StateMapRead[string, spellCheckState](w, nsSpellCheck)
	if sm == nil {
		return
	}
	state, ok := sm.Get(shape.ID)
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
