package gui

import (
	"strings"
	"testing"
)

func TestColorIsSet(t *testing.T) {
	t.Parallel()
	t.Run("RGBA", func(t *testing.T) {
		t.Parallel()
		if !RGBA(0, 0, 0, 0).IsSet() {
			t.Fatal("RGBA should produce a set color")
		}
	})
	t.Run("RGB", func(t *testing.T) {
		t.Parallel()
		c := RGB(10, 20, 30)
		if !c.IsSet() {
			t.Fatal("RGB should produce a set color")
		}
		if c.A != 255 {
			t.Fatalf("RGB alpha: got %d, want 255", c.A)
		}
	})
	t.Run("Hex", func(t *testing.T) {
		t.Parallel()
		if !Hex(0xFF0000).IsSet() {
			t.Fatal("Hex should produce a set color")
		}
	})
	t.Run("zero", func(t *testing.T) {
		t.Parallel()
		var c Color
		if c.IsSet() {
			t.Fatal("zero Color should not be set")
		}
	})
	t.Run("predefined", func(t *testing.T) {
		t.Parallel()
		for _, c := range []Color{
			Black, White, Red, Green, Blue, ColorTransparent,
		} {
			if !c.IsSet() {
				t.Fatalf("predefined color %v should be set", c)
			}
		}
	})
	t.Run("transparent", func(t *testing.T) {
		t.Parallel()
		if !ColorTransparent.IsSet() {
			t.Fatal("ColorTransparent should be set")
		}
		if ColorTransparent.R != 0 || ColorTransparent.A != 0 {
			t.Fatal("ColorTransparent should be fully transparent")
		}
	})
	t.Run("WithOpacity", func(t *testing.T) {
		t.Parallel()
		if !RGBA(255, 0, 0, 255).WithOpacity(0.5).IsSet() {
			t.Fatal("WithOpacity should preserve set")
		}
	})
	t.Run("Add", func(t *testing.T) {
		t.Parallel()
		if !Red.Add(Blue).IsSet() {
			t.Fatal("Add should produce set color")
		}
	})
	t.Run("Sub", func(t *testing.T) {
		t.Parallel()
		if !White.Sub(Red).IsSet() {
			t.Fatal("Sub should produce set color")
		}
	})
	t.Run("Over", func(t *testing.T) {
		t.Parallel()
		if !Red.WithOpacity(0.5).Over(Blue).IsSet() {
			t.Fatal("Over should produce set color")
		}
	})
}

func TestColorByteOrder(t *testing.T) {
	t.Parallel()
	c := RGBA(0xAA, 0xBB, 0xCC, 0xDD)
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"RGBA8", c.RGBA8(), 0xAABBCCDD},
		{"BGRA8", c.bGRA8(), 0xCCBBAADD},
		{"ABGR8", c.aBGR8(), 0xDDCCBBAA},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Errorf("%s() = 0x%X, want 0x%X",
					tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestColorSub(t *testing.T) {
	t.Parallel()
	t.Run("subtracts_alpha", func(t *testing.T) {
		t.Parallel()
		r := RGBA(200, 200, 200, 100).Sub(RGBA(50, 50, 50, 50))
		if r.A != 50 {
			t.Errorf("Sub alpha: got %d, want 50", r.A)
		}
	})
	t.Run("clamps_to_zero", func(t *testing.T) {
		t.Parallel()
		r := RGB(10, 10, 10).Sub(RGB(20, 20, 20))
		if r.R != 0 || r.G != 0 || r.B != 0 || r.A != 0 {
			t.Errorf("Sub should clamp to 0: got %v", r)
		}
	})
	t.Run("clamps_alpha_to_zero", func(t *testing.T) {
		t.Parallel()
		r := RGBA(200, 200, 200, 50).Sub(RGBA(10, 10, 10, 200))
		if r.A != 0 {
			t.Errorf("Sub alpha: got %d, want 0", r.A)
		}
	})
}

func TestColorFromString(t *testing.T) {
	t.Parallel()
	t.Run("named", func(t *testing.T) {
		t.Parallel()
		c := ColorFromString("red")
		if !c.eq(Red) {
			t.Errorf("ColorFromString(red) = %v, want %v", c, Red)
		}
	})
	t.Run("hex", func(t *testing.T) {
		t.Parallel()
		c := ColorFromString("#FF0000")
		if c.R != 255 || c.G != 0 || c.B != 0 {
			t.Errorf("ColorFromString(#FF0000) = %v, want red", c)
		}
	})
	t.Run("invalid_hex", func(t *testing.T) {
		t.Parallel()
		c := ColorFromString("#ZZZZZZ")
		if c.R != 0 || c.G != 0 || c.B != 0 || c.A != 255 {
			t.Errorf("invalid hex should return black: %v", c)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		t.Parallel()
		c := ColorFromString("chartreuse")
		if c.A != 255 || !c.IsSet() {
			t.Errorf("unknown name should return black: %v", c)
		}
	})
}

func TestEqIgnoresSet(t *testing.T) {
	t.Parallel()
	a := RGBA(255, 0, 0, 255)
	b := RGBA(255, 0, 0, 255)
	if !a.eq(b) {
		t.Fatal("Eq should compare only RGBA channels")
	}
}

func TestColorString(t *testing.T) {
	t.Parallel()
	c := RGB(10, 20, 30)
	want := "Color{10, 20, 30, 255}"
	if got := c.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestColorToCSSString(t *testing.T) {
	t.Parallel()
	c := RGBA(10, 20, 30, 128)
	want := "rgba(10,20,30,0.50)"
	if got := c.toCSSString(); got != want {
		t.Errorf("ToCSSString() = %q, want %q", got, want)
	}
	if got := RGB(10, 20, 30).toCSSString(); got != "rgba(10,20,30,1.00)" {
		t.Errorf("opaque ToCSSString() = %q", got)
	}
	if got := ColorTransparent.toCSSString(); got != "rgba(0,0,0,0.00)" {
		t.Errorf("transparent ToCSSString() = %q", got)
	}
}

func TestColorLookup(t *testing.T) {
	t.Parallel()
	if c, ok := ColorLookup("red"); !ok || !c.eq(Red) {
		t.Errorf("ColorLookup(red) = %v,%v, want Red", c, ok)
	}
	if c, ok := ColorLookup("Magenta"); !ok || !c.eq(Magenta) {
		t.Errorf("ColorLookup(Magenta) = %v,%v, want Magenta", c, ok)
	}
	if c, ok := ColorLookup("  cornflower_blue  "); !ok || !c.eq(CornflowerBlue) {
		t.Errorf("ColorLookup(padded) = %v,%v", c, ok)
	}
	if c, ok := ColorLookup("#FF0000"); !ok || c.R != 255 || c.G != 0 || c.B != 0 {
		t.Errorf("ColorLookup(#FF0000) = %v,%v", c, ok)
	}
	for _, s := range []string{"chartreuse", "#ZZZZZZ", "", "#", "   "} {
		if c, ok := ColorLookup(s); ok {
			t.Errorf("ColorLookup(%q) = %v, want ok=false", s, c)
		}
	}
	if _, ok := ColorLookup(strings.Repeat("r", 64)); ok {
		t.Error("ColorLookup(oversize) should return ok=false")
	}
}

func TestColorFromStringNormalized(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		in   string
		want Color
	}{
		{"RED", Red},
		{" Magenta ", Magenta},
		{"CORNFLOWER_BLUE", CornflowerBlue},
	} {
		if got := ColorFromString(tc.in); !got.eq(tc.want) {
			t.Errorf("ColorFromString(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestHexChannels(t *testing.T) {
	t.Parallel()
	c := Hex(0x1A2B3C)
	if c.R != 0x1A || c.G != 0x2B || c.B != 0x3C || c.A != 255 {
		t.Errorf("Hex channels wrong: %v", c)
	}
}

func TestOverBothTransparent(t *testing.T) {
	t.Parallel()
	r := RGBA(0, 0, 0, 0).Over(RGBA(0, 0, 0, 0))
	if !r.IsSet() {
		t.Error("Over of two transparent colors should return ColorTransparent (set)")
	}
	if r.A != 0 {
		t.Errorf("expected alpha 0, got %d", r.A)
	}
}

func TestOverSemiTransparent(t *testing.T) {
	t.Parallel()
	r := RGBA(255, 0, 0, 128).Over(RGBA(0, 0, 255, 255))
	if r.A == 0 {
		t.Error("Over result should not be fully transparent")
	}
	if r.R == 0 {
		t.Error("Over result should have some red")
	}
	if r.B == 0 {
		t.Error("Over result should have some blue")
	}
}

func TestOverRoundsToNearest(t *testing.T) {
	t.Parallel()
	// Half-opaque red over opaque blue: R≈128, G=0, B≈127, A=255.
	// Tolerance ±1 keeps float32 error out of the assertion while
	// catching a truncation regression (which reads 1 low).
	r := RGBA(255, 0, 0, 128).Over(RGBA(0, 0, 255, 255))
	for _, ch := range []struct {
		name string
		got  uint8
		want uint8
	}{
		{"R", r.R, 128}, {"G", r.G, 0}, {"B", r.B, 127}, {"A", r.A, 255},
	} {
		if d := int(ch.got) - int(ch.want); d < -1 || d > 1 {
			t.Errorf("Over %s = %d, want ~%d", ch.name, ch.got, ch.want)
		}
	}
}

func TestAddClampsTo255(t *testing.T) {
	t.Parallel()
	r := RGB(200, 200, 200).Add(RGB(200, 200, 200))
	if r.R != 255 || r.G != 255 || r.B != 255 {
		t.Errorf("Add should clamp to 255: got %v", r)
	}
}

func TestWithOpacityClampsRange(t *testing.T) {
	t.Parallel()
	c := RGB(255, 0, 0)
	over := c.WithOpacity(2.0)
	if over.A != 255 {
		t.Errorf("WithOpacity(2.0) should clamp: got alpha %d", over.A)
	}
	under := c.WithOpacity(-1.0)
	if under.A != 0 {
		t.Errorf("WithOpacity(-1.0) should clamp: got alpha %d", under.A)
	}
}

func TestArithUnsetPropagation(t *testing.T) {
	t.Parallel()
	var unset Color
	if got := unset.Add(unset); got.IsSet() {
		t.Errorf("Add(unset, unset) = %v, want unset", got)
	}
	if got := unset.Sub(unset); got.IsSet() {
		t.Errorf("Sub(unset, unset) = %v, want unset", got)
	}
	if got := unset.Over(unset); got.IsSet() {
		t.Errorf("Over(unset, unset) = %v, want unset", got)
	}
	// One set input keeps the result set.
	if got := Red.Add(unset); !got.IsSet() {
		t.Errorf("Add(set, unset) = %v, want set", got)
	}
	if got := unset.Sub(Red); !got.IsSet() {
		t.Errorf("Sub(unset, set) = %v, want set", got)
	}
	if got := unset.Over(Red); !got.IsSet() {
		t.Errorf("Over(unset, set) = %v, want set", got)
	}
}

func TestPredefinedColorsSet(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]Color{
		"Black": Black, "Gray": Gray, "White": White,
		"Red": Red, "Green": Green, "Blue": Blue,
		"Yellow": Yellow, "Magenta": Magenta, "Orange": Orange,
		"Purple": Purple, "Indigo": Indigo, "Pink": Pink,
		"Violet": Violet, "DarkBlue": DarkBlue, "DarkGray": DarkGray,
		"DarkGreen": DarkGreen, "DarkRed": DarkRed,
		"LightBlue": LightBlue, "LightGray": LightGray,
		"LightGreen": LightGreen, "LightRed": LightRed,
		"CornflowerBlue": CornflowerBlue, "RoyalBlue": RoyalBlue,
		"ColorTransparent": ColorTransparent,
	} {
		if !c.IsSet() {
			t.Errorf("%s should be set", name)
		}
	}
}
