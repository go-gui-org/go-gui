package gui

import (
	"strconv"
	"time"

	"github.com/go-gui-org/go-glyph"
	"github.com/rivo/uniseg"
)

// Content transitions animate a Text from one value to the next: a
// counter that rolls its digits, a label that fades to its new words,
// an icon that shrinks away while the next one grows in (#1021).
//
// TextCfg.Anim plays a canned effect when the Cfg asks for one. A
// content transition plays when the text itself changes, so the two
// are separate fields. While a transition runs it draws the text
// itself: Anim's opacity still applies, but its motion, shimmer and
// reveal wait until the transition ends.
//
// Every frame of a transition is render-only. The view pass that sees
// the new text starts one driver and points the shape at the run; the
// driver writes progress into the run, and renderText reads it there.
// No view function and no layout runs while the text moves.

// ContentTransitionKind names how a Text moves from its old text to
// its new text. The zero value replaces the text at once.
// exportaudit:keep — reachable from an exported field (ContentTransition.Kind)
type ContentTransitionKind uint8

// Content transition kinds. See ContentTransitionKind.
const (
	// ContentTransitionNone replaces the text at once.
	// exportaudit:keep — one member of a public enum; the set ships whole
	ContentTransitionNone ContentTransitionKind = iota
	// ContentTransitionFade fades the old text out while the new text
	// fades in.
	// exportaudit:keep — one member of a public enum; the set ships whole
	ContentTransitionFade
	// ContentTransitionNumeric rolls only the characters that change.
	// The texts are compared by grapheme from the right, so "99" to
	// "100" rolls every column and the new "1" rolls in at the left. A
	// larger number comes up from below, a smaller one comes down from
	// above. The text asks for tabular figures, so where the font has
	// them its width stays the same while the digits change.
	// exportaudit:keep — one member of a public enum; the set ships whole
	ContentTransitionNumeric
	// ContentTransitionReplace shrinks the old text away while the new
	// text grows in. It is made for an icon that changes to another.
	// exportaudit:keep — one member of a public enum; the set ships whole
	ContentTransitionReplace
)

// ContentTransition animates a change of TextCfg.Text. The zero value
// animates nothing.
//
// The transition is keyed by the text's effective ID, so TextCfg.ID
// must be set: without one it is a silent no-op, reported by
// gui.Debug.
//
// When the user asks for reduced motion, every kind fades. A wrapped
// text also fades: a roll or a scale is drawn for one line of plain
// glyphs only. A password does not animate: its text is never drawn
// in clear, and a fade between two rows of bullets shows nothing.
//
// A change that arrives while a transition runs starts a new one from
// the text the old one was moving to. A change to the empty text has
// nothing to move to, so the old text goes at once.
//
// While a transition runs, TextCfg.Anim's opacity still applies, but
// its motion, shimmer and reveal wait until the transition ends.
type ContentTransition struct {
	Kind ContentTransitionKind
	// Down makes a numeric roll move down, whatever the values. By
	// default the direction comes from the values: a larger number
	// rolls up, a smaller one rolls down.
	Down bool
}

// contentTransitionDuration is the length of one transition. It is the
// same as a text entrance (textAnimDurationEntrance), so the two read
// as one family.
const contentTransitionDuration = 300 * time.Millisecond

// contentTransitionTnum asks the font for tabular figures. Shared and
// never written, so a numeric text costs no allocation for it.
var contentTransitionTnum = &glyph.FontFeatures{
	OpenTypeFeatures: []glyph.FontFeature{{Tag: "tnum", Value: 1}},
}

// contentTransitionState is what one transitioning text keeps between
// view passes: the text it showed last, the slot its shape points at
// and the pass that last generated it, for pruning.
type contentTransitionState struct {
	text string
	slot *contentTransitionSlot
	seen uint64
}

// contentTransitionSlot is what the shape of a transitioning text points
// at, in every frame and not only while a run is in flight. It is
// allocated once per text. The render pass writes into it where it drew
// the text, so the next run knows where the old text is on screen.
//
// That position is needed because the shape is laid out for the new
// text. When the width changes, a parent that centers or right-aligns
// its children moves the shape, and the new box no longer says where
// the old text was.
type contentTransitionSlot struct {
	// run is the transition in flight. Nil when none runs.
	run *contentTransitionRun
	// boxX is the left edge of the content box and shapeW the width of
	// the shape in the render pass numbered pass. A pass of 0 means no
	// frame has drawn the text yet.
	boxX   float32
	shapeW float32
	pass   uint64
}

// contentTransitionRun is one transition from one text to the next. It
// is allocated once when the text changes. The shape points at it, and
// the driver writes progress into it, so a render-only frame reads the
// current progress without a view pass.
type contentTransitionRun struct {
	from string
	to   string
	// cols is the per-grapheme plan of a numeric roll, from the right.
	cols []contentTransitionCol
	// fromW and toW are the measured widths of from and to, for
	// alignment inside a box wider than the text.
	fromW float32
	toW   float32
	// fromLayout is the shaped old text, and toLayout the new text of
	// a Replace. Each is built on the first frame that draws it, and is
	// nil until then and without a text measurer. A Replace shapes from
	// with no width, to scale it about its own center. A fade shapes it
	// as the plain path shaped it in the old box, so its lines wrap and
	// align as they did.
	fromLayout *glyph.Layout
	toLayout   *glyph.Layout
	progress   float32
	// fromDX is the old box's left edge, from the left of the current
	// content box, and fromShapeW the old shape's width. Both are set
	// on the first frame that draws the run, from what the slot
	// recorded in the pass before; fromSet tells if they are set.
	fromDX     float32
	fromShapeW float32
	fromSet    bool
	kind       ContentTransitionKind
	down       bool
	done       bool
}

// contentTransitionCol is one column of a numeric roll. Byte ranges
// index the run's from and to strings; a start of -1 means the column
// has no grapheme on that side. x is the column's left edge, from the
// left of the new text; the old grapheme rolls in the same column.
type contentTransitionCol struct {
	x                  float32
	fromStart, fromEnd int
	toStart, toEnd     int
	changed            bool
}

// contentTransitionPruneAt is the entry count at which a new ID first
// drops the entries of texts that left the tree. See textAnimPruneAt.
const contentTransitionPruneAt = 256

// contentTransitionStates is the per-window map of transition state.
// It is unbounded; pruneContentTransitionStates keeps it to the texts
// in the tree.
func contentTransitionStates(
	w *Window,
) *BoundedMap[string, contentTransitionState] {
	return StateMap[string, contentTransitionState](
		w, nsContentTransition, 0)
}

// pruneContentTransitionStates drops the entries of texts that were
// not generated in this view pass or the one before. Same rule as
// pruneTextAnimStates.
func pruneContentTransitionStates(
	pm *BoundedMap[string, contentTransitionState], pass uint64,
) {
	type kv struct {
		k string
		v contentTransitionState
	}
	keep := make([]kv, 0, pm.Len())
	pm.Range(func(k string, v contentTransitionState) bool {
		if v.seen+1 >= pass {
			keep = append(keep, kv{k, v})
		}
		return true
	})
	if len(keep) == pm.Len() {
		return
	}
	pm.Clear()
	for _, e := range keep {
		pm.Set(e.k, e.v)
	}
}

// applyContentTransition starts a transition when the text changed
// since the last view pass, and points the shape at the run in flight.
//
// The first pass that sees a text records it and does nothing else:
// a text that appears has no old value to move from. So does a text
// whose entry the last pass did not refresh — its transition was off,
// it was a password, or it left the tree — because the text the entry
// holds is out of date. A run from it would draw a value the text has
// not shown for a while, and for a password just made visible, a
// value that was typed while masked. Reading staleness here costs the
// early returns below nothing, which matters: they run for every Text.
func applyContentTransition(tv *textView, w *Window, tc *shapeTextConfig) {
	cfg := &tv.cfg
	// A password starts nothing: the run draws its from and to texts
	// as given, and renderText is what masks a password.
	if cfg.Transition.Kind == ContentTransitionNone ||
		cfg.Transition.Kind > ContentTransitionReplace || cfg.IsPassword {
		return
	}
	if cfg.ID == "" {
		// Gated at the call site: the variadic args allocate whether or
		// not the check is on, and this runs every frame.
		if DebugCategory(debugMask.Load())&DebugMissingIDs != 0 {
			w.debugWarn(debugCheckContentTransitionNoID, cfg.Text,
				"text %q has a content transition and no ID; the "+
					"transition is keyed by ID, so nothing animates",
				cfg.Text)
		}
		return
	}
	key := w.EffID(cfg.ID)
	pm := contentTransitionStates(w)
	st, ok := pm.Get(key)
	if !ok && pm.Len() >= contentTransitionPruneAt {
		pruneContentTransitionStates(pm, w.viewPass)
	}
	if ok && st.seen+1 < w.viewPass {
		// Stale: start over as for a new text. A run still in flight
		// belongs to a text the user has not seen move since.
		st = contentTransitionState{}
		ok = false
	}
	// A change to "" starts nothing: renderText draws nothing for an
	// empty text, so the old text could not be drawn going out.
	if st.slot == nil {
		st.slot = &contentTransitionSlot{}
	}
	if ok && st.text != cfg.Text && cfg.Text != "" {
		st.slot.run = newContentTransitionRun(st.text, cfg, w)
		run := st.slot.run
		// AnimationAdd replaces a driver with the same ID, so a change
		// mid-flight retires the old run's driver. The old run is left
		// with no writer and is dropped with the state.
		w.AnimationAdd(&KeyframeAnimation{
			AnimID:   ScopeID("ctrans", key),
			Duration: contentTransitionDuration,
			Keyframes: []Keyframe{
				{At: 0, Value: 0},
				{At: 1, Value: 1, Easing: EaseLinear},
			},
			// The run is all a frame reads, so no view pass is needed.
			Refresh: AnimationRefreshRenderOnly,
			OnValue: func(v float32, _ *Window) {
				run.progress = v
			},
			OnDone: func(*Window) {
				run.progress = 1
				run.done = true
			},
		})
	}
	st.text = cfg.Text
	if st.slot.run != nil && st.slot.run.done {
		st.slot.run = nil
	}
	// Set in every frame, run or not: the render pass records where the
	// text is drawn, for the next run to start from.
	tc.ctrans = st.slot
	st.seen = w.viewPass
	pm.Set(key, st)
}

// newContentTransitionRun plans the move from text from to the new
// text of cfg. It runs once per change, so it may measure and allocate.
func newContentTransitionRun(
	from string, cfg *TextCfg, w *Window,
) *contentTransitionRun {
	run := &contentTransitionRun{
		from: from,
		to:   cfg.Text,
		kind: cfg.Transition.Kind,
	}
	// A wrapped text has more than one line to roll, so it fades. So
	// does every kind under reduced motion: a fade has no movement. A
	// password never gets here (applyContentTransition).
	if cfg.Mode != TextModeSingleLine || w.prefersReducedMotion() {
		run.kind = ContentTransitionFade
	}
	style := cfg.TextStyle
	run.fromW = w.TextWidth(run.from, style)
	run.toW = w.TextWidth(run.to, style)
	if run.kind == ContentTransitionNumeric {
		run.down = cfg.Transition.Down ||
			contentTransitionLess(run.to, run.from)
		run.cols = contentTransitionColumns(run.from, run.to, style, w)
	}
	return run
}

// contentTransitionLess reports whether a reads as a smaller number
// than b. Text that does not read as a number on both sides is not
// smaller, so it rolls up.
func contentTransitionLess(a, b string) bool {
	x, okA := contentTransitionNumber(a)
	y, okB := contentTransitionNumber(b)
	return okA && okB && x < y
}

// contentTransitionNumber reads the number in s, skipping every byte
// that is not a digit, a sign or a decimal point: "$1,204.50" reads as
// 1204.5. False when nothing that is left parses.
func contentTransitionNumber(s string) (float64, bool) {
	// A fixed buffer: a price or a counter fits, and a longer text is
	// not a number worth comparing.
	var buf [32]byte
	n := 0
	for i := 0; i < len(s) && n < len(buf); i++ {
		c := s[i]
		if (c >= '0' && c <= '9') || c == '.' || (c == '-' && n == 0) {
			buf[n] = c
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(string(buf[:n]), 64)
	return v, err == nil
}

// contentTransitionColumns pairs the graphemes of from and to from the
// right and measures where each column sits. A column whose graphemes
// match does not move; the others roll.
//
// A column's x comes from the shaped new text when there is one, not
// from the width of the text before it. The two differ by the kerning
// between the column and its left neighbour: a display face kerns
// "11". Each column is drawn alone during the roll and the whole text
// is drawn shaped after it, so a column placed by width jumped by that
// kerning when the roll ended.
func contentTransitionColumns(
	from, to string, style TextStyle, w *Window,
) []contentTransitionCol {
	fromB := graphemeBounds(from)
	toB := graphemeBounds(to)
	var toLayout glyph.Layout
	if w.textMeasurer != nil && to != "" {
		if l, err := w.textMeasurer.LayoutText(to, style, 0); err == nil {
			toLayout = l
		}
	}
	n := max(len(fromB), len(toB))
	cols := make([]contentTransitionCol, n)
	for i := range n {
		// i counts from the right; the column index counts from the
		// left, so the slice reads in paint order.
		col := &cols[n-1-i]
		col.fromStart, col.toStart = -1, -1
		if fi := len(fromB) - 1 - i; fi >= 0 {
			col.fromStart, col.fromEnd = fromB[fi][0], fromB[fi][1]
		}
		if ti := len(toB) - 1 - i; ti >= 0 {
			col.toStart, col.toEnd = toB[ti][0], toB[ti][1]
			x, ok := contentTransitionClusterX(&toLayout, col.toStart)
			if !ok {
				x = w.TextWidth(to[:col.toStart], style)
			}
			col.x = x
		} else {
			// The old text is longer: this column lies left of the new
			// text, as far left as the old graphemes between it and the
			// first shared column. It rolls out past the clip.
			// With no new graphemes at all, that column is the end of
			// the old text.
			first := len(fromB) - len(toB)
			anchor := len(from)
			if first < len(fromB) {
				anchor = fromB[first][0]
			}
			col.x = -w.TextWidth(from[col.fromStart:anchor], style)
		}
		col.changed = col.fromStart < 0 || col.toStart < 0 ||
			from[col.fromStart:col.fromEnd] != to[col.toStart:col.toEnd]
	}
	return cols
}

// contentTransitionClusterX returns where l draws the cluster that
// starts at byte start: the pen position of its first glyph plus that
// glyph's own offset, which is where kerning lands. A column drawn
// alone at this x puts its glyph where the shaped text does. False
// when l has no glyph for start — no measurer, or a shaper that
// returned no glyphs.
func contentTransitionClusterX(l *glyph.Layout, start int) (float32, bool) {
	for i := range l.Items {
		it := &l.Items[i]
		if it.GlyphStart < 0 || it.GlyphCount < 0 ||
			it.GlyphStart+it.GlyphCount > len(l.Glyphs) {
			continue
		}
		pen := it.X
		for _, g := range l.Glyphs[it.GlyphStart : it.GlyphStart+it.GlyphCount] {
			if int(g.Index&^glyph.PangoGlyphUnknownFlag) == start {
				x := float32(pen + g.XOffset)
				return x, f32IsFinite(x)
			}
			pen += g.XAdvance
		}
	}
	return 0, false
}

// graphemeBounds returns the [start, end) byte range of each grapheme
// cluster of s.
func graphemeBounds(s string) [][2]int {
	out := make([][2]int, 0, len(s))
	state := -1
	pos := 0
	rest := s
	for rest != "" {
		var cluster string
		cluster, rest, _, state = uniseg.FirstGraphemeClusterInString(
			rest, state)
		out = append(out, [2]int{pos, pos + len(cluster)})
		pos += len(cluster)
	}
	return out
}

// contentTransitionAlignX returns how far right of the content box's
// left edge a text of width textW starts, under the style's alignment.
func contentTransitionAlignX(
	align TextAlignment, boxW, textW float32,
) float32 {
	switch align {
	case TextAlignCenter:
		return (boxW - textW) / 2
	case TextAlignRight:
		return boxW - textW
	default:
		return 0
	}
}

// renderContentTransition draws one frame of a transition in place of
// the shape's text. It reports false when there is nothing to draw
// from — no run, or the run has finished — and renderText then draws
// the text as usual.
//
// A focused text, or one with a selection, also draws as usual. Its
// caret, its selection and an IME preedit are drawn by the plain path,
// on the plain text, and none of them has a meaning on two texts that
// move. So the new text shows at once.
//
// c is the text's paint color with the shape's opacity and disabled
// state applied; style is the render style that carries it.
func renderContentTransition(
	shape *Shape, slot *contentTransitionSlot,
	clip drawClip, c Color, style TextStyle, w *Window,
) bool {
	if slot == nil {
		return false
	}
	tc := shape.TC
	f := contentTransitionFrame{
		shape: shape,
		run:   slot.run,
		baseX: shape.X + shape.PaddingLeft(),
		baseY: shape.Y + shape.PaddingTop(),
		boxW:  shape.Width - shape.paddingWidth(),
		boxH:  shape.Height - shape.paddingHeight(),
		c:     c,
		style: style,
		w:     w,
	}
	// Read the box of the pass before, then record this one. A box is
	// only of use from the pass just before: renderText returns before
	// it gets here for a culled or a fully transparent text, so an
	// older box can be anywhere. The box read when a run starts is where
	// the old text was drawn last — by the plain path, or as the new
	// text of the run before.
	prevX, prevShapeW := slot.boxX, slot.shapeW
	prevOK := slot.pass != 0 && slot.pass+1 == w.renderPass
	slot.boxX, slot.shapeW, slot.pass = f.baseX, shape.Width, w.renderPass

	run := f.run
	if run == nil || run.done || tc == nil ||
		tc.textSelBeg != tc.textSelEnd || textShapeFocused(shape, w) {
		return false
	}
	f.p = EaseOutCubic(f32Clamp(run.progress, 0, 1))
	if !f32IsFinite(f.p) {
		return false
	}
	if !run.fromSet {
		run.fromSet = true
		run.fromDX, run.fromShapeW = 0, shape.Width
		if prevOK && f32IsFinite(prevX-f.baseX) && f32IsFinite(prevShapeW) {
			run.fromDX, run.fromShapeW = prevX-f.baseX, prevShapeW
		}
	}
	// Same rule as renderText: a style from the disabled role is
	// already quiet, so its gradient is not dimmed again.
	f.gradDim = shape.Disabled && !tc.TextStyle.disabledRole

	switch run.kind {
	case ContentTransitionNumeric:
		f.roll(clip)
	case ContentTransitionReplace:
		if !f.scale() {
			f.fade()
		}
	default:
		f.fade()
	}
	return true
}

// contentTransitionFrame is what the draws of one transition frame
// share. It lives on the stack of renderContentTransition.
type contentTransitionFrame struct {
	shape *Shape
	run   *contentTransitionRun
	w     *Window
	style TextStyle
	// p is the eased progress, 0 to 1.
	p float32
	// baseX, baseY, boxW and boxH are the current content box.
	baseX, baseY, boxW, boxH float32
	// c is the paint color with the shape's opacity and disabled state
	// applied. gradDim tells if a gradient fill is dimmed for the
	// disabled state.
	c       Color
	gradDim bool
}

// gradient is the style's gradient fill at alpha times the shape's
// opacity, as renderText dims it, or nil when the style has none.
func (f *contentTransitionFrame) gradient(alpha float32) *glyph.GradientConfig {
	return dimmedTextGradient(f.style.Gradient, f.shape.Opacity*alpha,
		f.gradDim, &f.w.scratch)
}

// oldContentW is the content width of the old box.
func (f *contentTransitionFrame) oldContentW() float32 {
	return f.run.fromShapeW - f.shape.paddingWidth()
}

// fade draws the old text at 1-p of its alpha and the new text at p,
// each the way renderText draws a plain text: through a glyph layout
// where renderText uses one, so that each line wraps and aligns as it
// will when the fade ends, and as a plain text draw elsewhere. The old
// text is drawn in the box it was drawn in.
func (f *contentTransitionFrame) fade() {
	run := f.run
	tc := f.shape.TC
	if f.w.textMeasurer != nil &&
		plainTextNeedsGlyphLayout(f.shape, tc, f.style) && f.fadeLayouts() {
		return
	}
	// The plain path draws at the box's left edge, and wraps at the
	// shape's width.
	var fromWrap, toWrap float32
	if tc.TextMode == TextModeWrap || tc.TextMode == TextModeWrapKeepSpaces {
		fromWrap, toWrap = run.fromShapeW, f.shape.Width
	}
	f.emitText(run.from, f.baseX+run.fromDX, f.baseY, fromWrap, 1-f.p)
	f.emitText(run.to, f.baseX, f.baseY, toWrap, f.p)
}

// fadeLayouts draws a fade from glyph layouts. The new text uses the
// shape's own cached layout. The old text is shaped once for the run,
// with the width argument the plain path gave it in the old box. It
// reports false when the shaper refuses either text.
func (f *contentTransitionFrame) fadeLayouts() bool {
	run := f.run
	toLayout, ok := inputGlyphLayoutResolved(run.to, f.shape, f.style, f.w, true)
	if !ok {
		return false
	}
	if run.fromLayout == nil {
		// A copy of the shape, for its width only: the width argument
		// depends on the mode, the alignment and the width.
		old := *f.shape
		old.Width = run.fromShapeW
		l, err := f.w.textMeasurer.LayoutText(run.from, f.style,
			plainTextLayoutWidthArg(&old, f.shape.TC, f.style))
		if err != nil {
			return false
		}
		run.fromLayout = &l
	}
	f.emitLayout(*run.fromLayout, run.from, f.baseX+run.fromDX, 1-f.p)
	f.emitLayout(toLayout, run.to, f.baseX, f.p)
	return true
}

// roll draws a numeric roll. A column that did not change is drawn in
// place. In a column that changed, the old grapheme leaves by one line
// height and the new one arrives from the other side, each fading as it
// goes. The content box clips both, so a digit is never seen outside
// the line. The columns follow the new text: the old digits roll out
// of them, wherever the old box was.
func (f *contentTransitionFrame) roll(clip drawClip) {
	run := f.run
	box := drawClip{X: f.baseX, Y: f.baseY, Width: f.boxW, Height: f.boxH}
	// No overlap clips the text away entirely, which is the safe
	// direction: a dropped RenderClip would leave the parent's wider
	// scissor in force.
	inner, _ := rectIntersection(box, clip)
	emitClipCmd(inner, f.w)

	// Up: the old digit rises out of the top and the new one rises in
	// from below. Down mirrors it.
	sign := float32(1)
	if run.down {
		sign = -1
	}
	fromDY := -sign * f.p * f.boxH
	toDY := sign * (1 - f.p) * f.boxH

	x0 := f.baseX + contentTransitionAlignX(f.style.Align, f.boxW, run.toW)
	for i := range run.cols {
		col := &run.cols[i]
		x := x0 + col.x
		if !col.changed {
			f.emitText(run.to[col.toStart:col.toEnd], x, f.baseY, 0, 1)
			continue
		}
		if col.fromStart >= 0 {
			f.emitText(run.from[col.fromStart:col.fromEnd],
				x, f.baseY+fromDY, 0, 1-f.p)
		}
		if col.toStart >= 0 {
			f.emitText(run.to[col.toStart:col.toEnd],
				x, f.baseY+toDY, 0, f.p)
		}
	}
	emitClipCmd(clip, f.w)
}

// scale draws a Replace: the old text shrinks to nothing about its
// center while the new one grows from nothing. It needs glyph layouts
// to carry the transform, so it reports false without a text measurer
// or when the shaper refuses a text, and the caller fades instead.
//
// Both layouts are shaped with no width, so each is placed by
// contentTransitionAlignX in its own box and scales about its own
// center: the old one in the old box, the new one in the current box.
func (f *contentTransitionFrame) scale() bool {
	run := f.run
	if f.w.textMeasurer == nil {
		return false
	}
	if run.toLayout == nil {
		l, err := f.w.textMeasurer.LayoutText(run.to, f.style, 0)
		if err != nil {
			return false
		}
		run.toLayout = &l
	}
	if run.fromLayout == nil {
		l, err := f.w.textMeasurer.LayoutText(run.from, f.style, 0)
		if err != nil {
			return false
		}
		run.fromLayout = &l
	}
	fromX := f.baseX + run.fromDX +
		contentTransitionAlignX(f.style.Align, f.oldContentW(), run.fromW)
	toX := f.baseX + contentTransitionAlignX(f.style.Align, f.boxW, run.toW)
	f.emitScaled(*run.fromLayout, run.from, fromX, 1-f.p, run.fromW)
	f.emitScaled(*run.toLayout, run.to, toX, f.p, run.toW)
	return true
}

// emitLayout emits one glyph-layout draw at (x, baseY) at alpha, as
// emitTextLayoutCmd draws a plain text: with the style's gradient and,
// when the style has one, its transform.
func (f *contentTransitionFrame) emitLayout(
	l glyph.Layout, text string, x, alpha float32,
) {
	c := f.c.WithOpacity(alpha)
	if c.A == 0 {
		return
	}
	style := f.style
	style.Color = c
	l.Items = recolorLayoutItems(l.Items, style, f.w)
	cmd := RenderCmd{
		Kind:         RenderLayout,
		X:            x,
		Y:            f.baseY,
		Text:         text,
		LayoutPtr:    f.w.scratch.renderGlyphLayouts.alloc(l),
		TextStylePtr: f.w.scratch.renderTextStyles.alloc(style),
		TextGradient: f.gradient(alpha),
	}
	if style.hasTextTransform() {
		cmd.Kind = RenderLayoutTransformed
		cmd.LayoutTransform = f.w.scratch.renderAffineTransforms.alloc(
			style.effectiveTextTransform())
	}
	emitRenderer(cmd, f.w)
}

// emitScaled emits one layout draw at (x, baseY) scaled by s about the
// center of a textW by boxH box, at alpha s.
func (f *contentTransitionFrame) emitScaled(
	l glyph.Layout, text string, x, s, textW float32,
) {
	c := f.c.WithOpacity(s)
	if c.A == 0 || s <= 0 {
		return
	}
	style := f.style
	style.Color = c
	l.Items = recolorLayoutItems(l.Items, style, f.w)
	xf := textAnimTransform(s, 0, 0, 0, textW/2, f.boxH/2)
	emitRenderer(RenderCmd{
		Kind:            RenderLayoutTransformed,
		X:               x,
		Y:               f.baseY,
		Text:            text,
		LayoutPtr:       f.w.scratch.renderGlyphLayouts.alloc(l),
		TextStylePtr:    f.w.scratch.renderTextStyles.alloc(style),
		TextGradient:    f.gradient(s),
		LayoutTransform: f.w.scratch.renderAffineTransforms.alloc(xf),
	}, f.w)
}

// emitText emits one plain text draw at (x, y) at alpha, wrapped at
// wrapW when it is above 0. A fully transparent one is skipped, as
// renderText skips its own.
func (f *contentTransitionFrame) emitText(
	text string, x, y, wrapW, alpha float32,
) {
	c := f.c.WithOpacity(alpha)
	if c.A == 0 || text == "" {
		return
	}
	style := f.style
	style.Color = c
	fontAscent := style.Size * 0.8 // fallback, as in renderText
	var textWidth float32
	if f.w.textMeasurer != nil {
		fontAscent = f.w.textMeasurer.FontAscent(style)
		textWidth = f.w.textMeasurer.TextWidth(text, style)
	}
	emitRenderer(RenderCmd{
		Kind:         RenderText,
		X:            x,
		Y:            y,
		Color:        c,
		Text:         text,
		FontName:     style.Family,
		FontSize:     style.Size,
		FontAscent:   fontAscent,
		TextWidth:    textWidth,
		W:            wrapW,
		TextStylePtr: f.w.scratch.renderTextStyles.alloc(style),
		TextGradient: f.gradient(alpha),
	}, f.w)
}
