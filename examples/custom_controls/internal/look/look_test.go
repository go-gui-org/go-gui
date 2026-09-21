package look

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestColorMath(t *testing.T) {
	c := gui.RGBA(100, 200, 0, 128)
	if got, want := Darken(c, 0.5), gui.RGBA(50, 100, 0, 128); got != want {
		t.Errorf("Darken = %+v, want %+v", got, want)
	}
	if got, want := Lighten(c, 0.5), gui.RGBA(177, 227, 127, 128); got != want {
		t.Errorf("Lighten = %+v, want %+v", got, want)
	}
	// Mix keeps the alpha of its first color and reaches b at f = 1.
	if got, want := Mix(c, gui.RGBA(0, 0, 255, 255), 1), gui.RGBA(0, 0, 255, 128); got != want {
		t.Errorf("Mix(f=1) = %+v, want %+v", got, want)
	}
	if got := Mix(c, gui.White, 0); got != c {
		t.Errorf("Mix(f=0) = %+v, want %+v", got, c)
	}
}

// Light takes its styles from ThemeLight, whatever the app theme is.
func TestLightFollowsThemeLight(t *testing.T) {
	if Light.Title != gui.ThemeLight.TextStyleTitle || Light.Body != gui.ThemeLight.TextStyleDef {
		t.Fatal("Light styles do not match ThemeLight")
	}
	if got, want := Light.Text(gui.White, 9), (func() gui.TextStyle {
		ts := gui.ThemeLight.TextStyleDef
		ts.Color, ts.Size = gui.White, 9
		return ts
	})(); got != want {
		t.Fatalf("Text = %+v, want %+v", got, want)
	}
}
