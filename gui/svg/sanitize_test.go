package svg

import (
	"math"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui/svg/css"
)

func TestSanitizeFlatnessHostileInputs(t *testing.T) {
	cases := []struct {
		name string
		in   float32
		want float32
	}{
		{"NaN", float32(math.NaN()), 0},
		{"+Inf", float32(math.Inf(1)), 0},
		{"-Inf", float32(math.Inf(-1)), 0},
		{"negative", -1, 0},
		{"zero", 0, 0},
		{"valid", 0.5, 0.5},
		{"at cap", maxFlatnessTolerance, maxFlatnessTolerance},
		{"over cap", 1000, maxFlatnessTolerance},
	}
	for _, c := range cases {
		got := sanitizeFlatness(c.in)
		if got != c.want {
			t.Errorf("sanitizeFlatness(%v) = %v want %v",
				c.in, got, c.want)
		}
	}
}

func TestClampElementIDTruncates(t *testing.T) {
	short := "abc"
	if got := clampElementID(short); got != short {
		t.Errorf("short id mutated: %q", got)
	}
	huge := strings.Repeat("x", 10000)
	got := clampElementID(huge)
	if len(got) != maxElementIDLen {
		t.Errorf("expected len %d, got %d", maxElementIDLen, len(got))
	}
}

func TestApplyPseudoStateHoverFocusMatch(t *testing.T) {
	state := &parseState{hoveredID: "h", focusedID: "f"}
	tests := []struct {
		id          string
		wantHover   bool
		wantFocused bool
	}{
		{"h", true, false},
		{"f", false, true},
		{"x", false, false},
		{"", false, false},
	}
	for _, tc := range tests {
		info := css.ElementInfo{ID: tc.id}
		applyPseudoState(&info, state)
		if info.State.Hover != tc.wantHover ||
			info.State.Focus != tc.wantFocused {
			t.Errorf("id=%q got hover=%v focus=%v, want hover=%v focus=%v",
				tc.id, info.State.Hover, info.State.Focus,
				tc.wantHover, tc.wantFocused)
		}
	}
}

func TestApplyPseudoStateNilStateNoop(t *testing.T) {
	info := css.ElementInfo{ID: "h"}
	applyPseudoState(&info, nil)
	if info.State.Hover || info.State.Focus {
		t.Fatalf("nil state must not toggle pseudo flags")
	}
}

func TestApplyPseudoStateEmptyIDsDisable(t *testing.T) {
	state := &parseState{}
	info := css.ElementInfo{ID: ""}
	applyPseudoState(&info, state)
	if info.State.Hover || info.State.Focus {
		t.Fatalf("empty hovered/focused IDs must not match anything")
	}
	info2 := css.ElementInfo{ID: "any"}
	applyPseudoState(&info2, state)
	if info2.State.Hover || info2.State.Focus {
		t.Fatalf("empty IDs in state must not match element id=any")
	}
}
