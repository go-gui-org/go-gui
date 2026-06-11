package gui

import "testing"

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
		{"BGRA8", c.BGRA8(), 0xCCBBAADD},
		{"ABGR8", c.ABGR8(), 0xDDCCBBAA},
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
		if !c.Eq(Red) {
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
	a := Color{R: 255, A: 255, set: true}
	b := Color{R: 255, A: 255}
	if !a.Eq(b) {
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
	want := "rgba(10,20,30,128)"
	if got := c.ToCSSString(); got != want {
		t.Errorf("ToCSSString() = %q, want %q", got, want)
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
