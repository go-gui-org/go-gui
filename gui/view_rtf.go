package gui

// view_rtf.go defines the Rich Text Format (RTF) view.
// Renders text with multiple typefaces, sizes, and styles.
// Supports text wrapping, clickable links, and custom runs.

import (
	"strconv"
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

// rtfFlatTextFromRuns concatenates the shaped run texts into a
// single string, in the same domain the glyph layout shapes:
// ready math contributes the object placeholder, anything else its
// fallback or plain text. Selection, cursor and highlight mapping
// all read glyph byte offsets, so this must match Layout.Text
// exactly — including across a math Loading→Ready transition,
// which is why the diagram cache rides along. Also returns the rune
// count in one pass so callers do not rescan the built string.
func rtfFlatTextFromRuns(
	rt *RichText, cache *BoundedDiagramCache,
) (string, int) {
	if rt == nil || len(rt.Runs) == 0 {
		return "", 0
	}
	if len(rt.Runs) == 1 {
		text := rtfShapedRunText(&rt.Runs[0], cache)
		return text, utf8RuneCount(text)
	}
	// Two passes over the runs, not one pass into a []string: this
	// runs per frame in the view phase, and sizing the builder from
	// a scratch slice would add a heap allocation per call. The
	// second pass costs a field read per run, or one cache lookup
	// per math run.
	totalBytes := 0
	for i := range rt.Runs {
		totalBytes += len(rtfShapedRunText(&rt.Runs[i], cache))
	}
	var b strings.Builder
	b.Grow(totalBytes)
	totalRunes := 0
	for i := range rt.Runs {
		seg := rtfShapedRunText(&rt.Runs[i], cache)
		b.WriteString(seg)
		totalRunes += utf8RuneCount(seg)
	}
	return b.String(), totalRunes
}

// rtfRuneCountFromRuns counts runes across all runs in the shaped
// domain (see rtfFlatTextFromRuns), without allocating a
// concatenated string.
func rtfRuneCountFromRuns(rt *RichText, cache *BoundedDiagramCache) int {
	if rt == nil {
		return 0
	}
	n := 0
	for i := range rt.Runs {
		n += utf8RuneCount(rtfShapedRunText(&rt.Runs[i], cache))
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

// rtfNonWrapLayout shapes a single-line RTF block, or returns the
// cross-frame cached layout for one. A non-wrap layout is
// width-independent (Block.Width -1), so a shaped layout stays valid
// until content, style, math state or theme moves — this is the same
// per-frame shaping the wrap path already avoids in layoutWrapRTF.
// The key carries zero width bits, which collide with nothing: the
// wrap walk drops any shape whose width is not finite and positive
// before layoutWrapRTF builds a key.
//
// Returns nil when the window carries no measurer that can shape
// rich text, or when shaping failed.
func rtfNonWrapLayout(
	v *rtfView, w *Window,
	vgRT glyph.RichText, baseStyle glyph.TextStyle,
	lineSpacing float32,
) *glyph.Layout {
	key := rtfLayoutCacheKey(
		rtfRunsKey(&v.RichText),
		rtfStyleKey(baseStyle),
		rtfMathStateKey(&v.RichText, w.viewState.diagramCache),
		0, -v.HangingIndent, lineSpacing)
	if cached, ok := rtfLayoutCacheFetch(w, key); ok {
		return cached
	}
	// A nil textMeasurer asserts false, so this covers the
	// no-measurer case too — as layoutWrapRTF does.
	tm, ok := w.textMeasurer.(interface {
		LayoutRichText(glyph.RichText, glyph.TextConfig) (glyph.Layout, error)
	})
	if !ok {
		return nil
	}
	l, err := tm.LayoutRichText(vgRT, glyph.TextConfig{
		Style: baseStyle,
		Block: glyph.BlockStyle{
			Wrap:        glyph.WrapWord,
			Width:       -1.0,
			Indent:      -v.HangingIndent,
			LineSpacing: lineSpacing,
		},
	})
	if err != nil {
		return nil
	}
	rtfSuppressInlineObjectGlyphs(&l)
	rtfLayoutCacheStore(w, key, &l)
	return &l
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

	var rtfLayout *glyph.Layout
	if !isWrap {
		rtfLayout = rtfNonWrapLayout(
			v, w, vgRT, baseStyle, lineSpacing)
	}

	// Flat text in the shaped domain (see rtfFlatTextFromRuns), with
	// the rune count from the same pass.
	flatText, flatRunes := rtfFlatTextFromRuns(
		&v.RichText, w.viewState.diagramCache)

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

	// A shaped layout — cached or fresh — carries the shape's
	// intrinsic size. Wrapped modes take theirs from layoutWrapRTF
	// instead, and an unshaped block has none yet: both stand a zero
	// layout in so the rTFLayout readers never see nil.
	var shapeW, shapeH float32
	if rtfLayout != nil {
		shapeW, shapeH = rtfLayout.Width, rtfLayout.Height
	} else {
		rtfLayout = &glyph.Layout{}
	}

	shape := w.allocShape(Shape{
		shapeType: shapeRTF,
		ID:        v.ID,
		// No Opacity knob on the cfg yet; default opaque so
		// the emit paths can treat 0 as transparent.
		Opacity:   1.0,
		Focusable: v.Focusable,
		A11YRole:  AccessRoleStaticText,
		a11Y:      v.a11yInfo(""),
		Width:     shapeW,
		Height:    shapeH,
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
			rTFLayout:          rtfLayout,
			rTFRuns:            &v.RichText,
			rTFFlatText:        flatText,
			markdownID:         v.markdownID,
			markdownBlockStart: v.markdownBlockStart,
			markdownRuneLen:    uint32(flatRunes),
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
	l *Layout, startIndex int, cache *BoundedDiagramCache,
) RichTextRun {
	if l == nil || l.Shape == nil || l.Shape.TC == nil ||
		l.Shape.TC.rTFRuns == nil {
		return RichTextRun{}
	}
	idx := 0
	for i := range l.Shape.TC.rTFRuns.Runs {
		// Shaped lengths: glyph StartIndex counts the object
		// placeholder or LaTeX fallback for math runs, not the
		// (usually empty) source text.
		runLen := len(rtfShapedRunText(
			&l.Shape.TC.rTFRuns.Runs[i], cache))
		if startIndex >= idx &&
			startIndex < idx+runLen {
			return l.Shape.TC.rTFRuns.Runs[i]
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
	cache := ctx.Window.viewState.diagramCache
	for _, run := range layout.Items {
		if run.IsObject {
			continue
		}
		if rtfHitTest(run, ctx.Event.MouseX, ctx.Event.MouseY) {
			found := rtfFindRunAtIndex(ctx.Layout, run.StartIndex, cache)
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
			if found.Link != "" && markdown.IsSafeURL(found.Link) {
				ctx.Window.SetMouseCursorPointingHand()
				ctx.Consume()
				return
			}
		}
	}
	ts.clearText()
}

// rtfTooltipPopupID derives the tooltip popup's widget ID from the
// tooltip text. The text is arbitrary document content and may hold
// a colon, which ScopeID forbids in a part (see id_scope.go) —
// hashing keeps the ID scope-safe while staying stable across
// frames for the same tooltip, so the popup keeps its identity
// while hovered.
func rtfTooltipPopupID(tipID string) string {
	return ScopeID("rtf_tip",
		strconv.FormatUint(Fnv64Str(Fnv64Offset, tipID), 16),
		"popup")
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
				ts.popupID = rtfTooltipPopupID(tipID)
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
	// diagramCacheMissSentinel is mixed into rtfMathStateKey
	// for math runs whose diagram cache entry is absent. Chosen
	// outside the DiagramState (uint8 0..2) range.
	diagramCacheMissSentinel uint64 = 0xFF
)

// rtfRunsKey computes an FNV-1a hash of RichText content
// including per-run layout style, Link, Tooltip, MathID, and
// MathLatex for tooltip/menu block matching and cross-frame
// caching. Run styles ride along because a size or family change
// with identical text reshapes the layout the key guards.
func rtfRunsKey(rt *RichText) uint64 {
	h := Fnv64Offset
	if rt == nil {
		return h
	}
	for _, r := range rt.Runs {
		h = Fnv64Str(h, r.Text)
		h = Fnv64Byte(h, fnvUnitSep)
		h = fnvTextStyle(h, r.Style)
		h = Fnv64Str(h, r.Link)
		h = Fnv64Byte(h, fnvUnitSep)
		h = Fnv64Str(h, r.Tooltip)
		h = Fnv64Byte(h, fnvUnitSep)
		h = Fnv64Str(h, r.MathID)
		h = Fnv64Byte(h, fnvUnitSep)
		h = Fnv64Str(h, r.MathLatex)
		h = Fnv64Byte(h, fnvUnitSep)
	}
	return h
}

// rtfStyleKey hashes layout-affecting fields of a base style
// for use in the cross-frame RTF layout cache key.
func rtfStyleKey(s glyph.TextStyle) uint64 {
	return fnvGlyphStyle(Fnv64Offset, s)
}

// rtfMathStateKey mixes per-math-run diagram cache state into
// the layout cache key. A Loading→Ready transition flips the
// key, forcing re-shape: raw LaTeX text fallback and the
// InlineObject placeholder produce different glyph runs and
// dimensions.
func rtfMathStateKey(
	rt *RichText, cache *BoundedDiagramCache,
) uint64 {
	h := Fnv64Offset
	if rt == nil || cache == nil {
		return h
	}
	for _, r := range rt.Runs {
		if r.MathID == "" {
			continue
		}
		entry, ok := cache.Get(diagramCacheHash(r.MathID))
		if !ok {
			h = Fnv64Byte(h, byte(diagramCacheMissSentinel))
			continue
		}
		h = Fnv64Byte(h, byte(entry.State))
		h = fnvU64(h, uint64(normFloat32Bits(entry.Width)))
		h = fnvU64(h, uint64(normFloat32Bits(entry.Height)))
		h = fnvU64(h, uint64(normFloat32Bits(entry.dPI)))
	}
	return h
}

// rtfTooltipMaxWidth caps the floating tooltip popup so a long
// abbreviation expansion wraps instead of spanning the window.
const rtfTooltipMaxWidth = 300

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
		Color:         d.Colors.Base,
		ColorBorder:   d.Colors.Border,
		SizeBorder:    Some(d.SizeBorder),
		Radius:        Some(d.Radius),
		Padding:       d.Padding,
		MaxWidth:      rtfTooltipMaxWidth,
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
		found := rtfFindRunAtIndex(
			ctx.Layout, run.StartIndex,
			ctx.Window.viewState.diagramCache)
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
// per window and the retained key stays bounded. Rune-safe: links may
// hold multibyte text, which a byte slice would split mid-rune.
func rtfLinkShort(link string) string {
	return truncatePreview(link, rtfLinkMaxReportLen)
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
			if _, found := w.layout.findByID(scoped); found {
				return scoped, true
			}
		}
	}
	if _, found := w.layout.findByID(slug); found {
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
