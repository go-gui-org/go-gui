package gui

// textHinter is an optional capability on TextMeasurer backends. Backends
// that wrap a *glyph.TextSystem forward it to TextSystem.SetHinting.
type textHinter interface {
	SetHinting(on bool)
}

// SetTextHinting turns stem snapping of glyph outlines on or off for this
// window (#1334). When it is on, vertical stems move to whole pixels, so
// they show as solid columns. Hinting is off by default: glyphs keep their
// exact outline, so the text looks softer but more uniform. The change
// clears the glyph cache and redraws the window. It has no effect on the
// web backend.
//
// Call it on the main goroutine (OnInit, an event callback). From another
// goroutine, use QueueCommand. WindowCfg.EnableTextHinting sets the start
// value.
//
// exportaudit:keep — caller-facing setting (issue #1334)
func (w *Window) SetTextHinting(on bool) {
	if w == nil {
		return
	}
	w.textHintingOn = on
	w.applyTextHinting()
	w.InvalidateLayout()
}

// applyTextHinting gives the window's hinting choice to the installed
// measurer. It is called on every install, also when hinting is off, so a
// text system that an earlier window turned on, or that keeps the go-glyph
// default (on in v1.28.0), is reset. glyph does
// nothing when the value does not change.
func (w *Window) applyTextHinting() {
	if th, ok := w.textMeasurer.(textHinter); ok {
		th.SetHinting(w.textHintingOn)
	}
}
