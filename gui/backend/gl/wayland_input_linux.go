//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"log"
	"time"

	"golang.org/x/sys/unix"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
	"github.com/go-gui-org/go-gui/gui/backend/internal/x11key"
	"github.com/go-gui-org/go-gui/gui/backend/internal/xkb"
)

// Wayland input (#919 phase 5): one wl_seat shared by every window, with
// its pointer, keyboard and touch devices.
//
// Each device sends its events to whichever surface has its focus (the
// enter event names it), so the seat keeps the focused Backend per device
// and looks windows up by surface in wlDisplay.wins.
//
// Keyboard events carry evdev key codes. libxkbcommon (package xkb) turns
// them into X11 keysyms and an X11 modifier mask, so the rest is the X11
// path: x11key maps keys and modifiers, and the X11 compose machine handles
// dead keys. The compositor does not repeat keys; the client does, at the
// rate wl_keyboard.repeat_info gives, timed by the run loop.

// evdev pointer buttons (linux/input-event-codes.h).
const (
	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112
)

// X11 button state bits, so x11key.MapModifiers reports held buttons as
// it does on X11.
const (
	x11MaskButton1 = 1 << 8
	x11MaskButton2 = 1 << 9
	x11MaskButton3 = 1 << 10
)

// wlButton maps an evdev button to a gui button and its X11 state bit. ok
// is false for buttons gui has no name for (side buttons).
func wlButton(button uint32) (gui.MouseButton, uint16, bool) {
	switch button {
	case btnLeft:
		return gui.MouseLeft, x11MaskButton1, true
	case btnMiddle:
		return gui.MouseMiddle, x11MaskButton2, true
	case btnRight:
		return gui.MouseRight, x11MaskButton3, true
	}
	return 0, 0, false
}

// wlSeat is the input state of the one seat the backend uses. A second
// seat (rare outside multi-seat setups) is ignored. Main thread only.
type wlSeat struct {
	d    *wlDisplay
	seat wl.Seat

	pointer  wl.Pointer
	keyboard wl.Keyboard
	touch    wl.Touch

	// Decoders built once; a device that comes back after being
	// unplugged reuses them.
	pointerDisp  wl.PointerDispatcher
	keyboardDisp wl.KeyboardDispatcher
	touchDisp    wl.TouchDispatcher

	// Pointer state. ptrFocus is the window under the pointer, nil when
	// none of ours is.
	ptrFocus   *Backend
	ptrX, ptrY float32 // logical, surface-relative
	buttons    uint16  // X11 button bits held
	axis       wlAxisAcc

	// Keyboard state. keymap is nil until the compositor sends one, and
	// keys are dropped until then. mods is the X11 modifier mask.
	keymap  *xkb.Keymap
	kbFocus *Backend
	mods    uint16
	repeat  wlRepeat

	touches wlTouches
	// touchOut holds the events one touch frame produces, kept here so a
	// frame allocates nothing.
	touchOut [3]gui.Event
}

// newWlSeat binds the seat global. Devices arrive with the capabilities
// event.
func newWlSeat(d *wlDisplay, name, version uint32) *wlSeat {
	s := &wlSeat{d: d, repeat: wlRepeat{rate: 25, delay: 600}}
	s.seat = wl.Seat{Proxy: d.registry.Bind(name, &wl.SeatInterface,
		min(version, wl.SeatInterface.Version()))}
	s.pointerDisp = wl.PointerHandlers{
		Enter:        s.pointerEnter,
		Leave:        s.pointerLeave,
		Motion:       s.pointerMotion,
		Button:       s.pointerButton,
		Axis:         s.pointerAxis,
		Frame:        s.pointerFrame,
		AxisDiscrete: func(axis uint32, discrete int32) { s.axis.addValue120(axis, discrete*120) },
		AxisValue120: s.axis.addValue120,
	}.Dispatcher()
	s.keyboardDisp = wl.KeyboardHandlers{
		Keymap:     s.keyboardKeymap,
		Enter:      s.keyboardEnter,
		Leave:      s.keyboardLeave,
		Key:        s.keyboardKey,
		Modifiers:  s.keyboardModifiers,
		RepeatInfo: s.repeat.setInfo,
	}.Dispatcher()
	s.touchDisp = wl.TouchHandlers{
		Down:   s.touchDown,
		Up:     func(_, _ uint32, id int32) { s.touches.up(id) },
		Motion: func(_ uint32, id int32, x, y wl.Fixed) { s.touches.motion(id, x, y) },
		Frame:  s.touchFrame,
		Cancel: s.touchCancel,
	}.Dispatcher()
	s.seat.SetHandlers(wl.SeatHandlers{Capabilities: s.capabilities})
	return s
}

// capabilities gets or releases each device as the seat gains or loses it.
func (s *wlSeat) capabilities(caps uint32) {
	has := caps&wl.SeatCapabilityPointer != 0
	switch {
	case has && !s.pointer.Valid():
		s.pointer = s.seat.GetPointer()
		s.pointer.SetDispatcher(s.pointerDisp)
	case !has && s.pointer.Valid():
		s.releasePointer()
	}
	has = caps&wl.SeatCapabilityKeyboard != 0
	switch {
	case has && !s.keyboard.Valid():
		s.keyboard = s.seat.GetKeyboard()
		s.keyboard.SetDispatcher(s.keyboardDisp)
	case !has && s.keyboard.Valid():
		s.releaseKeyboard()
	}
	has = caps&wl.SeatCapabilityTouch != 0
	switch {
	case has && !s.touch.Valid():
		s.touch = s.seat.GetTouch()
		s.touch.SetDispatcher(s.touchDisp)
	case !has && s.touch.Valid():
		s.releaseTouch()
	}
}

// release{Pointer,Keyboard,Touch} use the release request where the bound
// version has it (3), so the compositor frees its side too.
func (s *wlSeat) releasePointer() {
	if s.pointer.Version() >= 3 {
		s.pointer.Release()
	} else {
		s.pointer.DestroyProxy()
	}
	s.pointer = wl.Pointer{}
	s.ptrFocus, s.buttons, s.axis = nil, 0, wlAxisAcc{}
}

func (s *wlSeat) releaseKeyboard() {
	if s.keyboard.Version() >= 3 {
		s.keyboard.Release()
	} else {
		s.keyboard.DestroyProxy()
	}
	s.keyboard = wl.Keyboard{}
	s.clearKeyboard()
}

// clearKeyboard drops the keyboard state when the device goes away. An open
// dead-key sequence ends with it, as on a focus change (keyboardLeave), so
// it cannot compose with the first key of the next keyboard.
func (s *wlSeat) clearKeyboard() {
	if b := s.kbFocus; b != nil {
		b.plat.compose.reset()
	}
	s.kbFocus, s.mods = nil, 0
	s.repeat.stop()
	s.keymap.Destroy()
	s.keymap = nil
}

func (s *wlSeat) releaseTouch() {
	if s.touch.Version() >= 3 {
		s.touch.Release()
	} else {
		s.touch.DestroyProxy()
	}
	s.touch = wl.Touch{}
	s.touches = wlTouches{}
}

// destroy releases the devices and the seat. The display calls it before
// it disconnects.
func (s *wlSeat) destroy() {
	if s.pointer.Valid() {
		s.releasePointer()
	}
	if s.keyboard.Valid() {
		s.releaseKeyboard()
	}
	if s.touch.Valid() {
		s.releaseTouch()
	}
	if s.seat.Version() >= 5 {
		s.seat.Release()
	} else {
		s.seat.DestroyProxy()
	}
	s.seat = wl.Seat{}
}

// forget drops every reference to b, a window being destroyed, so no
// later event reaches it.
func (s *wlSeat) forget(b *Backend) {
	if s.ptrFocus == b {
		s.ptrFocus, s.buttons, s.axis = nil, 0, wlAxisAcc{}
	}
	if s.kbFocus == b {
		s.kbFocus = nil
		s.repeat.stop()
	}
	if s.touches.target == b {
		s.touches = wlTouches{}
	}
}

// --- pointer ---

func (s *wlSeat) pointerEnter(_ uint32, surface wl.Surface, x, y wl.Fixed) {
	s.ptrFocus = s.d.wins[surface.Ptr()]
	s.buttons = 0
	s.pointerMotion(0, x, y)
}

func (s *wlSeat) pointerLeave(uint32, wl.Surface) {
	if b := s.ptrFocus; b != nil {
		b.emit(gui.Event{Type: gui.EventMouseLeave})
	}
	s.ptrFocus, s.buttons, s.axis = nil, 0, wlAxisAcc{}
}

func (s *wlSeat) pointerMotion(_ uint32, x, y wl.Fixed) {
	s.ptrX, s.ptrY = float32(x.Float()), float32(y.Float())
	b := s.ptrFocus
	if b == nil {
		return
	}
	dx, dy := b.mouseDelta(s.ptrX, s.ptrY)
	b.emit(gui.Event{
		Type:      gui.EventMouseMove,
		MouseX:    s.ptrX,
		MouseY:    s.ptrY,
		MouseDX:   dx,
		MouseDY:   dy,
		Modifiers: x11key.MapModifiers(s.mods | s.buttons),
	})
}

func (s *wlSeat) pointerButton(_, _ uint32, button, state uint32) {
	btn, bit, ok := wlButton(button)
	b := s.ptrFocus
	if !ok || b == nil {
		return
	}
	// Like an X11 button event, the modifiers are those from before the
	// event: a press does not report its own button as held, a release
	// does.
	e := gui.Event{
		MouseX:      s.ptrX,
		MouseY:      s.ptrY,
		MouseButton: btn,
		Modifiers:   x11key.MapModifiers(s.mods | s.buttons),
	}
	if state == wl.PointerButtonStatePressed {
		s.buttons |= bit
		e.Type = gui.EventMouseDown
	} else {
		s.buttons &^= bit
		e.Type = gui.EventMouseUp
	}
	b.emit(e)
}

func (s *wlSeat) pointerAxis(_, axis uint32, value wl.Fixed) {
	s.axis.addValue(axis, value.Float())
	// Before wl_pointer v5 there is no frame event to group axis events,
	// so each one is a scroll of its own.
	if s.pointer.Version() < 5 {
		s.pointerFrame()
	}
}

// pointerFrame ends a group of pointer events. Only scrolling is grouped:
// both axes and the wheel clicks of one gesture arrive before it.
func (s *wlSeat) pointerFrame() {
	sx, sy, precise, ok := s.axis.take()
	b := s.ptrFocus
	if !ok || b == nil {
		return
	}
	b.emit(gui.Event{
		Type:          gui.EventMouseScroll,
		ScrollX:       sx,
		ScrollY:       sy,
		ScrollPrecise: precise,
		MouseX:        s.ptrX,
		MouseY:        s.ptrY,
		Modifiers:     x11key.MapModifiers(s.mods | s.buttons),
	})
}

// wlAxisAcc adds up the scroll of one pointer frame. Index 0 is the
// vertical axis, 1 the horizontal one (wl_pointer.axis).
type wlAxisAcc struct {
	value    [2]float64 // surface units
	value120 [2]int32   // wheel clicks × 120
	has      [2]bool    // the axis moved this frame
	wheel    [2]bool    // the axis had wheel clicks
}

func (a *wlAxisAcc) addValue(axis uint32, v float64) {
	if axis > 1 || v != v { // unknown axis, NaN
		return
	}
	a.value[axis] += v
	a.has[axis] = true
}

func (a *wlAxisAcc) addValue120(axis uint32, v int32) {
	if axis > 1 {
		return
	}
	a.value120[axis] += v
	a.wheel[axis] = true
}

// take returns the frame's scroll in gui units and clears it. A wheel
// reports lines (x11ScrollLines per click, as on X11); a touchpad reports
// points of finger travel, precise. Wayland's positive axis scrolls down
// or right, the opposite of gui's, hence the sign. ok is false when
// nothing scrolled.
func (a *wlAxisAcc) take() (sx, sy float32, precise, ok bool) {
	defer func() { *a = wlAxisAcc{} }()
	if !a.has[0] && !a.has[1] {
		return 0, 0, false, false
	}
	// A frame is a wheel frame only when every axis that moved clicked.
	// A wheel always sends both, so mixed frames come only from odd
	// hardware, which then scrolls by its continuous value.
	wheel := (!a.has[0] || a.wheel[0]) && (!a.has[1] || a.wheel[1])
	if wheel {
		const perClick = x11ScrollLines / 120
		return -float32(a.value120[1]) * perClick, -float32(a.value120[0]) * perClick, false, true
	}
	return -float32(a.value[1]), -float32(a.value[0]), true, true
}

// --- keyboard ---

// wlMaxKeymapSize bounds the keymap a compositor sends: a real one is about
// 60 KB, and xkb.NewKeymap refuses anything past its own bound anyway.
const wlMaxKeymapSize = 8 << 20

func (s *wlSeat) keyboardKeymap(format uint32, fd int, size uint32) {
	// The fd is ours whatever happens next.
	defer func() { _ = unix.Close(fd) }()
	s.keymap.Destroy()
	s.keymap = nil
	if format != wl.KeyboardKeymapFormatXkbV1 || size == 0 || size > wlMaxKeymapSize {
		log.Printf("gl: wayland: unusable keymap (format %d, %d bytes); keys are ignored", format, size)
		return
	}
	// Reading a mapping past the end of the file raises SIGBUS, so a
	// size larger than the file (a broken compositor) must not be mapped.
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil || st.Size < int64(size) {
		log.Printf("gl: wayland: keymap fd holds less than its %d bytes; keys are ignored", size)
		return
	}
	// MAP_PRIVATE: from wl_keyboard v7 the compositor may share one
	// read-only mapping between clients.
	buf, err := unix.Mmap(fd, 0, int(size), unix.PROT_READ, unix.MAP_PRIVATE)
	if err != nil {
		log.Printf("gl: wayland: map keymap: %v", err)
		return
	}
	defer func() { _ = unix.Munmap(buf) }()
	km, err := xkb.NewKeymap(buf)
	if err != nil {
		log.Printf("gl: wayland: %v; keys are ignored", err)
		return
	}
	s.keymap = km
	s.mods = 0
}

func (s *wlSeat) keyboardEnter(_ uint32, surface wl.Surface, _ []byte) {
	s.kbFocus = s.d.wins[surface.Ptr()]
}

func (s *wlSeat) keyboardLeave(uint32, wl.Surface) {
	if b := s.kbFocus; b != nil {
		// An open dead-key sequence must not survive a focus change, as
		// on X11.
		b.plat.compose.reset()
	}
	s.kbFocus = nil
	s.repeat.stop()
}

func (s *wlSeat) keyboardModifiers(_, depressed, latched, locked, group uint32) {
	if s.keymap == nil {
		return
	}
	s.keymap.UpdateMask(depressed, latched, locked, group)
	s.mods = s.keymap.State()
}

func (s *wlSeat) keyboardKey(_, _ uint32, key, state uint32) {
	if s.keymap == nil || s.kbFocus == nil {
		return
	}
	code := key + 8 // evdev → XKB keycode
	if state == wl.KeyboardKeyStatePressed {
		s.keyPress(code, false)
		if s.keymap.Repeats(code) {
			s.repeat.start(code, time.Now())
		}
		return
	}
	if s.repeat.key == code {
		s.repeat.stop()
	}
	s.kbFocus.emit(gui.Event{
		Type:      gui.EventKeyUp,
		KeyCode:   x11key.MapKeySym(s.keymap.BaseSym(code)),
		Modifiers: x11key.MapModifiers(s.mods | s.buttons),
	})
}

// keyPress emits the key down, and the character if the key types one.
func (s *wlSeat) keyPress(code uint32, repeat bool) {
	b := s.kbFocus
	state := s.mods | s.buttons
	b.emit(gui.Event{
		Type:      gui.EventKeyDown,
		KeyCode:   x11key.MapKeySym(s.keymap.BaseSym(code)),
		Modifiers: x11key.MapModifiers(state),
		KeyRepeat: repeat,
	})
	if r := wlKeyChar(&b.plat.compose, s.keymap.Sym(code), state, xkb.KeysymToRune); r != 0 {
		b.emitChar(r, state)
	}
}

// wlKeyChar is keyPressChar with a wider character table. The X11 compose
// machine falls back to x11key.KeysymToRune, which knows Latin-1 and
// Unicode keysyms only, so a Cyrillic or Greek layout would type nothing.
// When the machine produced nothing and holds no sequence, toRune
// (libxkbcommon's table, which knows every script) has the last word.
// Control characters (Return, Tab, Escape) are keys, not text.
func wlKeyChar(c *compose, sym uint32, state uint16, toRune func(uint32) rune) rune {
	if r := keyPressChar(c, sym, state); r != 0 || c.n > 0 || shortcutChord(state) ||
		isComposeKey(sym) || isModifierKey(sym) {
		return r
	}
	r := toRune(sym)
	if r < 0x20 || r == 0x7f {
		return 0
	}
	return r
}

// tickRepeat emits the key repeats that are due and reports how long until
// the next one, or -1 when no key repeats. The run loop calls it each pass
// and caps its wait with the result.
func (s *wlSeat) tickRepeat(now time.Time) time.Duration {
	if s.repeat.key == 0 {
		return -1
	}
	if s.kbFocus == nil || s.keymap == nil {
		s.repeat.stop()
		return -1
	}
	if s.repeat.due(now) {
		s.keyPress(s.repeat.key, true)
	}
	if s.repeat.key == 0 { // a handler's side effect ended it
		return -1
	}
	return max(s.repeat.at.Sub(now), 0)
}

// wlRepeat times the auto-repeat of one held key.
type wlRepeat struct {
	key         uint32 // XKB keycode repeating, 0 for none
	at          time.Time
	rate, delay int32 // keys per second, ms before the first repeat
}

// setInfo handles wl_keyboard.repeat_info. A rate of 0 turns repeat off.
func (r *wlRepeat) setInfo(rate, delay int32) {
	r.rate, r.delay = max(rate, 0), max(delay, 0)
	if r.rate == 0 {
		r.stop()
	}
}

func (r *wlRepeat) start(key uint32, now time.Time) {
	if r.rate == 0 {
		return
	}
	r.key = key
	r.at = now.Add(time.Duration(r.delay) * time.Millisecond)
}

func (r *wlRepeat) stop() { r.key = 0 }

// due reports whether a repeat fires at now, and schedules the next. After
// a stall (a slow frame) the next repeat is one interval from now, so held
// keys never fire in a burst.
func (r *wlRepeat) due(now time.Time) bool {
	if r.key == 0 || now.Before(r.at) {
		return false
	}
	interval := time.Second / time.Duration(max(r.rate, 1))
	r.at = r.at.Add(interval)
	if r.at.Before(now) {
		r.at = now.Add(interval)
	}
	return true
}

// --- touch ---

func (s *wlSeat) touchDown(_, _ uint32, surface wl.Surface, id int32, x, y wl.Fixed) {
	s.touches.down(s.d.wins[surface.Ptr()], id, x, y)
}

func (s *wlSeat) touchFrame() {
	b := s.touches.target
	n := s.touches.frame(&s.touchOut)
	if b == nil {
		return
	}
	for i := range n {
		b.emit(s.touchOut[i])
	}
}

func (s *wlSeat) touchCancel() {
	b := s.touches.target
	n := s.touches.cancel(&s.touchOut)
	if b == nil {
		return
	}
	for i := range n {
		b.emit(s.touchOut[i])
	}
}

// wlTouchPoint is one finger on the screen and what it did this frame.
type wlTouchPoint struct {
	id                int32
	x, y              float32
	down, moved, lift bool
}

// wlTouches tracks the fingers on one window. Wayland sends down, motion
// and up per finger, then a frame; gui wants one event per phase holding
// the fingers, so they are collected until the frame. All fingers belong
// to the window the first one touched: one touching another window at the
// same time is ignored. gui tracks at most len(gui.Event.Touches) fingers.
type wlTouches struct {
	target *Backend
	pts    [len(gui.Event{}.Touches)]wlTouchPoint
	n      int
}

func (t *wlTouches) find(id int32) *wlTouchPoint {
	for i := range t.n {
		if t.pts[i].id == id {
			return &t.pts[i]
		}
	}
	return nil
}

func (t *wlTouches) down(b *Backend, id int32, x, y wl.Fixed) {
	if b == nil || (t.n > 0 && b != t.target) || t.n == len(t.pts) || t.find(id) != nil {
		return
	}
	t.target = b
	t.pts[t.n] = wlTouchPoint{id: id, x: float32(x.Float()), y: float32(y.Float()), down: true}
	t.n++
}

func (t *wlTouches) motion(id int32, x, y wl.Fixed) {
	if p := t.find(id); p != nil {
		p.x, p.y, p.moved = float32(x.Float()), float32(y.Float()), true
	}
}

func (t *wlTouches) up(id int32) {
	if p := t.find(id); p != nil {
		p.lift = true
	}
}

// frame turns the frame's changes into gui events, in phase order: began,
// moved, ended. Began and moved list every finger and mark the ones that
// changed; ended lists only the lifted ones, like the web backend. Lifted
// fingers are then dropped.
func (t *wlTouches) frame(out *[3]gui.Event) int {
	n := 0
	var began, moved, lifted bool
	for i := range t.n {
		began = began || t.pts[i].down
		moved = moved || t.pts[i].moved
		lifted = lifted || t.pts[i].lift
	}
	if began {
		out[n] = t.event(gui.EventTouchesBegan, func(p *wlTouchPoint) (bool, bool) { return true, p.down })
		n++
	}
	if moved {
		out[n] = t.event(gui.EventTouchesMoved, func(p *wlTouchPoint) (bool, bool) { return true, p.moved })
		n++
	}
	if lifted {
		out[n] = t.event(gui.EventTouchesEnded, func(p *wlTouchPoint) (bool, bool) { return p.lift, true })
		n++
	}
	kept := 0
	for i := range t.n {
		if p := t.pts[i]; !p.lift {
			p.down, p.moved = false, false
			t.pts[kept] = p
			kept++
		}
	}
	t.n = kept
	if kept == 0 {
		t.target = nil
	}
	return n
}

// cancel ends every finger: the compositor took the touch sequence over
// (a gesture of its own).
func (t *wlTouches) cancel(out *[3]gui.Event) int {
	if t.n == 0 {
		t.target = nil
		return 0
	}
	out[0] = t.event(gui.EventTouchesCancelled, func(*wlTouchPoint) (bool, bool) { return true, true })
	*t = wlTouches{}
	return 1
}

// event builds one touch event from the fingers sel includes, marking
// those it reports changed.
func (t *wlTouches) event(typ gui.EventType, sel func(*wlTouchPoint) (include, changed bool)) gui.Event {
	e := gui.Event{Type: typ}
	for i := range t.n {
		p := &t.pts[i]
		include, changed := sel(p)
		if !include {
			continue
		}
		e.Touches[e.NumTouches] = gui.TouchPoint{
			Identifier: uint64(uint32(p.id)),
			PosX:       p.x,
			PosY:       p.y,
			ToolType:   gui.TouchToolFinger,
			Changed:    changed,
		}
		e.NumTouches++
	}
	return e
}
