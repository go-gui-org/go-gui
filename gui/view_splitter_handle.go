package gui

// splitterButtonSuffix maps SplitterCollapsed → button ID suffix.
var splitterButtonSuffix = [3]string{
	":button:0",
	":button:1",
	":button:2",
}

func splitterHandleView(cfg *SplitterCfg, core *splitterCore) View {
	content := make([]View, 0, 3)
	if cfg.ShowCollapseButtons &&
		(cfg.First.Collapsible || cfg.Second.Collapsible) {
		if cfg.First.Collapsible {
			content = append(content,
				splitterButton(cfg, core, SplitterCollapseFirst))
		}
		content = append(content, splitterGrip(cfg))
		if cfg.Second.Collapsible {
			content = append(content,
				splitterButton(cfg, core, SplitterCollapseSecond))
		}
	} else {
		content = append(content, splitterGrip(cfg))
	}

	orientation := cfg.Orientation
	colorHover := cfg.ColorHandleHover
	colorActive := cfg.ColorHandleActive

	s := &DefaultSplitterStyle
	handleSize := cfg.HandleSize.Get(s.HandleSize)
	var handleWidth, handleHeight float32
	if orientation == SplitterHorizontal {
		handleWidth = handleSize
	} else {
		handleHeight = handleSize
	}

	handleCfg := ContainerCfg{
		ID:          cfg.ID + ":handle",
		Sizing:      FixedFixed,
		Width:       handleWidth,
		Height:      handleHeight,
		Padding:     NoPadding,
		Spacing:     SomeF(1),
		Color:       cfg.ColorHandle,
		ColorBorder: cfg.ColorHandleBorder,
		SizeBorder:  cfg.SizeBorder,
		Radius:      cfg.Radius,
		HAlign:      HAlignCenter,
		VAlign:      VAlignMiddle,
		OnClick: func(_ *Layout, e *Event, w *Window) {
			splitterOnHandleClick(core, e, w)
		},
		OnHover: func(layout *Layout, e *Event, w *Window) {
			splitterOnHandleHover(orientation, colorHover,
				colorActive, layout, e, w)
		},
		Content: content,
	}

	if orientation == SplitterHorizontal {
		return Column(handleCfg)
	}
	return Row(handleCfg)
}

func splitterGrip(cfg *SplitterCfg) View {
	s := &DefaultSplitterStyle
	handleSize := cfg.HandleSize.Get(s.HandleSize)
	isHoriz := cfg.Orientation == SplitterHorizontal
	var w, h float32
	if isHoriz {
		w = f32Max(2, handleSize*0.35)
		h = f32Max(14, handleSize*2.0)
	} else {
		w = f32Max(14, handleSize*2.0)
		h = f32Max(2, handleSize*0.35)
	}
	return Rectangle(RectangleCfg{
		Width:  w,
		Height: h,
		Color:  cfg.ColorGrip,
		Radius: cfg.RadiusBorder.Get(s.RadiusBorder),
		Sizing: FixedFixed,
	})
}

func splitterButton(cfg *SplitterCfg, core *splitterCore,
	target SplitterCollapsed) View {
	s := &DefaultSplitterStyle
	size := f32Max(4, cfg.HandleSize.Get(s.HandleSize)-2)
	ts := TextStyle{
		Color: cfg.ColorButtonIcon,
		Size:  size,
	}
	return Button(ButtonCfg{
		ID:         cfg.ID + splitterButtonSuffix[target],
		Width:      size,
		Height:     size,
		Sizing:     FixedFixed,
		Padding:    NoPadding,
		Color:      cfg.ColorButton,
		ColorHover: cfg.ColorButtonHover,
		ColorClick: cfg.ColorButtonActive,
		ColorFocus: cfg.ColorButtonHover,
		Radius:     cfg.RadiusBorder,
		OnClick: func(_ *Layout, e *Event, w *Window) {
			splitterOnButtonClick(core, target, e, w)
		},
		Content: []View{
			Text(TextCfg{
				Text:      splitterButtonIcon(core, target),
				TextStyle: ts,
			}),
		},
	})
}

func splitterButtonIcon(core *splitterCore, target SplitterCollapsed) string {
	current := splitterEffectiveCollapsed(core, core.collapsed)
	if core.orientation == SplitterHorizontal {
		if target == SplitterCollapseFirst {
			if current == SplitterCollapseFirst {
				return "▶"
			}
			return "◀"
		}
		if current == SplitterCollapseSecond {
			return "◀"
		}
		return "▶"
	}
	if target == SplitterCollapseFirst {
		if current == SplitterCollapseFirst {
			return "▼"
		}
		return "▲"
	}
	if current == SplitterCollapseSecond {
		return "▲"
	}
	return "▼"
}

// --- Event handlers ---

func splitterOnKeydown(core *splitterCore, e *Event, w *Window) {
	if core.disabled {
		return
	}
	ly, ok := w.layout.FindByID(core.id)
	if !ok {
		return
	}
	mainSz := splitterMainSize(ly, core.orientation)
	handle := splitterHandleSizeFromLayout(ly, core.orientation,
		core.handleSize)
	available := f32Max(0, mainSz-handle)

	nextRatio := splitterClampRatio(core, available, core.ratio)
	nextCollapsed := splitterEffectiveCollapsed(core, core.collapsed)
	handled := false

	isNone := e.Modifiers == ModNone

	switch e.KeyCode {
	case KeyLeft:
		nextRatio, handled = splitterArrowStep(core,
			SplitterHorizontal, -1, e.Modifiers, available, nextRatio)
	case KeyRight:
		nextRatio, handled = splitterArrowStep(core,
			SplitterHorizontal, +1, e.Modifiers, available, nextRatio)
	case KeyUp:
		nextRatio, handled = splitterArrowStep(core,
			SplitterVertical, -1, e.Modifiers, available, nextRatio)
	case KeyDown:
		nextRatio, handled = splitterArrowStep(core,
			SplitterVertical, +1, e.Modifiers, available, nextRatio)
	case KeyHome:
		if isNone && core.first.collapsible {
			nextCollapsed = SplitterCollapseFirst
			handled = true
		}
	case KeyEnd:
		if isNone && core.second.collapsible {
			nextCollapsed = SplitterCollapseSecond
			handled = true
		}
	case KeyEnter:
		if isNone {
			nextCollapsed, handled = splitterToggleCollapse(
				core, nextCollapsed)
		}
	default:
		if e.CharCode == CharSpace && isNone {
			nextCollapsed, handled = splitterToggleCollapse(
				core, nextCollapsed)
		}
	}
	// Arrow keys clear collapse state.
	if handled {
		switch e.KeyCode {
		case KeyLeft, KeyRight, KeyUp, KeyDown:
			nextCollapsed = SplitterCollapseNone
		}
	}

	if handled {
		splitterEmitChange(core, nextRatio, nextCollapsed, e, w)
	}
}

func splitterOnHandleClick(core *splitterCore, e *Event, w *Window) {
	if core.disabled {
		return
	}
	splitterSetCursor(core.orientation, w)
	splitterFocus(core, w)

	focusID := core.focusID
	w.MouseLock(MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			splitterOnDragMove(core, e, w)
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			w.MouseUnlock()
			if focusID != "" {
				w.SetFocus(focusID)
			}
		},
	})
	e.IsHandled = true
}

func splitterOnHandleHover(
	orientation SplitterOrientation,
	colorHover, colorActive Color,
	layout *Layout, e *Event, w *Window,
) {
	splitterSetCursor(orientation, w)
	layout.Shape.Color = colorHover
	if e.MouseButton == MouseLeft {
		layout.Shape.Color = colorActive
	}
	e.IsHandled = true
}

func splitterOnButtonClick(
	core *splitterCore,
	target SplitterCollapsed,
	e *Event, w *Window,
) {
	if core.disabled {
		return
	}
	validTarget := splitterEffectiveCollapsed(core, target)
	if validTarget == SplitterCollapseNone {
		return
	}
	ratio := splitterCurrentRatio(core, w)
	current := splitterEffectiveCollapsed(core, core.collapsed)
	next := validTarget
	if current == validTarget {
		next = SplitterCollapseNone
	}
	splitterEmitChange(core, ratio, next, e, w)
}
