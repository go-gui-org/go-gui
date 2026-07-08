//go:build !windows && !js && !linux

package gl

import (
	"runtime"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/sdlkey"
	"github.com/veandco/go-sdl2/sdl"
)

// mapEvent converts an SDL2 event to a gui.Event.
// Returns the event and true to continue, or false to quit.
func mapEvent(ev sdl.Event, b *Backend) (gui.Event, bool) {
	switch e := ev.(type) {
	case *sdl.QuitEvent:
		return gui.Event{}, false

	case *sdl.MouseButtonEvent:
		btn := sdlkey.MapMouseButton(e.Button)
		if e.Type == sdl.MOUSEBUTTONDOWN {
			return gui.Event{
				Type:        gui.EventMouseDown,
				MouseX:      float32(e.X),
				MouseY:      float32(e.Y),
				MouseButton: btn,
				Modifiers:   sdlkey.MapKeyMod(sdl.GetModState()),
			}, true
		}
		return gui.Event{
			Type:        gui.EventMouseUp,
			MouseX:      float32(e.X),
			MouseY:      float32(e.Y),
			MouseButton: btn,
			Modifiers:   sdlkey.MapKeyMod(sdl.GetModState()),
		}, true

	case *sdl.MouseMotionEvent:
		return gui.Event{
			Type:      gui.EventMouseMove,
			MouseX:    float32(e.X),
			MouseY:    float32(e.Y),
			MouseDX:   float32(e.XRel),
			MouseDY:   float32(e.YRel),
			Modifiers: sdlkey.MapKeyMod(sdl.GetModState()) | sdlkey.MapMouseButtons(e.State),
		}, true

	case *sdl.MouseWheelEvent:
		mx, my, _ := sdl.GetMouseState()
		// PreciseX/Y (SDL >= 2.0.18) carry sub-unit resolution from
		// trackpad pixel-scroll and high-resolution wheels; prefer them
		// when non-zero, fall back to integer X/Y for older SDL runtimes
		// or for devices that only populate the integer field.
		sx, sy := float32(e.X), float32(e.Y)
		if e.PreciseX != 0 {
			sx = e.PreciseX
		}
		if e.PreciseY != 0 {
			sy = e.PreciseY
		}
		return gui.Event{
			Type:      gui.EventMouseScroll,
			ScrollX:   sx,
			ScrollY:   sy,
			MouseX:    float32(mx),
			MouseY:    float32(my),
			Modifiers: sdlkey.MapKeyMod(sdl.GetModState()),
		}, true

	case *sdl.KeyboardEvent:
		if e.Type == sdl.KEYDOWN {
			return gui.Event{
				Type:      gui.EventKeyDown,
				KeyCode:   sdlkey.MapKeyCode(e.Keysym.Sym),
				Modifiers: sdlkey.MapKeyMod(sdl.Keymod(e.Keysym.Mod)),
				KeyRepeat: e.Repeat > 0,
			}, true
		}
		return gui.Event{
			Type:      gui.EventKeyUp,
			KeyCode:   sdlkey.MapKeyCode(e.Keysym.Sym),
			Modifiers: sdlkey.MapKeyMod(sdl.Keymod(e.Keysym.Mod)),
		}, true

	case *sdl.TextInputEvent:
		text := e.GetText()
		if len(text) == 0 {
			return gui.Event{}, true
		}
		r, sz := utf8.DecodeRuneInString(text)
		if r == utf8.RuneError && sz == 1 {
			return gui.Event{}, true
		}
		return gui.Event{
			Type:      gui.EventChar,
			CharCode:  uint32(r),
			IMEText:   text,
			Modifiers: sdlkey.MapKeyMod(sdl.GetModState()),
		}, true

	case *sdl.TextEditingEvent:
		return gui.Event{
			Type:      gui.EventIMEComposition,
			IMEText:   e.GetText(),
			IMEStart:  e.Start,
			IMELength: e.Length,
		}, true

	case *sdl.WindowEvent:
		switch e.Event {
		case sdl.WINDOWEVENT_RESIZED,
			sdl.WINDOWEVENT_SIZE_CHANGED:
			// On macOS, live-resize redraw is handled by the
			// event watcher in backend.Run to avoid duplicate work.
			if runtime.GOOS != "darwin" {
				b.handleResize()
			}
			return gui.Event{
				Type:         gui.EventResized,
				WindowWidth:  int(e.Data1),
				WindowHeight: int(e.Data2),
			}, true
		case sdl.WINDOWEVENT_FOCUS_GAINED:
			return gui.Event{Type: gui.EventFocused}, true
		case sdl.WINDOWEVENT_FOCUS_LOST:
			return gui.Event{Type: gui.EventUnfocused}, true
		}
	}
	return gui.Event{}, true
}

// mapEventMulti is like mapEvent but for multi-window mode.
// A QuitEvent returns false. Window close events are handled
// by the caller.
func mapEventMulti(ev sdl.Event,
	b *Backend) (gui.Event, bool) {

	switch e := ev.(type) {
	case *sdl.QuitEvent:
		return gui.Event{}, false

	case *sdl.WindowEvent:
		if e.Event == sdl.WINDOWEVENT_CLOSE {
			return gui.Event{
				Type: gui.EventQuitRequested,
			}, true
		}
	}
	// Delegate to normal mapping (b may be nil for
	// unowned events).
	if b != nil {
		return mapEvent(ev, b)
	}
	return gui.Event{}, true
}
