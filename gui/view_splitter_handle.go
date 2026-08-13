package gui

// splitterButtonSuffix maps SplitterCollapsed → button ID suffix.
var splitterButtonSuffix = [3]string{
	":button:0",
	":button:1",
	":button:2",
}

// splitterHandleView builds the drag handle. id is the splitter's
// effective ID, so the handle and its buttons are named under the same
// path the framework stamps on the splitter's own shape.
func splitterHandleView(cfg *SplitterCfg, core *splitterCore, id string) View {
	content := make([]View, 0, 3)
	if cfg.ShowCollapseButtons &&
		(cfg.First.collapsible || cfg.Second.collapsible) {
		if cfg.First.collapsible {
			content = append(content,
				splitterButton(cfg, core, SplitterCollapseFirst, id))
		}
		content = append(content, splitterGrip(cfg))
		if cfg.Second.collapsible {
			content = append(content,
				splitterButton(cfg, core, SplitterCollapseSecond, id))
		}
	} else {
		content = append(content, splitterGrip(cfg))
	}

	orientation := cfg.Orientation

	s := &defaultSplitterStyle
	handleSize := cfg.HandleSize.Get(s.HandleSize)
	var handleWidth, handleHeight float32
	if orientation == SplitterHorizontal {
		handleWidth = handleSize
	} else {
		handleHeight = handleSize
	}

	handleCfg := ContainerCfg{
		ID:          ScopeID(id, "handle"),
		Sizing:      FixedFixed,
		Width:       handleWidth,
		Height:      handleHeight,
		Padding:     NoPadding,
		Spacing:     SomeF(1),
		Color:       cfg.colorHandle,
		ColorBorder: cfg.colorHandleBorder,
		SizeBorder:  cfg.SizeBorder,
		Radius:      cfg.Radius,
		HAlign:      HAlignCenter,
		VAlign:      VAlignMiddle,
		OnClick: func(ctx EventCtx) {
			splitterOnHandleClick(core, ctx.Layout, ctx.Event, ctx.Window)
		},
		OnHover: func(ctx EventCtx) {
			splitterOnHandleHover(core, ctx.Layout, ctx.Event, ctx.Window)
		},
		Content: content,
	}

	if orientation == SplitterHorizontal {
		return Column(handleCfg)
	}
	return Row(handleCfg)
}

func splitterGrip(cfg *SplitterCfg) View {
	s := &defaultSplitterStyle
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
		Color:  cfg.colorGrip,
		Radius: cfg.radiusBorder.Get(s.radiusBorder),
		Sizing: FixedFixed,
	})
}

func splitterButton(cfg *SplitterCfg, core *splitterCore,
	target SplitterCollapsed, id string) View {
	s := &defaultSplitterStyle
	size := f32Max(4, cfg.HandleSize.Get(s.HandleSize)-2)
	ts := TextStyle{
		Color: cfg.colorButtonIcon,
		Size:  size,
	}
	return Button(ButtonCfg{
		ID:      id + splitterButtonSuffix[target],
		Width:   size,
		Height:  size,
		Sizing:  FixedFixed,
		Padding: NoPadding,
		Color:   cfg.colorButton,
		Colors:  ColorSet{Hover: cfg.colorButtonHover, Click: cfg.colorButtonActive, Focus: cfg.colorButtonHover},
		Radius:  cfg.radiusBorder,
		OnClick: func(ctx EventCtx) {
			splitterOnButtonClick(core, target, ctx.Event, ctx.Window)
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
	// Space reads from KeyCode, not CharCode: backends populate
	// CharCode only on EventChar, so a keydown-only handler that tested
	// CharCode never saw the spacebar. Matches DatePicker, Tree, Menu
	// and the rest.
	case KeyEnter, KeySpace:
		if isNone {
			nextCollapsed, handled = splitterToggleCollapse(
				core, nextCollapsed)
		}
	}
	// Arrow keys clear collapse state.
	if handled {
		switch e.KeyCode {
		case KeyLeft, KeyRight, KeyUp, KeyDown:
			nextCollapsed = splitterCollapseNone
		}
	}

	if handled {
		splitterEmitChange(core, nextRatio, nextCollapsed, e, w)
	}
}

// splitterOnHandleClick arms the drag: it locks the mouse, records the
// pressed flag the amend paint keys on, and paints the handle active
// immediately (the press frame's amend pass already ran, so this is the
// only write that colors this frame).
func splitterOnHandleClick(core *splitterCore, layout *Layout, e *Event, w *Window) {
	if core.disabled {
		return
	}
	splitterSetCursor(core.orientation, w)
	splitterFocus(core, w)

	// Pressed state is keyed by the splitter's effective ID — stamped
	// on the core by splitterAmendLayout and the same key the amend
	// paint reads off the root shape. The core itself is rebuilt by the
	// view function every frame, so cross-frame drag state lives in
	// window state, mirroring the slider's nsSliderPress.
	StateMap[string, bool](w, nsSplitterDrag, capModerate).Set(core.id, true)
	layout.Shape.Color = core.colorHandleActive

	focusID := core.focusID
	w.MouseLock(MouseLockCfg{
		MouseMove: func(ctx EventCtx) {
			splitterOnDragMove(core, ctx.Event, ctx.Window)
		},
		MouseUp: func(ctx EventCtx) {
			// Clear the pressed look first; the unlock that follows
			// lets hover take over painting the handle next frame.
			StateMap[string, bool](ctx.Window, nsSplitterDrag, capModerate).
				Set(core.id, false)
			ctx.Window.MouseUnlock()
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
		},
		Cancel: func(w *Window) {
			// Capture revoked without a release (see MouseCancel,
			// issue #237): the pressed flag must not pin the handle
			// active forever.
			StateMap[string, bool](w, nsSplitterDrag, capModerate).
				Set(core.id, false)
		},
	})
	e.IsHandled = true
}

// splitterOnHandleHover paints the hover color. The active color is not
// this handler's job: layoutHover bails while the mouse is locked and
// always synthesizes its event with MouseButton: MouseInvalid, so a
// button branch here could never fire through the real pipeline
// (issue #265) — the drag paint lives in splitterAmendLayout instead.
func splitterOnHandleHover(core *splitterCore, layout *Layout, e *Event, w *Window) {
	splitterSetCursor(core.orientation, w)
	layout.Shape.Color = core.colorHandleHover
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
	if validTarget == splitterCollapseNone {
		return
	}
	ratio := splitterCurrentRatio(core, w)
	current := splitterEffectiveCollapsed(core, core.collapsed)
	next := validTarget
	if current == validTarget {
		next = splitterCollapseNone
	}
	splitterEmitChange(core, ratio, next, e, w)
}
