package gui

import "github.com/go-gui-org/go-glyph"

// TextCfg configures a text view. Use for labels, headings, or
// multiline text blocks. Set Focusable to enable text selection
// and clipboard copy.
type TextCfg struct {
	TextStyle TextStyle
	ID        string
	Text      string

	A11YCfg
	Opacity   Opt[float32]
	Focusable bool

	// TabSize sets the tab stop width in spaces (default 4).
	TabSize uint32

	MinWidth float32
	Sizing   Sizing

	// Mode controls text wrapping and overflow behavior. See
	// TextMode constants.
	Mode textMode

	Invisible  bool
	Clip       bool
	FocusSkip  bool
	Disabled   bool
	IsPassword bool

	// PlaceholderActive enables placeholder styling (dimmed).
	// Set by input widgets; not typically set directly.
	placeholderActive bool

	// Hero marks this text element for hero transition
	// animations between views.
	Hero bool

	// Anim animates the text. The zero value animates nothing. An
	// animated text needs a non-empty ID, because the animation is
	// keyed by identity; without one it is a silent no-op that
	// gui.Debug reports.
	Anim TextAnimCfg

	// readOnly is set by input widgets (view_input.go) to suppress
	// IME preedit on a read-only field that stays Focusable.
	// Unexported: not a meaningful knob for standalone Text callers.
	readOnly bool

	// focusOwner is set by input widgets (view_input.go) to the ID of
	// the container that owns the focus and per-widget state this text
	// renders from. See Shape.focusOwner. Unexported: a standalone
	// Text owns its identity through ID.
	focusOwner string

	// scrollOverflowX opts this text into ink-overflow accounting, so a
	// run too wide to wrap can be reached by an ancestor scroll
	// container. See Shape.inkOverflowW. Set by Input; unexported
	// because a standalone Text has no scroll container to reach it.
	scrollOverflowX bool
}

// textView implements View for text rendering.
type textView struct {
	cfg TextCfg
	tc  shapeTextConfig

	// affine and shimmer are scratch for TextCfg.Anim. They live on
	// the view so the style can point at them for the frame: the view
	// outlives generation and render, which a local would not.
	affine  glyph.AffineTransform
	shimmer textAnimShimmer
}

// textEventHandlers is a shared handler set for focused text
// widgets, avoiding per-frame heap allocations.
var textEventHandlers = &eventHandlers{
	OnClick:     textOnClick,
	OnKeyDown:   textOnKeyDown,
	AmendLayout: textAmendLayout,
}

func (tv *textView) GenerateLayout(w *Window) Layout {
	c := &tv.cfg
	ts := &c.TextStyle

	tv.tc = shapeTextConfig{
		Text:              c.Text,
		TextStyle:         ts,
		textIsPassword:    c.IsPassword,
		textIsPlaceholder: c.placeholderActive,
		TextMode:          c.Mode,
		TextTabSize:       c.TabSize,
		textReadOnly:      c.readOnly,
		overflowScrollX:   c.scrollOverflowX,
	}

	layout := Layout{
		Shape: w.allocShape(Shape{
			shapeType:  shapeText,
			ID:         c.ID,
			focusOwner: c.focusOwner,
			Focusable:  c.Focusable,
			A11YRole:   AccessRoleStaticText,
			a11Y:       c.a11yInfo(c.Text),
			Clip:       c.Clip,
			FocusSkip:  c.FocusSkip,
			Disabled:   c.Disabled,
			MinWidth:   c.MinWidth,
			Sizing:     c.Sizing,
			Hero:       c.Hero,
			Opacity:    c.Opacity.Get(1.0),
			TC:         &tv.tc,
		}),
	}

	// The animation runs before measuring: a typewriter reveal must
	// reach tc.Text in time to shorten what is painted. Measuring below
	// still uses the full string, so a reveal reserves its final width
	// and nothing around it reflows as the text types itself out.
	animFrame := applyTextAnim(tv, w, layout.Shape)

	// Measure what is painted, not what is stored: a password renders
	// as bullets, whose advance differs from the raw text's. Measuring
	// the raw text would size the box wrong and, for an Input, park the
	// horizontal scroll at the wrong maximum offset. maskPassword is
	// the same helper renderText masks with, so the measured string and
	// the painted one agree, newlines included.
	measured := c.Text
	if c.IsPassword {
		measured = maskPassword(measured)
	}
	layout.Shape.Width = w.TextWidth(measured, *ts)
	if w.textMeasurer != nil {
		layout.Shape.Height = w.textMeasurer.FontHeight(*ts)
	} else {
		layout.Shape.Height = fallbackLineHeight(*ts)
	}
	if c.Mode == TextModeSingleLine ||
		layout.Shape.Sizing.Width == sizingFixed {
		layout.Shape.MinWidth = f32Max(
			layout.Shape.Width, layout.Shape.MinWidth,
		)
		layout.Shape.Width = layout.Shape.MinWidth
	}
	if c.Mode == TextModeSingleLine ||
		layout.Shape.Sizing.Height == sizingFixed {
		layout.Shape.MinHeight = f32Max(
			layout.Shape.Height, layout.Shape.MinHeight,
		)
		layout.Shape.Height = layout.Shape.MinHeight
	}
	applyFixedSizingConstraints(layout.Shape)

	// After sizing: the frame's scale and rotation turn about the
	// measured box's center.
	applyTextAnimTransform(tv, layout.Shape, animFrame)

	if c.Focusable {
		layout.Shape.events = textEventHandlers
	}

	return layout
}

// Label is the thin form of Text for the common case: a text string
// and a style. Pass the zero TextStyle to get the default theme
// style. Callers needing Focusable, Sizing, Mode, or the rest of
// TextCfg use Text directly.
func Label(text string, style TextStyle) View {
	return Text(TextCfg{Text: text, TextStyle: style})
}

// Text creates a text view for displaying text content.
func Text(cfg TextCfg) View {
	if cfg.Invisible {
		return invisibleContainerView()
	}
	sizing := cfg.Sizing
	if !sizing.IsSet() {
		if cfg.Mode == TextModeWrap ||
			cfg.Mode == TextModeWrapKeepSpaces {
			sizing = FillFit
		} else {
			sizing = FitFit
		}
	}
	if cfg.TabSize == 0 {
		cfg.TabSize = 4
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = DefaultTextStyle
		// Marks the style as taking the default color, so a filled
		// button variant can recolor it (see TextStyle.defaultedColor).
		cfg.TextStyle.defaultedColor = true
	}
	if cfg.TextStyle.Size == 0 {
		cfg.TextStyle.Size = sizeTextMedium
	}
	cfg.Sizing = sizing
	return &textView{cfg: cfg}
}
