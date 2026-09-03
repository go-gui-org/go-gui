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

// RTFCfg configures a Rich Text View.
type RTFCfg struct {
	BaseTextStyle *TextStyle

	ID string
	A11YCfg
	RichText  RichText
	MinWidth  float32
	Focusable bool
	// HangingIndent is the negative indent for wrapped lines
	// (a hanging indent pulls the first line left of the rest).
	// exportaudit:keep — caller-facing config (issue #372)
	HangingIndent float32

	// markdownID is non-empty when this block belongs to a markdown
	// widget. markdownBlockStart is the rune offset of this block in
	// the markdown's flat text. All three are set by view_markdown.go
	// only: markdownID is stamped on every markdown block — it is the
	// document identity anchor resolution keys on — while markdownSel
	// gates the cross-block selection machinery and is set only for
	// focusable documents.
	markdownID         string
	markdownBlockStart uint32
	markdownSel        bool
	Mode               textMode
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
	RTFCfg
	sizing Sizing
}

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

	// Determine base style. LineSpacing lives on glyph's BlockStyle,
	// not glyph.TextStyle, so ToGlyphStyle drops it; carry it here.
	var baseStyle glyph.TextStyle
	var lineSpacing float32
	if v.BaseTextStyle != nil {
		baseStyle = v.BaseTextStyle.toGlyphStyle()
		lineSpacing = v.BaseTextStyle.LineSpacing
	} else if len(v.RichText.Runs) > 0 {
		baseStyle = vgRT.Runs[0].Style
		lineSpacing = v.RichText.Runs[0].Style.LineSpacing
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
				Wrap:        glyph.WrapWord,
				Width:       -1.0,
				Indent:      -v.HangingIndent,
				LineSpacing: lineSpacing,
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
	case v.markdownSel:
		events = w.allocEventHandlers(eventHandlers{
			OnClick:     markdownBlockOnClick,
			OnMouseMove: rtfMouseMove,
			AmendLayout: rtfMarkdownAmendLayout,
		})
	case v.Focusable:
		events = w.allocEventHandlers(eventHandlers{
			OnClick:     rtfSelectOnClick,
			OnKeyDown:   rtfSelectOnKeyDown,
			OnMouseMove: rtfMouseMove,
			AmendLayout: rtfSelectAmendLayout,
		})
	default:
		events = w.allocEventHandlers(eventHandlers{
			OnClick:     rtfOnClick,
			OnMouseMove: rtfMouseMove,
			AmendLayout: rtfAmendTooltip,
		})
	}

	shape := w.allocShape(Shape{
		shapeType: shapeRTF,
		ID:        v.ID,
		Focusable: v.Focusable,
		A11YRole:  AccessRoleStaticText,
		a11Y:      v.a11yInfo(""),
		Width:     layout.Width,
		Height:    layout.Height,
		Clip:      v.Clip,
		FocusSkip: v.FocusSkip,
		Disabled:  v.Disabled,
		MinWidth:  v.MinWidth,
		Sizing:    v.sizing,
		events:    events,
		TC: &shapeTextConfig{
			TextMode:           v.Mode,
			hangingIndent:      v.HangingIndent,
			rTFBaseStyle:       baseStyle,
			rTFLineSpacing:     lineSpacing,
			rTFLayout:          &layout,
			rTFRuns:            &v.RichText,
			rTFFlatText:        flatText,
			markdownID:         v.markdownID,
			markdownBlockStart: v.markdownBlockStart,
			markdownRuneLen:    uint32(utf8RuneCount(flatText)),
			rtfGlyphRT:         &vgRT,
			rtfMathHashes:      mathHashes,
		},
	})
	l := Layout{Shape: shape}
	blockKey := rtfRunsKey(shape.TC.rTFRuns)
	if ts := &w.viewState.tooltip; ts.id != "" &&
		ts.text != "" && ts.blockKey != 0 &&
		blockKey == ts.blockKey {
		l.Children = []Layout{
			generateViewLayout(rtfTooltipView(ts), w),
		}
	}
	// Link context menu popup — only on the owning RTF block.
	if st := StateReadOr(
		w, nsRtfLinkMenu, nsRtfLinkMenu,
		rtfLinkMenuState{}); st.Open &&
		st.BlockKey == blockKey {
		l.Children = append(l.Children,
			generateViewLayout(rtfLinkMenuView(w, st), w))
	}
	return l
}

// RTF creates a rich text view.
func RTF(cfg RTFCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}
	sizing := FitFit
	if cfg.Mode == TextModeWrap ||
		cfg.Mode == TextModeWrapKeepSpaces {
		sizing = FillFit
	}
	return &rtfView{RTFCfg: cfg, sizing: sizing}
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
	if l == nil || l.Shape == nil || l.Shape.TC == nil ||
		l.Shape.TC.rTFRuns == nil {
		return RichTextRun{}
	}
	idx := 0
	for _, r := range l.Shape.TC.rTFRuns.Runs {
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

func rtfMouseMove(ctx EventCtx) {
	if !ctx.Layout.Shape.hasRtfLayout() {
		return
	}
	ts := &ctx.Window.viewState.tooltip
	layout := ctx.Layout.Shape.TC.rTFLayout
	for _, run := range layout.Items {
		if run.IsObject {
			continue
		}
		if rtfHitTest(run, ctx.Event.MouseX, ctx.Event.MouseY) {
			found := rtfFindRunAtIndex(ctx.Layout, run.StartIndex)
			if found.Tooltip != "" {
				tipID := found.Tooltip
				if ts.hoverID == tipID {
					ctx.Consume()
					return
				}
				r := rtfRunRect(run)
				ts.hoverID = tipID
				ts.text = found.Tooltip
				ts.bounds = drawClip{
					X:      ctx.Layout.Shape.X + r.X,
					Y:      ctx.Layout.Shape.Y + r.Y,
					Width:  r.Width,
					Height: r.Height,
				}
				ts.floatOffsetX = r.X + r.Width/2
				ts.floatOffsetY = r.Y - 3
				ts.blockKey = rtfRunsKey(
					ctx.Layout.Shape.TC.rTFRuns)
				ts.hoverStart = time.Now()
				ctx.Window.AnimationAdd(rtfTooltipAnimation(tipID))
				ctx.Consume()
				return
			}
			if found.Link != "" {
				ctx.Window.SetMouseCursorPointingHand()
				ctx.Consume()
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
		Delay:  defaultTooltipStyle.Delay,
		Callback: func(_ *Animate, w *Window) {
			ts := &w.viewState.tooltip
			if ts.hoverID == tipID && ts.text != "" {
				ts.id = tipID
				ts.popupID = ScopeID(tipID, "rtf_popup")
			}
		},
	}
}

// rtfAmendTooltip clears RTF tooltip state when the mouse
// leaves the stored bounds, and dismisses the link context
// menu when focus is lost.
func rtfAmendTooltip(ctx EventCtx) {
	ts := &ctx.Window.viewState.tooltip
	if ts.text != "" {
		mx := ctx.Window.viewState.mousePosX
		my := ctx.Window.viewState.mousePosY
		b := ts.bounds
		if mx < b.X || my < b.Y ||
			mx >= b.X+b.Width || my >= b.Y+b.Height {
			ts.clearText()
		}
	}
	// Dismiss link context menu when focus moves away.
	if !ctx.Window.IsFocus(rtfLinkMenuFocusID) {
		sm := StateMapRead[string, rtfLinkMenuState](
			ctx.Window, nsRtfLinkMenu)
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
		h ^= uint64(math.Float32bits(entry.dPI))
		h *= fnvPrime64
	}
	return h
}

// rtfTooltipView builds a floating tooltip popup positioned
// relative to the owning RTF shape via the float system.
func rtfTooltipView(ts *tooltipState) View {
	d := &defaultTooltipStyle
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
		Padding:       d.Padding,
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

func rtfOnClick(ctx EventCtx) {
	rtfClickLink(ctx)
}

// rtfClickLink activates the link under the pointer, if there is one,
// and reports whether it took the click. A selectable RTF needs the
// answer: a click that navigated must not also start a drag-select,
// which would lock the mouse and leave the caller tracking the pointer
// after the button is released (the release lands on whatever the
// navigation put in front, so the lock is never lifted).
func rtfClickLink(ctx EventCtx) bool {
	if !ctx.Layout.Shape.hasRtfLayout() {
		return false
	}
	layout := ctx.Layout.Shape.TC.rTFLayout
	for _, run := range layout.Items {
		if run.IsObject {
			continue
		}
		if !rtfHitTest(run, ctx.Event.MouseX, ctx.Event.MouseY) {
			continue
		}
		found := rtfFindRunAtIndex(ctx.Layout, run.StartIndex)
		if found.Link == "" || !markdown.IsSafeURL(found.Link) {
			return false
		}
		if ctx.Event.MouseButton == MouseRight {
			showLinkContextMenu(ctx.Window, found.Link,
				ctx.Event.MouseX,
				ctx.Event.MouseY,
				rtfRunsKey(ctx.Layout.Shape.TC.rTFRuns),
				ctx.Layout.Shape.TC.markdownID)
			ctx.Consume()
			return true
		}
		rtfOpenLink(ctx.Window, found.Link,
			ctx.Layout.Shape.TC.markdownID)
		ctx.Consume()
		return true
	}
	return false
}

// rtfOpenLink activates a link the user clicked or chose "Open Link"
// on. It is the single activation path: left-click and the context
// menu both route here, so an in-document anchor behaves the same way
// from either. markdownID scopes the anchor lookup (see
// [rtfResolveAnchor]); it is "" for a standalone RTF block.
//
// Three kinds of link reach this point, because the render gate
// (markdown.IsSafeURL) admits all three:
//
//   - '#slug' — scroll the named target into view.
//   - http/https/mailto — hand to the platform opener.
//   - anything else (relative paths, '?query') — nothing the widget
//     can act on. There is no document base URI to resolve against, so
//     the link is reported and dropped rather than handed to OpenURI,
//     which rejects every scheme outside the allowlist anyway.
//
// Failures are reported through the [Debug] gate; a link that will not
// open is a development-time mistake, not something the frame can act
// on at runtime.
func rtfOpenLink(w *Window, link, markdownID string) {
	// markdown.IsSafeURL classifies the trimmed link, so " #slug" is
	// rendered as an anchor. Trim here too, or the branch below reads
	// the space and the anchor is reported as unopenable.
	link = strings.TrimSpace(link)
	if link == "" {
		return
	}
	if link[0] == '#' {
		if id, ok := rtfResolveAnchor(w, markdownID, link[1:]); ok {
			w.scrollToView(id)
			return
		}
		short := rtfLinkShort(link)
		w.debugWarn(debugCheckLinkNotOpened, short,
			"link %q names no target in this window "+
				"(anchor unresolved)", short)
		return
	}
	if !rtfLinkIsOpenable(link) {
		short := rtfLinkShort(link)
		w.debugWarn(debugCheckLinkNotOpened, short,
			"link %q is relative; the platform opener takes only "+
				"http, https and mailto URIs, and the widget has no "+
				"base URI to resolve against", short)
		return
	}
	if w.nativePlatform == nil {
		return
	}
	if err := w.nativePlatform.OpenURI(link); err != nil {
		short := rtfLinkShort(link)
		w.debugWarn(debugCheckLinkNotOpened, short,
			"link %q could not be opened: %v", short, err)
	}
}

// rtfLinkMaxReportLen caps the link text a diagnostic carries. A link
// comes from the rendered document, so its length is the document
// author's choice, and the warn-once key is retained for the life of
// the window: a document full of long links would otherwise grow the
// warn map by their full size.
const rtfLinkMaxReportLen = 120

// rtfLinkShort caps a link for a diagnostic. The shortened text is
// both the message and the warn-once key, so one bad link reports once
// per window and the retained key stays bounded.
func rtfLinkShort(link string) string {
	if len(link) > rtfLinkMaxReportLen {
		return link[:rtfLinkMaxReportLen] + "..."
	}
	return link
}

// rtfLinkOpenableSchemes mirrors the scheme allowlist in
// nativehost.ValidateOpenURI, which lives in a backend-internal
// package the gui package cannot import. Package level so the
// activation path ranges over it without copying the array.
var rtfLinkOpenableSchemes = [...]string{
	"http://", "https://", "mailto:",
}

// rtfLinkIsOpenable reports whether the platform opener accepts link.
func rtfLinkIsOpenable(link string) bool {
	for _, scheme := range &rtfLinkOpenableSchemes {
		if len(link) >= len(scheme) &&
			strings.EqualFold(link[:len(scheme)], scheme) {
			return true
		}
	}
	return false
}

// rtfResolveAnchor resolves an in-document anchor ('#slug') to the
// scrollable target it names. A link inside a markdown document (TC
// markdownID non-empty) names a heading of that document, whose ID is
// scoped to the document: ScopeID(markdownID, "h", slug) — the
// "md:h:slug" (or "panel:md:h:slug") path the resolve pass treats as
// absolute — so the scoped spelling is tried first. Targets that are
// not headings keep working: an arbitrary absolute ID ("#view:bottom")
// or a standalone RTF link (markdownID == "") falls back to the bare
// slug. ok is false when neither lookup finds a shape.
func rtfResolveAnchor(
	w *Window, markdownID, slug string,
) (id string, ok bool) {
	if markdownID != "" {
		if scoped := ScopeID(markdownID, "h", slug); scoped != "" {
			if _, found := w.layout.FindByID(scoped); found {
				return scoped, true
			}
		}
	}
	if _, found := w.layout.FindByID(slug); found {
		return slug, true
	}
	return "", false
}

// rtfLinkMenuState holds state for the RTF link context menu.
type rtfLinkMenuState struct {
	Link string
	// MarkdownID scopes an anchor link's target lookup, captured from
	// the owning block when the menu opened. The Action callback runs
	// with no RTF layout in reach, so it cannot read it back.
	MarkdownID string
	BlockKey   uint64 // identifies the owning RTF block
	X          float32
	Y          float32
	Open       bool
}

// Absolute: the popup is generated as a child of the RTF block, so a
// plain leaf would resolve under that block's identity — while
// showLinkContextMenu and the dismiss check, which run from event
// handlers, would keep using the bare constant and never match. See
// gui/id_resolve.go.
const rtfLinkMenuFocusID = "gui:rtf:link_menu"

// showLinkContextMenu opens a context menu for an RTF link.
func showLinkContextMenu(
	w *Window, link string, mx, my float32,
	blockKey uint64, markdownID string,
) {
	sm := StateMap[string, rtfLinkMenuState](
		w, nsRtfLinkMenu, capFew)
	sm.Set(nsRtfLinkMenu, rtfLinkMenuState{
		Open:       true,
		Link:       link,
		MarkdownID: markdownID,
		X:          mx,
		Y:          my,
		BlockKey:   blockKey,
	})
	w.SetFocus(rtfLinkMenuFocusID)
}

// rtfLinkMenuDismiss clears the link context menu state.
func rtfLinkMenuDismiss(w *Window) {
	sm := StateMapRead[string, rtfLinkMenuState](
		w, nsRtfLinkMenu)
	if sm != nil {
		sm.Delete(nsRtfLinkMenu)
	}
	w.ClearFocus()
}

// rtfLinkMenuView builds the floating context menu popup
// for RTF link right-click.
func rtfLinkMenuView(w *Window, st rtfLinkMenuState) View {
	link := st.Link
	markdownID := st.MarkdownID
	return menu(w, MenubarCfg{
		ID: rtfLinkMenuFocusID,
		Items: []MenuItemCfg{
			{ID: "open_link", Text: "Open Link"},
			{ID: "copy_link", Text: "Copy Link"},
		},
		Action: func(id string, ctx EventCtx) {
			switch id {
			case "open_link":
				if markdown.IsSafeURL(link) {
					rtfOpenLink(ctx.Window, link, markdownID)
				}
			case "copy_link":
				ctx.Window.SetClipboard(link)
			}
			rtfLinkMenuDismiss(ctx.Window)
		},
		Float:         true,
		FloatAutoFlip: true,
		FloatAnchor:   FloatTopLeft,
		FloatTieOff:   FloatTopLeft,
		FloatOffsetX:  st.X,
		FloatOffsetY:  st.Y,
	})
}
