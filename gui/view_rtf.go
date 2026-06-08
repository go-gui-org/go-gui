package gui

// view_rtf.go defines the Rich Text Format (RTF) view.
// Renders text with multiple typefaces, sizes, and styles.
// Supports text wrapping, clickable links, and custom runs.

import (
	"math"
	"strings"
	"time"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui/markdown"
)

// RtfCfg configures a Rich Text View.
type RtfCfg struct {
	BaseTextStyle *TextStyle

	ID              string
	A11YLabel       string
	A11YDescription string
	RichText        RichText
	MinWidth        float32
	IDFocus         uint32
	HangingIndent   float32 // negative indent for wrapped lines

	// markdownID > 0 when this block belongs to a markdown widget.
	// markdownBlockStart is the rune offset of this block in the
	// markdown's flat text. Both are set by view_markdown.go only.
	markdownID         uint32
	markdownBlockStart uint32
	Mode               TextMode
	Invisible          bool
	Clip               bool
	FocusSkip          bool
	Disabled           bool
}

// rtfFlatTextFromRuns concatenates all run texts into a single string.
// Used as the flat text for rune↔byte conversion during selection.
func rtfFlatTextFromRuns(rt *RichText) string {
	if rt == nil {
		return ""
	}
	if len(rt.Runs) == 1 {
		return rt.Runs[0].Text
	}
	var b strings.Builder
	for _, r := range rt.Runs {
		b.WriteString(r.Text)
	}
	return b.String()
}

// rtfRuneCountFromRuns counts runes across all runs without allocating a
// concatenated string.
func rtfRuneCountFromRuns(rt *RichText) int {
	if rt == nil {
		return 0
	}
	n := 0
	for _, r := range rt.Runs {
		n += utf8RuneCount(r.Text)
	}
	return n
}

type rtfView struct {
	RtfCfg
	sizing Sizing
}

func (v *rtfView) Content() []View { return nil }

// rtfSuppressInlineObjectGlyphs prevents object placeholder glyphs from
// painting when a later render pass draws the actual inline object.
func rtfSuppressInlineObjectGlyphs(layout *glyph.Layout) {
	if layout == nil {
		return
	}
	for i := range layout.Items {
		if !layout.Items[i].IsObject {
			continue
		}
		layout.Items[i].GlyphCount = 0
	}
}

func (v *rtfView) GenerateLayout(w *Window) Layout {
	// Convert RichText to glyph.RichText.
	vgRT, mathHashes := v.RichText.toGlyphRichTextWithMath(
		w.viewState.diagramCache)

	// Determine base style.
	var baseStyle glyph.TextStyle
	if v.BaseTextStyle != nil {
		baseStyle = v.BaseTextStyle.ToGlyphStyle()
	} else if len(vgRT.Runs) > 0 {
		baseStyle = vgRT.Runs[0].Style
	}

	// For wrapped modes, skip the initial LayoutRichText — Width is
	// overridden by Fill sizing and Height by layoutWrapRTF. The
	// expensive glyph shaping runs once in layoutWrapRTF instead.
	isWrap := v.Mode == TextModeWrap ||
		v.Mode == TextModeWrapKeepSpaces

	var layout glyph.Layout
	if !isWrap {
		cfg := glyph.TextConfig{
			Style: baseStyle,
			Block: glyph.BlockStyle{
				Wrap:   glyph.WrapWord,
				Width:  -1.0,
				Indent: -v.HangingIndent,
			},
		}
		if w.textMeasurer != nil {
			if tm, ok := w.textMeasurer.(interface {
				LayoutRichText(glyph.RichText, glyph.TextConfig) (glyph.Layout, error)
			}); ok {
				if l, err := tm.LayoutRichText(vgRT, cfg); err == nil {
					layout = l
					rtfSuppressInlineObjectGlyphs(&layout)
				}
			}
		}
	}

	flatText := rtfFlatTextFromRuns(&v.RichText)

	var events *eventHandlers
	switch {
	case v.markdownID > 0:
		events = &eventHandlers{
			OnClick:     markdownBlockOnClick,
			OnMouseMove: rtfMouseMove,
			AmendLayout: rtfMarkdownAmendLayout,
		}
	case v.IDFocus > 0:
		events = &eventHandlers{
			OnClick:     rtfSelectOnClick,
			OnKeyDown:   rtfSelectOnKeyDown,
			OnMouseMove: rtfMouseMove,
			AmendLayout: rtfSelectAmendLayout,
		}
	default:
		events = &eventHandlers{
			OnClick:     rtfOnClick,
			OnMouseMove: rtfMouseMove,
			AmendLayout: rtfAmendTooltip,
		}
	}

	shape := &Shape{
		shapeType: shapeRTF,
		ID:        v.ID,
		IDFocus:   v.IDFocus,
		A11YRole:  AccessRoleStaticText,
		A11Y:      makeA11YInfo(v.A11YLabel, v.A11YDescription),
		Width:     layout.Width,
		Height:    layout.Height,
		Clip:      v.Clip,
		FocusSkip: v.FocusSkip,
		Disabled:  v.Disabled,
		MinWidth:  v.MinWidth,
		Sizing:    v.sizing,
		events:    events,
		TC: &ShapeTextConfig{
			TextMode:           v.Mode,
			HangingIndent:      v.HangingIndent,
			RtfBaseStyle:       baseStyle,
			RtfLayout:          &layout,
			RtfRuns:            &v.RichText,
			RtfFlatText:        flatText,
			MarkdownID:         v.markdownID,
			MarkdownBlockStart: v.markdownBlockStart,
			MarkdownRuneLen:    uint32(utf8RuneCount(flatText)),
			rtfGlyphRT:         &vgRT,
			rtfMathHashes:      mathHashes,
		},
	}
	l := Layout{Shape: shape}
	blockKey := rtfRunsKey(shape.TC.RtfRuns)
	if ts := &w.viewState.tooltip; ts.id != "" &&
		ts.text != "" && ts.blockKey != 0 &&
		blockKey == ts.blockKey {
		l.Children = []Layout{
			GenerateViewLayout(rtfTooltipView(ts), w),
		}
	}
	// Link context menu popup — only on the owning RTF block.
	if st := StateReadOr(
		w, nsRtfLinkMenu, nsRtfLinkMenu,
		rtfLinkMenuState{}); st.Open &&
		st.BlockKey == blockKey {
		l.Children = append(l.Children,
			GenerateViewLayout(rtfLinkMenuView(w, st), w))
	}
	return l
}

// RTF creates a rich text view.
func RTF(cfg RtfCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}
	sizing := FitFit
	if cfg.Mode == TextModeWrap ||
		cfg.Mode == TextModeWrapKeepSpaces {
		sizing = FillFit
	}
	return &rtfView{RtfCfg: cfg, sizing: sizing}
}

// --- Hit testing ---

func rtfRunRect(run glyph.Item) drawClip {
	return drawClip{
		X:      float32(run.X),
		Y:      float32(run.Y - run.Ascent),
		Width:  float32(run.Width),
		Height: float32(run.Ascent + run.Descent),
	}
}

func rtfHitTest(run glyph.Item, mx, my float32) bool {
	r := rtfRunRect(run)
	return mx >= r.X && my >= r.Y &&
		mx < r.X+r.Width && my < r.Y+r.Height
}

func rtfFindRunAtIndex(
	l *Layout, startIndex int,
) RichTextRun {
	if l.Shape == nil || l.Shape.TC == nil ||
		l.Shape.TC.RtfRuns == nil {
		return RichTextRun{}
	}
	idx := 0
	for _, r := range l.Shape.TC.RtfRuns.Runs {
		runLen := len(r.Text)
		if startIndex >= idx &&
			startIndex < idx+runLen {
			return r
		}
		idx += runLen
	}
	return RichTextRun{}
}

// --- Event handlers ---

func rtfMouseMove(l *Layout, e *Event, w *Window) {
	if !l.Shape.hasRtfLayout() {
		return
	}
	ts := &w.viewState.tooltip
	layout := l.Shape.TC.RtfLayout
	for _, run := range layout.Items {
		if run.IsObject {
			continue
		}
		if rtfHitTest(run, e.MouseX, e.MouseY) {
			found := rtfFindRunAtIndex(l, run.StartIndex)
			if found.Tooltip != "" {
				tipID := found.Tooltip
				if ts.hoverID == tipID {
					e.IsHandled = true
					return
				}
				r := rtfRunRect(run)
				ts.hoverID = tipID
				ts.text = found.Tooltip
				ts.bounds = drawClip{
					X:      l.Shape.X + r.X,
					Y:      l.Shape.Y + r.Y,
					Width:  r.Width,
					Height: r.Height,
				}
				ts.floatOffsetX = r.X + r.Width/2
				ts.floatOffsetY = r.Y - 3
				ts.blockKey = rtfRunsKey(
					l.Shape.TC.RtfRuns)
				ts.hoverStart = time.Now()
				w.AnimationAdd(rtfTooltipAnimation(tipID))
				e.IsHandled = true
				return
			}
			if found.Link != "" {
				w.SetMouseCursorPointingHand()
				e.IsHandled = true
				return
			}
		}
	}
	ts.clearText()
}

// rtfTooltipAnimation returns an Animate that activates
// the RTF tooltip after the configured delay.
func rtfTooltipAnimation(tipID string) *Animate {
	return &Animate{
		AnimID: "___tooltip___",
		Delay:  DefaultTooltipStyle.Delay,
		Callback: func(_ *Animate, w *Window) {
			ts := &w.viewState.tooltip
			if ts.hoverID == tipID && ts.text != "" {
				ts.id = tipID
				ts.popupID = tipID + "_rtf_popup"
			}
		},
	}
}

// rtfAmendTooltip clears RTF tooltip state when the mouse
// leaves the stored bounds, and dismisses the link context
// menu when focus is lost.
func rtfAmendTooltip(_ *Layout, w *Window) {
	ts := &w.viewState.tooltip
	if ts.text != "" {
		mx := w.viewState.mousePosX
		my := w.viewState.mousePosY
		b := ts.bounds
		if mx < b.X || my < b.Y ||
			mx >= b.X+b.Width || my >= b.Y+b.Height {
			ts.clearText()
		}
	}
	// Dismiss link context menu when focus moves away.
	if !w.IsFocus(rtfLinkMenuIDFocus) {
		sm := StateMapRead[string, rtfLinkMenuState](
			w, nsRtfLinkMenu)
		if sm != nil {
			sm.Delete(nsRtfLinkMenu)
		}
	}
}

const (
	fnvOffset64 uint64 = 14695981039346656037
	fnvPrime64  uint64 = 1099511628211
	// fnvFieldSep marks boundaries between hashed fields so
	// concatenating different fields cannot produce the same
	// digest as a single longer field.
	fnvFieldSep uint64 = 0x1F
	// diagramCacheMissSentinel is mixed into rtfMathStateKey
	// for math runs whose diagram cache entry is absent. Chosen
	// outside the DiagramState (uint8 0..2) range.
	diagramCacheMissSentinel uint64 = 0xFF
)

// rtfRunsKey computes an FNV-1a hash of RichText content
// including Link, Tooltip, MathID, and MathLatex for
// tooltip/menu block matching and cross-frame caching.
func rtfRunsKey(rt *RichText) uint64 {
	h := fnvOffset64
	for _, r := range rt.Runs {
		for i := range len(r.Text) {
			h ^= uint64(r.Text[i])
			h *= fnvPrime64
		}
		h ^= fnvFieldSep
		h *= fnvPrime64
		for i := range len(r.Link) {
			h ^= uint64(r.Link[i])
			h *= fnvPrime64
		}
		h ^= fnvFieldSep
		h *= fnvPrime64
		for i := range len(r.Tooltip) {
			h ^= uint64(r.Tooltip[i])
			h *= fnvPrime64
		}
		h ^= fnvFieldSep
		h *= fnvPrime64
		for i := range len(r.MathID) {
			h ^= uint64(r.MathID[i])
			h *= fnvPrime64
		}
		h ^= fnvFieldSep
		h *= fnvPrime64
		for i := range len(r.MathLatex) {
			h ^= uint64(r.MathLatex[i])
			h *= fnvPrime64
		}
		h ^= fnvFieldSep
		h *= fnvPrime64
	}
	return h
}

// rtfStyleKey hashes layout-affecting fields of a base style
// for use in the cross-frame RTF layout cache key.
func rtfStyleKey(s glyph.TextStyle) uint64 {
	h := fnvOffset64
	for i := range len(s.FontName) {
		h ^= uint64(s.FontName[i])
		h *= fnvPrime64
	}
	h ^= uint64(s.Typeface)
	h *= fnvPrime64
	h ^= uint64(math.Float32bits(s.Size))
	h *= fnvPrime64
	h ^= uint64(math.Float32bits(s.LetterSpacing))
	h *= fnvPrime64
	return h
}

// rtfMathStateKey mixes per-math-run diagram cache state into
// the layout cache key. A Loading→Ready transition flips the
// key, forcing re-shape: raw LaTeX text fallback and the
// InlineObject placeholder produce different glyph runs and
// dimensions.
func rtfMathStateKey(
	rt *RichText, cache *BoundedDiagramCache,
) uint64 {
	h := fnvOffset64
	if rt == nil || cache == nil {
		return h
	}
	for _, r := range rt.Runs {
		if r.MathID == "" {
			continue
		}
		entry, ok := cache.Get(diagramCacheHash(r.MathID))
		if !ok {
			h ^= diagramCacheMissSentinel
			h *= fnvPrime64
			continue
		}
		h ^= uint64(entry.State)
		h *= fnvPrime64
		h ^= uint64(math.Float32bits(entry.Width))
		h *= fnvPrime64
		h ^= uint64(math.Float32bits(entry.Height))
		h *= fnvPrime64
		h ^= uint64(math.Float32bits(entry.DPI))
		h *= fnvPrime64
	}
	return h
}

// rtfTooltipView builds a floating tooltip popup positioned
// relative to the owning RTF shape via the float system.
func rtfTooltipView(ts *tooltipState) View {
	d := &DefaultTooltipStyle
	return Column(ContainerCfg{
		ID:            ts.popupID,
		Float:         true,
		FloatAutoFlip: true,
		FloatTieOff:   FloatBottomCenter,
		FloatOffsetX:  ts.floatOffsetX,
		FloatOffsetY:  ts.floatOffsetY,
		Color:         d.Color,
		ColorBorder:   d.ColorBorder,
		SizeBorder:    Some(d.SizeBorder),
		Radius:        Some(d.Radius),
		Padding:       Some(d.Padding),
		MaxWidth:      300,
		Content: []View{
			Text(TextCfg{
				Text:      ts.text,
				TextStyle: d.TextStyle,
				Mode:      TextModeWrap,
			}),
		},
	})
}

func rtfOnClick(l *Layout, e *Event, w *Window) {
	if !l.Shape.hasRtfLayout() {
		return
	}
	layout := l.Shape.TC.RtfLayout
	for _, run := range layout.Items {
		if run.IsObject {
			continue
		}
		if rtfHitTest(run, e.MouseX, e.MouseY) {
			found := rtfFindRunAtIndex(l, run.StartIndex)
			if found.Link != "" && markdown.IsSafeURL(found.Link) {
				if e.MouseButton == MouseRight {
					showLinkContextMenu(w, found.Link,
						e.MouseX,
						e.MouseY,
						rtfRunsKey(l.Shape.TC.RtfRuns))
					e.IsHandled = true
					return
				}
				if len(found.Link) > 0 &&
					found.Link[0] == '#' {
					w.ScrollToView(found.Link[1:])
				} else if w.nativePlatform != nil {
					_ = w.nativePlatform.OpenURI(found.Link)
				}
				e.IsHandled = true
			}
			return
		}
	}
}

// rtfLinkMenuState holds state for the RTF link context menu.
type rtfLinkMenuState struct {
	Link     string
	BlockKey uint64 // identifies the owning RTF block
	X        float32
	Y        float32
	Open     bool
}

const rtfLinkMenuIDFocus uint32 = 8492137

// showLinkContextMenu opens a context menu for an RTF link.
func showLinkContextMenu(
	w *Window, link string, mx, my float32,
	blockKey uint64,
) {
	sm := StateMap[string, rtfLinkMenuState](
		w, nsRtfLinkMenu, capFew)
	sm.Set(nsRtfLinkMenu, rtfLinkMenuState{
		Open:     true,
		Link:     link,
		X:        mx,
		Y:        my,
		BlockKey: blockKey,
	})
	w.SetIDFocus(rtfLinkMenuIDFocus)
}

// rtfLinkMenuDismiss clears the link context menu state.
func rtfLinkMenuDismiss(w *Window) {
	sm := StateMapRead[string, rtfLinkMenuState](
		w, nsRtfLinkMenu)
	if sm != nil {
		sm.Delete(nsRtfLinkMenu)
	}
	w.SetIDFocus(0)
}

// rtfLinkMenuView builds the floating context menu popup
// for RTF link right-click.
func rtfLinkMenuView(w *Window, st rtfLinkMenuState) View {
	link := st.Link
	return Menu(w, MenubarCfg{
		ID:      "rtf_link_menu",
		IDFocus: rtfLinkMenuIDFocus,
		Items: []MenuItemCfg{
			{ID: "open_link", Text: "Open Link"},
			{ID: "copy_link", Text: "Copy Link"},
		},
		Action: func(id string, _ *Event, w *Window) {
			switch id {
			case "open_link":
				if w.nativePlatform != nil &&
					markdown.IsSafeURL(link) {
					_ = w.nativePlatform.OpenURI(link)
				}
			case "copy_link":
				w.SetClipboard(link)
			}
			rtfLinkMenuDismiss(w)
		},
		Float:         true,
		FloatAutoFlip: true,
		FloatAnchor:   FloatTopLeft,
		FloatTieOff:   FloatTopLeft,
		FloatOffsetX:  st.X,
		FloatOffsetY:  st.Y,
	})
}

// --- RTF standalone text selection ---

// rtfSelectAmendLayout copies InputState selection into the shape's
// TextSelBeg/TextSelEnd for rendering and calls rtfAmendTooltip.
func rtfSelectAmendLayout(l *Layout, w *Window) {
	rtfAmendTooltip(l, w)
	if l.Shape.IDFocus == 0 || l.Shape.TC == nil {
		return
	}
	is := StateReadOr(w, nsInput, l.Shape.IDFocus, InputState{})
	l.Shape.TC.TextSelBeg = is.SelectBeg
	l.Shape.TC.TextSelEnd = is.SelectEnd
}

// rtfMarkdownAmendLayout calls rtfAmendTooltip and the markdown block
// selection handler. The markdown block handler is defined in markdown_select.go.
func rtfMarkdownAmendLayout(l *Layout, w *Window) {
	rtfAmendTooltip(l, w)
	markdownBlockAmendSel(l, w)
}

// rtfSelectOnClick handles clicks for an RTF widget with selection enabled.
// Link navigation (rtfOnClick) runs first; selection state is always updated.
func rtfSelectOnClick(l *Layout, e *Event, w *Window) {
	rtfOnClick(l, e, w)
	if e.MouseButton == MouseRight {
		return
	}
	shape := l.Shape
	if shape.TC == nil || !shape.hasRtfLayout() || shape.IDFocus == 0 {
		return
	}
	w.SetIDFocus(shape.IDFocus)

	gl := shape.TC.RtfLayout
	flatText := shape.TC.RtfFlatText

	byteIdx := gl.GetClosestOffset(e.MouseX, e.MouseY)
	runePos := byteToRuneIndex(flatText, byteIdx)

	idFocus := shape.IDFocus
	imap := StateMap[uint32, InputState](w, nsInput, capMany)
	is, _ := imap.Get(idFocus)

	now := time.Now().UnixMilli()
	doubleClick := is.LastClickTime > 0 &&
		now-is.LastClickTime <= doubleClickThresholdMs
	is.LastClickTime = now

	if doubleClick {
		bBeg, bEnd := gl.GetWordAtIndex(byteIdx)
		beg := byteToRuneIndex(flatText, bBeg)
		end := byteToRuneIndex(flatText, bEnd)
		is.CursorPos = end
		is.SelectBeg = uint32(beg)
		is.SelectEnd = uint32(end)
	} else {
		is.CursorPos = runePos
		is.SelectBeg = uint32(runePos)
		is.SelectEnd = uint32(runePos)
	}
	is.CursorOffset = -1
	imap.Set(idFocus, is)
	e.IsHandled = true

	anchorPos := is.SelectBeg
	anchorEnd := is.SelectEnd
	dragShapeX := shape.X
	dragShapeY := shape.Y

	var lastMouseX, lastMouseY float32
	scrollID := uint32(0)
	dragScrollY0 := float32(0)
	viewTop := float32(0)
	viewBot := float32(0)
	maxScrollNeg := float32(0)
	for p := l.Parent; p != nil; p = p.Parent {
		if p.Shape != nil && p.Shape.IDScroll > 0 {
			scrollID = p.Shape.IDScroll
			sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
			dragScrollY0, _ = sy.Get(scrollID)
			sp := p.Shape
			viewTop = sp.Y + sp.Padding.Top
			viewH := sp.Height - sp.paddingHeight()
			viewBot = viewTop + viewH
			maxScrollNeg = f32Min(0, viewH-contentHeight(p))
			break
		}
	}

	computeRunePos := func(mx, my float32, w *Window) int {
		scrollDelta := float32(0)
		if scrollID > 0 {
			sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
			sNow, _ := sy.Get(scrollID)
			scrollDelta = sNow - dragScrollY0
		}
		rx := mx - dragShapeX
		ry := my - (dragShapeY + scrollDelta)
		bi := gl.GetClosestOffset(rx, ry)
		return byteToRuneIndex(flatText, bi)
	}

	updateDrag := func(rp int, w *Window) {
		dim := StateMap[uint32, InputState](w, nsInput, capMany)
		dis, _ := dim.Get(idFocus)
		if doubleClick {
			bi := runeToByteIndex(flatText, rp)
			bBeg, bEnd := gl.GetWordAtIndex(bi)
			wb := byteToRuneIndex(flatText, bBeg)
			we := byteToRuneIndex(flatText, bEnd)
			if rp < int(anchorPos) {
				dis.SelectBeg = anchorEnd
				dis.SelectEnd = uint32(wb)
				dis.CursorPos = wb
			} else {
				dis.SelectBeg = anchorPos
				dis.SelectEnd = uint32(we)
				dis.CursorPos = we
			}
		} else {
			dis.CursorPos = rp
			dis.SelectBeg = anchorPos
			dis.SelectEnd = uint32(rp)
		}
		dis.CursorOffset = -1
		dim.Set(idFocus, dis)
	}

	dragScrollCB := func(_ *Animate, w *Window) {
		var delta float32
		if lastMouseY < viewTop {
			delta = (viewTop - lastMouseY) * 0.3
		} else if lastMouseY > viewBot {
			delta = -((lastMouseY - viewBot) * 0.3)
		} else {
			w.AnimationRemove(animIDTextDragScroll)
			return
		}
		sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
		cur, _ := sy.Get(scrollID)
		newScroll := f32Clamp(cur+delta, maxScrollNeg, 0)
		if newScroll == cur {
			return
		}
		sy.Set(scrollID, newScroll)
		rp := computeRunePos(lastMouseX, lastMouseY, w)
		updateDrag(rp, w)
	}

	w.MouseLock(MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			lastMouseX = e.MouseX
			lastMouseY = e.MouseY
			rp := computeRunePos(e.MouseX, e.MouseY, w)
			updateDrag(rp, w)
			if scrollID > 0 {
				outside := e.MouseY < viewTop || e.MouseY > viewBot
				if outside && !w.HasAnimation(animIDTextDragScroll) {
					w.AnimationAdd(&Animate{
						AnimID:   animIDTextDragScroll,
						Delay:    32 * time.Millisecond,
						Repeat:   true,
						Refresh:  AnimationRefreshLayout,
						Callback: dragScrollCB,
					})
				} else if !outside {
					w.AnimationRemove(animIDTextDragScroll)
				}
			}
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			w.AnimationRemove(animIDTextDragScroll)
			w.MouseUnlock()
		},
	})
}

// rtfSelectOnKeyDown handles keyboard navigation and copy for selectable RTF.
func rtfSelectOnKeyDown(l *Layout, e *Event, w *Window) {
	shape := l.Shape
	if shape.TC == nil || shape.IDFocus == 0 ||
		!w.IsFocus(shape.IDFocus) {
		return
	}
	id := shape.IDFocus
	flatText := shape.TC.RtfFlatText
	gl := *shape.TC.RtfLayout

	imap := StateMap[uint32, InputState](w, nsInput, capMany)
	is, _ := imap.Get(id)
	savedOffset := is.CursorOffset
	savedTrailing := is.CursorTrailing
	is.CursorOffset = -1
	is.CursorTrailing = false
	runeLen := utf8RuneCount(flatText)
	pos := min(is.CursorPos, runeLen)
	isShift := e.Modifiers.Has(ModShift)
	isWordMod := e.Modifiers.HasAny(ModCtrl, ModAlt, ModSuper)
	handled := true

	switch e.KeyCode {
	case KeyLeft:
		inputKeyLeft(imap, id, is, flatText, pos,
			isShift, isWordMod, gl, true)
	case KeyRight:
		inputKeyRight(imap, id, is, flatText, pos, runeLen,
			isShift, isWordMod, gl, true)
	case KeyHome:
		inputKeyHome(imap, id, is, flatText, pos,
			isShift, savedTrailing, gl, true)
	case KeyEnd:
		inputKeyEnd(imap, id, is, flatText, pos,
			isShift, savedTrailing, gl, true)
	case KeyUp:
		handled = textKeyVertical(imap, id, is, flatText,
			pos, isShift, savedOffset, true,
			shape.TC.TextMode, gl, true)
	case KeyDown:
		handled = textKeyVertical(imap, id, is, flatText,
			pos, isShift, savedOffset, false,
			shape.TC.TextMode, gl, true)
	case KeyEscape:
		inputKeyEscape(imap, id, is)
		handled = false
	case KeyA:
		if e.Modifiers.HasAny(ModCtrl, ModSuper) {
			inputSelectAll(flatText, id, w)
		} else {
			handled = false
		}
	case KeyC:
		handled = inputKeyCopy(flatText, id, false, e, w)
	default:
		handled = false
	}

	if handled {
		e.IsHandled = true
	}
}
