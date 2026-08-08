package gui

// ColorSet groups the per-state colors of an interactive widget into
// one value, so a caller who wants a single consistent appearance says
// it once instead of assigning five or six flat Color* fields.
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

// themeColorSet packs a theme style's six flat colors into a ColorSet
// so it can be handed to resolved. Theme styles keep their flat fields:
// they are internal defaults, not caller-facing API, and each one holds
// a different mix of extra colors that no shared struct would fit.
func themeColorSet(base, hover, click, focus, border, borderFocus Color) ColorSet {
	return ColorSet{
		Base:        base,
		Hover:       hover,
		Click:       click,
		Focus:       focus,
		Border:      border,
		BorderFocus: borderFocus,
	}
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

// setIfUnset assigns src to *dst only when dst holds no explicit color
// and src has one. Nil dst is tolerated so a widget can pass nil for a
// state it does not have.
func setIfUnset(dst *Color, src Color) {
	if dst != nil && !dst.IsSet() && src.IsSet() {
		*dst = src
	}
}
