package gui

// Character constants used in event handling.
const (
	charBSP    = 0x08 // backspace
	charDel    = 0x7F // delete
	charSpace  = 0x20
	charEscape = 0x1B
	charLF     = 0x0A
	charCR     = 0x0D
	charCmdA   = 0x61
	charCmdC   = 0x63
	charCmdV   = 0x76
	charCmdX   = 0x78
	charCmdZ   = 0x7A
	charCtrlA  = 0x01
	charCtrlC  = 0x03
	charCtrlV  = 0x16
	charCtrlX  = 0x18
	charCtrlZ  = 0x1A
)

const reservedDialogID = "___dialog_reserved_do_not_use___"

// EventType identifies the kind of input event.
type EventType uint8

// EventType values.
const (
	EventInvalid EventType = iota
	EventKeyDown
	EventKeyUp
	EventChar
	EventMouseDown
	EventMouseUp
	EventMouseScroll
	EventMouseMove
	EventMouseEnter
	EventMouseLeave
	EventTouchesBegan
	EventTouchesMoved
	EventTouchesEnded
	EventTouchesCancelled
	EventResized
	eventIconified
	eventRestored
	EventFocused
	EventUnfocused
	eventSuspended
	eventResumed
	eventQuitRequested
	EventClipboardPasted
	EventFileDropped
	EventIMEComposition
	eventGesture     // gesture recognized from touch input
	EventScrollBegan // trackpad finger touch (zero-delta phase begin)
)

// MouseButton identifies which mouse button was pressed/released.
type MouseButton uint16

// MouseButton values.
const (
	MouseLeft    MouseButton = 0
	MouseRight   MouseButton = 1
	MouseMiddle  MouseButton = 2
	MouseInvalid MouseButton = 256
)

// MouseCursor represents the shape of the mouse cursor.
type MouseCursor uint8

// MouseCursor values.
const (
	CursorDefault MouseCursor = iota
	CursorArrow
	CursorIBeam
	CursorCrosshair
	CursorPointingHand
	CursorResizeEW
	CursorResizeNS
	CursorResizeNWSE
	CursorResizeNESW
	CursorResizeAll
	CursorNotAllowed
)

// Modifier is a bitmask of keyboard/mouse modifier flags.
type Modifier uint32

// Modifier values.
const (
	ModNone  Modifier = 0
	ModShift Modifier = 1
	ModCtrl  Modifier = 2
	ModAlt   Modifier = 4
	ModSuper Modifier = 8
	ModLMB   Modifier = 0x100
	ModRMB   Modifier = 0x200
	ModMMB   Modifier = 0x400

	ModCtrlShift    Modifier = ModCtrl | ModShift
	ModCtrlAlt      Modifier = ModCtrl | ModAlt
	ModCtrlAltShift Modifier = ModCtrl | ModAlt | ModShift
	modCtrlSuper    Modifier = ModCtrl | ModSuper
	ModAltShift     Modifier = ModAlt | ModShift
	modAltSuper     Modifier = ModAlt | ModSuper
	ModSuperShift   Modifier = ModSuper | ModShift
)

// Has checks if the modifier bitmask contains the given flag.
func (m Modifier) Has(mod Modifier) bool {
	return m&mod == mod
}

// HasAny checks if the modifier bitmask contains any of the
// given flags.
func (m Modifier) HasAny(mods ...Modifier) bool {
	for _, mod := range mods {
		if m&mod == mod {
			return true
		}
	}
	return false
}

// KeyCode identifies a keyboard key.
type KeyCode uint16

// KeyCode values.
const (
	KeyInvalid      KeyCode = 0
	KeySpace        KeyCode = 32
	KeyApostrophe   KeyCode = 39
	KeyComma        KeyCode = 44
	KeyMinus        KeyCode = 45
	KeyPeriod       KeyCode = 46
	KeySlash        KeyCode = 47
	Key0            KeyCode = 48
	Key1            KeyCode = 49
	Key2            KeyCode = 50
	Key3            KeyCode = 51
	Key4            KeyCode = 52
	Key5            KeyCode = 53
	Key6            KeyCode = 54
	Key7            KeyCode = 55
	Key8            KeyCode = 56
	Key9            KeyCode = 57
	KeySemicolon    KeyCode = 59
	KeyEqual        KeyCode = 61
	KeyA            KeyCode = 65
	KeyB            KeyCode = 66
	KeyC            KeyCode = 67
	KeyD            KeyCode = 68
	KeyE            KeyCode = 69
	KeyF            KeyCode = 70
	KeyG            KeyCode = 71
	KeyH            KeyCode = 72
	KeyI            KeyCode = 73
	KeyJ            KeyCode = 74
	KeyK            KeyCode = 75
	KeyL            KeyCode = 76
	KeyM            KeyCode = 77
	KeyN            KeyCode = 78
	KeyO            KeyCode = 79
	KeyP            KeyCode = 80
	KeyQ            KeyCode = 81
	KeyR            KeyCode = 82
	KeyS            KeyCode = 83
	KeyT            KeyCode = 84
	KeyU            KeyCode = 85
	KeyV            KeyCode = 86
	KeyW            KeyCode = 87
	KeyX            KeyCode = 88
	KeyY            KeyCode = 89
	KeyZ            KeyCode = 90
	KeyLeftBracket  KeyCode = 91
	KeyBackslash    KeyCode = 92
	KeyRightBracket KeyCode = 93
	KeyGraveAccent  KeyCode = 96
	KeyWorld1       KeyCode = 161
	KeyWorld2       KeyCode = 162
	KeyEscape       KeyCode = 256
	KeyEnter        KeyCode = 257
	KeyTab          KeyCode = 258
	KeyBackspace    KeyCode = 259
	KeyInsert       KeyCode = 260
	KeyDelete       KeyCode = 261
	KeyRight        KeyCode = 262
	KeyLeft         KeyCode = 263
	KeyDown         KeyCode = 264
	KeyUp           KeyCode = 265
	KeyPageUp       KeyCode = 266
	KeyPageDown     KeyCode = 267
	KeyHome         KeyCode = 268
	KeyEnd          KeyCode = 269
	KeyCapsLock     KeyCode = 280
	keyScrollLock   KeyCode = 281
	KeyNumLock      KeyCode = 282
	keyPrintScreen  KeyCode = 283
	keyPause        KeyCode = 284
	KeyF1           KeyCode = 290
	KeyF2           KeyCode = 291
	KeyF3           KeyCode = 292
	KeyF4           KeyCode = 293
	KeyF5           KeyCode = 294
	KeyF6           KeyCode = 295
	KeyF7           KeyCode = 296
	KeyF8           KeyCode = 297
	KeyF9           KeyCode = 298
	KeyF10          KeyCode = 299
	KeyF11          KeyCode = 300
	KeyF12          KeyCode = 301
	KeyF13          KeyCode = 302
	KeyF14          KeyCode = 303
	KeyF15          KeyCode = 304
	KeyF16          KeyCode = 305
	KeyF17          KeyCode = 306
	KeyF18          KeyCode = 307
	KeyF19          KeyCode = 308
	KeyF20          KeyCode = 309
	keyF21          KeyCode = 310
	keyF22          KeyCode = 311
	keyF23          KeyCode = 312
	keyF24          KeyCode = 313
	KeyF25          KeyCode = 314
	KeyKP0          KeyCode = 320
	KeyKP1          KeyCode = 321
	KeyKP2          KeyCode = 322
	KeyKP3          KeyCode = 323
	KeyKP4          KeyCode = 324
	KeyKP5          KeyCode = 325
	KeyKP6          KeyCode = 326
	KeyKP7          KeyCode = 327
	KeyKP8          KeyCode = 328
	KeyKP9          KeyCode = 329
	KeyKPDecimal    KeyCode = 330
	KeyKPDivide     KeyCode = 331
	KeyKPMultiply   KeyCode = 332
	KeyKPSubtract   KeyCode = 333
	KeyKPAdd        KeyCode = 334
	KeyKPEnter      KeyCode = 335
	KeyKPEqual      KeyCode = 336
	KeyLeftShift    KeyCode = 340
	KeyLeftControl  KeyCode = 341
	KeyLeftAlt      KeyCode = 342
	KeyLeftSuper    KeyCode = 343
	KeyRightShift   KeyCode = 344
	KeyRightControl KeyCode = 345
	KeyRightAlt     KeyCode = 346
	KeyRightSuper   KeyCode = 347
	KeyMenu         KeyCode = 348
)

// GestureType identifies a recognized gesture from touch input.
// exportaudit:keep — collides with the gestureState's gestureType field
type GestureType uint8

// GestureType values.
const (
	gestureNone      GestureType = iota
	GestureTap                   // single finger tap
	GestureDoubleTap             // two taps in quick succession
	GestureLongPress             // finger held without movement
	GesturePan                   // single finger drag
	GestureSwipe                 // fast pan ending with high velocity
	GesturePinch                 // two-finger spread/squeeze
	GestureRotate                // two-finger twist
)

// GesturePhase tracks the lifecycle of a continuous gesture.
type gesturePhase uint8

// GesturePhase values.
const (
	gesturePhaseBegan     gesturePhase = iota // first recognition
	GesturePhaseChanged                       // ongoing update
	gesturePhaseEnded                         // final event
	gesturePhaseCancelled                     // cancelled
)

// TouchToolType identifies the input device type for touch events.
type touchToolType uint8

// TouchToolType values.
const (
	touchToolUnknown touchToolType = iota
	TouchToolFinger
	touchToolStylus
	touchToolMouse
	touchToolEraser
	touchToolPalm
)

// TouchPoint holds data for a single touch event point.
type TouchPoint struct {
	Identifier uint64
	PosX       float32
	PosY       float32
	ToolType   touchToolType
	Changed    bool
}

// Event holds input event data.
//
// CharCode carries only the FIRST rune of the typed text. An IME
// commit routinely delivers several runes in one event (CJK), and the
// full string lives only in IMEText — read IMEText whenever the text
// may come from an input method.
//
// IMEStart and IMELength delimit the selected clause within IMEText, in
// characters. They are an absolute range, not a caret plus a length:
// IMEStart is where the clause begins, which a backend must not assume
// is the insertion point. IMELength == 0 means no clause is selected,
// and IMEStart is then whatever the platform calls the cursor.
type Event struct {
	IMEText        string
	FilePath       string
	Touches        [8]TouchPoint
	FrameCount     uint64
	NumTouches     int
	WindowWidth    int
	WindowHeight   int
	gestureTouches int // touch count for this gesture
	WindowID       uint32
	MouseX         float32
	MouseY         float32
	MouseDX        float32
	MouseDY        float32
	// ScrollX and ScrollY carry scroll distance in one of two units,
	// selected by ScrollPrecise. For a discrete mouse wheel
	// (ScrollPrecise false) the unit is LINES OF TEXT: every backend
	// converts its native notch representation to the platform's
	// lines-per-notch, so one notch reports ~3 on all of them. For a
	// precise/trackpad delta (ScrollPrecise true) the unit is points of
	// finger travel, pre-scaled by the backend. Consumers that care about
	// the distinction — a terminal grid scrolling whole rows, say — must
	// branch on ScrollPrecise rather than assuming one unit.
	ScrollX         float32
	ScrollY         float32
	Modifiers       Modifier
	CharCode        uint32
	IMEStart        int32
	IMELength       int32
	GestureDX       float32 // pan/swipe delta from previous
	GestureDY       float32
	VelocityX       float32 // px/s at gesture end
	VelocityY       float32
	PinchScale      float32 // cumulative scale (1.0 = unchanged)
	GestureRotation float32 // cumulative radians
	CentroidX       float32 // center of active touches
	CentroidY       float32
	KeyCode         KeyCode
	MouseButton     MouseButton
	Type            EventType
	GestureType     GestureType
	GesturePhase    gesturePhase
	KeyRepeat       bool
	IsHandled       bool
	// ScrollPrecise is true for high-res / trackpad scroll deltas
	// (already carrying OS momentum). False for discrete mouse-wheel
	// notches, which the gui side eases via scrollSmoothAnimation.
	ScrollPrecise bool
}

// eventRelativeTo returns a copy of the event with mouse
// coordinates relative to the given shape's position.
func eventRelativeTo(shape *Shape, e *Event) Event {
	ev := *e
	ev.MouseX = e.MouseX - shape.X
	ev.MouseY = e.MouseY - shape.Y
	return ev
}
