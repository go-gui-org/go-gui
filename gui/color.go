package gui

import (
	"fmt"
	"strings"
)

// Color represents a 32-bit color value in sRGB format.
// The set field distinguishes "not set" (zero value) from intentionally
// transparent (RGBA 0,0,0,0). Use RGBA(), RGB(), Hex() constructors
// to create set colors. Use IsSet() to check.
type Color struct {
	R, G, B, A uint8
	set        bool
}

// Predefined colors.
// exportaudit:keep — public palette for app and consumer code;
// newly exported names gain outside references as siblings adopt them.
var (
	Black            = Color{0, 0, 0, 255, true}
	Gray             = Color{128, 128, 128, 255, true}
	White            = Color{255, 255, 255, 255, true}
	Red              = Color{255, 0, 0, 255, true}
	Green            = Color{0, 255, 0, 255, true}
	Blue             = Color{0, 0, 255, 255, true}
	Yellow           = Color{255, 255, 0, 255, true}
	Magenta          = Color{255, 0, 255, 255, true}
	Orange           = Color{255, 165, 0, 255, true}
	Purple           = Color{128, 0, 128, 255, true}
	Indigo           = Color{75, 0, 130, 255, true}
	Pink             = Color{255, 192, 203, 255, true}
	Violet           = Color{238, 130, 238, 255, true}
	DarkBlue         = Color{0, 0, 139, 255, true}
	DarkGray         = Color{169, 169, 169, 255, true}
	DarkGreen        = Color{0, 100, 0, 255, true}
	DarkRed          = Color{139, 0, 0, 255, true}
	LightBlue        = Color{173, 216, 230, 255, true}
	LightGray        = Color{211, 211, 211, 255, true}
	LightGreen       = Color{144, 238, 144, 255, true}
	LightRed         = Color{255, 204, 203, 255, true}
	CornflowerBlue   = Color{100, 149, 237, 255, true}
	RoyalBlue        = Color{65, 105, 225, 255, true}
	ColorTransparent = Color{0, 0, 0, 0, true}
)

// Lowercase aliases for the pre-export spellings still used inside
// this module (error placeholders, benchmarks). New code uses the
// exported names above.
var (
	magenta   = Magenta
	lightGray = LightGray
)

// Hex creates a Color from a hexadecimal integer (0xRRGGBB).
func Hex(color int) Color {
	return Color{
		R:   uint8((color >> 16) & 0xFF),
		G:   uint8((color >> 8) & 0xFF),
		B:   uint8(color & 0xFF),
		A:   255,
		set: true,
	}
}

// RGB builds a Color from r, g, b values. Alpha defaults to 255.
func RGB(r, g, b uint8) Color {
	return Color{r, g, b, 255, true}
}

// RGBA builds a Color from r, g, b, a values.
func RGBA(r, g, b, a uint8) Color {
	return Color{r, g, b, a, true}
}

// IsSet reports whether the color was explicitly set (via a constructor
// or predefined var) as opposed to being the zero value.
func (c Color) IsSet() bool {
	return c.set
}

// WithOpacity returns color with alpha multiplied by opacity (0.0–1.0).
func (c Color) WithOpacity(opacity float32) Color {
	return Color{
		R:   c.R,
		G:   c.G,
		B:   c.B,
		A:   uint8(float32(c.A) * f32Clamp(opacity, 0, 1)),
		set: c.set,
	}
}

// Add returns c + b, clamping each channel to 255.
// The result is set if either input is set: combining two unset
// colors must stay unset rather than conjure an explicit color.
func (c Color) Add(b Color) Color {
	return Color{
		R:   clampAdd(c.R, b.R),
		G:   clampAdd(c.G, b.G),
		B:   clampAdd(c.B, b.B),
		A:   clampAdd(c.A, b.A),
		set: c.set || b.set,
	}
}

// Sub returns c - b, clamping each channel to 0.
// Set-propagation mirrors Add: unset in, unset out.
func (c Color) Sub(b Color) Color {
	ca := clampSub(c.A, b.A)
	return Color{
		R:   clampSub(c.R, b.R),
		G:   clampSub(c.G, b.G),
		B:   clampSub(c.B, b.B),
		A:   ca,
		set: c.set || b.set,
	}
}

// Over implements Porter-Duff "c over b" compositing.
func (c Color) Over(b Color) Color {
	ca := float32(c.A) / 255
	ba := float32(b.A) / 255
	ra := ca + ba*(1-ca)
	if ra == 0 {
		if c.set || b.set {
			return ColorTransparent
		}
		return Color{}
	}
	rr := (float32(c.R)*ca + float32(b.R)*ba*(1-ca)) / ra
	gr := (float32(c.G)*ca + float32(b.G)*ba*(1-ca)) / ra
	br := (float32(c.B)*ca + float32(b.B)*ba*(1-ca)) / ra
	return Color{
		R:   uint8(rr + 0.5),
		G:   uint8(gr + 0.5),
		B:   uint8(br + 0.5),
		A:   uint8(ra*255 + 0.5),
		set: c.set || b.set,
	}
}

// Eq checks if two colors are equal in every channel.
func (c Color) eq(c2 Color) bool {
	return c.R == c2.R && c.G == c2.G && c.B == c2.B && c.A == c2.A
}

// String returns a string representation.
func (c Color) String() string {
	return fmt.Sprintf("Color{%d, %d, %d, %d}", c.R, c.G, c.B, c.A)
}

// RGBA8 converts to an int in RGBA8 order.
func (c Color) RGBA8() int {
	return int(uint32(c.R)<<24 | uint32(c.G)<<16 | uint32(c.B)<<8 | uint32(c.A))
}

// BGRA8 converts to an int in BGRA8 order.
func (c Color) bGRA8() int {
	return int(uint32(c.B)<<24 | uint32(c.G)<<16 | uint32(c.R)<<8 | uint32(c.A))
}

// ABGR8 converts to an int in ABGR8 order.
func (c Color) aBGR8() int {
	return int(uint32(c.A)<<24 | uint32(c.B)<<16 | uint32(c.G)<<8 | uint32(c.R))
}

// ToCSSString returns CSS-compatible "rgba(r,g,b,a)" with alpha in
// 0–1, e.g. "rgba(10,20,30,0.50)".
func (c Color) toCSSString() string {
	return fmt.Sprintf("rgba(%d,%d,%d,%.2f)",
		c.R, c.G, c.B, float64(c.A)/255)
}

var stringColors = map[string]Color{
	"blue":            Blue,
	"red":             Red,
	"green":           Green,
	"yellow":          Yellow,
	"magenta":         Magenta,
	"orange":          Orange,
	"purple":          Purple,
	"black":           Black,
	"gray":            Gray,
	"indigo":          Indigo,
	"pink":            Pink,
	"violet":          Violet,
	"white":           White,
	"cornflower_blue": CornflowerBlue,
	"royal_blue":      RoyalBlue,
	"dark_blue":       DarkBlue,
	"dark_gray":       DarkGray,
	"dark_green":      DarkGreen,
	"dark_red":        DarkRed,
	"light_blue":      LightBlue,
	"light_gray":      LightGray,
	"light_green":     LightGreen,
	"light_red":       LightRed,
}

// ColorLookup returns the Color for a name ("red", "cornflower_blue")
// or "#RRGGBB[AA]" hex string, reporting ok=false for unknown input.
// Lookup trims surrounding space and folds case, so " Red " and
// "RED" both match. A hex field uses this while the user is still
// typing: ok=false leaves the current color untouched.
//
// exportaudit:keep — app-facing config/user-input lookup.
func ColorLookup(s string) (Color, bool) {
	t := strings.TrimSpace(s)
	// No valid input is longer than "cornflower_blue" (14): reject
	// longer strings before ToLower allocates for them.
	if len(t) > 32 {
		return Color{}, false
	}
	if len(t) > 0 && t[0] == '#' {
		return colorFromHexString(t)
	}
	if c, ok := stringColors[strings.ToLower(t)]; ok {
		return c, true
	}
	return Color{}, false
}

// ColorFromString returns a Color for the given name or "#RRGGBB" hex
// string. Unknown input returns opaque black; use ColorLookup when
// the caller must tell a typo apart from black.
func ColorFromString(s string) Color {
	if c, ok := ColorLookup(s); ok {
		return c
	}
	return Color{A: 255, set: true}
}

func clampAdd(a, b uint8) uint8 {
	s := int(a) + int(b)
	if s > 255 {
		return 255
	}
	return uint8(s)
}

func clampSub(a, b uint8) uint8 {
	s := int(a) - int(b)
	if s < 0 {
		return 0
	}
	return uint8(s)
}
