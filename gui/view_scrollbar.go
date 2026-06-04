package gui

// ScrollbarOverflow determines when scrollbars are shown.
type ScrollbarOverflow uint8

// ScrollbarOverflow values.
const (
	ScrollbarAuto ScrollbarOverflow = iota
	ScrollbarHidden
	ScrollbarVisible
	ScrollbarOnHover
)

// ScrollbarCfg configures the style of a scrollbar.
type ScrollbarCfg struct {
	ID              string
	Size            float32
	MinThumbSize    float32
	Radius          float32
	RadiusThumb     float32
	GapEdge         float32
	GapEnd          float32
	IDScroll        uint32
	ColorThumb      Color
	ColorBackground Color
	Overflow        ScrollbarOverflow
	Orientation     ScrollbarOrientation
}

// Scrollbar layout constants.
const (
	scrollExtend  = 10
	scrollSnapMin = float32(0.03)
	scrollSnapMax = float32(0.97)
	thumbIndex    = 0
)

func applyScrollbarDefaults(cfg *ScrollbarCfg) {
	if !cfg.ColorThumb.IsSet() {
		cfg.ColorThumb = DefaultScrollbarStyle.ColorThumb
	}
	if !cfg.ColorBackground.IsSet() {
		cfg.ColorBackground = DefaultScrollbarStyle.ColorBackground
	}
	if cfg.Size == 0 {
		cfg.Size = DefaultScrollbarStyle.Size
	}
	if cfg.MinThumbSize == 0 {
		cfg.MinThumbSize = DefaultScrollbarStyle.MinThumbSize
	}
	if cfg.Radius == 0 {
		cfg.Radius = DefaultScrollbarStyle.Radius
	}
	if cfg.RadiusThumb == 0 {
		cfg.RadiusThumb = DefaultScrollbarStyle.RadiusThumb
	}
	if cfg.GapEdge == 0 {
		cfg.GapEdge = DefaultScrollbarStyle.GapEdge
	}
	if cfg.GapEnd == 0 {
		cfg.GapEnd = DefaultScrollbarStyle.GapEnd
	}
}

// Scrollbar creates a scrollbar overlay view.
func Scrollbar(cfg ScrollbarCfg) View {
	applyScrollbarDefaults(&cfg)

	thumbView := scrollbarThumb(cfg)

	if cfg.Orientation == ScrollbarHorizontal {
		return Row(ContainerCfg{
			ID:                   cfg.ID,
			A11YRole:             AccessRoleScrollBar,
			Color:                cfg.ColorBackground,
			OverDraw:             true,
			Padding:              NoPadding,
			scrollbarOrientation: ScrollbarHorizontal,
			AmendLayout:          makeScrollbarAmendLayout(cfg),
			OnHover:              makeScrollbarOnHover(cfg),
			OnClick:              makeScrollbarGutterClick(cfg),
			Content:              []View{thumbView},
		})
	}
	return Column(ContainerCfg{
		ID:                   cfg.ID,
		A11YRole:             AccessRoleScrollBar,
		Color:                cfg.ColorBackground,
		OverDraw:             true,
		Padding:              NoPadding,
		scrollbarOrientation: ScrollbarVertical,
		AmendLayout:          makeScrollbarAmendLayout(cfg),
		OnHover:              makeScrollbarOnHover(cfg),
		OnClick:              makeScrollbarGutterClick(cfg),
		Content:              []View{thumbView},
	})
}

func scrollbarThumb(cfg ScrollbarCfg) View {
	return Column(ContainerCfg{
		Color:   cfg.ColorThumb,
		Radius:  Some(cfg.RadiusThumb),
		Padding: NoPadding,
		OnClick: makeScrollbarOnMouseDown(cfg),
	})
}

func makeScrollbarAmendLayout(cfg ScrollbarCfg) func(*Layout, *Window) {
	return func(layout *Layout, w *Window) {
		scrollbarAmendLayout(cfg, layout, w)
	}
}

func makeScrollbarOnHover(cfg ScrollbarCfg) func(*Layout, *Event, *Window) {
	return func(layout *Layout, _ *Event, w *Window) {
		if len(layout.Children) == 0 {
			return
		}
		if layout.Children[thumbIndex].Shape.Color != ColorTransparent ||
			cfg.Overflow == ScrollbarOnHover {
			layout.Children[thumbIndex].Shape.Color = cfg.ColorThumb
			w.SetMouseCursor(CursorArrow)
		}
	}
}

func scrollbarAmendLayout(cfg ScrollbarCfg, layout *Layout, w *Window) {
	if layout.Parent == nil || len(layout.Children) == 0 {
		return
	}
	parent := layout.Parent

	if cfg.Orientation == ScrollbarHorizontal {
		layout.Shape.X = parent.Shape.X + parent.Shape.Padding.Left
		layout.Shape.Y = parent.Shape.Y + parent.Shape.Height - cfg.Size
		layout.Shape.Width = parent.Shape.Width - parent.Shape.Padding.Width()
		layout.Shape.Height = cfg.Size

		cWidth := contentWidth(parent)
		if cWidth == 0 {
			return
		}
		tWidth := layout.Shape.Width * (layout.Shape.Width / cWidth)
		thumbWidth := f32Clamp(tWidth, cfg.MinThumbSize, layout.Shape.Width)
		availWidth := layout.Shape.Width - thumbWidth

		sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
		scrollOffset := float32(0)
		if v, ok := sx.Get(cfg.IDScroll); ok {
			scrollOffset = -v
		}

		layout.Shape.X -= cfg.GapEnd
		layout.Shape.Y -= cfg.GapEdge
		layout.Shape.Width -= cfg.GapEnd + cfg.GapEnd

		offset := float32(0)
		if availWidth > 0 {
			offset = f32Clamp(
				(scrollOffset/(cWidth-layout.Shape.Width))*availWidth,
				0, availWidth)
		}
		layout.Children[thumbIndex].Shape.X = layout.Shape.X + offset
		layout.Children[thumbIndex].Shape.Y = layout.Shape.Y
		layout.Children[thumbIndex].Shape.Width = thumbWidth - cfg.GapEnd - cfg.GapEnd
		layout.Children[thumbIndex].Shape.Height = cfg.Size

		if (cfg.Overflow != ScrollbarVisible && availWidth < 0.1) ||
			cfg.Overflow == ScrollbarOnHover {
			layout.Children[thumbIndex].Shape.Color = ColorTransparent
		}
	} else {
		layout.Shape.X = parent.Shape.X + parent.Shape.Width - cfg.Size
		layout.Shape.Y = parent.Shape.Y + parent.Shape.Padding.Top
		layout.Shape.Width = cfg.Size
		layout.Shape.Height = parent.Shape.Height - parent.Shape.Padding.Height()

		cHeight := contentHeight(parent)
		if cHeight == 0 {
			return
		}
		tHeight := layout.Shape.Height * (layout.Shape.Height / cHeight)
		thumbHeight := f32Clamp(tHeight, cfg.MinThumbSize, layout.Shape.Height)
		availHeight := layout.Shape.Height - thumbHeight

		sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
		scrollOffset := float32(0)
		if v, ok := sy.Get(cfg.IDScroll); ok {
			scrollOffset = -v
		}

		layout.Shape.X -= cfg.GapEdge
		layout.Shape.Y += cfg.GapEnd
		layout.Shape.Height -= cfg.GapEnd + cfg.GapEnd

		layout.Children[thumbIndex].Shape.X = layout.Shape.X
		offset := float32(0)
		if availHeight > 0 {
			offset = f32Clamp(
				(scrollOffset/(cHeight-layout.Shape.Height))*availHeight,
				0, availHeight)
		}
		layout.Children[thumbIndex].Shape.Y = layout.Shape.Y + offset
		layout.Children[thumbIndex].Shape.Height = thumbHeight - cfg.GapEnd - cfg.GapEnd
		layout.Children[thumbIndex].Shape.Width = cfg.Size

		if (cfg.Overflow != ScrollbarVisible && availHeight < 0.1) ||
			cfg.Overflow == ScrollbarOnHover {
			layout.Children[thumbIndex].Shape.Color = ColorTransparent
		}
	}
}

// makeScrollbarOnMouseDown creates the thumb OnClick handler
// that initiates a drag via MouseLock.
func makeScrollbarOnMouseDown(cfg ScrollbarCfg) func(*Layout, *Event, *Window) {
	orientation := cfg.Orientation
	idScroll := cfg.IDScroll
	return func(_ *Layout, e *Event, w *Window) {
		w.MouseLock(MouseLockCfg{
			MouseMove: func(layout *Layout, e *Event, w *Window) {
				scrollbarMouseMove(orientation, idScroll, layout, e, w)
			},
			MouseUp: func(_ *Layout, _ *Event, w *Window) {
				w.MouseUnlock()
			},
		})
		e.IsHandled = true
	}
}

// makeScrollbarGutterClick creates the scrollbar container
// OnClick that jumps to the click position then locks mouse
// for continued dragging.
func makeScrollbarGutterClick(cfg ScrollbarCfg) func(*Layout, *Event, *Window) {
	orientation := cfg.Orientation
	idScroll := cfg.IDScroll
	return func(_ *Layout, e *Event, w *Window) {
		if w.MouseIsLocked() {
			return
		}
		if orientation == ScrollbarHorizontal {
			offsetFromMouseX(&w.layout, e.MouseX, idScroll, w)
		} else {
			offsetFromMouseY(&w.layout, e.MouseY, idScroll, w)
		}
		w.MouseLock(MouseLockCfg{
			MouseMove: func(layout *Layout, e *Event, w *Window) {
				scrollbarMouseMove(orientation, idScroll, layout, e, w)
			},
			MouseUp: func(_ *Layout, _ *Event, w *Window) {
				w.MouseUnlock()
			},
		})
		e.IsHandled = true
	}
}

// scrollbarMouseMove handles mouse movement during thumb drag.
func scrollbarMouseMove(orientation ScrollbarOrientation, idScroll uint32, layout *Layout, e *Event, w *Window) {
	ly, ok := FindLayoutByIDScroll(layout, idScroll)
	if !ok {
		return
	}
	if orientation == ScrollbarHorizontal {
		if e.MouseX >= ly.Shape.X-scrollExtend &&
			e.MouseX <= ly.Shape.X+ly.Shape.Width+scrollExtend {
			offset := offsetMouseChangeX(ly, e.MouseDX, idScroll, w)
			sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
			sx.Set(idScroll, offset)
			fireOnScroll(ly, w)
		}
	} else {
		if e.MouseY >= ly.Shape.Y-scrollExtend &&
			e.MouseY <= ly.Shape.Y+ly.Shape.Height+scrollExtend {
			offset := offsetMouseChangeY(ly, e.MouseDY, idScroll, w)
			sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
			sy.Set(idScroll, offset)
			fireOnScroll(ly, w)
		}
	}
}

// offsetMouseChangeX calculates new horizontal offset based on
// mouse movement delta.
func offsetMouseChangeX(layout *Layout, mouseDX float32, idScroll uint32, w *Window) float32 {
	totalWidth := contentWidth(layout)
	shapeWidth := layout.Shape.Width - layout.Shape.PaddingWidth()
	sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
	oldOffset, _ := sx.Get(idScroll) // ok ignored: zero offset is correct initial scroll
	newOffset := mouseDX * (totalWidth / shapeWidth)
	offset := oldOffset - newOffset
	return f32Min(0, f32Max(offset, shapeWidth-totalWidth))
}

// offsetMouseChangeY calculates new vertical offset based on
// mouse movement delta.
func offsetMouseChangeY(layout *Layout, mouseDY float32, idScroll uint32, w *Window) float32 {
	totalHeight := contentHeight(layout)
	shapeHeight := layout.Shape.Height - layout.Shape.PaddingHeight()
	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	oldOffset, _ := sy.Get(idScroll) // ok ignored: zero offset is correct initial scroll
	newOffset := mouseDY * (totalHeight / shapeHeight)
	offset := oldOffset - newOffset
	return f32Min(0, f32Max(offset, shapeHeight-totalHeight))
}

// offsetFromMouseX calculates and applies horizontal offset
// from absolute mouse x position.
func offsetFromMouseX(layout *Layout, mouseX float32, idScroll uint32, w *Window) {
	sb, ok := FindLayoutByIDScroll(layout, idScroll)
	if !ok {
		return
	}
	totalWidth := contentWidth(sb)
	percent := (mouseX - sb.Shape.X) / sb.Shape.Width
	percent = f32Clamp(percent, 0, 1)
	if percent <= scrollSnapMin {
		percent = 0
	}
	if percent >= scrollSnapMax {
		percent = 1
	}
	sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
	sx.Set(idScroll, -percent*(totalWidth-sb.Shape.Width))
	fireOnScroll(sb, w)
}

// offsetFromMouseY calculates and applies vertical offset
// from absolute mouse y position.
func offsetFromMouseY(layout *Layout, mouseY float32, idScroll uint32, w *Window) {
	sb, ok := FindLayoutByIDScroll(layout, idScroll)
	if !ok {
		return
	}
	totalHeight := contentHeight(sb)
	percent := (mouseY - sb.Shape.Y) / sb.Shape.Height
	percent = f32Clamp(percent, 0, 1)
	if percent <= scrollSnapMin {
		percent = 0
	}
	if percent >= scrollSnapMax {
		percent = 1
	}
	sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
	sy.Set(idScroll, -percent*(totalHeight-sb.Shape.Height))
	fireOnScroll(sb, w)
}
