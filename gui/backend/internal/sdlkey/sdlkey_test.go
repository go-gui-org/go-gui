//go:build !js && !ios

package sdlkey

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/veandco/go-sdl2/sdl"
)

func TestMapKeyCodeKeypadEnter(t *testing.T) {
	if got := MapKeyCode(sdl.K_KP_ENTER); got != gui.KeyEnter {
		t.Fatalf("MapKeyCode(K_KP_ENTER) = %v, want %v",
			got, gui.KeyEnter)
	}
}

func TestMapKeyCodeModifiers(t *testing.T) {
	tests := []struct {
		sym  sdl.Keycode
		want gui.KeyCode
	}{
		{sdl.K_LSHIFT, gui.KeyLeftShift},
		{sdl.K_RSHIFT, gui.KeyRightShift},
		{sdl.K_LCTRL, gui.KeyLeftControl},
		{sdl.K_RCTRL, gui.KeyRightControl},
		{sdl.K_LALT, gui.KeyLeftAlt},
		{sdl.K_RALT, gui.KeyRightAlt},
		{sdl.K_LGUI, gui.KeyLeftSuper},
		{sdl.K_RGUI, gui.KeyRightSuper},
		{sdl.K_CAPSLOCK, gui.KeyCapsLock},
	}
	for _, tt := range tests {
		if got := MapKeyCode(tt.sym); got != tt.want {
			t.Errorf("MapKeyCode(%v) = %v, want %v",
				tt.sym, got, tt.want)
		}
	}
}

func TestMapKeyCodeArrows(t *testing.T) {
	tests := []struct {
		sym  sdl.Keycode
		want gui.KeyCode
	}{
		{sdl.K_UP, gui.KeyUp},
		{sdl.K_DOWN, gui.KeyDown},
		{sdl.K_LEFT, gui.KeyLeft},
		{sdl.K_RIGHT, gui.KeyRight},
	}
	for _, tt := range tests {
		if got := MapKeyCode(tt.sym); got != tt.want {
			t.Errorf("MapKeyCode(%v) = %v, want %v",
				tt.sym, got, tt.want)
		}
	}
}

func TestMapKeyCodeFunctionKeys(t *testing.T) {
	fkeys := []sdl.Keycode{
		sdl.K_F1, sdl.K_F2, sdl.K_F3, sdl.K_F4,
		sdl.K_F5, sdl.K_F6, sdl.K_F7, sdl.K_F8,
		sdl.K_F9, sdl.K_F10, sdl.K_F11, sdl.K_F12,
	}
	expected := []gui.KeyCode{
		gui.KeyF1, gui.KeyF2, gui.KeyF3, gui.KeyF4,
		gui.KeyF5, gui.KeyF6, gui.KeyF7, gui.KeyF8,
		gui.KeyF9, gui.KeyF10, gui.KeyF11, gui.KeyF12,
	}
	for i, sym := range fkeys {
		if got := MapKeyCode(sym); got != expected[i] {
			t.Errorf("MapKeyCode(F%d) = %v, want %v",
				i+1, got, expected[i])
		}
	}
}

func TestMapKeyCodePrintable(t *testing.T) {
	if got := MapKeyCode('a'); got != gui.KeyCode('A') {
		t.Errorf("MapKeyCode('a') = %v, want %v", got, gui.KeyCode('A'))
	}
	if got := MapKeyCode('z'); got != gui.KeyCode('Z') {
		t.Errorf("MapKeyCode('z') = %v, want %v", got, gui.KeyCode('Z'))
	}
	if got := MapKeyCode('0'); got != gui.KeyCode('0') {
		t.Errorf("MapKeyCode('0') = %v, want %v", got, gui.KeyCode('0'))
	}
	if got := MapKeyCode('9'); got != gui.KeyCode('9') {
		t.Errorf("MapKeyCode('9') = %v, want %v", got, gui.KeyCode('9'))
	}
}

func TestMapKeyCodeNavigation(t *testing.T) {
	tests := []struct {
		sym  sdl.Keycode
		want gui.KeyCode
	}{
		{sdl.K_HOME, gui.KeyHome},
		{sdl.K_END, gui.KeyEnd},
		{sdl.K_PAGEUP, gui.KeyPageUp},
		{sdl.K_PAGEDOWN, gui.KeyPageDown},
		{sdl.K_INSERT, gui.KeyInsert},
		{sdl.K_DELETE, gui.KeyDelete},
		{sdl.K_BACKSPACE, gui.KeyBackspace},
		{sdl.K_TAB, gui.KeyTab},
		{sdl.K_ESCAPE, gui.KeyEscape},
		{sdl.K_SPACE, gui.KeySpace},
	}
	for _, tt := range tests {
		if got := MapKeyCode(tt.sym); got != tt.want {
			t.Errorf("MapKeyCode(%v) = %v, want %v",
				tt.sym, got, tt.want)
		}
	}
}

func TestMapKeyCodeUnknown(t *testing.T) {
	if got := MapKeyCode(0xFFFF); got != gui.KeyInvalid {
		t.Errorf("MapKeyCode(unknown) = %v, want KeyInvalid", got)
	}
}

func TestMapMouseButton(t *testing.T) {
	tests := []struct {
		btn  uint8
		want gui.MouseButton
	}{
		{sdl.BUTTON_LEFT, gui.MouseLeft},
		{sdl.BUTTON_RIGHT, gui.MouseRight},
		{sdl.BUTTON_MIDDLE, gui.MouseMiddle},
		{255, gui.MouseInvalid},
	}
	for _, tt := range tests {
		if got := MapMouseButton(tt.btn); got != tt.want {
			t.Errorf("MapMouseButton(%d) = %v, want %v",
				tt.btn, got, tt.want)
		}
	}
}

func TestMapMouseButtons(t *testing.T) {
	tests := []struct {
		state uint32
		want  gui.Modifier
	}{
		{0, 0},
		{sdl.ButtonLMask(), gui.ModLMB},
		{sdl.ButtonRMask(), gui.ModRMB},
		{sdl.ButtonMMask(), gui.ModMMB},
		{sdl.ButtonLMask() | sdl.ButtonRMask(), gui.ModLMB | gui.ModRMB},
		{sdl.ButtonLMask() | sdl.ButtonRMask() | sdl.ButtonMMask(),
			gui.ModLMB | gui.ModRMB | gui.ModMMB},
	}
	for _, tt := range tests {
		if got := MapMouseButtons(tt.state); got != tt.want {
			t.Errorf("MapMouseButtons(%v) = %v, want %v",
				tt.state, got, tt.want)
		}
	}
}

func TestMapKeyMod(t *testing.T) {
	tests := []struct {
		mod  sdl.Keymod
		want gui.Modifier
	}{
		{0, 0},
		{sdl.KMOD_SHIFT, gui.ModShift},
		{sdl.KMOD_CTRL, gui.ModCtrl},
		{sdl.KMOD_ALT, gui.ModAlt},
		{sdl.KMOD_GUI, gui.ModSuper},
		{sdl.KMOD_SHIFT | sdl.KMOD_CTRL, gui.ModShift | gui.ModCtrl},
		{sdl.KMOD_SHIFT | sdl.KMOD_ALT | sdl.KMOD_GUI,
			gui.ModShift | gui.ModAlt | gui.ModSuper},
	}
	for _, tt := range tests {
		if got := MapKeyMod(tt.mod); got != tt.want {
			t.Errorf("MapKeyMod(%v) = %v, want %v",
				tt.mod, got, tt.want)
		}
	}
}
