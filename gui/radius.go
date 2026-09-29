package gui

// Radius is the corner radius a Cfg asks for. It is one of three
// kinds:
//
//   - unset (the zero value): the widget's theme default applies.
//   - a role (RadiusSmall, RadiusMedium, RadiusLarge): the radius is
//     read from the active theme's radius ladder when the widget is
//     built, so a platform theme or a ThemeCfg that changes a step
//     moves every call site that names it (issue #867).
//   - a fixed number of pixels (RadiusPx, NoRadius): the radius stays
//     the same under every theme. Use it for geometry that must not
//     move with the theme, such as a pill (RadiusPx(h/2)) or a circle.
//
// Radius self-flags like Spacing: the unexported fields tell "not
// set" from "set to zero", so a raw Radius{} literal reads as unset.
// Read it with Or.
//
// The value is three words and holds no pointer, so building one
// never allocates.
//
// exportaudit:keep — the type of every Cfg radius field; callers name
// its values (RadiusMedium, RadiusPx) more often than the type.
type Radius struct {
	px   float32
	role radiusRole
	set  bool
}

// radiusRole names a step of the theme's radius ladder. The zero value
// means "no role": the Radius holds a fixed px value.
type radiusRole uint8

const (
	radiusRoleNone radiusRole = iota
	radiusRoleSmall
	radiusRoleMedium
	radiusRoleLarge
)

// Radius roles. Each one resolves to the matching Theme.Radius* step
// of the theme that is active when the widget is built. The seeds are
// radiusSmall/Medium/Large (styles.go); platform themes set their own.
var (
	// RadiusSmall is the corner of a small control: a checkbox, a
	// chip, a menu item. It follows Theme.RadiusSmall.
	RadiusSmall = Radius{role: radiusRoleSmall, set: true}
	// RadiusMedium is the corner of a standard control: a button, an
	// input, a card. It follows Theme.RadiusMedium.
	RadiusMedium = Radius{role: radiusRoleMedium, set: true}
	// RadiusLarge is the corner of a floating surface: a dialog, a
	// panel, a popover. It follows Theme.RadiusLarge.
	RadiusLarge = Radius{role: radiusRoleLarge, set: true}

	// NoRadius is an explicit square corner. It is set, so it does NOT
	// fall through to the theme default like Radius{} (unset) does.
	NoRadius = Radius{set: true}
)

// RadiusPx returns a fixed radius of px logical pixels. It does not
// follow the theme. Prefer a role; ergonomics-audit -mode spacing
// flags a literal px > 0.
func RadiusPx(px float32) Radius {
	return Radius{px: px, set: true}
}

// IsSet reports whether the radius was set (a role, RadiusPx or
// NoRadius) as opposed to being the zero value.
func (r Radius) IsSet() bool { return r.set }

// Or returns the radius in logical pixels: the role's step from the
// active theme, the fixed px value, or def when r is unset.
//
// Widgets call Or while they build their layout, inside the frame
// pass of the window being generated, when guiTheme holds that
// window's theme (see Spacing.Or).
func (r Radius) Or(def float32) float32 {
	if !r.set {
		return def
	}
	return r.resolve(&guiTheme)
}

// resolve returns the radius of a set Radius against theme t. A role
// reads t's step; any other value is its fixed px.
func (r Radius) resolve(t *Theme) float32 {
	switch r.role {
	case radiusRoleSmall:
		return t.RadiusSmall
	case radiusRoleMedium:
		return t.RadiusMedium
	case radiusRoleLarge:
		return t.RadiusLarge
	default:
		return r.px
	}
}
