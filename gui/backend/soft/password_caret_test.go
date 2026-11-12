//go:build !wasm

package soft

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// A password field draws bullets, three bytes each, and its arrow moves
// query the shaped bullets. Its caret is in bytes of the text, so a move
// taken in the text's offsets landed a third of the way in: End, Left,
// then a typed X went in before "def", not before "f".
func TestPasswordArrowMovesByCharacter(t *testing.T) {
	cases := []struct {
		name string
		keys []gui.KeyCode
		want string
	}{
		{"left", []gui.KeyCode{gui.KeyEnd, gui.KeyLeft}, "abcdeXf"},
		{"left twice", []gui.KeyCode{gui.KeyEnd, gui.KeyLeft, gui.KeyLeft}, "abcdXef"},
		{"right", []gui.KeyCode{gui.KeyHome, gui.KeyRight}, "aXbcdef"},
		{"right twice", []gui.KeyCode{gui.KeyHome, gui.KeyRight, gui.KeyRight}, "abXcdef"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text := "abcdef"
			win := newWin(t, 300, 80, func(*gui.Window) gui.View {
				return gui.Input(gui.InputCfg{ID: "pw", Text: text,
					IsPassword: true, Sizing: gui.FillFit,
					TextStyle:     gui.TextStyle{Size: 16, Color: gui.RGB(255, 255, 255)},
					OnTextChanged: func(s string, _ gui.EventCtx) { text = s }})
			})
			if _, err := RenderToImage(win, 1); err != nil {
				t.Fatal(err)
			}
			for _, k := range c.keys {
				if err := win.TestKey("pw", k, gui.ModNone); err != nil {
					t.Fatal(err)
				}
			}
			if err := win.TestType("pw", "X"); err != nil {
				t.Fatal(err)
			}
			if text != c.want {
				t.Fatalf("got %q, want %q", text, c.want)
			}
		})
	}
}
