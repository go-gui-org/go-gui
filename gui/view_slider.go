package gui

import (
	"log"
	"math"
)

// SliderCfg configures a slider view.
type SliderCfg struct {
	// Label names this field. Empty renders exactly as before: no
	// wrapper and no extra shape. Set, it stacks above the field in
	// the theme's label role, and fills A11YLabel when that is unset.
	// See gui/field_label.go for the convention and why it is one.
	Label    string
	OnChange func(float32, EventCtx)
	ID       string `gui:"required"`

	// Accessibility
	A11YCfg
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
	// RadiusBorder overrides the track border radius. Unset falls
	// back to the theme.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusBorder Opt[float32]
	Value        float32
	Min          float32
	Max          float32
	Step         float32
	Width        float32
	Height       float32
	Size         float32 // ergonomics-audit:opt-plain — a zero-size slider is meaningless; 0 falls back to the theme
	ThumbSize    float32
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool

	// SoundDisabled suppresses the slider's sound regardless of the
	// theme. There is no Sound field to pair with it: a slider has
	// no activation moment to sound at — arrow movement and drag are
	// both deliberately silent — so Theme.Sounds.Error on an arrow
	// key already at Min or Max is the only cue it emits (#468).
	// exportaudit:keep — caller-facing config (issue #468)
	SoundDisabled bool

	Color       Color
	ColorBorder Color
	// ColorThumb paints the thumb. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorThumb Color
	ColorFocus Color
	ColorHover Color
	// ColorLeft paints the filled (value) side of the track. Unset
	// takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorLeft Color
	// ColorClick paints the track while the thumb is dragged. Unset
	// takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorClick Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it; the other flat
	// Color* fields win over their Colors slots the same way.
	Colors ColorSet
	Sizing Sizing
	// RoundValue snaps the thumb and reported value to integer
	// steps. Off by default.
	// exportaudit:keep — caller-facing config (issue #372)
	RoundValue bool
	// Vertical renders the slider top-to-bottom instead of
	// left-to-right.
	// exportaudit:keep — caller-facing config (issue #372)
	Vertical  bool
	Disabled  bool
	Invisible bool

	// Look draws the slider from parts the app builds, in place of the
	// stock track and thumb. The slider keeps its behavior: drag, keys,
	// wheel, rounding, vertical mode, focus and the screen reader value.
	// Look runs during layout generation, under the frame lock, with the
	// interaction state and the value as a fraction of the range.
	//
	// Track stays in the layout flow and sets its own size, usually
	// FillFixed (FixedFill when Vertical). Fill and Handle sit outside
	// the flow: give them a fixed or fit size across the track. After
	// arrange the slider centers both on the track, moves the Handle so
	// its center is at the value, and sets the Fill length from the start
	// of the track to that center. The Handle center stays inside the
	// track, half a handle from each end, and a press maps to a value
	// over the same span. A nil part is left out.
	//
	// With Look set, the color, border, radius, thumb size and focus ring
	// fields are not used; the look draws them. Width and Height still
	// size the slider (#664, docs/specs/slider-look-hook.md).
	Look func(SliderLookState) SliderParts
}

// SliderLookState is what a slider Look gets to pick its appearance.
type SliderLookState struct {
	InteractionState
	// Pct is the value as a fraction of the range, 0 to 1.
	Pct float32
}

// SliderParts are the views a slider Look returns. See [SliderCfg.Look].
type SliderParts struct {
	// Track is the bar the value runs along. It stays in the layout flow.
	Track View
	// Fill runs from the start of the track to the handle center.
	Fill View
	// Handle is centered on the value. Its children move with it.
	Handle View
}

// Slider creates a slider view.
func Slider(cfg SliderCfg) View {
	RequireID("Slider", cfg.ID)
	applySliderDefaults(&cfg)
	sizeBorder := cfg.SizeBorder.Get(guiTheme.sliderStyle.SizeBorder)
	if cfg.Size == 0 {
		cfg.Size = guiTheme.sliderStyle.Size
	}
	if cfg.ThumbSize == 0 {
		cfg.ThumbSize = guiTheme.sliderStyle.ThumbSize
	}
	radius := cfg.Radius.Get(guiTheme.sliderStyle.Radius)
	radiusBorder := cfg.RadiusBorder.Get(radius)
	if cfg.Max == 0 && cfg.Min == 0 {
		cfg.Max = 100
	}
	if cfg.Step == 0 {
		cfg.Step = 1
	}

	if cfg.Min >= cfg.Max {
		log.Printf("slider: min (%f) >= max (%f); adjusting",
			cfg.Min, cfg.Max)
		cfg.Max = cfg.Min + 1
	}

	wrapperWidth := cfg.Size
	wrapperHeight := f32Max(cfg.Size, cfg.ThumbSize)
	trackWidth := float32(0)
	trackHeight := cfg.Size

	if cfg.Vertical {
		wrapperWidth = f32Max(cfg.Size, cfg.ThumbSize)
		wrapperHeight = cfg.Size
		trackWidth = cfg.Size
		trackHeight = 0
	}
	if cfg.Width > 0 {
		wrapperWidth = cfg.Width
	}
	if cfg.Height > 0 {
		wrapperHeight = cfg.Height
	}

	sliderID := cfg.ID
	onChange := cfg.OnChange
	value := cfg.Value
	minVal := cfg.Min
	maxVal := cfg.Max
	step := cfg.Step
	vertical := cfg.Vertical
	roundValue := cfg.RoundValue
	// Resolved at generation time and captured by value: the key
	// handler reports a refused move, which dispatch never sees
	// because nothing was activated (issue #468).
	rejectCue := resolveSoundCue(
		guiTheme.Sounds.Error, SoundNone, cfg.SoundDisabled)
	size := cfg.Size
	szBorder := sizeBorder
	thumbSize := cfg.ThumbSize
	colorFocus := cfg.ColorFocus
	colorHover := cfg.ColorHover
	disabled := cfg.Disabled

	trackSizing := FillFixed
	if cfg.Vertical {
		trackSizing = FixedFill
	}

	trackAxis := axisLeftToRight
	if cfg.Vertical {
		trackAxis = axisTopToBottom
	}

	wrapperAxis := axisLeftToRight
	if cfg.Vertical {
		wrapperAxis = axisTopToBottom
	}

	cfg.A11YLabel = a11yLabel(cfg.A11YLabel, cfg.Label)
	if cfg.Look != nil {
		look := sliderLookView{
			cfg: cfg, width: wrapperWidth, height: wrapperHeight,
			axis: wrapperAxis, rejectCue: rejectCue,
		}
		return labelledField(cfg.Label, TextStyle{}, HAlignLeft, cfg.Sizing, look)
	}
	field := container(ContainerCfg{
		ID:        cfg.ID,
		Focusable: !cfg.FocusDisabled,
		A11YRole:  AccessRoleSlider,
		a11Y: &accessInfo{
			Label:       a11yLabel(cfg.A11YLabel, cfg.ID),
			Description: cfg.A11YDescription,
			ValueNum:    cfg.Value,
			ValueMin:    cfg.Min,
			ValueMax:    cfg.Max,
		},
		Width:     wrapperWidth,
		Height:    wrapperHeight,
		Disabled:  cfg.Disabled,
		Invisible: cfg.Invisible,
		Padding:   NoPadding,
		Sizing:    cfg.Sizing,
		HAlign:    HAlignCenter,
		VAlign:    VAlignMiddle,
		axis:      wrapperAxis,
		OnClick: sliderPressHandler(sliderID, onChange, value,
			minVal, maxVal, vertical, roundValue, sliderLookSlots{}),
		AmendLayout: amendAll(
			func(ctx EventCtx) {
				sliderAmendLayoutSlide(ctx.Layout, ctx.Window,
					onChange, value, minVal, maxVal, step, size, szBorder,
					vertical, colorFocus, cfg.ColorLeft, disabled,
					ctx.Layout.Shape.idKey(), roundValue)
			},
			// Ring shadow on the focusable wrapper; the track keeps its
			// own fill and the thumb its accent (visual-refresh § 5.4).
			focusRingAmend(Color{}, Color{})),
		OnHover: func(ctx EventCtx) {
			ctx.Window.SetMouseCursorPointingHand()
			if len(ctx.Layout.Children) > 0 {
				ctx.Layout.Children[0].Shape.ColorBorder = colorHover
			}
		},
		OnKeyDown: func(ctx EventCtx) {
			sliderOnKeyDown(ctx.Event, ctx.Window,
				onChange, value, minVal, maxVal, step, roundValue,
				rejectCue)
		},
		Content: []View{
			container(ContainerCfg{
				Width:       trackWidth,
				Height:      trackHeight,
				Sizing:      trackSizing,
				Color:       cfg.Color,
				ColorBorder: cfg.ColorBorder,
				SizeBorder:  Some(sizeBorder),
				Radius:      Some(radiusBorder),
				Padding:     NoPadding,
				axis:        trackAxis,
				Content: []View{
					Rectangle(RectangleCfg{
						Sizing:      FillFill,
						Color:       cfg.ColorLeft,
						ColorBorder: cfg.ColorLeft,
					}),
					Circle(ContainerCfg{
						Sizing:      FixedFixed,
						Width:       cfg.ThumbSize,
						Height:      cfg.ThumbSize,
						Color:       cfg.ColorThumb,
						ColorBorder: cfg.ColorBorder,
						SizeBorder:  Some(sizeBorder),
						Padding:     NoPadding,
						AmendLayout: func(ctx EventCtx) {
							sliderAmendLayoutThumb(
								ctx.Layout, ctx.Window, value,
								minVal, maxVal, thumbSize,
								vertical)
						},
					}),
				},
			}),
		},
	})
	return labelledField(cfg.Label, TextStyle{}, HAlignLeft, cfg.Sizing, field)
}

// sliderPressHandler sets the value under a press and starts the drag.
// The stock slider and a Look share it; slots is zero for the stock one.
func sliderPressHandler(
	sliderID string,
	onChange func(float32, EventCtx),
	value, minVal, maxVal float32,
	vertical, roundValue bool, slots sliderLookSlots,
) func(EventCtx) {
	return func(ctx EventCtx) {
		// Press state is keyed by the wrapper's effective ID — the
		// same key sliderAmendLayoutSlide reads off the shape. The
		// captured sliderID is only a leaf: Slider builds its tree
		// with no Window in hand.
		pressID := ctx.Layout.Shape.idKey()
		ps := StateMap[string, bool](ctx.Window, nsSliderPress, capModerate)
		ps.Set(pressID, true)
		ev := *ctx.Event
		ev.MouseX = ctx.Event.MouseX + ctx.Layout.Shape.X
		ev.MouseY = ctx.Event.MouseY + ctx.Layout.Shape.Y
		sliderMouseMove(ctx.Layout, &ev, ctx.Window,
			sliderID, onChange, value,
			minVal, maxVal, vertical, roundValue, slots)
		ctx.Window.MouseLock(MouseLockCfg{
			MouseMove: func(ctx EventCtx) {
				sliderMouseMove(ctx.Layout, ctx.Event, ctx.Window,
					sliderID, onChange, value,
					minVal, maxVal, vertical, roundValue, slots)
			},
			MouseUp: func(ctx EventCtx) {
				ps := StateMap[string, bool](ctx.Window, nsSliderPress, capModerate)
				ps.Set(pressID, false)
				ctx.Window.MouseUnlock()
			},
			Cancel: func(w *Window) {
				// Clear the pressed look; the value keeps
				// whatever the last move set, as on release.
				ps := StateMap[string, bool](w, nsSliderPress, capModerate)
				ps.Set(pressID, false)
			},
		})
		// A press on the track sets the value and starts a drag.
		// Consume it: a slider nested in a focusable container (the
		// color picker's alpha slider inside "picker") would
		// otherwise hand that container the focus once spec §4.3b
		// removes the pre-mark.
		ctx.Consume()
	}
}

// sliderLookSlots records which children of a Look slider hold which
// part. A nil part is left out, so the indexes vary; -1 is absent. The
// zero value, on false, is the stock slider.
type sliderLookSlots struct {
	on                  bool
	track, fill, handle int
}

// part returns the child in slot i, or nil when the slot is absent or the
// tree does not have that child.
func (s sliderLookSlots) part(layout *Layout, i int) *Layout {
	if i < 0 || i >= len(layout.Children) {
		return nil
	}
	return &layout.Children[i]
}

// sliderLookView defers a Look to layout generation, where the effective
// ID, and so the interaction state, can be read. Slider has computed the
// defaults and the wrapper size before this runs.
type sliderLookView struct {
	cfg           SliderCfg
	width, height float32
	axis          Axis
	rejectCue     SoundCue
}

func (v sliderLookView) GenerateLayout(w *Window) Layout {
	cfg := v.cfg
	pct := (f32Clamp(cfg.Value, cfg.Min, cfg.Max) - cfg.Min) / (cfg.Max - cfg.Min)
	parts := cfg.Look(SliderLookState{
		InteractionState: w.interactionState(w.EffID(cfg.ID)),
		Pct:              pct,
	})

	slots := sliderLookSlots{on: true, track: -1, fill: -1, handle: -1}
	content := make([]View, 0, 3)
	if parts.Track != nil {
		slots.track = len(content)
		content = append(content, parts.Track)
	}
	if parts.Fill != nil {
		slots.fill = len(content)
		content = append(content, parts.Fill)
	}
	if parts.Handle != nil {
		slots.handle = len(content)
		content = append(content, parts.Handle)
	}

	onChange := cfg.OnChange
	value, minVal, maxVal := cfg.Value, cfg.Min, cfg.Max
	step, roundValue, vertical := cfg.Step, cfg.RoundValue, cfg.Vertical
	rejectCue := v.rejectCue
	field := container(ContainerCfg{
		ID:        cfg.ID,
		Focusable: !cfg.FocusDisabled,
		A11YRole:  AccessRoleSlider,
		a11Y: &accessInfo{
			Label:       a11yLabel(cfg.A11YLabel, cfg.ID),
			Description: cfg.A11YDescription,
			ValueNum:    cfg.Value,
			ValueMin:    cfg.Min,
			ValueMax:    cfg.Max,
		},
		Width:      v.width,
		Height:     v.height,
		Disabled:   cfg.Disabled,
		Invisible:  cfg.Invisible,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Sizing:     cfg.Sizing,
		HAlign:     HAlignCenter,
		VAlign:     VAlignMiddle,
		axis:       v.axis,
		OnClick: sliderPressHandler(cfg.ID, onChange, value,
			minVal, maxVal, vertical, roundValue, slots),
		AmendLayout: func(ctx EventCtx) {
			sliderAmendLayoutLook(ctx.Layout, slots, pct, vertical)
		},
		OnHover: func(ctx EventCtx) {
			ctx.Window.SetMouseCursorPointingHand()
		},
		OnKeyDown: func(ctx EventCtx) {
			sliderOnKeyDown(ctx.Event, ctx.Window,
				onChange, value, minVal, maxVal, step, roundValue,
				rejectCue)
		},
		OnMouseScroll: func(ctx EventCtx) {
			sliderOnMouseScroll(ctx.Event, ctx.Window, onChange,
				value, minVal, maxVal, step, roundValue)
		},
		Content: content,
	})
	layout := generateViewLayout(field, w)
	// Fill and Handle leave the flow here, after generation, because a
	// View has no field for it. Sizing runs later, in arrange, so the
	// track is sized as if they were not there.
	if c := slots.part(&layout, slots.fill); c != nil {
		c.Shape.outOfFlow = true
	}
	if c := slots.part(&layout, slots.handle); c != nil {
		c.Shape.outOfFlow = true
	}
	return layout
}

// sliderLookSpan returns where the handle center travels along the main
// axis: the track, or the whole slider without one, inset by half the
// handle at each end.
func sliderLookSpan(layout *Layout, slots sliderLookSlots, vertical bool) (start, length float32) {
	bar := layout
	if t := slots.part(layout, slots.track); t != nil {
		bar = t
	}
	start, length = bar.Shape.X, bar.Shape.Width
	if vertical {
		start, length = bar.Shape.Y, bar.Shape.Height
	}
	if h := slots.part(layout, slots.handle); h != nil {
		half := h.Shape.Width / 2
		if vertical {
			half = h.Shape.Height / 2
		}
		start += half
		length -= 2 * half
	}
	return start, length
}

// sliderAmendLayoutLook places Fill and Handle on the arranged track. It
// runs after the parts' own AmendLayout (layoutAmend is post-order) and
// before clips are computed, so moved parts get clips for where they
// end up.
func sliderAmendLayoutLook(layout *Layout, slots sliderLookSlots, pct float32, vertical bool) {
	bar := layout
	if t := slots.part(layout, slots.track); t != nil {
		bar = t
	}
	start, length := sliderLookSpan(layout, slots, vertical)
	center := start + f32Max(length, 0)*pct
	// Cross-axis center of the track: both parts sit on it.
	crossMid := bar.Shape.Y + bar.Shape.Height/2
	if vertical {
		crossMid = bar.Shape.X + bar.Shape.Width/2
	}

	if f := slots.part(layout, slots.fill); f != nil {
		if vertical {
			layoutShift(f, crossMid-f.Shape.Width/2-f.Shape.X, bar.Shape.Y-f.Shape.Y)
			f.Shape.Height = f32Max(center-bar.Shape.Y, 0)
		} else {
			layoutShift(f, bar.Shape.X-f.Shape.X, crossMid-f.Shape.Height/2-f.Shape.Y)
			f.Shape.Width = f32Max(center-bar.Shape.X, 0)
		}
	}
	if h := slots.part(layout, slots.handle); h != nil {
		if vertical {
			layoutShift(h, crossMid-h.Shape.Width/2-h.Shape.X, center-h.Shape.Height/2-h.Shape.Y)
		} else {
			layoutShift(h, center-h.Shape.Width/2-h.Shape.X, crossMid-h.Shape.Height/2-h.Shape.Y)
		}
	}
}

// layoutShift moves a shape and its whole subtree, so a part's children
// stay where they were drawn inside it.
func layoutShift(layout *Layout, dx, dy float32) {
	layoutShiftDepth(layout, dx, dy, 0)
}

func layoutShiftDepth(layout *Layout, dx, dy float32, depth int) {
	if overMaxDepth(depth) || (dx == 0 && dy == 0) {
		return
	}
	layout.Shape.X += dx
	layout.Shape.Y += dy
	for i := range layout.Children {
		layoutShiftDepth(&layout.Children[i], dx, dy, depth+1)
	}
}

func sliderAmendLayoutSlide(
	layout *Layout, w *Window,
	onChange func(float32, EventCtx),
	value, minVal, maxVal, step, size, sizeBorder float32,
	vertical bool, colorFocus, colorLeft Color,
	disabled bool, focusID string, roundValue bool,
) {
	if layout.Shape.events == nil {
		layout.Shape.events = &eventHandlers{}
	}
	layout.Shape.events.OnMouseScroll = func(ctx EventCtx) {
		sliderOnMouseScroll(ctx.Event, ctx.Window, onChange,
			value, minVal, maxVal, step, roundValue)
	}

	if len(layout.Children) == 0 {
		return
	}
	track := &layout.Children[0]
	if len(track.Children) < 2 {
		return
	}
	leftBar := &track.Children[0]
	thumb := &track.Children[1]

	clamped := f32Clamp(value, minVal, maxVal)
	percent := (clamped - minVal) / (maxVal - minVal)

	if vertical {
		h := track.Shape.Height
		y := f32Min(h*percent, h)
		leftBar.Shape.Height = y
		leftBar.Shape.Width = size - sizeBorder*2
	} else {
		wd := track.Shape.Width
		x := f32Min(wd*percent, wd)
		leftBar.Shape.Width = x
		leftBar.Shape.Height = size - sizeBorder*2
	}

	if disabled {
		return
	}
	if w != nil {
		ps := StateMapRead[string, bool](w, nsSliderPress)
		if ps != nil {
			if pressed, ok := ps.Get(layout.Shape.idKey()); ok && pressed {
				thumb.Shape.Color = colorLeft
				return
			}
		}
		if w.IsFocus(focusID) {
			thumb.Shape.Color = colorFocus
		}
	}
}

func sliderAmendLayoutThumb(
	layout *Layout, _ *Window,
	value, minVal, maxVal, thumbSize float32, vertical bool,
) {
	if layout.Parent == nil {
		return
	}
	clamped := f32Clamp(value, minVal, maxVal)
	percent := (clamped - minVal) / (maxVal - minVal)
	radius := thumbSize / 2

	if vertical {
		h := layout.Parent.Shape.Height
		y := f32Min(h*percent, h)
		layout.Shape.Y = layout.Parent.Shape.Y + y - radius
		layout.Shape.X = layout.Parent.Shape.X +
			layout.Parent.Shape.Width/2 - radius
	} else {
		wd := layout.Parent.Shape.Width
		x := f32Min(wd*percent, wd)
		layout.Shape.X = layout.Parent.Shape.X + x - radius
		layout.Shape.Y = layout.Parent.Shape.Y +
			layout.Parent.Shape.Height/2 - radius
	}
}

func sliderMouseMove(
	layout *Layout, e *Event, w *Window,
	sliderID string,
	onChange func(float32, EventCtx),
	curValue, minVal, maxVal float32,
	vertical, roundValue bool, slots sliderLookSlots,
) {
	if onChange == nil {
		return
	}
	sl, ok := layout.FindLayout(func(n Layout) bool {
		return n.Shape.ID == sliderID
	})
	if !ok {
		return
	}
	w.SetMouseCursorPointingHand()
	shape := sl.Shape
	if slots.on {
		// A Look maps the press over the span the handle center travels,
		// so a press on the handle's rest position at either end is the
		// bound, as the handle is drawn.
		start, length := sliderLookSpan(sl, slots, vertical)
		pos := e.MouseX
		if vertical {
			pos = e.MouseY
		}
		pct := float32(0)
		if length > 0 {
			pct = f32Clamp((pos-start)/length, 0, 1)
		}
		v := minVal + (maxVal-minVal)*pct
		if roundValue {
			v = float32(math.Round(float64(v)))
		}
		if v != curValue {
			onChange(v, EventCtx{nil, e, w})
		}
		return
	}
	if vertical {
		h := shape.Height
		pct := f32Clamp((e.MouseY-shape.Y)/h, 0, 1)
		val := minVal + (maxVal-minVal)*pct
		v := f32Clamp(val, minVal, maxVal)
		if roundValue {
			v = float32(math.Round(float64(v)))
		}
		if v != curValue {
			onChange(v, EventCtx{nil, e, w})
		}
	} else {
		wd := shape.Width
		pct := f32Clamp((e.MouseX-shape.X)/wd, 0, 1)
		val := minVal + (maxVal-minVal)*pct
		v := f32Clamp(val, minVal, maxVal)
		if roundValue {
			v = float32(math.Round(float64(v)))
		}
		if v != curValue {
			onChange(v, EventCtx{nil, e, w})
		}
	}
}

// sliderOnKeyDown moves the value by one step, or to a bound.
//
// rejectCue sounds when the key was understood but the value could not
// move — an arrow already at Min or Max. Movement itself is silent by
// decision: a held arrow key would machine-gun the cue (issue #468).
func sliderOnKeyDown(
	e *Event, w *Window,
	onChange func(float32, EventCtx),
	curValue, minVal, maxVal, step float32, roundValue bool,
	rejectCue SoundCue,
) {
	if onChange == nil || e.Modifiers != ModNone {
		return
	}
	v := curValue
	switch e.KeyCode {
	case KeyHome:
		v = minVal
	case KeyEnd:
		v = maxVal
	case KeyLeft, KeyUp:
		v = f32Clamp(v-step, minVal, maxVal)
	case KeyRight, KeyDown:
		v = f32Clamp(v+step, minVal, maxVal)
	default:
		return
	}
	e.IsHandled = true
	if roundValue {
		v = float32(math.Round(float64(v)))
	}
	if v != curValue {
		onChange(v, EventCtx{nil, e, w})
		return
	}
	playSoundCue(rejectCue, w)
}

// applySliderDefaults fills zero-value color fields from the theme.
func applySliderDefaults(cfg *SliderCfg) {
	d := &defaultSliderStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, d.Colors)
	cfg.Colors.applyTo(&cfg.Color, &cfg.ColorHover, &cfg.ColorClick,
		&cfg.ColorFocus, &cfg.ColorBorder, nil)
	if !cfg.ColorThumb.IsSet() {
		cfg.ColorThumb = d.colorThumb
	}
	if !cfg.ColorLeft.IsSet() {
		cfg.ColorLeft = d.colorLeft
	}
}

func sliderOnMouseScroll(
	e *Event, w *Window,
	onChange func(float32, EventCtx),
	curValue, minVal, maxVal, step float32, roundValue bool,
) {
	e.IsHandled = true
	if onChange == nil || e.Modifiers != ModNone {
		return
	}
	// ScrollY is lines for a wheel and points for a trackpad (see
	// Event), so its size means nothing here. Only the sign is used:
	// one Step per event, the same distance as an arrow key. NumericInput
	// does the same.
	var v float32
	switch {
	case e.ScrollY > 0:
		v = f32Clamp(curValue+step, minVal, maxVal)
	case e.ScrollY < 0:
		v = f32Clamp(curValue-step, minVal, maxVal)
	default:
		return
	}
	if roundValue {
		v = float32(math.Round(float64(v)))
	}
	if v != curValue {
		onChange(v, EventCtx{nil, e, w})
	}
}
