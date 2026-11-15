package soak

import (
	"fmt"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// typedTexts is what a scripted run types: plain ASCII, accented
// Latin, emoji with a skin-tone modifier and a ZWJ sequence, Arabic
// (right to left), CJK and a combining mark. Each one has broken a
// text path in some toolkit.
var typedTexts = []string{
	"hello",
	"The quick brown fox",
	"café crème brûlée",
	"naïve façade",
	"👋🏽",
	"👩‍💻 ok",
	"مرحبا بالعالم",
	"日本語のテキスト", //nolint:gosmopolitan // CJK input is the point of the soak
	"é",
	"12345",
	" ",
}

// scriptedKeys are the keys a scripted run presses.
var scriptedKeys = []gui.KeyCode{
	gui.KeyEscape, gui.KeyTab, gui.KeyEnter, gui.KeyBackspace,
	gui.KeyDelete, gui.KeyLeft, gui.KeyRight, gui.KeyUp, gui.KeyDown,
	gui.KeyHome, gui.KeyEnd, gui.KeyPageUp, gui.KeyPageDown,
	gui.KeySpace,
}

// clickTries is how many random IDs a click tries before it gives up.
// Most IDs in a tree have no click handler or are scrolled away; a
// Test* error is the cheap way to find out.
const clickTries = 8

// scriptedAction takes one user-like action and returns the frames to
// step after it. The weights roughly follow what a person does with a
// widget gallery: mostly click, then type and keys, a little of the
// rest.
func (r *runner) scriptedAction() int {
	after := 1 + r.rng.IntN(30)
	switch p := r.rng.IntN(100); {
	case p < 40:
		r.click()
	case p < 55:
		r.typeText()
	case p < 70:
		r.pressKey()
	case p < 80:
		r.tab()
	case p < 90:
		r.scroll()
	case p < 93:
		r.resize(400+r.rng.IntN(1200), 300+r.rng.IntN(900))
	case p < 95:
		r.switchTheme()
	default:
		// The user waits: animations finish, carets park, tooltips show.
		wait := time.Duration(500+r.rng.IntN(4500)) * time.Millisecond
		r.record(func() string { return fmt.Sprintf("wait %s", wait) })
		after = int(wait / FrameDt)
	}
	return after
}

// ids returns the effective IDs of the current frame that SkipID does
// not exclude.
func (r *runner) ids() []string {
	var out []string
	// Listing IDs is the harness's cost, not the app's.
	r.excluded(func() {
		all := r.w.EffectiveIDs()
		out = all[:0]
		for _, id := range all {
			if !r.cfg.SkipID(id) {
				out = append(out, id)
			}
		}
	})
	return out
}

// pick returns a random ID of the current frame, or "" when there is
// none.
func (r *runner) pick(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[r.rng.IntN(len(ids))]
}

func (r *runner) click() {
	ids := r.ids()
	for range clickTries {
		id := r.pick(ids)
		if id == "" {
			return
		}
		if err := r.w.TestClick(id); err == nil {
			r.record(func() string { return "click " + id })
			return
		}
	}
	r.record(func() string { return "click (no target)" })
}

// target returns the focused widget when one is focused and not
// skipped, else a random one.
func (r *runner) target() string {
	var focused string
	// SkipID is the harness's cost (the showcase lowercases the ID).
	r.excluded(func() {
		if id := r.w.FocusID(); id != "" && !r.cfg.SkipID(id) {
			focused = id
		}
	})
	if focused != "" {
		return focused
	}
	return r.pick(r.ids())
}

func (r *runner) typeText() {
	id := r.target()
	text := typedTexts[r.rng.IntN(len(typedTexts))]
	err := r.w.TestType(id, text)
	r.record(func() string { return fmt.Sprintf("type %q into %s (%v)", text, id, err) })
}

func (r *runner) pressKey() {
	id := r.target()
	key := scriptedKeys[r.rng.IntN(len(scriptedKeys))]
	mods := gui.ModNone
	switch r.rng.IntN(10) {
	case 0:
		mods = gui.ModShift
	case 1:
		mods = gui.ModCtrl
	}
	err := r.w.TestKey(id, key, mods)
	r.record(func() string { return fmt.Sprintf("key %d mods %d on %s (%v)", key, mods, id, err) })
}

func (r *runner) tab() {
	dir := gui.TabForward
	if r.rng.IntN(4) == 0 {
		dir = gui.TabBackward
	}
	id, err := r.w.TestTab(dir)
	r.record(func() string { return fmt.Sprintf("tab %d -> %s (%v)", dir, id, err) })
}

func (r *runner) scroll() {
	ids := r.ids()
	for range clickTries {
		id := r.pick(ids)
		if id == "" {
			return
		}
		dy := float32(20 + r.rng.IntN(400))
		if r.rng.IntN(2) == 0 {
			dy = -dy
		}
		if err := r.w.TestScroll(id, 0, dy); err == nil {
			r.record(func() string { return fmt.Sprintf("scroll %s by %v", id, dy) })
			return
		}
	}
	r.record(func() string { return "scroll (no target)" })
}

// resize sends the event a backend sends when the user drags the
// window edge.
func (r *runner) resize(width, height int) {
	r.w.EventFn(&gui.Event{
		Type: gui.EventResized, WindowWidth: width, WindowHeight: height,
	})
	r.record(func() string { return fmt.Sprintf("resize %dx%d", width, height) })
}

// switchTheme flips the window between the dark and light themes. An
// app with a theme transition fades between them, so the fade runs on
// the manual clock too.
func (r *runner) switchTheme() {
	r.darkTheme = !r.darkTheme
	t := gui.ThemeLight
	if r.darkTheme {
		t = gui.ThemeDark
	}
	r.w.SetTheme(t)
	r.record(func() string { return fmt.Sprintf("theme dark=%v", r.darkTheme) })
}

// randomEventTypes are the raw events random mode sends. Left out:
// quit (it ends the app), file drop (it reads the file system),
// clipboard paste (it reads the host clipboard) and suspend/resume,
// which only mobile backends send.
var randomEventTypes = []gui.EventType{
	gui.EventKeyDown, gui.EventKeyUp, gui.EventChar,
	gui.EventMouseDown, gui.EventMouseUp, gui.EventMouseScroll,
	gui.EventMouseMove, gui.EventMouseEnter, gui.EventMouseLeave,
	gui.EventTouchesBegan, gui.EventTouchesMoved, gui.EventTouchesEnded,
	gui.EventTouchesCancelled, gui.EventResized, gui.EventIconified,
	gui.EventRestored, gui.EventFocused, gui.EventUnfocused,
	gui.EventIMEComposition, gui.EventScrollBegan,
}

// randomRunes are the code points random mode types, beside random
// ones: control characters, a lone surrogate (invalid as a rune),
// the largest code point, a zero-width joiner, a combining mark, a
// right-to-left mark and an emoji.
var randomRunes = []uint32{
	0, 8, 9, 10, 13, 27, 127, 0xD800, 0x10FFFF, 0x200D, 0x0301,
	0x200F, 0x1F44B, 'a', 'Z', ' ',
}

// randomEvent sends one raw event with random fields and returns the
// frames to step after it.
func (r *runner) randomEvent() int {
	width, height := r.w.WindowSize()
	e := gui.Event{
		Type:        randomEventTypes[r.rng.IntN(len(randomEventTypes))],
		MouseX:      r.coord(width),
		MouseY:      r.coord(height),
		MouseDX:     float32(r.rng.NormFloat64() * 50),
		MouseDY:     float32(r.rng.NormFloat64() * 50),
		ScrollX:     float32(r.rng.NormFloat64() * 100),
		ScrollY:     float32(r.rng.NormFloat64() * 100),
		Modifiers:   gui.Modifier(r.rng.IntN(16)),
		MouseButton: gui.MouseButton(r.rng.IntN(3)),
		KeyCode:     gui.KeyCode(r.rng.IntN(350)),
		KeyRepeat:   r.rng.IntN(5) == 0,
	}
	switch e.Type {
	case gui.EventChar:
		if r.rng.IntN(2) == 0 {
			e.CharCode = randomRunes[r.rng.IntN(len(randomRunes))]
		} else {
			e.CharCode = r.rng.Uint32N(0x110000)
		}
	case gui.EventResized:
		// Include degenerate sizes: zero, one pixel, very wide.
		sizes := []int{0, 1, 50, 320, 800, 1280, 2560}
		e.WindowWidth = sizes[r.rng.IntN(len(sizes))]
		e.WindowHeight = sizes[r.rng.IntN(len(sizes))]
	case gui.EventIMEComposition:
		e.IMEText = typedTexts[r.rng.IntN(len(typedTexts))]
		e.IMEStart = int32(r.rng.IntN(8)) - 2
		e.IMELength = int32(r.rng.IntN(8)) - 2
	case gui.EventTouchesBegan, gui.EventTouchesMoved,
		gui.EventTouchesEnded, gui.EventTouchesCancelled:
		e.NumTouches = r.rng.IntN(len(e.Touches) + 1)
		for i := range e.NumTouches {
			e.Touches[i] = gui.TouchPoint{
				Identifier: uint64(r.rng.IntN(4)),
				PosX:       r.coord(width),
				PosY:       r.coord(height),
				Changed:    r.rng.IntN(2) == 0,
			}
		}
	}
	r.record(func() string {
		return fmt.Sprintf("event type=%d key=%d char=%#x at %.0f,%.0f size %dx%d",
			e.Type, e.KeyCode, e.CharCode, e.MouseX, e.MouseY, e.WindowWidth, e.WindowHeight)
	})
	r.w.EventFn(&e)
	return 1 + r.rng.IntN(5)
}

// coord returns a random coordinate on an axis of the given size,
// sometimes a little outside the window, as a drag past the edge does.
func (r *runner) coord(size int) float32 {
	return float32(r.rng.IntN(size+200) - 100)
}
