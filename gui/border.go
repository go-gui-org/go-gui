package gui

// Border is the border width a Cfg asks for. It is one of three
// kinds:
//
//   - unset (the zero value): the widget's theme default applies.
//   - BorderThin: the active theme's SizeBorder, read when the widget
//     is built. Theme.WithBorders(false) sets that to 0, so a
//     BorderThin border goes away with every themed border
//     (issue #867).
//   - a fixed number of pixels (BorderPx, NoBorder): the width stays
//     the same under every theme. Use it for a stroke that is part of
//     the widget's meaning, such as a focus or selection ring, not
//     for an ordinary outline.
//
// Border self-flags like Spacing: the unexported fields tell "not
// set" from "set to zero", so a raw Border{} literal reads as unset.
// Read it with Or.
//
// exportaudit:keep — the type of every Cfg border-width field; callers
// name its values (NoBorder, BorderThin) more often than the type.
type Border struct {
	px   float32
	thin bool
	set  bool
}

var (
	// BorderThin is the theme's border width. It follows
	// Theme.SizeBorder, so it is 0 under WithBorders(false).
	BorderThin = Border{thin: true, set: true}

	// NoBorder is an explicit zero width. A structural wrapper sets it
	// so it never draws the theme border. It is set, so it does NOT
	// fall through to the theme default like Border{} (unset) does.
	NoBorder = Border{set: true}
)

// BorderPx returns a fixed border width of px logical pixels. It does
// not follow the theme. Prefer BorderThin; ergonomics-audit -mode
// spacing flags a literal px > 0.
func BorderPx(px float32) Border {
	return Border{px: px, set: true}
}

// IsSet reports whether the width was set (BorderThin, BorderPx or
// NoBorder) as opposed to being the zero value.
func (b Border) IsSet() bool { return b.set }

// Or returns the width in logical pixels: the theme's SizeBorder for
// BorderThin, the fixed px value, or def when b is unset. It reads
// guiTheme for the same reason Spacing.Or does.
func (b Border) Or(def float32) float32 {
	if !b.set {
		return def
	}
	return b.resolve(&guiTheme)
}

// resolve returns the width of a set Border against theme t.
func (b Border) resolve(t *Theme) float32 {
	if b.thin {
		return t.SizeBorder
	}
	return b.px
}
