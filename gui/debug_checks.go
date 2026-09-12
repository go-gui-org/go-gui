package gui

import "strconv"

// The internal check identifiers and their mapping to the public
// categories that gate them. Split from debug.go, which holds the
// gate itself, the per-window warn-once state and the frame audit.

// debugCheck identifies one class of finding. Warn-once state is
// keyed by (check, subject), so a window reports each distinct defect
// once rather than at the frame rate.
type debugCheck uint8

const (
	debugCheckDupID debugCheck = iota
	debugCheckFocusNoID
	debugCheckScrollNoID
	debugCheckMouseLeaveNoID
	// debugCheckUnconsumed is the only check that runs from dispatch
	// rather than from the per-frame layout audit; see debug_event.go.
	debugCheckUnconsumed
	// debugCheckListBoxNoHeight fires from the listbox view phase
	// rather than from the layout audit; see listBoxVisibleRange.
	debugCheckListBoxNoHeight
	// debugCheckListHeightsCapped fires from the list height registry
	// when a variable-height list is too large for per-item storage.
	debugCheckListHeightsCapped
	// debugCheckListWidthRatchet fires from the VirtualList measurement
	// hook when the list widens frame after frame with the window
	// standing still; see virtualListNoteWidth.
	debugCheckListWidthRatchet
	// debugCheckUnscopedID reports an identity that has no ID-bearing
	// ancestor, so it is still a window-global name.
	debugCheckUnscopedID
	// debugCheckGradientResampled fires from the GPU backends' draw
	// pass when a fill gradient has more stops than the shader uniform
	// layout can carry.
	debugCheckGradientResampled
	// debugCheckWrapOverflow fires from layoutOverflow when a container
	// sets both Wrap and Overflow; wrap wins and overflow is ignored.
	debugCheckWrapOverflow
	// debugCheckDeferredLoop fires from flushDeferredCallbacks when
	// deferred app callbacks keep re-queueing past the batch bound.
	debugCheckDeferredLoop
	// debugCheckLinkNotOpened fires from rtfOpenLink when a link the
	// user activated does nothing: an unresolved anchor, a relative
	// reference with no base URI, or a platform opener that failed.
	debugCheckLinkNotOpened
	// debugCheckWindowTransparency fires from a backend's window
	// creation when WindowCfg.Transparent could not be honoured.
	debugCheckWindowTransparency
	// debugCheckWindowOpacity fires from a backend when
	// Window.SetWindowOpacity could not be honoured.
	debugCheckWindowOpacity
	// debugCheckUnresolvedKey fires from the state-key audit when a
	// StateMap key is a bare leaf that the resolve pass scoped; see
	// debug_state_keys.go.
	debugCheckUnresolvedKey
	// debugCheckEffIDPhase fires from (*Window).EffID when it is called
	// outside layout generation, where the ID scope is empty and the
	// call cannot do its job.
	debugCheckEffIDPhase
	// debugCheckStampDrift fires from resolveFocusOwners when a shape's
	// stamped identity does not match the scope it was arranged under,
	// which is what a shape that skipped layout generation looks like.
	debugCheckStampDrift
	// debugCheckUnknownFocus fires from the frame audit when the
	// window's focus ID names no focusable shape in the frame.
	debugCheckUnknownFocus
	// debugCheckTextAnimNoID fires from applyTextAnim when a TextCfg
	// asks for an animation but carries no ID; the animation and its
	// progress are keyed by identity, so there is nothing to key on.
	debugCheckTextAnimNoID
	// debugCheckGlyphLayoutFallback fires from plainTextLayoutResolved
	// when the shaper refuses a text, so the frame falls back to
	// approximate metrics with degraded caret, selection and delete
	// precision.
	debugCheckGlyphLayoutFallback
	// debugCheckTextTruncated fires from textView.GenerateLayout when
	// app-supplied input text exceeds the rune budget and is
	// truncated to it, so the frame never shapes unbounded content.
	debugCheckTextTruncated
)

// checkCategory maps an internal check to the public category that
// gates it. debugWarn consults this so every finding site stays behind
// the mask even when reached outside the gated entry points.
func checkCategory(check debugCheck) DebugCategory {
	switch check {
	case debugCheckDupID:
		return DebugDuplicates
	case debugCheckFocusNoID, debugCheckScrollNoID,
		debugCheckMouseLeaveNoID, debugCheckTextAnimNoID:
		return DebugMissingIDs
	case debugCheckUnconsumed:
		return DebugUnconsumed
	case debugCheckListBoxNoHeight, debugCheckListHeightsCapped,
		debugCheckListWidthRatchet:
		return DebugListBoxNoHeight
	case debugCheckUnscopedID:
		return DebugUnscopedIDs
	case debugCheckGradientResampled:
		return DebugGradientResampled
	case debugCheckWrapOverflow:
		return DebugWrapOverflow
	case debugCheckDeferredLoop, debugCheckLinkNotOpened:
		return DebugCallbacks
	case debugCheckWindowTransparency, debugCheckWindowOpacity:
		return DebugWindowDegraded
	case debugCheckUnresolvedKey, debugCheckEffIDPhase:
		return DebugUnresolvedKeys
	case debugCheckStampDrift:
		return DebugStampDrift
	case debugCheckUnknownFocus:
		return DebugUnknownFocus
	case debugCheckGlyphLayoutFallback, debugCheckTextTruncated:
		return DebugGlyphLayoutFallback
	default:
		panic("gui: checkCategory has no category for debugCheck " +
			strconv.Itoa(int(check)))
	}
}
