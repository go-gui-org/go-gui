package gui

// ScrollbarOverflow determines when scrollbars are shown.
type ScrollbarOverflow uint8

// ScrollbarOverflow values.
const (
	ScrollbarAuto ScrollbarOverflow = iota
	ScrollbarHidden
	// exportaudit:keep — one member of a public enum; the set ships whole.
	ScrollbarVisible
	scrollbarOnHover
)

// ScrollbarCfg configures the style of a scrollbar.
type ScrollbarCfg struct {
	ID   string
	Size float32 // ergonomics-audit:opt-plain — a zero-thickness scrollbar is meaningless; 0 means the theme default
	// MinThumbSize is the smallest the thumb may shrink to. Zero
	// takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	MinThumbSize float32
	Radius       Opt[float32] // 0 is a real choice (square thumb); unset falls back to the theme
	// RadiusThumb rounds the thumb. Zero takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusThumb float32
	// GapEdge insets the bar from the edge it tracks; GapEnd shortens
	// the track at both ends. Opt, not plain float32: zero is a real
	// choice (a bar flush against the edge, a full-length track), so
	// it must stay distinguishable from unset, which takes the theme.
	GapEdge  Opt[float32]
	GapEnd   Opt[float32]
	scrollID string `gui:"required"`
	// ColorThumb paints the thumb. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorThumb      Color
	ColorBackground Color
	Overflow        ScrollbarOverflow
	Orientation     ScrollbarOrientation

	// Thumb draws the thumb in place of the stock one. The scrollbar
	// still sizes and moves the thumb, starts the drag and hides it when
	// nothing overflows; the view fills the thumb, so give its root a
	// Fill sizing. The ColorThumb and RadiusThumb fields are not used.
	//
	// Track draws the gutter under the thumb, filling the bar. A press
	// on it still jumps the scroll position there.
	//
	// Both run during layout generation, under the frame lock. The size
	// in ScrollbarState is from the last layout pass; when it changes the
	// scrollbar asks for one more pass, so a frame shows the view built
	// for its size. A scrollbar with either hook gets the ID leaf
	// "scrollbar-x" or "scrollbar-y" when ID is empty, so hover and press
	// can be read (#664, docs/specs/scrollbar-look-hook.md).
	Thumb func(ScrollbarState) View
	Track func(ScrollbarState) View
}

// ScrollbarState is what a scrollbar Thumb or Track hook gets.
type ScrollbarState struct {
	// Hovered is true when the pointer is over the scrollbar, track or
	// thumb.
	Hovered bool
	// Pressed is true while a press that started on the scrollbar is
	// held, which is while its drag runs.
	Pressed bool
	// Vertical is true for the vertical scrollbar.
	Vertical bool
	// Width and Height are the size of the part from the last layout
	// pass. Both are 0 before the first pass.
	Width, Height float32
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
		cfg.ColorThumb = DefaultScrollbarStyle.colorThumb
	}
	if !cfg.ColorBackground.IsSet() {
		cfg.ColorBackground = DefaultScrollbarStyle.ColorBackground
	}
	if cfg.Size == 0 {
		cfg.Size = DefaultScrollbarStyle.Size
	}
	if cfg.MinThumbSize == 0 {
		cfg.MinThumbSize = DefaultScrollbarStyle.minThumbSize
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = Some(DefaultScrollbarStyle.Radius)
	}
	if cfg.RadiusThumb == 0 {
		cfg.RadiusThumb = DefaultScrollbarStyle.radiusThumb
	}
	if !cfg.GapEdge.IsSet() {
		cfg.GapEdge = SomeF(DefaultScrollbarStyle.GapEdge)
	}
	if !cfg.GapEnd.IsSet() {
		cfg.GapEnd = SomeF(DefaultScrollbarStyle.GapEnd)
	}
}

// Scrollbar creates a scrollbar overlay view.
func scrollbar(cfg ScrollbarCfg) View {
	applyScrollbarDefaults(&cfg)
	if cfg.Thumb != nil || cfg.Track != nil {
		return scrollbarLookView{cfg: cfg}
	}

	thumbView := scrollbarThumb(cfg)

	if cfg.Orientation == scrollbarHorizontal {
		return Row(ContainerCfg{
			ID:                   cfg.ID,
			A11YRole:             AccessRoleScrollBar,
			Color:                cfg.ColorBackground,
			OverDraw:             true,
			Padding:              NoPadding,
			scrollbarOrientation: scrollbarHorizontal,
			AmendLayout:          makeScrollbarAmendLayout(cfg),
			OnHover:              makeScrollbarOnHover(cfg, thumbIndex),
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
		scrollbarOrientation: scrollbarVertical,
		AmendLayout:          makeScrollbarAmendLayout(cfg),
		OnHover:              makeScrollbarOnHover(cfg, thumbIndex),
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

func makeScrollbarAmendLayout(cfg ScrollbarCfg) func(EventCtx) {
	return func(ctx EventCtx) {
		scrollbarAmendLayout(cfg, ctx, ctx.Layout, ctx.Window, thumbIndex)
	}
}

// makeScrollbarOnHover shows the stock thumb in its color under the
// pointer. thumbAt is the thumb's child index. A Thumb hook draws its own
// hover look from ScrollbarState, so its color is left alone.
func makeScrollbarOnHover(cfg ScrollbarCfg, thumbAt int) func(EventCtx) {
	return func(ctx EventCtx) {
		if len(ctx.Layout.Children) <= thumbAt {
			return
		}
		if cfg.Thumb != nil {
			ctx.Window.setMouseCursor(CursorArrow)
			return
		}
		if ctx.Layout.Children[thumbAt].Shape.Color != ColorTransparent ||
			cfg.Overflow == scrollbarOnHover {
			ctx.Layout.Children[thumbAt].Shape.Color = cfg.ColorThumb
			ctx.Window.setMouseCursor(CursorArrow)
		}
	}
}

func scrollbarAmendLayout(
	cfg ScrollbarCfg, ctx EventCtx, layout *Layout, w *Window, thumbAt int,
) {
	if layout.Parent == nil || len(layout.Children) <= thumbAt {
		return
	}
	// ScrollID names the scrollable this bar drives, and it arrives as
	// the leaf its container was written with. The scroll maps are keyed
	// by effective ID, so resolve it against this shape's ancestors —
	// the scrollable is one of them. cfg is already a copy, so this
	// costs nothing beyond the lookup.
	cfg.scrollID = ctx.EffID(cfg.scrollID)
	// applyScrollbarDefaults has already filled both gaps, so the
	// zero fallback here is unreachable; read them once rather than
	// unwrap the Opt at every arithmetic site below.
	gapEdge := cfg.GapEdge.Get(0)
	gapEnd := cfg.GapEnd.Get(0)
	parent := layout.Parent

	if cfg.Orientation == scrollbarHorizontal {
		layout.Shape.X = parent.Shape.X + parent.Shape.Padding.Left
		layout.Shape.Y = parent.Shape.Y + parent.Shape.Height - cfg.Size
		layout.Shape.Width = parent.Shape.Width - parent.Shape.Padding.Width()
		layout.Shape.Height = cfg.Size

		cWidth := contentWidth(parent)
		if cWidth == 0 {
			return
		}
		// The track is the viewport width here, before GapEnd insets it.
		// Keep it: the offset below maps the content travel onto the
		// thumb travel, and both are measured against the viewport.
		viewW := layout.Shape.Width
		thumbWidth, availWidth := scrollbarThumbGeom(viewW, cWidth, cfg.MinThumbSize)

		sx := w.scrollX()
		scrollOffset := float32(0)
		if v, ok := sx.Get(cfg.scrollID); ok {
			scrollOffset = -v
		}

		layout.Shape.X += gapEnd
		layout.Shape.Y -= gapEdge
		layout.Shape.Width -= gapEnd + gapEnd

		offset := float32(0)
		if availWidth > 0 {
			offset = f32Clamp(
				(scrollOffset/(cWidth-viewW))*availWidth,
				0, availWidth)
			// The gutter is drawn left to right whichever way the row runs,
			// but an RTL row's start is its right edge (scrollMirrorsX), so
			// the unscrolled thumb belongs at the right end.
			if scrollMirrorsX(parent.Shape) {
				offset = availWidth - offset
			}
		}
		layout.Children[thumbAt].Shape.X = layout.Shape.X + offset
		layout.Children[thumbAt].Shape.Y = layout.Shape.Y
		layout.Children[thumbAt].Shape.Width = thumbWidth - gapEnd - gapEnd
		layout.Children[thumbAt].Shape.Height = cfg.Size

		if (cfg.Overflow != ScrollbarVisible && availWidth < 0.1) ||
			cfg.Overflow == scrollbarOnHover {
			layout.Children[thumbAt].Shape.Color = ColorTransparent
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
		// See the horizontal branch: keep the viewport height, which
		// GapEnd shrinks below.
		viewH := layout.Shape.Height
		thumbHeight, availHeight := scrollbarThumbGeom(viewH, cHeight, cfg.MinThumbSize)

		sy := w.scrollY()
		scrollOffset := float32(0)
		if v, ok := sy.Get(cfg.scrollID); ok {
			scrollOffset = -v
		}

		layout.Shape.X -= gapEdge
		layout.Shape.Y += gapEnd
		layout.Shape.Height -= gapEnd + gapEnd

		layout.Children[thumbAt].Shape.X = layout.Shape.X
		offset := float32(0)
		if availHeight > 0 {
			offset = f32Clamp(
				(scrollOffset/(cHeight-viewH))*availHeight,
				0, availHeight)
		}
		layout.Children[thumbAt].Shape.Y = layout.Shape.Y + offset
		layout.Children[thumbAt].Shape.Height = thumbHeight - gapEnd - gapEnd
		layout.Children[thumbAt].Shape.Width = cfg.Size

		if (cfg.Overflow != ScrollbarVisible && availHeight < 0.1) ||
			cfg.Overflow == scrollbarOnHover {
			layout.Children[thumbAt].Shape.Color = ColorTransparent
		}
	}
}

// scrollbarThumbGeom returns the thumb length for a track of length
// track that shows a window onto content of length content, and how far
// the thumb can travel along the track. The thumb is the viewport's share
// of the track, but never shorter than minThumb. The drag handlers use the
// travel too, so a thumb and the pointer that drags it move together.
func scrollbarThumbGeom(track, content, minThumb float32) (thumb, travel float32) {
	thumb = f32Clamp(track*(track/content), minThumb, track)
	return thumb, track - thumb
}

// makeScrollbarOnMouseDown creates the thumb OnClick handler
// that initiates a drag via MouseLock.
func makeScrollbarOnMouseDown(cfg ScrollbarCfg) func(EventCtx) {
	orientation := cfg.Orientation
	minThumb := cfg.MinThumbSize
	leafScrollID := cfg.scrollID
	return func(ctx EventCtx) {
		scrollID := ctx.EffID(leafScrollID)
		ctx.Window.MouseLock(MouseLockCfg{
			MouseMove: func(ctx EventCtx) {
				scrollbarMouseMove(orientation, scrollID, ctx.Layout, ctx.Event, ctx.Window, minThumb)
			},
			MouseUp: func(ctx EventCtx) {
				ctx.Window.MouseUnlock()
			},
		})
		ctx.Consume()
	}
}

// makeScrollbarGutterClick creates the scrollbar container
// OnClick that jumps to the click position then locks mouse
// for continued dragging.
func makeScrollbarGutterClick(cfg ScrollbarCfg) func(EventCtx) {
	orientation := cfg.Orientation
	minThumb := cfg.MinThumbSize
	leafScrollID := cfg.scrollID
	return func(ctx EventCtx) {
		scrollID := ctx.EffID(leafScrollID)
		if ctx.Window.mouseIsLocked() {
			// A drag already owns the pointer, so this click is not the
			// gutter's to act on — but it is still not the enclosing
			// scroll container's either. Consume so the early return
			// keeps swallowing it once the pre-mark is gone.
			ctx.Consume()
			return
		}
		if orientation == scrollbarHorizontal {
			offsetFromMouseX(&ctx.Window.layout, ctx.Event.MouseX, scrollID, ctx.Window)
		} else {
			offsetFromMouseY(&ctx.Window.layout, ctx.Event.MouseY, scrollID, ctx.Window)
		}
		ctx.Window.MouseLock(MouseLockCfg{
			MouseMove: func(ctx EventCtx) {
				scrollbarMouseMove(orientation, scrollID, ctx.Layout, ctx.Event, ctx.Window, minThumb)
			},
			MouseUp: func(ctx EventCtx) {
				ctx.Window.MouseUnlock()
			},
		})
		ctx.Consume()
	}
}

// scrollbarMouseMove handles mouse movement during thumb drag.
func scrollbarMouseMove(orientation ScrollbarOrientation, scrollID string, layout *Layout, e *Event, w *Window, minThumb float32) {
	ly, ok := findLayoutByScrollID(layout, scrollID)
	if !ok {
		return
	}
	if orientation == scrollbarHorizontal {
		if e.MouseX >= ly.Shape.X-scrollExtend &&
			e.MouseX <= ly.Shape.X+ly.Shape.Width+scrollExtend {
			sx := w.scrollX()
			offset := offsetMouseChangeX(sx, ly, e.MouseDX, scrollID, minThumb)
			sx.Set(scrollID, offset)
			scrollSmoothCancel(w, scrollID, scrollAxisX)
			fireOnScroll(ly, w)
		}
	} else {
		if e.MouseY >= ly.Shape.Y-scrollExtend &&
			e.MouseY <= ly.Shape.Y+ly.Shape.Height+scrollExtend {
			sy := w.scrollY()
			offset := offsetMouseChangeY(sy, ly, e.MouseDY, scrollID, minThumb)
			sy.Set(scrollID, offset)
			scrollSmoothCancel(w, scrollID, scrollAxisY)
			fireOnScroll(ly, w)
		}
	}
}

// offsetMouseChangeX calculates new horizontal offset based on
// mouse movement delta.
func offsetMouseChangeX(sx *BoundedMap[string, float32], layout *Layout, mouseDX float32, scrollID string, minThumb float32) float32 {
	totalWidth := contentWidth(layout)
	shapeWidth := layout.Shape.Width - layout.Shape.paddingWidth()
	// Default 0: unscrolled position when no offset recorded yet.
	oldOffset := sx.GetOr(scrollID, 0)
	// Degenerate viewport: avoid division by zero / NaN offset.
	if shapeWidth <= 0 {
		return oldOffset
	}
	// A thumb that fills its track has nowhere to go.
	_, travel := scrollbarThumbGeom(shapeWidth, totalWidth, minThumb)
	if travel <= 0 {
		return oldOffset
	}
	// An RTL row's thumb travels toward the overflow as it moves left
	// (scrollMirrorsX), so the drag delta arrives mirrored.
	if scrollMirrorsX(layout.Shape) {
		mouseDX = -mouseDX
	}
	// One pixel of thumb travel moves the content by the content travel
	// over the thumb travel. total/view gives the same figure only while
	// the thumb has its natural size; once minThumb clamps it, the thumb
	// travels less and the thumb drifts away from the pointer.
	newOffset := mouseDX * ((totalWidth - shapeWidth) / travel)
	offset := oldOffset - newOffset
	return f32Min(0, f32Max(offset, shapeWidth-totalWidth))
}

// offsetMouseChangeY calculates new vertical offset based on
// mouse movement delta.
func offsetMouseChangeY(sy *BoundedMap[string, float32], layout *Layout, mouseDY float32, scrollID string, minThumb float32) float32 {
	totalHeight := contentHeight(layout)
	shapeHeight := layout.Shape.Height - layout.Shape.paddingHeight()
	// Default 0: unscrolled position when no offset recorded yet.
	oldOffset := sy.GetOr(scrollID, 0)
	// Degenerate viewport: avoid division by zero / NaN offset.
	if shapeHeight <= 0 {
		return oldOffset
	}
	// See offsetMouseChangeX for the travel ratio.
	_, travel := scrollbarThumbGeom(shapeHeight, totalHeight, minThumb)
	if travel <= 0 {
		return oldOffset
	}
	newOffset := mouseDY * ((totalHeight - shapeHeight) / travel)
	offset := oldOffset - newOffset
	return f32Min(0, f32Max(offset, shapeHeight-totalHeight))
}

// offsetFromMouseX calculates and applies horizontal offset
// from absolute mouse x position.
func offsetFromMouseX(layout *Layout, mouseX float32, scrollID string, w *Window) {
	sb, ok := findLayoutByScrollID(layout, scrollID)
	if !ok {
		return
	}
	// Measure against the viewport, not the outer box: padding narrows
	// the gutter the bar is drawn in, and scrollMaxOffsetX is the range
	// the pipeline clamps to. Mapping onto the outer width stopped a
	// press at the far end short by the padding, and wrote a positive
	// offset over content that fits.
	viewW := sb.Shape.Width - sb.Shape.paddingWidth()
	if viewW <= 0 || !f32IsFinite(viewW) {
		return
	}
	percent := (mouseX - (sb.Shape.X + sb.Shape.PaddingLeft())) / viewW
	if !f32IsFinite(percent) {
		return
	}
	percent = f32Clamp(percent, 0, 1)
	if percent <= scrollSnapMin {
		percent = 0
	}
	if percent >= scrollSnapMax {
		percent = 1
	}
	// The left end of an RTL bar is the end of its range (scrollMirrorsX).
	if scrollMirrorsX(sb.Shape) {
		percent = 1 - percent
	}
	sx := w.scrollX()
	sx.Set(scrollID, percent*scrollMaxOffsetX(sb))
	scrollSmoothCancel(w, scrollID, scrollAxisX)
	fireOnScroll(sb, w)
}

// offsetFromMouseY calculates and applies vertical offset
// from absolute mouse y position.
func offsetFromMouseY(layout *Layout, mouseY float32, scrollID string, w *Window) {
	sb, ok := findLayoutByScrollID(layout, scrollID)
	if !ok {
		return
	}
	// See offsetFromMouseX: measure against the viewport.
	viewH := sb.Shape.Height - sb.Shape.paddingHeight()
	if viewH <= 0 || !f32IsFinite(viewH) {
		return
	}
	percent := (mouseY - (sb.Shape.Y + sb.Shape.PaddingTop())) / viewH
	if !f32IsFinite(percent) {
		return
	}
	percent = f32Clamp(percent, 0, 1)
	if percent <= scrollSnapMin {
		percent = 0
	}
	if percent >= scrollSnapMax {
		percent = 1
	}
	sy := w.scrollY()
	sy.Set(scrollID, percent*scrollMaxOffsetY(sb))
	scrollSmoothCancel(w, scrollID, scrollAxisY)
	fireOnScroll(sb, w)
}
