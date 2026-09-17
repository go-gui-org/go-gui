// Package look holds what the custom control pages share: the text styles,
// the page padding and spacing, the color math and the bevel.
//
// Every page paints its own light background. So the text styles come from
// gui.ThemeLight, not from the app's theme: under a dark app theme, text
// taken from the app theme would be light text on a light page.
package look

import "github.com/go-gui-org/go-gui/gui"

// Styles are the text styles a page uses.
type Styles struct {
	// Title is the page heading.
	Title gui.TextStyle
	// Heading is a section heading that stands out more than Label.
	Heading gui.TextStyle
	// Label is a quiet section title or field label.
	Label gui.TextStyle
	// Secondary is a note under a heading.
	Secondary gui.TextStyle
	// Body is plain text, such as the log line.
	Body gui.TextStyle
	// Icon is the icon font at its body size.
	Icon gui.TextStyle
	// Bold4, Bold5 and Bold6 are the smaller bold steps of the theme.
	Bold4, Bold5, Bold6 gui.TextStyle
}

// Light is built once, when the package loads. gui sets ThemeLight in its
// init, and gui's init runs before this package's variables are set. Pages
// read these fields and do not copy the theme (a Theme is about 12 KB).
var Light = Styles{
	Title:     gui.ThemeLight.B2,
	Heading:   gui.ThemeLight.B3,
	Label:     gui.ThemeLight.TextStyleLabel,
	Secondary: gui.ThemeLight.TextStyleSecondary,
	Body:      gui.ThemeLight.TextStyleDef,
	Icon:      gui.ThemeLight.Icon4,
	Bold4:     gui.ThemeLight.B4,
	Bold5:     gui.ThemeLight.B5,
	Bold6:     gui.ThemeLight.B6,
}

// Text returns the body style with a color and a size.
func (s *Styles) Text(c gui.Color, size float32) gui.TextStyle {
	ts := s.Body
	ts.Color = c
	ts.Size = size
	return ts
}

// IconStyle returns the icon style with a color and a size.
func (s *Styles) IconStyle(c gui.Color, size float32) gui.TextStyle {
	ts := s.Icon
	ts.Color = c
	ts.Size = size
	return ts
}

// PagePadding and PageSpacing set the space around and between the parts of
// a page. They are the theme's large padding step and medium spacing step,
// so the pages share one rhythm with the rest of go-gui.
var (
	PagePadding = gui.PaddingLarge
	PageSpacing = gui.SomeF(gui.SpacingMedium)
)

// white is the target of Lighten.
var white = gui.Hex(0xffffff)

// Darken scales the RGB channels by f. Alpha stays the same.
func Darken(c gui.Color, f float32) gui.Color {
	return gui.RGBA(uint8(float32(c.R)*f), uint8(float32(c.G)*f), uint8(float32(c.B)*f), c.A)
}

// Lighten moves each RGB channel toward white by f. Alpha stays the same.
func Lighten(c gui.Color, f float32) gui.Color {
	return Mix(c, white, f)
}

// Mix moves each RGB channel of a toward b by f. The alpha of a stays.
func Mix(a, b gui.Color, f float32) gui.Color {
	return gui.RGBA(mixChannel(a.R, b.R, f), mixChannel(a.G, b.G, f), mixChannel(a.B, b.B, f), a.A)
}

func mixChannel(x, y uint8, f float32) uint8 {
	return uint8(float32(x) + (float32(y)-float32(x))*f)
}

// Bevel pads content with one color on the sides that pad names. Two nested
// bevels, a dark one padded bottom and right and a light one padded top and
// left, draw a raised or sunken 3D edge.
func Bevel(c gui.Color, pad gui.Padding, content gui.View) gui.View {
	return gui.Column(gui.ContainerCfg{
		Color:      c,
		Radius:     gui.SomeF(0),
		Padding:    pad,
		SizeBorder: gui.NoBorder,
		Content:    []gui.View{content},
	})
}
