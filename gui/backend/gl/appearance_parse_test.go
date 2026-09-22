//go:build !js && !android

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestParseGsettingsColorScheme(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   gui.Appearance
		wantOK bool
	}{
		{"get dark", "'prefer-dark'\n", gui.AppearanceDark, true},
		{"get light", "'prefer-light'\n", gui.AppearanceLight, true},
		{"get default", "'default'\n", gui.AppearanceLight, true},
		{"monitor dark", "org.gnome.desktop.interface color-scheme: 'prefer-dark'\n", gui.AppearanceDark, true},
		{"monitor light", "org.gnome.desktop.interface color-scheme: 'prefer-light'\n", gui.AppearanceLight, true},
		{"garbage", "oops\n", gui.AppearanceLight, false},
		{"empty", "", gui.AppearanceLight, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := parseGsettingsColorScheme(c.in)
			if got != c.want || ok != c.wantOK {
				t.Errorf("parse(%q): got (%v, %v), want (%v, %v)",
					c.in, got, ok, c.want, c.wantOK)
			}
		})
	}
}
