package gui

// Character constants used in event handling.
const (
	CharBSP    = 0x08 // backspace
	CharDel    = 0x7F // delete
	CharSpace  = 0x20
	CharEscape = 0x1B
	CharLF     = 0x0A
	CharCR     = 0x0D
	CharCmdA   = 0x61
	CharCmdC   = 0x63
	CharCmdV   = 0x76
	CharCmdX   = 0x78
	CharCmdZ   = 0x7A
	CharCtrlA  = 0x01
	CharCtrlC  = 0x03
	CharCtrlV  = 0x16
	CharCtrlX  = 0x18
	CharCtrlZ  = 0x1A
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
	EventIconified
	EventRestored
	EventFocused
	EventUnfocused
	EventSuspended
	EventResumed
	EventQuitRequested
	EventClipboardPasted
	EventFileDropped
	EventIMEComposition
	EventGesture     // gesture recognized from touch input
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
	ModCtrlSuper    Modifier = ModCtrl | ModSuper
	ModAltShift     Modifier = ModAlt | ModShift
	ModAltSuper     Modifier = ModAlt | ModSuper
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
	KeyScrollLock   KeyCode = 281
	KeyNumLock      KeyCode = 282
	KeyPrintScreen  KeyCode = 283
	KeyPause        KeyCode = 284
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
	KeyF21          KeyCode = 310
	KeyF22          KeyCode = 311
	KeyF23          KeyCode = 312
	KeyF24          KeyCode = 313
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
type GestureType uint8

// GestureType values.
const (
	GestureNone      GestureType = iota
	GestureTap                   // single finger tap
	GestureDoubleTap             // two taps in quick succession
	GestureLongPress             // finger held without movement
	GesturePan                   // single finger drag
	GestureSwipe                 // fast pan ending with high velocity
	GesturePinch                 // two-finger spread/squeeze
	GestureRotate                // two-finger twist
)

// GesturePhase tracks the lifecycle of a continuous gesture.
type GesturePhase uint8

// GesturePhase values.
const (
	GesturePhaseBegan     GesturePhase = iota // first recognition
	GesturePhaseChanged                       // ongoing update
	GesturePhaseEnded                         // final event
	GesturePhaseCancelled                     // cancelled
)

// TouchToolType identifies the input device type for touch events.
type TouchToolType uint8

// TouchToolType values.
const (
	TouchToolUnknown TouchToolType = iota
	TouchToolFinger
	TouchToolStylus
	TouchToolMouse
	TouchToolEraser
	TouchToolPalm
)

// TouchPoint holds data for a single touch event point.
type TouchPoint struct {
	Identifier uint64
	PosX       float32
	PosY       float32
	ToolType   TouchToolType
	Changed    bool
}

// Event holds input event data.
type Event struct {
	IMEText           string
	FilePath          string
	Touches           [8]TouchPoint
	FrameCount        uint64
	NumTouches        int
	WindowWidth       int
	WindowHeight      int
	FramebufferWidth  int
	FramebufferHeight int
	GestureTouches    int // touch count for this gesture
	WindowID          uint32
	MouseX            float32
	MouseY            float32
	MouseDX           float32
	MouseDY           float32
	ScrollX           float32
	ScrollY           float32
	Modifiers         Modifier
	CharCode          uint32
	IMEStart          int32
	IMELength         int32
	GestureDX         float32 // pan/swipe delta from previous
	GestureDY         float32
	VelocityX         float32 // px/s at gesture end
	VelocityY         float32
	PinchScale        float32 // cumulative scale (1.0 = unchanged)
	GestureRotation   float32 // cumulative radians
	CentroidX         float32 // center of active touches
	CentroidY         float32
	KeyCode           KeyCode
	MouseButton       MouseButton
	Type              EventType
	GestureType       GestureType
	GesturePhase      GesturePhase
	KeyRepeat         bool
	IsHandled         bool
}

// eventRelativeTo returns a copy of the event with mouse
// coordinates relative to the given shape's position.
func eventRelativeTo(shape *Shape, e *Event) Event {
	ev := *e
	ev.MouseX = e.MouseX - shape.X
	ev.MouseY = e.MouseY - shape.Y
	return ev
}

// spacebarToClick wraps an onClick handler to fire when
// spacebar is pressed. Enables keyboard activation for
// clickable elements.
func spacebarToClick(onClick func(*Layout, *Event, *Window)) func(*Layout, *Event, *Window) {
	if onClick == nil {
		return nil
	}
	return func(layout *Layout, e *Event, w *Window) {
		if e.CharCode == CharSpace {
			onClick(layout, e, w)
			e.IsHandled = true
		}
	}
}

// enterToClick wraps an onClick handler to fire when
// Enter is pressed.
func enterToClick(onClick func(*Layout, *Event, *Window)) func(*Layout, *Event, *Window) {
	if onClick == nil {
		return nil
	}
	return func(layout *Layout, e *Event, w *Window) {
		if e.KeyCode == KeyEnter {
			onClick(layout, e, w)
			e.IsHandled = true
		}
	}
}

// leftClickOnly wraps a click handler to fire only on
// left mouse button.
func leftClickOnly(onClick func(*Layout, *Event, *Window)) func(*Layout, *Event, *Window) {
	if onClick == nil {
		return nil
	}
	return func(layout *Layout, e *Event, w *Window) {
		if e.MouseButton == MouseLeft {
			onClick(layout, e, w)
		}
	}
}
