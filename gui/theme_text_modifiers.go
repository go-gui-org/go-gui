package gui

import "github.com/go-gui-org/go-glyph"

// Face modifiers for the text roles (issue #734). A role fixes face,
// size and color for one purpose; a modifier re-faces a role for the
// tail a fixed role set cannot cover: a donor style a caller resizes
// (2048 reads B1 only to override its size), markdown emphasis
// (italic at body-small), a mono run at an ad-hoc size.
//
// The TextStyle modifiers only retarget Typeface and never touch
// Color or any other field, so a donor keeps the caller's explicit
// size and color: that is what makes Display.Regular() with an
// overridden size spell exactly what the N1 donor pattern spelled.
// Each maps every face, so chaining is order-independent (Bold().
// Italic() and Italic().Bold() are both bold-italic) and repeating is
// idempotent.
//
// Mono is a method on Theme rather than TextStyle because only the
// theme knows the mono family (ThemeCfg.MonoFontFamily). It also
// applies the +1 optical compensation the M ladder bakes in; an
// overridden Size discards it, so a donor that states its size is not
// compensated twice.

// Bold returns s in the bold face, keeping an italic slant it has.
func (s TextStyle) Bold() TextStyle {
	switch s.Typeface {
	case glyph.TypefaceItalic, glyph.TypefaceBoldItalic:
		s.Typeface = glyph.TypefaceBoldItalic
	default:
		s.Typeface = glyph.TypefaceBold
	}
	return s
}

// Italic returns s in the italic face, keeping a bold weight it has.
func (s TextStyle) Italic() TextStyle {
	switch s.Typeface {
	case glyph.TypefaceBold, glyph.TypefaceBoldItalic:
		s.Typeface = glyph.TypefaceBoldItalic
	default:
		s.Typeface = glyph.TypefaceItalic
	}
	return s
}

// Regular returns s in the regular face, dropping any bold or italic.
// Display.Regular() is the N1 rung: extra-large regular.
func (s TextStyle) Regular() TextStyle {
	s.Typeface = glyph.TypefaceRegular
	return s
}

// Mono returns s in the theme's mono family, keeping face weight and
// color. It applies the mono optical size compensation (+1, the same
// step the M ladder bakes into every rung): mono faces draw optically
// smaller than regular ones at the same point size. Overriding Size
// afterwards discards the compensation, so a donor that states its
// size renders exactly that size. Prefer the Code roles where one
// fits; this method is the donor path for ad-hoc mono sizes.
func (t Theme) Mono(s TextStyle) TextStyle {
	s.Family = t.Cfg.MonoFontFamily
	s.Size++
	return s
}
