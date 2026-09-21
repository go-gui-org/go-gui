package gui

import "testing"

// Tests for the Selected and Disabled slots of ColorSet (#741).

// selectedTestSet is pickTestSet plus the two new slots. Selected is a
// mid-tone so the OKLCH step moves it both ways without clamping.
var selectedTestSet = ColorSet{
	Base:        RGB(1, 1, 1),
	Hover:       RGB(2, 2, 2),
	Click:       RGB(3, 3, 3),
	Focus:       RGB(4, 4, 4),
	Border:      RGB(5, 5, 5),
	BorderFocus: RGB(6, 6, 6),
	Selected:    RGB(40, 90, 200),
	Disabled:    RGB(7, 7, 7),
}

// A selected fill keeps pick's order but starts from Selected: hover
// and press are one OKLCH step up and down, focus and rest are
// Selected. The border rule does not change with selection.
func TestColorSetPickSelected(t *testing.T) {
	cs := selectedTestSet
	up := accentShift(cs.Selected, oklchRampDelta)
	down := accentShift(cs.Selected, -oklchRampDelta)
	tests := []struct {
		name         string
		flags        stateFlags
		fill, border Color
	}{
		{"resting", stateFlags{selected: true}, cs.Selected, cs.Border},
		{"hovered", stateFlags{selected: true, hovered: true}, up, cs.Border},
		{"pressed", stateFlags{selected: true, pressed: true}, down, cs.Border},
		{"focused", stateFlags{selected: true, focused: true},
			cs.Selected, cs.BorderFocus},
		{"pressed beats hovered",
			stateFlags{selected: true, pressed: true, hovered: true},
			down, cs.Border},
		{"focused and hovered",
			stateFlags{selected: true, focused: true, hovered: true},
			up, cs.BorderFocus},
		// Disabled wins over selected, as it wins over everything.
		{"disabled", stateFlags{selected: true, disabled: true},
			cs.Disabled, cs.Border},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fill, border := cs.pick(tc.flags)
			if fill != tc.fill {
				t.Errorf("fill = %v, want %v", fill, tc.fill)
			}
			if border != tc.border {
				t.Errorf("border = %v, want %v", border, tc.border)
			}
		})
	}
	if up == cs.Selected || down == cs.Selected {
		t.Fatal("the OKLCH step did not move Selected; the test proves nothing")
	}
}

// With no Selected anywhere, a selected element paints as an
// unselected one rather than as an unset color.
func TestColorSetPickSelectedUnsetFallsThrough(t *testing.T) {
	cs := pickTestSet
	fill, _ := cs.pick(stateFlags{selected: true, hovered: true})
	if fill != cs.Hover {
		t.Errorf("fill = %v, want Hover %v", fill, cs.Hover)
	}
}

// A disabled fill is Disabled when set, and Base when not (the rule
// before #741).
func TestColorSetPickDisabledSlot(t *testing.T) {
	fill, border := selectedTestSet.pick(stateFlags{disabled: true, hovered: true})
	if fill != selectedTestSet.Disabled || border != selectedTestSet.Border {
		t.Errorf("pick = %v, %v; want Disabled, Border", fill, border)
	}
	fill, _ = pickTestSet.pick(stateFlags{disabled: true})
	if fill != pickTestSet.Base {
		t.Errorf("unset Disabled: fill = %v, want Base", fill)
	}
}

// A translucent wash keeps its alpha through the step, so a selected
// row tint stays a tint on hover.
func TestColorSetPickSelectedKeepsAlpha(t *testing.T) {
	cs := ColorSet{Selected: RGBA(40, 90, 200, 60)}
	fill, _ := cs.pick(stateFlags{selected: true, hovered: true})
	if fill.A != 60 {
		t.Errorf("hover alpha = %d, want 60", fill.A)
	}
}

func TestColorSetResolvedTakesThemeSelectedAndDisabled(t *testing.T) {
	theme := ColorSet{Selected: Red, Disabled: Blue}
	got := ColorSet{}.resolved(Color{}, theme)
	if got.Selected != Red || got.Disabled != Blue {
		t.Errorf("resolved = %v, %v; want theme Red, Blue",
			got.Selected, got.Disabled)
	}
	own := ColorSet{Selected: Green}.resolved(Color{}, theme)
	if own.Selected != Green {
		t.Errorf("caller Selected lost to the theme: %v", own.Selected)
	}
}

// Flat is the "do not react" set. Pinning Selected or Disabled would
// override the theme's selected color and drop the disabled dim for
// every Flat caller, so it leaves both unset.
func TestFlatLeavesSelectedAndDisabledUnset(t *testing.T) {
	cs := Flat(Green)
	if cs.Selected.IsSet() || cs.Disabled.IsSet() {
		t.Errorf("Flat set Selected %v / Disabled %v, want both unset",
			cs.Selected, cs.Disabled)
	}
}

func TestColorSetIsSetSeesSelectedAndDisabled(t *testing.T) {
	if !(ColorSet{Selected: Red}).IsSet() {
		t.Error("IsSet misses Selected")
	}
	if !(ColorSet{Disabled: Red}).IsSet() {
		t.Error("IsSet misses Disabled")
	}
}

func TestColorSetPickSelectedAllocFree(t *testing.T) {
	cs := selectedTestSet
	s := stateFlags{selected: true, hovered: true}
	allocs := testing.AllocsPerRun(100, func() {
		pickFillSink, pickBorderSink = cs.pick(s)
	})
	if allocs != 0 {
		t.Errorf("selected pick allocated %v times per run, want 0", allocs)
	}
}

// An explicit disabled fill is painted as given. The border still
// dims: only the fill has a Disabled slot.
func TestRenderShapeExplicitDisabledFillSkipsDim(t *testing.T) {
	w := &Window{}
	fill := RGBA(200, 100, 50, 255)
	shape := &Shape{
		shapeType: shapeRectangle,
		X:         0, Y: 0, Width: 80, Height: 60,
		Color:         fill,
		colorDisabled: fill,
		ColorBorder:   RGBA(10, 10, 10, 255),
		SizeBorder:    1,
		Opacity:       1.0, Disabled: true,
	}
	renderShape(shape, ColorTransparent, makeClip(0, 0, 500, 500), w)
	var sawFill, sawBorder bool
	for _, r := range w.renderers {
		switch r.Kind {
		case RenderRect:
			sawFill = true
			if r.Color != fill {
				t.Errorf("fill = %v, want undimmed %v", r.Color, fill)
			}
		case RenderStrokeRect:
			sawBorder = true
			if r.Color.A != 127 {
				t.Errorf("border alpha = %d, want dimmed 127", r.Color.A)
			}
		}
	}
	if !sawFill || !sawBorder {
		t.Fatalf("fill emitted %v, border emitted %v; want both", sawFill, sawBorder)
	}
}

// A Button disabled by an ancestor, not by its own cfg, still paints
// Colors.Disabled: only layoutDisables knows about the ancestor.
func TestButtonInheritedDisabledPaintsColorsDisabled(t *testing.T) {
	dis := RGB(10, 200, 30)
	base := RGB(1, 2, 3)
	v := Column(ContainerCfg{
		Disabled: true,
		Content: []View{Button(ButtonCfg{
			ID:      "b",
			Label:   "B",
			Colors:  ColorSet{Base: base, Disabled: dis},
			OnClick: func(EventCtx) {},
		})},
	})
	layout := generateViewLayout(v, &Window{})
	btn := &layout.Children[0]
	if !btn.Shape.Color.eq(base) {
		t.Fatalf("before layoutDisables: fill = %v, want Base %v",
			btn.Shape.Color, base)
	}
	layoutDisables(&layout, false)
	if !btn.Shape.Color.eq(dis) {
		t.Errorf("inherited disabled fill = %v, want %v", btn.Shape.Color, dis)
	}
	if fillDims(btn.Shape) {
		t.Error("explicit disabled fill still takes the render dim")
	}
}

// Without Colors.Disabled a disabled Button keeps Base and the dim.
func TestButtonDisabledWithoutSlotKeepsDim(t *testing.T) {
	base := RGB(1, 2, 3)
	v := Button(ButtonCfg{
		ID:       "b",
		Label:    "B",
		Colors:   ColorSet{Base: base},
		Disabled: true,
		OnClick:  func(EventCtx) {},
	})
	layout := generateViewLayout(v, &Window{})
	layoutDisables(&layout, false)
	if !layout.Shape.Color.eq(base) {
		t.Errorf("fill = %v, want Base %v", layout.Shape.Color, base)
	}
	if !fillDims(layout.Shape) {
		t.Error("disabled fill without a Disabled slot must still dim")
	}
}

// A disabled trail disables every crumb through layoutDisables, and
// each crumb then paints ColorsCrumb.Disabled.
func TestBreadcrumbDisabledTrailPaintsColorsDisabled(t *testing.T) {
	dis := RGB(10, 200, 30)
	v := Breadcrumb(BreadcrumbCfg{
		ID:          "bc",
		Selected:    "a",
		Disabled:    true,
		Items:       []BreadcrumbItemCfg{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}},
		ColorsCrumb: ColorSet{Disabled: dis},
	})
	layout := generateViewLayout(v, &Window{})
	layoutDisables(&layout, false)
	trail := &layout.Children[0]
	found := 0
	for i := range trail.Children {
		crumb := &trail.Children[i]
		if !crumb.Shape.colorDisabled.IsSet() {
			continue // separator
		}
		found++
		if !crumb.Shape.Color.eq(dis) {
			t.Errorf("crumb %d fill = %v, want %v", i, crumb.Shape.Color, dis)
		}
	}
	if found != 2 {
		t.Fatalf("found %d crumbs carrying a disabled fill, want 2", found)
	}
}
