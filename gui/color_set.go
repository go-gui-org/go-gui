package gui

// ColorSet groups the per-state colors of an interactive widget into
// one value, so a caller who wants a single consistent appearance says
// it once instead of assigning five or six flat Color* fields.
//
// Eighteen widgets carry it, and it is the only spelling on all of
// them: Button, Switch, Toggle, Radio, DatePicker, InputDate, Input,
// NumericInput, Select, Combobox, ListBox, VirtualList, Tree, Slider,
// ContextMenu, Menubar, Table, ExpandPanel. The flat per-state Color*
// Cfg fields the last twelve used to carry are gone (issue #721); the
// flat Color survives as the shorthand for Base.
//
// Zero value is "nothing specified": every field falls back, and a
// ColorSet a caller never touches changes nothing.
//
// Why plain Color and not Opt[Color]: Color already carries its own
// set flag (see Color.IsSet), and ColorTransparent is an explicitly-set
// fully-transparent color, so "unset" and "deliberately transparent"
// are already distinguishable without a wrapper. Wrapping would give
// the type two independent notions of unset — Some(Color{}) would mean
// "set to unset" — and every existing widget already branches on
// Color.IsSet.
type ColorSet struct {
	// Base is the resting background color. It is also the fallback
	// for Hover, Click and Focus when those are unset.
	Base Color

	// Hover, Click and Focus are the background colors for the three
	// interactive states. Unset means "same as Base".
	//
	// Focus is the rarest of the three. pick gives the fill to the
	// pointer, so the focus fill shows only where a control is focused
	// and the pointer is elsewhere — a control reached by the keyboard.
	// It is not dead; do not delete it because a hover test never
	// reaches it.
	Hover Color
	Click Color
	Focus Color

	// Border is the border color. It does NOT fall back to Base —
	// a border the same color as the fill reads as no border at all,
	// which is not what omitting the field should mean. Unset falls
	// through to the theme.
	Border Color

	// BorderFocus is the border color while keyboard-focused. Unset
	// falls back to Border, then to the theme.
	BorderFocus Color

	// Selected is the resting fill of a selected element: the current
	// tab, the current crumb, a selected row. Hover and press on a
	// selected element do not take Hover and Click. pick derives them
	// from Selected with the same OKLCH lightness step ThemeMaker uses
	// for the accent ramp (#732), so the change reads the same on every
	// hue and a caller sets one color, not three (#741). Unset falls
	// through to the theme; with no theme value either, a selected
	// element paints as an unselected one.
	Selected Color

	// Disabled is the fill of a disabled element. When it is set, it
	// REPLACES the renderer's disabled dim for the fill: the color is
	// painted as given, not at half alpha, so a caller says exactly
	// what a disabled element looks like (#741). The border and the
	// text still dim. Unset keeps the old rule: Base, dimmed.
	Disabled Color
}

// Flat returns a ColorSet whose every field is c, including the two
// border fields. This is the "do not react" case: a widget that keeps
// one appearance through hover, press and focus.
//
// Flat is not the same as ColorSet{Base: c}. Base only backs the three
// interactive states; Flat also pins the borders, which is what makes
// the widget visually inert rather than merely uniform in its fill.
//
// Flat leaves Selected and Disabled unset. Those two are not
// interaction states the pointer drives, and pinning them would
// override the theme's selected color and remove the disabled dim
// from every Flat caller.
func Flat(c Color) ColorSet {
	return ColorSet{
		Base:        c,
		Hover:       c,
		Click:       c,
		Focus:       c,
		Border:      c,
		BorderFocus: c,
	}
}

// IsSet reports whether any field of the set was specified. Used by
// widgets to skip resolution entirely for the common zero-value case.
func (cs ColorSet) IsSet() bool {
	return cs.Base.IsSet() || cs.Hover.IsSet() || cs.Click.IsSet() ||
		cs.Focus.IsSet() || cs.Border.IsSet() || cs.BorderFocus.IsSet() ||
		cs.Selected.IsSet() || cs.Disabled.IsSet()
}

// resolve returns the set with its internal fallbacks applied: the
// three interactive states default to Base, and BorderFocus defaults
// to Border. Fields still unset after this have nothing to say and are
// left for the theme.
func (cs ColorSet) resolve() ColorSet {
	if cs.Base.IsSet() {
		if !cs.Hover.IsSet() {
			cs.Hover = cs.Base
		}
		if !cs.Click.IsSet() {
			cs.Click = cs.Base
		}
		if !cs.Focus.IsSet() {
			cs.Focus = cs.Base
		}
	}
	if !cs.BorderFocus.IsSet() && cs.Border.IsSet() {
		cs.BorderFocus = cs.Border
	}
	return cs
}

// resolved returns a fully-populated set: the caller's own fallbacks
// first, then the theme for anything still unspecified.
//
// shorthand is the widget's flat Color field, which survives as the
// spelling for the single-color case. It takes precedence over
// Colors.Base: code that set Color keeps its appearance when a
// ColorSet arrives.
//
// This is the seam every widget's apply*Defaults calls, replacing the
// six near-identical `if !cfg.ColorX.IsSet()` blocks each of them
// carried.
func (cs ColorSet) resolved(shorthand Color, theme ColorSet) ColorSet {
	// The shorthand is applied AFTER resolve, and the ordering is the
	// whole point. Base backs the interactive states, so folding Color
	// in beforehand would make `Color: c` silently pin hover, click and
	// focus to c as well — a widget that sets only its background would
	// stop reacting to the pointer and to focus. That is a behavior
	// change, not a refactor, and Flat(c) is how a caller asks for it
	// deliberately.
	//
	// Applied after, `Color: c` sets the resting color and leaves the
	// states to the theme, which is exactly what it did before ColorSet
	// existed.
	cs = cs.resolve()
	if shorthand.IsSet() {
		cs.Base = shorthand
	}
	setIfUnset(&cs.Base, theme.Base)
	setIfUnset(&cs.Hover, theme.Hover)
	setIfUnset(&cs.Click, theme.Click)
	setIfUnset(&cs.Focus, theme.Focus)
	setIfUnset(&cs.Border, theme.Border)
	setIfUnset(&cs.BorderFocus, theme.BorderFocus)
	setIfUnset(&cs.Selected, theme.Selected)
	setIfUnset(&cs.Disabled, theme.Disabled)
	return cs
}

// stateFlags is the interaction state of one widget at one point in one
// frame. The caller assembles it from what its pass can see: the amend
// pass knows focus and a held Space but has no Event and so no pointer;
// the hover pass knows the pointer and the held mouse button as well.
//
// disabled makes the rule total: pick answers for every state rather
// than leaving one to the caller, so a pass with no guard of its own
// can be routed through it without inventing one.
//
// It is still not what short-circuits most disabled widgets. Every
// hover pass is already unreachable on a disabled shape — layoutHoverDepth
// skips them, unlike layoutAmend — and most amend passes return early on
// Shape.Disabled, because that same guard also covers a shape with no
// events and a nil OnClick and is the only thing stopping a disabled
// widget's user OnAmend callback.
//
// The two amend passes that do NOT guard are Input's and ExpandPanel's
// header (#721): both reach pick with disabled true on a disabled
// widget, and both get the resting colors back, which is what the shape
// already carried out of generation.
//
// selected is not an interaction state; it is the widget's own model
// (the current tab). It changes which colors the fill states use, not
// which state wins.
type stateFlags struct {
	disabled bool
	selected bool
	pressed  bool
	focused  bool
	hovered  bool
}

// pick returns the fill and border for one interaction state.
//
// The two channels take separate rules, and that is the design, not an
// oversight:
//
//	fill:   disabled > pressed > hovered > focused > base
//	border: disabled > focused > base
//
// A disabled fill is Disabled when set, else Base. A selected fill
// keeps the same order but starts from Selected: pressed and hovered
// are Selected moved one OKLCH lightness step down and up, and
// focused and resting are Selected itself. The border rule does not
// change with selection, so a selected element still shows the focus
// border.
//
// The fill follows the pointer. A mouse user clicks a control, the
// click focuses it, and the pointer is still over it: under a
// focus-first fill the control would hold its focus color until the
// pointer left, which reads as stuck.
//
// The border does not follow the pointer, because the border is how a
// focused control says so. A single hover-first rule over both channels
// would drop the focus border exactly while someone is pointing at the
// control, and for Input the border IS the focus affordance
// (gui/view_input.go). Most toolkits split the same way: Fluent draws
// its PointerOver fill and its focus rect at once, and the web design
// systems pair a hover background with a focus ring. Material ranks
// focus above hover instead, but it has to — its state layer is one
// channel and cannot show both.
//
// Note what this makes of Focus: under a hover-first fill it is the
// tint of a control that is focused with the pointer somewhere else,
// which in practice means one reached by the keyboard. That is a real
// state worth painting, but a rare one. See the field's own comment.
//
// cs must already have been through resolved(): pick does no fallback
// of its own, so an unresolved set returns the zero Color for any state
// the caller never set. Every widget resolves in its apply*Defaults.
//
// There is deliberately no "did anything change" return. Every caller
// assigns unconditionally, which is the whole point — a reported change
// would put back the per-site if that produced the bug history above.
func (cs ColorSet) pick(s stateFlags) (fill, border Color) {
	if s.disabled {
		if cs.Disabled.IsSet() {
			return cs.Disabled, cs.Border
		}
		return cs.Base, cs.Border
	}
	border = cs.Border
	if s.focused {
		border = cs.BorderFocus
	}
	if s.selected && cs.Selected.IsSet() {
		// Derived here, not stored by resolved: the shift is a few
		// float operations with no allocation, it runs only while a
		// selected element is hovered or pressed, and storing it would
		// add hidden fields that make a resolved set differ from the
		// same set built by hand.
		switch {
		case s.pressed:
			return accentShift(cs.Selected, -oklchRampDelta), border
		case s.hovered:
			return accentShift(cs.Selected, oklchRampDelta), border
		}
		return cs.Selected, border
	}
	switch {
	case s.pressed:
		return cs.Click, border
	case s.hovered:
		return cs.Hover, border
	case s.focused:
		return cs.Focus, border
	}
	return cs.Base, border
}

// PickState is the state of one element that ColorSet.Pick chooses
// colors for. It is the exported form of the rule the widgets in gui/
// use, so a widget outside gui/ (the data grid, a sibling repo) gets
// the same order and does not write its own (#741).
//
// The fields are flat and not an embedded InteractionState. A caller in
// an OnHover callback knows only Hovered, and a literal with embedded
// fields cannot name them directly. A caller that has an
// InteractionState copies the fields it needs.
//
// Pressed is the pressed look. A push button that must release when the
// pointer is dragged off sets it from InteractionState.Armed, not from
// InteractionState.Pressed.
type PickState struct {
	// Disabled wins over every other state. See ColorSet.Disabled.
	Disabled bool
	// Selected is the element's own model (the current row, the current
	// tab), not an interaction state. It changes which colors the fill
	// uses, not which state wins.
	Selected bool
	Pressed  bool
	Focused  bool
	Hovered  bool
}

// Pick returns the fill and border for one state. The order is the one
// every widget in gui/ uses:
//
//	fill:   disabled > pressed > hovered > focused > base
//	border: disabled > focused > base
//
// A selected fill starts from Selected: hover and press are Selected
// moved one OKLCH lightness step up and down.
//
// cs must come from Resolved. Pick does no fallback of its own, so a
// set that was not resolved returns the zero Color for a state the
// caller did not set.
func (cs ColorSet) Pick(s PickState) (fill, border Color) {
	return cs.pick(stateFlags{
		disabled: s.Disabled,
		selected: s.Selected,
		pressed:  s.Pressed,
		focused:  s.Focused,
		hovered:  s.Hovered,
	})
}

// Resolved returns the set with its own fallbacks applied first (Hover,
// Click and Focus take Base; BorderFocus takes Border), then theme for
// every slot still unset. It is the step a widget outside gui/ runs in
// its defaults pass before it calls Pick.
func (cs ColorSet) Resolved(theme ColorSet) ColorSet {
	return cs.resolved(Color{}, theme)
}

// setIfUnset assigns src to *dst only when dst holds no explicit color
// and src has one.
func setIfUnset(dst *Color, src Color) {
	if !dst.IsSet() && src.IsSet() {
		*dst = src
	}
}
