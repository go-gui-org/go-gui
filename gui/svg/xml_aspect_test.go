package svg

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestParsePreserveAspectRatio(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in        string
		wantAlign gui.SvgAlign
		wantSlice bool
	}{
		{"", gui.SvgAlignXMidYMid, false},
		{"xMidYMid meet", gui.SvgAlignXMidYMid, false},
		{"xMidYMid", gui.SvgAlignXMidYMid, false},
		{"xMinYMin meet", gui.SvgAlignXMinYMin, false},
		{"xMaxYMax slice", gui.SvgAlignXMaxYMax, true},
		{"none", gui.SvgAlignNone, false},
		{"none slice", gui.SvgAlignNone, true},
		{"  xMidYMax   slice  ", gui.SvgAlignXMidYMax, true},
		// Unknown tokens are ignored — defaults preserved.
		{"defer xMidYMid meet", gui.SvgAlignXMidYMid, false},
		// Bogus input → defaults.
		{"garbage", gui.SvgAlignXMidYMid, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()
			a, s := parsePreserveAspectRatio(tt.in)
			if a != tt.wantAlign {
				t.Errorf("align = %v, want %v", a, tt.wantAlign)
			}
			if s != tt.wantSlice {
				t.Errorf("slice = %v, want %v", s, tt.wantSlice)
			}
		})
	}
}

func TestParsePreserveAspectRatioFromRoot(t *testing.T) {
	t.Parallel()
	src := `<svg xmlns="http://www.w3.org/2000/svg"
		preserveAspectRatio="xMaxYMin slice"></svg>`
	vg, err := parseSvg(src)
	if err != nil {
		t.Fatalf("parseSvg: %v", err)
	}
	if vg.PreserveAlign != gui.SvgAlignXMaxYMin {
		t.Errorf("PreserveAlign = %v, want SvgAlignXMaxYMin",
			vg.PreserveAlign)
	}
	if !vg.PreserveSlice {
		t.Error("PreserveSlice = false, want true")
	}
}

func TestParsePreserveAspectRatioDefault(t *testing.T) {
	t.Parallel()
	src := `<svg xmlns="http://www.w3.org/2000/svg"></svg>`
	vg, err := parseSvg(src)
	if err != nil {
		t.Fatalf("parseSvg: %v", err)
	}
	if vg.PreserveAlign != gui.SvgAlignXMidYMid {
		t.Errorf("default align = %v, want SvgAlignXMidYMid",
			vg.PreserveAlign)
	}
	if vg.PreserveSlice {
		t.Error("default slice should be false")
	}
}

func TestParsePreserveAspectRatioBoundsLongInput(t *testing.T) {
	t.Parallel()
	// Pathological attribute: huge whitespace + valid token at end.
	// Cap is 64 chars, so the trailing valid token is sliced off
	// and parser falls back to default.
	huge := strings.Repeat(" ", 5000) + "xMaxYMax slice"
	a, s := parsePreserveAspectRatio(huge)
	if a != gui.SvgAlignXMidYMid {
		t.Errorf("align = %v, want default xMidYMid (truncated)", a)
	}
	if s {
		t.Error("slice = true; expected false (truncated)")
	}
	// Valid input within bound still parses normally.
	a2, s2 := parsePreserveAspectRatio("xMaxYMax slice")
	if a2 != gui.SvgAlignXMaxYMax || !s2 {
		t.Errorf("normal input misparsed: align=%v slice=%v", a2, s2)
	}
}
