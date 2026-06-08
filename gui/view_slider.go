package gui

import (
	"log"
	"math"
)

// SliderCfg configures a slider view.
type SliderCfg struct {
	OnChange func(float32, *Event, *Window)
	ID       string `gui:"required"`

	// Accessibility
	A11YLabel       string
	A11YDescription string
	Padding         Opt[Padding]
	SizeBorder      Opt[float32]
	Radius          Opt[float32]
	RadiusBorder    Opt[float32]
	Value           float32
	Min             float32
	Max             float32
	Step            float32
	Width           float32
	Height          float32
	Size            float32
	ThumbSize       float32
	IDFocus         uint32
	Color           Color
	ColorBorder     Color
	ColorThumb      Color
	ColorFocus      Color
	ColorHover      Color
	ColorLeft       Color
	ColorClick      Color
	Sizing          Sizing
	RoundValue      bool
	Vertical        bool
	Disabled        bool
	Invisible       bool
}

// Slider creates a slider view.
func Slider(cfg SliderCfg) View {
	requireID("Slider", cfg.ID)
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.SliderStyle.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = guiTheme.SliderStyle.ColorBorder
	}
	if !cfg.ColorThumb.IsSet() {
		cfg.ColorThumb = guiTheme.SliderStyle.ColorThumb
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = guiTheme.SliderStyle.ColorFocus
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = guiTheme.SliderStyle.ColorHover
	}
	if !cfg.ColorLeft.IsSet() {
		cfg.ColorLeft = guiTheme.SliderStyle.ColorLeft
	}
	if !cfg.ColorClick.IsSet() {
		cfg.ColorClick = guiTheme.SliderStyle.ColorClick
	}
	sizeBorder := cfg.SizeBorder.Get(guiTheme.SliderStyle.SizeBorder)
	if cfg.Size == 0 {
		cfg.Size = guiTheme.SliderStyle.Size
	}
	if cfg.ThumbSize == 0 {
		cfg.ThumbSize = guiTheme.SliderStyle.ThumbSize
	}
	radius := cfg.Radius.Get(guiTheme.SliderStyle.Radius)
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
	size := cfg.Size
	szBorder := sizeBorder
	thumbSize := cfg.ThumbSize
	colorFocus := cfg.ColorFocus
	colorHover := cfg.ColorHover
	disabled := cfg.Disabled
	idFocus := cfg.IDFocus

	trackSizing := FillFixed
	if cfg.Vertical {
		trackSizing = Sizing{SizingFixed, SizingFill}
	}

	trackAxis := AxisLeftToRight
	if cfg.Vertical {
		trackAxis = AxisTopToBottom
	}

	wrapperAxis := AxisLeftToRight
	if cfg.Vertical {
		wrapperAxis = AxisTopToBottom
	}

	return container(ContainerCfg{
		ID:       cfg.ID,
		IDFocus:  cfg.IDFocus,
		A11YRole: AccessRoleSlider,
		A11Y: &AccessInfo{
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
		OnClick: func(layout *Layout, e *Event, w *Window) {
			ps := StateMap[string, bool](w, nsSliderPress, capModerate)
			ps.Set(sliderID, true)
			ev := *e
			ev.MouseX = e.MouseX + layout.Shape.X
			ev.MouseY = e.MouseY + layout.Shape.Y
			sliderMouseMove(layout, &ev, w,
				sliderID, onChange, value,
				minVal, maxVal, vertical, roundValue)
			w.MouseLock(MouseLockCfg{
				MouseMove: func(
					layout *Layout, e *Event, w *Window,
				) {
					sliderMouseMove(layout, e, w,
						sliderID, onChange, value,
						minVal, maxVal, vertical, roundValue)
				},
				MouseUp: func(
					_ *Layout, _ *Event, w *Window,
				) {
					ps := StateMap[string, bool](w, nsSliderPress, capModerate)
					ps.Set(sliderID, false)
					w.MouseUnlock()
				},
			})
			e.IsHandled = true
		},
		AmendLayout: func(layout *Layout, w *Window) {
			sliderAmendLayoutSlide(layout, w,
				onChange, value, minVal, maxVal, size, szBorder,
				vertical, colorFocus, cfg.ColorLeft, disabled, idFocus,
				roundValue)
		},
		OnHover: func(layout *Layout, _ *Event, w *Window) {
			w.SetMouseCursorPointingHand()
			if len(layout.Children) > 0 {
				layout.Children[0].Shape.ColorBorder = colorHover
			}
		},
		OnKeyDown: func(layout *Layout, e *Event, w *Window) {
			sliderOnKeyDown(layout, e, w,
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
						AmendLayout: func(
							layout *Layout, w *Window,
						) {
							sliderAmendLayoutThumb(
								layout, w, value,
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
	onChange func(float32, *Event, *Window),
	value, minVal, maxVal, size, sizeBorder float32,
	vertical bool, colorFocus, colorLeft Color,
	disabled bool, idFocus uint32, roundValue bool,
) {
	if layout.Shape.events == nil {
		layout.Shape.events = &eventHandlers{}
	}
	layout.Shape.events.OnMouseScroll = func(
		_ *Layout, e *Event, w *Window,
	) {
		sliderOnMouseScroll(e, w, onChange,
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
			if pressed, ok := ps.Get(layout.Shape.ID); ok && pressed {
				thumb.Shape.Color = colorLeft
				return
			}
		}
		if w.IsFocus(idFocus) {
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
	onChange func(float32, *Event, *Window),
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
			onChange(v, e, w)
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
			onChange(v, e, w)
		}
	}
}

func sliderOnKeyDown(
	_ *Layout, e *Event, w *Window,
	onChange func(float32, *Event, *Window),
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
		onChange(v, e, w)
	}
}

func sliderOnMouseScroll(
	e *Event, w *Window,
	onChange func(float32, *Event, *Window),
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
		onChange(v, e, w)
	}
}
