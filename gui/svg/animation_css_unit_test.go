package svg

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestParseTimeValue_CSSCases(t *testing.T) {
	cases := []struct {
		in   string
		want float32
	}{
		{"1s", 1},
		{"0.25s", 0.25},
		{"200ms", 0.2},
		{"-1s", -1},
		{"1.5", 1.5}, // bare number = seconds (tolerant)
		{"", 0},
		{"abc", 0},
		{"  500ms  ", 0.5},
	}
	for _, tc := range cases {
		got := parseTimeValue(tc.in)
		if got != tc.want {
			t.Errorf("parseTimeValue(%q) = %v, want %v",
				tc.in, got, tc.want)
		}
	}
}

func TestParseIterCount_Table(t *testing.T) {
	cases := []struct {
		in   string
		want uint16
	}{
		{"infinite", gui.SvgAnimIterInfinite},
		{"INFINITE", gui.SvgAnimIterInfinite},
		{"1", 1},
		{"3", 3},
		{"0", 1},                               // <=0 clamps to 1
		{"-2", 1},                              // negative clamps to 1
		{"0.5", 1},                             // fractional below 1 clamps up
		{"abc", 1},                             // parse fail → 0 → clamps to 1
		{"99999", gui.SvgAnimIterInfinite - 1}, // saturates below sentinel
	}
	for _, tc := range cases {
		got := parseIterCount(tc.in)
		if got != tc.want {
			t.Errorf("parseIterCount(%q) = %d, want %d",
				tc.in, got, tc.want)
		}
	}
}

func TestApplyAnimShorthand_TwoTimesAssignsDurationThenDelay(t *testing.T) {
	var s cssAnimSpec
	applyAnimShorthand("2s 0.5s spin", &s)
	if s.DurationSec != 2 {
		t.Errorf("Duration=%v want 2", s.DurationSec)
	}
	if s.DelaySec != 0.5 {
		t.Errorf("Delay=%v want 0.5", s.DelaySec)
	}
	if s.Name != "spin" {
		t.Errorf("Name=%q want spin", s.Name)
	}
}

func TestApplyAnimShorthand_PreservesCubicBezierAsSingleToken(t *testing.T) {
	var s cssAnimSpec
	applyAnimShorthand("1s cubic-bezier(0.25,0.1,0.25,1) spin", &s)
	if s.TimingFn != cssAnimTimingCubic {
		t.Errorf("TimingFn=%v want cubic", s.TimingFn)
	}
	if s.TimingArgs != [4]float32{0.25, 0.1, 0.25, 1} {
		t.Errorf("TimingArgs=%v", s.TimingArgs)
	}
	if s.Name != "spin" {
		t.Errorf("Name=%q want spin", s.Name)
	}
}

func TestApplyAnimShorthand_DirectionAndFillMode(t *testing.T) {
	var s cssAnimSpec
	applyAnimShorthand("2s alternate forwards spin", &s)
	if s.Direction != cssAnimDirAlternate {
		t.Errorf("Direction=%v want alternate", s.Direction)
	}
	if s.FillMode != cssAnimFillForwards {
		t.Errorf("FillMode=%v want forwards", s.FillMode)
	}
}

func TestApplyAnimShorthand_IterationCountInfinite(t *testing.T) {
	var s cssAnimSpec
	applyAnimShorthand("1s infinite spin", &s)
	if s.IterCount != gui.SvgAnimIterInfinite || !s.IterCountSet {
		t.Errorf("IterCount=%d set=%v want infinite,true",
			s.IterCount, s.IterCountSet)
	}
}

func TestApplyCSSAnimProp_AnimationNoneResetsName(t *testing.T) {
	s := cssAnimSpec{Name: "spin", DurationSec: 5}
	if !applyCSSAnimProp("animation", "none", &s) {
		t.Fatal("applyCSSAnimProp returned false")
	}
	if s.Name != "" || s.DurationSec != 0 {
		t.Errorf("expected fully reset spec; got %+v", s)
	}
}

func TestApplyCSSAnimProp_UnknownReturnsFalse(t *testing.T) {
	var s cssAnimSpec
	if applyCSSAnimProp("animation-bogus", "1s", &s) {
		t.Error("unknown sub-property should return false")
	}
}

func TestSplitShorthandTokens_NestedParensStayOneToken(t *testing.T) {
	got := splitShorthandTokens("a cubic-bezier(0,0,1,1) b")
	want := []string{"a", "cubic-bezier(0,0,1,1)", "b"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d (%v)", len(got), len(want), got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] %q want %q", i, got[i], want[i])
		}
	}
}
