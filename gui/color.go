package gui

import "fmt"

// Color represents a 32-bit color value in sRGB format.
// The set field distinguishes "not set" (zero value) from intentionally
// transparent (RGBA 0,0,0,0). Use RGBA(), RGB(), Hex() constructors
// to create set colors. Use IsSet() to check.
type Color struct {
	R, G, B, A uint8
	set        bool
}

// Predefined colors.
var (
	Black            = Color{0, 0, 0, 255, true}
	Gray             = Color{128, 128, 128, 255, true}
	White            = Color{255, 255, 255, 255, true}
	Red              = Color{255, 0, 0, 255, true}
	Green            = Color{0, 255, 0, 255, true}
	Blue             = Color{0, 0, 255, 255, true}
	Yellow           = Color{255, 255, 0, 255, true}
	Magenta          = Color{255, 0, 255, 255, true}
	Cyan             = Color{0, 255, 255, 255, true}
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
func (c Color) Add(b Color) Color {
	return Color{
		R:   clampAdd(c.R, b.R),
		G:   clampAdd(c.G, b.G),
		B:   clampAdd(c.B, b.B),
		A:   clampAdd(c.A, b.A),
		set: true,
	}
}

// Sub returns c - b, clamping each channel to 0.
func (c Color) Sub(b Color) Color {
	ca := clampSub(c.A, b.A)
	return Color{
		R:   clampSub(c.R, b.R),
		G:   clampSub(c.G, b.G),
		B:   clampSub(c.B, b.B),
		A:   ca,
		set: true,
	}
}

// Over implements Porter-Duff "c over b" compositing.
func (c Color) Over(b Color) Color {
	ca := float32(c.A) / 255
	ba := float32(b.A) / 255
	ra := ca + ba*(1-ca)
	if ra == 0 {
		return ColorTransparent
	}
	rr := (float32(c.R)*ca + float32(b.R)*ba*(1-ca)) / ra
	gr := (float32(c.G)*ca + float32(b.G)*ba*(1-ca)) / ra
	br := (float32(c.B)*ca + float32(b.B)*ba*(1-ca)) / ra
	return Color{
		R:   uint8(rr),
		G:   uint8(gr),
		B:   uint8(br),
		A:   uint8(ra * 255),
		set: true,
	}
}

// Eq checks if two colors are equal in every channel.
func (c Color) Eq(c2 Color) bool {
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
func (c Color) BGRA8() int {
	return int(uint32(c.B)<<24 | uint32(c.G)<<16 | uint32(c.R)<<8 | uint32(c.A))
}

// ABGR8 converts to an int in ABGR8 order.
func (c Color) ABGR8() int {
	return int(uint32(c.A)<<24 | uint32(c.B)<<16 | uint32(c.G)<<8 | uint32(c.R))
}

// ToCSSString returns CSS-compatible "rgba(r,g,b,a)".
func (c Color) ToCSSString() string {
	return fmt.Sprintf("rgba(%d,%d,%d,%d)", c.R, c.G, c.B, c.A)
}

var stringColors = map[string]Color{
	"blue":            Blue,
	"red":             Red,
	"green":           Green,
	"yellow":          Yellow,
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

// ColorFromString returns a Color for the given name or "#RRGGBB" hex
// string. Returns Black if not found.
func ColorFromString(s string) Color {
	if len(s) > 0 && s[0] == '#' {
		c, ok := ColorFromHexString(s)
		if ok {
			return c
		}
		return Color{A: 255, set: true}
	}
	if c, ok := stringColors[s]; ok {
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
