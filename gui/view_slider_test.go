package gui

import (
	"math"
	"testing"
)

func TestSliderDefaultLayout(t *testing.T) {
	t.Parallel()
	v := Slider(SliderCfg{
		ID:       "rs",
		Value:    50,
		OnChange: func(_ float32, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, &Window{})
	// Wrapper container with 1 child (track)
	if len(layout.Children) != 1 {
		t.Fatalf("children: got %d, want 1", len(layout.Children))
	}
	track := layout.Children[0]
	// Track has fill bar + thumb
	if len(track.Children) != 2 {
		t.Fatalf("track children: got %d, want 2",
			len(track.Children))
	}
}

func TestSliderA11Y(t *testing.T) {
	t.Parallel()
	v := Slider(SliderCfg{
		ID:       "rs",
		Value:    30,
		Min:      0,
		Max:      100,
		OnChange: func(_ float32, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.A11YRole != AccessRoleSlider {
		t.Errorf("role = %d, want Slider", layout.Shape.A11YRole)
	}
	a := layout.Shape.a11Y
	if a == nil {
		t.Fatal("a11y should be set")
	}
	if a.ValueNum != 30 {
		t.Errorf("value_num = %f, want 30", a.ValueNum)
	}
	if a.ValueMin != 0 || a.ValueMax != 100 {
		t.Errorf("range = %f-%f, want 0-100",
			a.ValueMin, a.ValueMax)
	}
}

func TestSliderMinMaxValidation(t *testing.T) {
	t.Parallel()
	v := Slider(SliderCfg{
		ID:       "rs",
		Min:      50,
		Max:      50, // invalid: min >= max
		OnChange: func(_ float32, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, &Window{})
	// Should auto-adjust max to min+1
	if layout.Shape.a11Y.ValueMax != 51 {
		t.Errorf("adjusted max = %f, want 51",
			layout.Shape.a11Y.ValueMax)
	}
}

func TestSliderKeyDown(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		key  KeyCode
		want float32
	}{
		{"home", KeyHome, 0},
		{"end", KeyEnd, 100},
		{"right", KeyRight, 51},
		{"left", KeyLeft, 49},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got float32
			onChange := func(v float32, ctx EventCtx) { got = v }
			e := &Event{KeyCode: tt.key}
			sliderOnKeyDown(e, &Window{},
				onChange, 50, 0, 100, 1, false, SoundNone)
			if got != tt.want {
				t.Errorf("key %d: got %f, want %f",
					tt.key, got, tt.want)
			}
		})
	}
}

func TestSliderMouseScroll(t *testing.T) {
	t.Parallel()
	var got float32
	onChange := func(v float32, ctx EventCtx) { got = v }
	e := &Event{ScrollY: 5}
	sliderOnMouseScroll(e, &Window{}, onChange,
		50, 0, 100, 2, false)
	if got != 52 {
		t.Errorf("scroll: got %f, want 52", got)
	}
	if !e.IsHandled {
		t.Error("scroll should mark handled")
	}
}

func TestSliderVertical(t *testing.T) {
	t.Parallel()
	v := Slider(SliderCfg{
		ID:       "rs",
		Value:    50,
		Vertical: true,
		OnChange: func(_ float32, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.Axis != axisTopToBottom {
		t.Error("vertical slider should use top-to-bottom axis")
	}
}

func TestSliderRoundValue(t *testing.T) {
	t.Parallel()
	var got float32
	onChange := func(v float32, ctx EventCtx) { got = v }
	e := &Event{ScrollY: 0.1}
	sliderOnMouseScroll(e, &Window{}, onChange,
		50, 0, 100, 0.7, true)
	if got != float32(math.Round(50.7)) {
		t.Errorf("rounded: got %f, want %f",
			got, float32(math.Round(50.7)))
	}
}

func TestSliderNonZeroMin(t *testing.T) {
	t.Parallel()
	// B1: percent calc with non-zero Min
	t.Run("percent", func(t *testing.T) {
		t.Parallel()
		v := Slider(SliderCfg{
			ID:       "rs",
			Value:    60,
			Min:      10,
			Max:      110,
			OnChange: func(_ float32, ctx EventCtx) {},
		})
		layout := generateViewLayout(v, &Window{})
		track := layout.Children[0]
		leftBar := track.Children[0]
		// value=60, min=10, max=110 → 50% of track width
		want := track.Shape.Width * 0.5
		if diff := leftBar.Shape.Width - want; diff > 1 || diff < -1 {
			t.Errorf("left bar width = %f, want ~%f",
				leftBar.Shape.Width, want)
		}
	})
	// B2: mouse value with non-zero Min
	t.Run("mouse_value", func(t *testing.T) {
		t.Parallel()
		var got float32
		onChange := func(v float32, ctx EventCtx) { got = v }
		v := Slider(SliderCfg{
			ID:       "rs-nz",
			Value:    10,
			Min:      10,
			Max:      110,
			Sizing:   FixedFit,
			Width:    100,
			OnChange: onChange,
		})
		w := &Window{}
		layout := generateViewLayout(v, w)
		// Simulate click at 50% along the slider
		e := &Event{
			MouseX: layout.Shape.X + layout.Shape.Width/2,
			MouseY: layout.Shape.Y + layout.Shape.Height/2,
		}
		sliderMouseMove(&layout, e, w,
			"rs-nz", onChange, 10, 10, 110, false, false, sliderLookSlots{})
		// 50% of [10,110] → 60
		if got < 59 || got > 61 {
			t.Errorf("mouse value = %f, want ~60", got)
		}
	})
}

func TestSliderKeyDownHandled(t *testing.T) {
	t.Parallel()
	onChange := func(_ float32, ctx EventCtx) {}
	// Recognized key sets IsHandled
	e := &Event{KeyCode: KeyRight}
	sliderOnKeyDown(e, &Window{},
		onChange, 50, 0, 100, 1, false, SoundNone)
	if !e.IsHandled {
		t.Error("arrow key should set IsHandled")
	}
	// Unrecognized key does not set IsHandled
	e2 := &Event{KeyCode: KeyA}
	sliderOnKeyDown(e2, &Window{},
		onChange, 50, 0, 100, 1, false, SoundNone)
	if e2.IsHandled {
		t.Error("unrecognized key should not set IsHandled")
	}
}

func TestSliderVerticalMouseDedup(t *testing.T) {
	t.Parallel()
	callCount := 0
	onChange := func(_ float32, ctx EventCtx) { callCount++ }
	v := Slider(SliderCfg{
		ID:       "rs-vd",
		Value:    50,
		Vertical: true,
		OnChange: onChange,
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	// Mouse position that maps to curValue=50 (50% of 100)
	e := &Event{
		MouseX: layout.Shape.X,
		MouseY: layout.Shape.Y + layout.Shape.Height/2,
	}
	sliderMouseMove(&layout, e, w,
		"rs-vd", onChange, 50, 0, 100, true, true, sliderLookSlots{})
	if callCount != 0 {
		t.Errorf("onChange called %d times, want 0 (value unchanged)",
			callCount)
	}
}

// sliderTestLayout builds the wrapper → track → [leftBar, thumb]
// tree sliderAmendLayoutSlide navigates, with the geometry a laid-out
// slider would have.
func sliderTestLayout() Layout {
	track := Layout{
		Shape: &Shape{Width: 200, Height: 20},
		Children: []Layout{
			{Shape: &Shape{}},
			{Shape: &Shape{}},
		},
	}
	return Layout{
		Shape:    &Shape{ID: "rs", Width: 200, Height: 20},
		Children: []Layout{track},
	}
}

func TestSliderAmendLayoutSlideHorizontal(t *testing.T) {
	layout := sliderTestLayout()
	onChange := func(float32, EventCtx) {}
	sliderAmendLayoutSlide(&layout, nil,
		onChange, 50, 0, 100, 1, 20, 2, false,
		RGB(255, 0, 0), RGB(0, 0, 255), false, "rs", false)

	leftBar := &layout.Children[0].Children[0]
	// 50% of 200 width = 100; height = size - 2*border = 16.
	if leftBar.Shape.Width != 100 {
		t.Errorf("left bar width = %v, want 100", leftBar.Shape.Width)
	}
	if leftBar.Shape.Height != 16 {
		t.Errorf("left bar height = %v, want 16", leftBar.Shape.Height)
	}
	// A scroll handler must have been attached.
	if layout.Shape.events == nil || layout.Shape.events.OnMouseScroll == nil {
		t.Error("slider shape should carry an OnMouseScroll handler")
	}
}

func TestSliderAmendLayoutSlideVertical(t *testing.T) {
	layout := sliderTestLayout()
	sliderAmendLayoutSlide(&layout, nil, nil,
		25, 0, 100, 1, 20, 2, true,
		RGB(255, 0, 0), RGB(0, 0, 255), false, "rs", false)

	leftBar := &layout.Children[0].Children[0]
	// 25% of 20 height = 5; width = size - 2*border = 16.
	if leftBar.Shape.Height != 5 {
		t.Errorf("left bar height = %v, want 5", leftBar.Shape.Height)
	}
	if leftBar.Shape.Width != 16 {
		t.Errorf("left bar width = %v, want 16", leftBar.Shape.Width)
	}
}

func TestSliderAmendLayoutSlideClamps(t *testing.T) {
	layout := sliderTestLayout()
	// Value above max must not overflow the track.
	sliderAmendLayoutSlide(&layout, nil, nil,
		250, 0, 100, 1, 20, 2, false,
		RGB(255, 0, 0), RGB(0, 0, 255), false, "rs", false)
	leftBar := &layout.Children[0].Children[0]
	if leftBar.Shape.Width != 200 {
		t.Errorf("left bar width = %v, want 200 (clamped)", leftBar.Shape.Width)
	}

	// Value below min clamps to zero.
	layout2 := sliderTestLayout()
	sliderAmendLayoutSlide(&layout2, nil, nil,
		-50, 0, 100, 1, 20, 2, false,
		RGB(255, 0, 0), RGB(0, 0, 255), false, "rs", false)
	leftBar2 := &layout2.Children[0].Children[0]
	if leftBar2.Shape.Width != 0 {
		t.Errorf("left bar width = %v, want 0 (clamped)", leftBar2.Shape.Width)
	}
}

func TestSliderAmendLayoutSlideFocused(t *testing.T) {
	w := &Window{}
	layout := sliderTestLayout()
	sliderAmendLayoutSlide(&layout, w, nil,
		50, 0, 100, 1, 20, 2, false,
		RGB(255, 0, 0), RGB(0, 0, 255), false, "rs", false)

	// Not focused yet: thumb keeps its original color.
	thumb := &layout.Children[0].Children[1]
	if thumb.Shape.Color == RGB(255, 0, 0) {
		t.Error("thumb should not be focus-colored before focus")
	}

	w.SetFocus("rs")
	sliderAmendLayoutSlide(&layout, w, nil,
		50, 0, 100, 1, 20, 2, false,
		RGB(255, 0, 0), RGB(0, 0, 255), false, "rs", false)
	if thumb.Shape.Color != RGB(255, 0, 0) {
		t.Error("thumb should be focus-colored while focused")
	}
}

func TestSliderAmendLayoutSlidePressed(t *testing.T) {
	w := &Window{}
	layout := sliderTestLayout()
	// Press state is keyed by the shape's idKey.
	ps := StateMap[string, bool](w, nsSliderPress, capModerate)
	ps.Set(layout.Shape.idKey(), true)

	sliderAmendLayoutSlide(&layout, w, nil,
		50, 0, 100, 1, 20, 2, false,
		RGB(255, 0, 0), RGB(0, 0, 255), false, "rs", false)
	thumb := &layout.Children[0].Children[1]
	if thumb.Shape.Color != RGB(0, 0, 255) {
		t.Error("thumb should be press-colored while pressed")
	}
}

func TestSliderAmendLayoutSlideDisabled(t *testing.T) {
	w := &Window{}
	w.SetFocus("rs")
	layout := sliderTestLayout()
	sliderAmendLayoutSlide(&layout, w, nil,
		50, 0, 100, 1, 20, 2, false,
		RGB(255, 0, 0), RGB(0, 0, 255), true, "rs", false)

	thumb := &layout.Children[0].Children[1]
	if thumb.Shape.Color == RGB(255, 0, 0) {
		t.Error("disabled slider must not paint the focus color")
	}
}

func TestSliderAmendLayoutSlideDegenerate(t *testing.T) {
	// Empty and short layouts are no-ops, not panics.
	sliderAmendLayoutSlide(&Layout{Shape: &Shape{}}, nil, nil,
		50, 0, 100, 1, 20, 2, false,
		RGB(255, 0, 0), RGB(0, 0, 255), false, "", false)
	sliderAmendLayoutSlide(&Layout{
		Shape:    &Shape{},
		Children: []Layout{{Shape: &Shape{}}},
	}, nil, nil,
		50, 0, 100, 1, 20, 2, false,
		RGB(255, 0, 0), RGB(0, 0, 255), false, "", false)
}

func TestSliderAmendLayoutThumbHorizontal(t *testing.T) {
	parent := &Layout{Shape: &Shape{X: 10, Y: 5, Width: 200, Height: 20}}
	thumb := Layout{Shape: &Shape{X: 10, Y: 5}, Parent: parent}
	sliderAmendLayoutThumb(&thumb, nil, 50, 0, 100, 12, false)

	// Thumb radius = 6. Center at 50% of 200 → x = 10+100-6 = 104;
	// y centered on the track: 5 + 10 - 6 = 9.
	if thumb.Shape.X != 104 {
		t.Errorf("thumb X = %v, want 104", thumb.Shape.X)
	}
	if thumb.Shape.Y != 9 {
		t.Errorf("thumb Y = %v, want 9", thumb.Shape.Y)
	}
}

func TestSliderAmendLayoutThumbVertical(t *testing.T) {
	parent := &Layout{Shape: &Shape{X: 10, Y: 5, Width: 200, Height: 20}}
	thumb := Layout{Shape: &Shape{X: 10, Y: 5}, Parent: parent}
	sliderAmendLayoutThumb(&thumb, nil, 50, 0, 100, 12, true)

	// Vertical: y = parent.Y + 50% of 20 - 6 = 9; x centered on
	// parent width: 10 + 100 - 6 = 104.
	if thumb.Shape.Y != 9 {
		t.Errorf("thumb Y = %v, want 9", thumb.Shape.Y)
	}
	if thumb.Shape.X != 104 {
		t.Errorf("thumb X = %v, want 104", thumb.Shape.X)
	}
}

func TestSliderAmendLayoutThumbClamped(t *testing.T) {
	parent := &Layout{Shape: &Shape{X: 0, Y: 0, Width: 100, Height: 10}}
	thumb := Layout{Shape: &Shape{X: 0, Y: 0}, Parent: parent}
	// Value above max must clamp to the far edge, not overflow.
	sliderAmendLayoutThumb(&thumb, nil, 999, 0, 100, 10, false)
	if thumb.Shape.X != 95 {
		t.Errorf("thumb X = %v, want 95 (edge minus radius)", thumb.Shape.X)
	}
}

func TestSliderAmendLayoutThumbNoParent(t *testing.T) {
	thumb := Layout{Shape: &Shape{}}
	sliderAmendLayoutThumb(&thumb, nil, 50, 0, 100, 12, false)
	if thumb.Shape.X != 0 || thumb.Shape.Y != 0 {
		t.Error("thumb without parent must be left untouched")
	}
}

// The wheel moves a slider by one Step per event, in the sign of ScrollY,
// on the stock slider and on a Look slider. ScrollY is lines for a wheel
// and points for a trackpad (see Event), so its size must not matter: a
// 0..1 slider with Step 0.1 must not reach an end on one wheel notch
// (#668).
func TestSliderWheelMovesByStep(t *testing.T) {
	tests := []struct {
		name    string
		look    bool
		precise bool
		scrollY float32
		round   bool
		start   float32
		step    float32
		want    float32
	}{
		{name: "wheel up", scrollY: 3, start: 0.5, step: 0.1, want: 0.6},
		{name: "wheel down", scrollY: -3, start: 0.5, step: 0.1, want: 0.4},
		{name: "trackpad points", precise: true, scrollY: 40, start: 0.5,
			step: 0.1, want: 0.6},
		{name: "clamps at max", scrollY: 3, start: 0.95, step: 0.1, want: 1},
		{name: "zero delta", scrollY: 0, start: 0.5, step: 0.1, want: 0.5},
		{name: "look slider", look: true, scrollY: -3, start: 0.5, step: 0.1,
			want: 0.4},
		{name: "default step", scrollY: 3, start: 0, step: 0, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.start
			cfg := SliderCfg{
				ID:    "vol",
				Value: value,
				Min:   0,
				Max:   1,
				Step:  tt.step,
				Width: 200,
				OnChange: func(v float32, ctx EventCtx) {
					value = v
					ctx.Consume()
				},
			}
			if tt.look {
				cfg.Look = func(SliderLookState) SliderParts {
					return SliderParts{Track: Row(ContainerCfg{ID: "track",
						Height: 6, Sizing: FillFixed, SizeBorder: NoBorder})}
				}
			}
			w := NewTestWindow(WindowCfg{})
			w.TestRender(func(*Window) View {
				c := cfg
				c.Value = value
				return Column(ContainerCfg{SizeBorder: NoBorder,
					Padding: PadAll(10), Content: []View{Slider(c)}})
			})
			ly, err := w.testTarget("vol")
			if err != nil {
				t.Fatal(err)
			}
			x, y, err := testHitPoint(ly, "vol")
			if err != nil {
				t.Fatal(err)
			}
			e := Event{Type: EventMouseScroll, ScrollPrecise: tt.precise,
				MouseX: x, MouseY: y, ScrollY: tt.scrollY}
			w.EventFn(&e)
			if d := value - tt.want; d > 1e-5 || d < -1e-5 {
				t.Errorf("value %v, want %v", value, tt.want)
			}
		})
	}
}
