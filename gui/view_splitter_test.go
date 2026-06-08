package gui

import (
	"encoding/json"
	"math"
	"testing"
)

func TestSplitterBasic(t *testing.T) {
	v := Splitter(SplitterCfg{
		ID:    "sp",
		Ratio: SomeF(0.5),
		First: SplitterPaneCfg{
			Content: []View{Text(TextCfg{Text: "left"})},
		},
		Second: SplitterPaneCfg{
			Content: []View{Text(TextCfg{Text: "right"})},
		},
		OnChange: func(_ float32, _ SplitterCollapsed, _ *Event, _ *Window) {},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	// Canvas: pane1, handle, pane2.
	if len(layout.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(layout.Children))
	}
}

func TestSplitAlias(t *testing.T) {
	v := Splitter(SplitterCfg{
		ID:       "sp",
		OnChange: func(_ float32, _ SplitterCollapsed, _ *Event, _ *Window) {},
		First:    SplitterPaneCfg{},
		Second:   SplitterPaneCfg{},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(layout.Children))
	}
}

func TestSplitterCompute(t *testing.T) {
	core := &splitterCore{
		ratio:      0.5,
		handleSize: 10,
		first:      splitterPaneCore{collapsible: true},
		second:     splitterPaneCore{collapsible: true},
	}
	c := splitterCompute(core, 200)
	if c.handleMain != 10 {
		t.Errorf("handle = %f, want 10", c.handleMain)
	}
	// available = 190, ratio 0.5 → first ~95
	if math.Abs(float64(c.firstMain-95)) > 1 {
		t.Errorf("first = %f, want ~95", c.firstMain)
	}
	if math.Abs(float64(c.secondMain-95)) > 1 {
		t.Errorf("second = %f, want ~95", c.secondMain)
	}
}

func TestSplitterComputeCollapsedFirst(t *testing.T) {
	core := &splitterCore{
		ratio:      0.5,
		handleSize: 10,
		collapsed:  SplitterCollapseFirst,
		first:      splitterPaneCore{collapsible: true, collapsedSize: 0},
		second:     splitterPaneCore{collapsible: true},
	}
	c := splitterCompute(core, 200)
	if c.firstMain != 0 {
		t.Errorf("first = %f, want 0", c.firstMain)
	}
	if c.secondMain != 190 {
		t.Errorf("second = %f, want 190", c.secondMain)
	}
}

func TestSplitterComputeCollapsedSecond(t *testing.T) {
	core := &splitterCore{
		ratio:      0.5,
		handleSize: 10,
		collapsed:  SplitterCollapseSecond,
		first:      splitterPaneCore{collapsible: true},
		second:     splitterPaneCore{collapsible: true, collapsedSize: 0},
	}
	c := splitterCompute(core, 200)
	if c.secondMain != 0 {
		t.Errorf("second = %f, want 0", c.secondMain)
	}
	if c.firstMain != 190 {
		t.Errorf("first = %f, want 190", c.firstMain)
	}
}

func TestSplitterClampRatio(t *testing.T) {
	core := &splitterCore{
		first:  splitterPaneCore{minSize: 50},
		second: splitterPaneCore{minSize: 50},
	}
	// With available=200, min 50 each → ratio clamped to [0.25, 0.75].
	r := splitterClampRatio(core, 200, 0.1)
	if r < 0.24 || r > 0.26 {
		t.Errorf("clamped low ratio = %f, want ~0.25", r)
	}
	r = splitterClampRatio(core, 200, 0.9)
	if r < 0.74 || r > 0.76 {
		t.Errorf("clamped high ratio = %f, want ~0.75", r)
	}
}

func TestSplitterNormalizeRatio(t *testing.T) {
	if r := splitterNormalizeRatio(-0.5); r != 0 {
		t.Errorf("got %f, want 0", r)
	}
	if r := splitterNormalizeRatio(1.5); r != 1 {
		t.Errorf("got %f, want 1", r)
	}
}

func TestSplitterStateNormalize(t *testing.T) {
	s := SplitterStateNormalize(SplitterState{Ratio: 1.5})
	if s.Ratio != 1 {
		t.Errorf("ratio = %f, want 1", s.Ratio)
	}
}

func TestSplitterEffectiveCollapsed(t *testing.T) {
	core := &splitterCore{
		first:  splitterPaneCore{collapsible: true},
		second: splitterPaneCore{collapsible: false},
	}
	if c := splitterEffectiveCollapsed(core, SplitterCollapseFirst); c != SplitterCollapseFirst {
		t.Error("first collapsible → first")
	}
	if c := splitterEffectiveCollapsed(core, SplitterCollapseSecond); c != SplitterCollapseNone {
		t.Error("second not collapsible → none")
	}
}

func TestSplitterButtonIcon(t *testing.T) {
	core := &splitterCore{
		orientation: SplitterHorizontal,
		first:       splitterPaneCore{collapsible: true},
		second:      splitterPaneCore{collapsible: true},
	}
	icon := splitterButtonIcon(core, SplitterCollapseFirst)
	if icon != "◀" {
		t.Errorf("got %q, want ◀", icon)
	}
	icon = splitterButtonIcon(core, SplitterCollapseSecond)
	if icon != "▶" {
		t.Errorf("got %q, want ▶", icon)
	}
}

func TestSplitterToggleTarget(t *testing.T) {
	core := &splitterCore{
		first:  splitterPaneCore{collapsible: true},
		second: splitterPaneCore{collapsible: true},
	}
	// Not collapsed → toggle targets first collapsible.
	if tgt := splitterToggleTarget(core, SplitterCollapseNone); tgt != SplitterCollapseFirst {
		t.Errorf("none → got %d, want first", tgt)
	}
	// Already collapsed first → returns first (to uncollapse).
	if tgt := splitterToggleTarget(core, SplitterCollapseFirst); tgt != SplitterCollapseFirst {
		t.Errorf("first → got %d, want first", tgt)
	}
	// Only second collapsible.
	core2 := &splitterCore{
		first:  splitterPaneCore{collapsible: false},
		second: splitterPaneCore{collapsible: true},
	}
	if tgt := splitterToggleTarget(core2, SplitterCollapseNone); tgt != SplitterCollapseSecond {
		t.Errorf("only-second → got %d, want second", tgt)
	}
	// Neither collapsible.
	core3 := &splitterCore{}
	if tgt := splitterToggleTarget(core3, SplitterCollapseNone); tgt != SplitterCollapseNone {
		t.Errorf("neither → got %d, want none", tgt)
	}
}

func TestSplitterToggleCollapse(t *testing.T) {
	core := &splitterCore{
		first:  splitterPaneCore{collapsible: true},
		second: splitterPaneCore{collapsible: true},
	}
	// Not collapsed → collapse first.
	next, ok := splitterToggleCollapse(core, SplitterCollapseNone)
	if !ok || next != SplitterCollapseFirst {
		t.Errorf("none → got %d/%v, want first/true", next, ok)
	}
	// Already collapsed first → uncollapse.
	next, ok = splitterToggleCollapse(core, SplitterCollapseFirst)
	if !ok || next != SplitterCollapseNone {
		t.Errorf("first → got %d/%v, want none/true", next, ok)
	}
	// Neither collapsible → no-op.
	core2 := &splitterCore{}
	next, ok = splitterToggleCollapse(core2, SplitterCollapseNone)
	if ok {
		t.Errorf("neither → got %d/%v, want unchanged/false", next, ok)
	}
}

func TestSplitterArrowStep(t *testing.T) {
	core := &splitterCore{
		orientation:   SplitterHorizontal,
		dragStep:      0.05,
		dragStepLarge: 0.10,
	}
	// Matching orientation, no modifier → step applied.
	r, ok := splitterArrowStep(core, SplitterHorizontal, +1,
		ModNone, 200, 0.5)
	if !ok {
		t.Fatal("expected handled=true for matching orientation")
	}
	if r <= 0.5 {
		t.Errorf("expected ratio > 0.5, got %f", r)
	}
	// Wrong orientation → not handled.
	_, ok = splitterArrowStep(core, SplitterVertical, +1,
		ModNone, 200, 0.5)
	if ok {
		t.Error("expected handled=false for wrong orientation")
	}
	// Shift modifier → large step.
	rSmall, _ := splitterArrowStep(core, SplitterHorizontal, +1,
		ModNone, 200, 0.5)
	rLarge, _ := splitterArrowStep(core, SplitterHorizontal, +1,
		ModShift, 200, 0.5)
	if rLarge <= rSmall {
		t.Errorf("shift step (%f) should exceed normal (%f)", rLarge, rSmall)
	}
	// Unsupported modifier → not handled.
	_, ok = splitterArrowStep(core, SplitterHorizontal, +1,
		ModCtrl, 200, 0.5)
	if ok {
		t.Error("expected handled=false for ctrl modifier")
	}
}

// --- MarshalText / UnmarshalText ---

func TestSplitterOrientationMarshalText(t *testing.T) {
	tests := []struct {
		o    SplitterOrientation
		want string
	}{
		{SplitterHorizontal, "horizontal"},
		{SplitterVertical, "vertical"},
	}
	for _, tt := range tests {
		b, err := tt.o.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText(%d): %v", tt.o, err)
		}
		if string(b) != tt.want {
			t.Errorf("got %q, want %q", b, tt.want)
		}
		var got SplitterOrientation
		if err := got.UnmarshalText(b); err != nil {
			t.Fatalf("UnmarshalText(%q): %v", b, err)
		}
		if got != tt.o {
			t.Errorf("round-trip: got %d, want %d", got, tt.o)
		}
	}
}

func TestSplitterOrientationUnmarshalTextUnknown(t *testing.T) {
	var o SplitterOrientation
	if err := o.UnmarshalText([]byte("diagonal")); err == nil {
		t.Error("expected error for unknown value")
	}
}

func TestSplitterCollapsedMarshalText(t *testing.T) {
	tests := []struct {
		c    SplitterCollapsed
		want string
	}{
		{SplitterCollapseNone, "none"},
		{SplitterCollapseFirst, "first"},
		{SplitterCollapseSecond, "second"},
	}
	for _, tt := range tests {
		b, err := tt.c.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText(%d): %v", tt.c, err)
		}
		if string(b) != tt.want {
			t.Errorf("got %q, want %q", b, tt.want)
		}
		var got SplitterCollapsed
		if err := got.UnmarshalText(b); err != nil {
			t.Fatalf("UnmarshalText(%q): %v", b, err)
		}
		if got != tt.c {
			t.Errorf("round-trip: got %d, want %d", got, tt.c)
		}
	}
}

func TestSplitterCollapsedUnmarshalTextUnknown(t *testing.T) {
	var c SplitterCollapsed
	if err := c.UnmarshalText([]byte("both")); err == nil {
		t.Error("expected error for unknown value")
	}
}

func TestSplitterCollapsedMarshalTextUnknown(t *testing.T) {
	_, err := SplitterCollapsed(99).MarshalText()
	if err == nil {
		t.Error("expected error for unknown value")
	}
}

// --- SplitterStateNormalize edge cases ---

func TestSplitterStateNormalizeNaN(t *testing.T) {
	s := SplitterStateNormalize(SplitterState{
		Ratio: float32(math.NaN()),
	})
	if s.Ratio != splitterDefaultRatio {
		t.Errorf("NaN ratio = %f, want %f", s.Ratio, splitterDefaultRatio)
	}
}

func TestSplitterStateNormalizeInf(t *testing.T) {
	s := SplitterStateNormalize(SplitterState{
		Ratio: float32(math.Inf(1)),
	})
	if s.Ratio != splitterDefaultRatio {
		t.Errorf("+Inf ratio = %f, want %f", s.Ratio, splitterDefaultRatio)
	}
	s = SplitterStateNormalize(SplitterState{
		Ratio: float32(math.Inf(-1)),
	})
	if s.Ratio != splitterDefaultRatio {
		t.Errorf("-Inf ratio = %f, want %f", s.Ratio, splitterDefaultRatio)
	}
}

func TestSplitterStateNormalizeInvalidCollapsed(t *testing.T) {
	s := SplitterStateNormalize(SplitterState{
		Ratio:     0.5,
		Collapsed: SplitterCollapsed(99),
	})
	if s.Collapsed != SplitterCollapseNone {
		t.Errorf("collapsed = %d, want SplitterCollapseNone", s.Collapsed)
	}
}

func TestSplitterStateNormalizeNegativeRatio(t *testing.T) {
	s := SplitterStateNormalize(SplitterState{Ratio: -0.5})
	if s.Ratio != 0 {
		t.Errorf("negative ratio = %f, want 0", s.Ratio)
	}
}

// --- SplitterState JSON round-trip ---

func TestSplitterStateJSONRoundTrip(t *testing.T) {
	orig := SplitterState{Ratio: 0.3, Collapsed: SplitterCollapseFirst}
	data, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if s != `{"ratio":0.3,"collapsed":"first"}` {
		t.Errorf("JSON = %s", s)
	}
	var got SplitterState
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != orig {
		t.Errorf("round-trip: got %+v, want %+v", got, orig)
	}
}
