package gui

import (
	"math"
	"strings"
	"testing"
)

func TestQuantizeFlatnessHostileInputs(t *testing.T) {
	cases := []struct {
		name string
		in   float32
		want int32
	}{
		{"NaN", float32(math.NaN()), 0},
		{"+Inf", float32(math.Inf(1)), 0},
		{"-Inf", float32(math.Inf(-1)), 0},
		{"negative", -1, 0},
		{"zero", 0, 0},
		{"valid", 0.5, 5000},
		{"overflow", 1e30, math.MaxInt32},
	}
	for _, c := range cases {
		got := quantizeFlatness(c.in)
		if got != c.want {
			t.Errorf("quantizeFlatness(%v) = %v want %v",
				c.in, got, c.want)
		}
	}
}

func TestClampSvgCacheIDTruncates(t *testing.T) {
	if got := clampSvgCacheID("abc"); got != "abc" {
		t.Errorf("short id mutated: %q", got)
	}
	huge := strings.Repeat("y", 10000)
	got := clampSvgCacheID(huge)
	if len(got) != maxSvgCacheElementIDLen {
		t.Errorf("expected len %d, got %d",
			maxSvgCacheElementIDLen, len(got))
	}
}

func TestBuildSvgCacheLookupKeyDifferentiates(t *testing.T) {
	base := SvgParseOpts{}
	a := buildSvgCacheLookupKey(0, 0, 10, 10, base)

	flat := SvgParseOpts{FlatnessTolerance: 0.5}
	b := buildSvgCacheLookupKey(0, 0, 10, 10, flat)
	if a == b {
		t.Errorf("flatness must produce distinct cache key")
	}

	hov := SvgParseOpts{HoveredElementID: "x"}
	c := buildSvgCacheLookupKey(0, 0, 10, 10, hov)
	if a == c {
		t.Errorf("hovered id must produce distinct cache key")
	}

	foc := SvgParseOpts{FocusedElementID: "x"}
	d := buildSvgCacheLookupKey(0, 0, 10, 10, foc)
	if a == d {
		t.Errorf("focused id must produce distinct cache key")
	}
	if c == d {
		t.Errorf("hovered=x and focused=x must produce distinct keys")
	}
}

func TestBuildSvgCacheLookupKeySanitizesHostileInputs(t *testing.T) {
	bad := SvgParseOpts{
		FlatnessTolerance: float32(math.NaN()),
		HoveredElementID:  strings.Repeat("z", 5000),
	}
	k := buildSvgCacheLookupKey(0, 0, 10, 10, bad)
	if k.flatness10000 != 0 {
		t.Errorf("NaN flatness must quantize to 0, got %d", k.flatness10000)
	}
	if len(k.hoveredID) > maxSvgCacheElementIDLen {
		t.Errorf("hostile hovered id not capped: len=%d",
			len(k.hoveredID))
	}
}
