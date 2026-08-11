package gui

import (
	"log"
	"math"
)

// SliderCfg configures a slider view.
type SliderCfg struct {
	OnChange func(float32, EventCtx)
	ID       string `gui:"required"`

	// Accessibility
	A11YLabel       string
	A11YDescription string
	Padding         Padding
	SizeBorder      Opt[float32]
	Radius          Opt[float32]
	radiusBorder    Opt[float32]
	Value           float32
	Min             float32
	Max             float32
	Step            float32
	Width           float32
	Height          float32
	Size            float32 // ergonomics-audit:opt-plain — a zero-size slider is meaningless; 0 falls back to the theme
	ThumbSize       float32
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Color         Color
	ColorBorder   Color
	colorThumb    Color
	ColorFocus    Color
	ColorHover    Color
	colorLeft     Color
	colorClick    Color
	Sizing        Sizing
	roundValue    bool
	vertical      bool
	Disabled      bool
	Invisible     bool
}

// Slider creates a slider view.
func Slider(cfg SliderCfg) View {
	RequireID("Slider", cfg.ID)
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.sliderStyle.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = guiTheme.sliderStyle.ColorBorder
	}
	if !cfg.colorThumb.IsSet() {
		cfg.colorThumb = guiTheme.sliderStyle.colorThumb
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = guiTheme.sliderStyle.ColorFocus
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = guiTheme.sliderStyle.ColorHover
	}
	if !cfg.colorLeft.IsSet() {
		cfg.colorLeft = guiTheme.sliderStyle.colorLeft
	}
	if !cfg.colorClick.IsSet() {
		cfg.colorClick = guiTheme.sliderStyle.colorClick
	}
	sizeBorder := cfg.SizeBorder.Get(guiTheme.sliderStyle.SizeBorder)
	if cfg.Size == 0 {
		cfg.Size = guiTheme.sliderStyle.Size
	}
	if cfg.ThumbSize == 0 {
		cfg.ThumbSize = guiTheme.sliderStyle.ThumbSize
	}
	radius := cfg.Radius.Get(guiTheme.sliderStyle.Radius)
	radiusBorder := cfg.radiusBorder.Get(radius)
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

	if cfg.vertical {
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
	vertical := cfg.vertical
	roundValue := cfg.roundValue
	size := cfg.Size
	szBorder := sizeBorder
	thumbSize := cfg.ThumbSize
	colorFocus := cfg.ColorFocus
	colorHover := cfg.ColorHover
	disabled := cfg.Disabled

	trackSizing := FillFixed
	if cfg.vertical {
		trackSizing = FixedFill
	}

	trackAxis := axisLeftToRight
	if cfg.vertical {
		trackAxis = axisTopToBottom
	}

	wrapperAxis := axisLeftToRight
	if cfg.vertical {
		wrapperAxis = axisTopToBottom
	}

	return container(ContainerCfg{
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
		OnClick: func(ctx EventCtx) {
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
				minVal, maxVal, vertical, roundValue)
			ctx.Window.MouseLock(MouseLockCfg{
				MouseMove: func(ctx EventCtx) {
					sliderMouseMove(ctx.Layout, ctx.Event, ctx.Window,
						sliderID, onChange, value,
						minVal, maxVal, vertical, roundValue)
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
		},
		AmendLayout: func(ctx EventCtx) {
			sliderAmendLayoutSlide(ctx.Layout, ctx.Window,
				onChange, value, minVal, maxVal, size, szBorder,
				vertical, colorFocus, cfg.colorLeft, disabled,
				ctx.Layout.Shape.idKey(), roundValue)
		},
		OnHover: func(ctx EventCtx) {
			ctx.Window.SetMouseCursorPointingHand()
			if len(ctx.Layout.Children) > 0 {
				ctx.Layout.Children[0].Shape.ColorBorder = colorHover
			}
		},
		OnKeyDown: func(ctx EventCtx) {
			sliderOnKeyDown(ctx.Layout, ctx.Event, ctx.Window,
				onChange, value, minVal, maxVal, step, roundValue)
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
						Color:       cfg.colorLeft,
						ColorBorder: cfg.colorLeft,
					}),
					Circle(ContainerCfg{
						Sizing:      FixedFixed,
						Width:       cfg.ThumbSize,
						Height:      cfg.ThumbSize,
						Color:       cfg.colorThumb,
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
}

func sliderAmendLayoutSlide(
	layout *Layout, w *Window,
	onChange func(float32, EventCtx),
	value, minVal, maxVal, size, sizeBorder float32,
	vertical bool, colorFocus, colorLeft Color,
	disabled bool, focusID string, roundValue bool,
) {
	if layout.Shape.events == nil {
		layout.Shape.events = &eventHandlers{}
	}
	layout.Shape.events.OnMouseScroll = func(ctx EventCtx) {
		sliderOnMouseScroll(ctx.Event, ctx.Window, onChange,
			value, minVal, maxVal, roundValue)
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
	vertical, roundValue bool,
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

func sliderOnKeyDown(
	_ *Layout, e *Event, w *Window,
	onChange func(float32, EventCtx),
	curValue, minVal, maxVal, step float32, roundValue bool,
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
	}
}

func sliderOnMouseScroll(
	e *Event, w *Window,
	onChange func(float32, EventCtx),
	curValue, minVal, maxVal float32, roundValue bool,
) {
	e.IsHandled = true
	if onChange == nil || e.Modifiers != ModNone {
		return
	}
	v := f32Clamp(curValue+e.ScrollY, minVal, maxVal)
	if roundValue {
		v = float32(math.Round(float64(v)))
	}
	if v != curValue {
		onChange(v, EventCtx{nil, e, w})
	}
}
