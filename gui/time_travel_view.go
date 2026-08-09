package gui

import (
	"fmt"
)

// TimeTravelController drives the time-travel scrubber UI. Holds
// a reference to the app window being debugged and a cursor into
// its history ring. All motion methods auto-freeze the app
// window; ResumeLive releases the freeze.
//
// Safe to construct without history or with a nil app window —
// methods are no-ops in those cases so the UI renders gracefully
// before state exists.
type timeTravelController struct {
	// App is the window being debugged. Nil is allowed; methods
	// become no-ops.
	App *Window
	// Cursor indexes into App.history (0-based, 0 = oldest).
	// Out-of-range values are clamped by Jump.
	Cursor int
	// sliderValue holds the fractional thumb position so mid-
	// drag frames display the live mouse position instead of
	// snapping to the integer Cursor. Synced to Cursor by
	// keyboard/button moves and by the commit path inside the
	// slider's OnChange.
	sliderValue float32
}

// hist returns the app window's history ring, or nil if the
// controller, app, or history is unset. Single nil-guard used
// by every motion method.
func (c *timeTravelController) hist() *snapshotRing {
	if c == nil || c.App == nil {
		return nil
	}
	return c.App.history
}

// Len returns the current history length.
func (c *timeTravelController) Len() int {
	r := c.hist()
	if r == nil {
		return 0
	}
	return r.len()
}

// Bytes returns the current history byte usage.
func (c *timeTravelController) Bytes() int {
	r := c.hist()
	if r == nil {
		return 0
	}
	return r.bytes()
}

// Cause returns the cause label of the entry at the cursor.
func (c *timeTravelController) cause() string {
	r := c.hist()
	if r == nil {
		return ""
	}
	e, ok := r.at(c.Cursor)
	if !ok {
		return ""
	}
	return e.cause
}

// Jump moves the cursor to idx, clamped to [0, len-1]. Auto-
// freezes the app window and posts a restore request.
func (c *timeTravelController) jump(idx int) {
	r := c.hist()
	if r == nil {
		return
	}
	n := r.len()
	if n == 0 {
		return
	}
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	c.Cursor = idx
	c.sliderValue = float32(idx)
	c.App.Freeze()
	c.App.postRestore(idx)
}

// StepBack moves one entry toward the past. Clamps at 0.
func (c *timeTravelController) stepBack() {
	if c == nil {
		return
	}
	c.jump(c.Cursor - 1)
}

// StepForward moves one entry toward the present. Clamps at len-1.
func (c *timeTravelController) stepForward() {
	if c == nil {
		return
	}
	c.jump(c.Cursor + 1)
}

// First jumps to the oldest entry.
func (c *timeTravelController) First() {
	c.jump(0)
}

// Last jumps to the newest entry.
func (c *timeTravelController) Last() {
	r := c.hist()
	if r == nil {
		return
	}
	c.jump(r.len() - 1)
}

// ResumeLive releases the app window's freeze and snaps the
// cursor to the newest entry. The app resumes accepting input
// and w.Now() returns live time.
func (c *timeTravelController) resumeLive() {
	if c == nil || c.App == nil {
		return
	}
	c.App.resume()
	if r := c.hist(); r != nil {
		if n := r.len(); n > 0 {
			c.Cursor = n - 1
		}
	}
}

// ToggleFreeze flips the frozen state. Intended for a freeze
// button or Space-key shortcut.
func (c *timeTravelController) toggleFreeze() {
	if c == nil || c.App == nil {
		return
	}
	if c.App.isFrozen() {
		c.resumeLive()
	} else {
		c.App.Freeze()
	}
}

// View returns the debug scrubber UI composed from the
// controller's current state. Requires a non-nil host Window
// (the window the view renders into) for sizing. Typically
// called from a view generator installed on a dedicated debug
// window; host must not be nil.
func (c *timeTravelController) View(w *Window) View {
	if w == nil {
		return Column(ContainerCfg{})
	}
	n := c.Len()
	bytes := c.Bytes()
	displayIdx := c.Cursor + 1
	if n == 0 {
		displayIdx = 0
	}
	counter := fmt.Sprintf("%d / %d  (%d KiB)", displayIdx, n, bytes>>10)
	cause := c.cause()
	if cause == "" {
		cause = "(empty)"
	}

	sliderMax := float32(0)
	if n > 1 {
		sliderMax = float32(n - 1)
	}

	ww, wh := w.WindowSize()
	return Column(ContainerCfg{
		ID:        ttDebugFocusID,
		Focusable: true,
		Width:     float32(ww),
		Height:    float32(wh),
		OnKeyDown: c.handleKey,
		Sizing:    FixedFixed,
		HAlign:    HAlignCenter,
		VAlign:    VAlignMiddle,
		Spacing:   SomeF(10),
		Padding:   Some(PadAll(12)),
		Content: []View{
			Text(TextCfg{Text: counter}),
			Text(TextCfg{Text: cause}),
			Slider(SliderCfg{
				ID:       ttSliderID,
				Min:      0,
				Max:      sliderMax,
				Value:    c.sliderValue,
				Height:   ttSliderHeight,
				Sizing:   FillFixed,
				OnChange: c.onSliderChange,
			}),
			ttButton(ttFreezeID, freezeLabel(c.App), c.toggleFreeze),
		},
	})
}

// onSliderChange commits the slider's new value as an integer
// cursor jump and pins the fractional v as sliderValue so the
// thumb tracks the mouse between renders. Rejects NaN and ±Inf
// before the int conversion (implementation-defined garbage);
// clamps v against the valid cursor range so an out-of-bounds
// value from a misbehaving slider backend can't render the
// thumb past the track ends on the next frame.
func (c *timeTravelController) onSliderChange(v float32, _ EventCtx) {
	if c == nil || !f32IsFinite(v) {
		return
	}
	n := c.Len()
	if n <= 0 {
		return
	}
	v = f32Clamp(v, 0, float32(n-1))
	// Only re-commit when the integer bucket actually changes;
	// sub-pixel drag still updates sliderValue so the thumb
	// tracks the mouse.
	if idx := int(v); idx != c.Cursor {
		c.jump(idx)
	}
	c.sliderValue = v
}

// ttSliderHeight pins the track thickness so the Column's
// Fill sizing doesn't stretch it vertically; ttSliderID keys
// the slider's per-window press-state entry.
const (
	ttSliderHeight = 20
	ttSliderID     = "gui.time_travel.slider"
	ttFreezeID     = "gui.time_travel.freeze"
)

// ttDebugFocusID is the focus ID for the debug window's root
// container.
const ttDebugFocusID = "__tt_debug__"

// handleKey maps scrubber keyboard shortcuts to controller
// actions. Called from the root container's OnKeyDown.
func (c *timeTravelController) handleKey(ctx EventCtx) {
	if c == nil || ctx.Event == nil {
		return
	}
	switch ctx.Event.KeyCode {
	case KeyLeft:
		c.stepBack()
	case KeyRight:
		c.stepForward()
	case KeyHome:
		c.First()
	case KeyEnd:
		c.Last()
	case KeySpace:
		c.toggleFreeze()
	case KeyEscape:
		c.resumeLive()
	default:
		return
	}
	// OnKeyDown is notify-class, so the shortcut must consume
	// explicitly to keep the key from bubbling.
	ctx.Consume()
}

// ttButton builds a labelled scrubber button whose click
// handler invokes the given zero-arg function.
//
// id is a parameter rather than derived from label because the
// label is not stable: the freeze button reads "Pause" or
// "Resume" depending on state, and an ID that changed with it
// would move the widget's focus and state identity mid-session.
func ttButton(id, label string, fn func()) View {
	return Button(ButtonCfg{
		ID:      id,
		Content: []View{Text(TextCfg{Text: label})},
		OnClick: func(ctx EventCtx) { fn() },
	})
}

// freezeLabel returns the text for the freeze-toggle button.
// "Resume" when frozen (click → unfreeze + snap to newest),
// "Pause" when live (click → freeze at current moment).
func freezeLabel(w *Window) string {
	if w != nil && w.isFrozen() {
		return "Resume"
	}
	return "Pause"
}
