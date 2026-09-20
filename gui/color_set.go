package gui

// ColorSet groups the per-state colors of an interactive widget into
// one value, so a caller who wants a single consistent appearance says
// it once instead of assigning five or six flat Color* fields.
//
// Seventeen widgets carry it: Button, Switch, Toggle, Radio, DatePicker,
// InputDate (flat state fields deleted), and Input, NumericInput, Select,
// Combobox, ListBox, Tree, Slider, ContextMenu, Menubar, Table,
// ExpandPanel (flat fields retained and winning over the set; see
// applyTo).
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
}

// Flat returns a ColorSet whose every field is c, including the two
// border fields. This is the "do not react" case: a widget that keeps
// one appearance through hover, press and focus.
//
// Flat is not the same as ColorSet{Base: c}. Base only backs the three
// interactive states; Flat also pins the borders, which is what makes
// the widget visually inert rather than merely uniform in its fill.
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
		cs.Focus.IsSet() || cs.Border.IsSet() || cs.BorderFocus.IsSet()
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
// Colors.Base for the same migration reason applyTo encodes — code
// that set Color keeps its appearance when a ColorSet arrives.
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
	return cs
}

// applyTo fills each of the six flat Color* destinations from the set,
// but only where the destination is still unset.
//
// That ordering is the precedence rule, and it is deliberately the
// unintuitive direction: a flat field that the caller assigned wins
// over the ColorSet. The reason is migration safety. Existing code
// assigns flat fields; when a ColorSet arrives — from a preset, a
// shared style value, or a half-finished edit — that code must keep
// the appearance it has today rather than silently changing color.
// The newer, more specific-looking API is the one that yields.
//
// Fields left unset by both are untouched, so the caller's theme
// defaults still apply afterwards.
// Written as straight-line assignments rather than a table because
// this runs once per styled widget per frame, and the table form puts
// a composite literal on the view path for no readability gain.
func (cs ColorSet) applyTo(
	base, hover, click, focus, border, borderFocus *Color,
) {
	r := cs.resolve()
	setIfUnset(base, r.Base)
	setIfUnset(hover, r.Hover)
	setIfUnset(click, r.Click)
	setIfUnset(focus, r.Focus)
	setIfUnset(border, r.Border)
	setIfUnset(borderFocus, r.BorderFocus)
}

// stateFlags is the interaction state of one widget at one point in one
// frame. The caller assembles it from what its pass can see: the amend
// pass knows focus and a held Space but has no Event and so no pointer;
// the hover pass knows the pointer and the held mouse button as well.
//
// disabled makes the rule total: pick answers for every state rather
// than leaving one to the caller. It is not what short-circuits a
// disabled widget today. Every current call site still returns early
// on Shape.Disabled, because that same guard also covers a shape with
// no events and a nil OnClick, and on the amend path it is the only
// thing stopping a disabled widget's user OnAmend callback — layoutAmend
// walks disabled shapes, unlike layoutHoverDepth.
//
// So the flag reaches pick false at all eight sites, and that is fine:
// it is what lets a widget with no guard of its own be routed through
// pick without inventing one. Several have no such guard — tree rows,
// slider, expand panel, listbox items, table rows (#721).
type stateFlags struct {
	disabled bool
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
		return cs.Base, cs.Border
	}
	border = cs.Border
	if s.focused {
		border = cs.BorderFocus
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

// setIfUnset assigns src to *dst only when dst holds no explicit color
// and src has one. Nil dst is tolerated so a widget can pass nil for a
// state it does not have.
func setIfUnset(dst *Color, src Color) {
	if dst != nil && !dst.IsSet() && src.IsSet() {
		*dst = src
	}
}
