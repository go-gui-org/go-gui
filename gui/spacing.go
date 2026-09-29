package gui

// Spacing is the gap a Cfg puts between its children. It is one of
// three kinds:
//
//   - unset (the zero value): the widget's theme default applies.
//   - a role (SpacingTight, SpacingSmall, SpacingMedium, SpacingLarge):
//     the gap is read from the active theme when the widget is built,
//     so a ThemeCfg that changes a step moves every call site that
//     names it (issue #866).
//   - a fixed number of pixels (SpacingPx, NoSpacing): the gap stays
//     the same under every theme. Use it only for geometry that must
//     not move with the theme, such as a 1px hairline.
//
// Spacing self-flags like Padding: the unexported fields tell "not
// set" from "set to zero", so a raw Spacing{} literal reads as unset.
// Read it with Or.
//
// The value is three words and holds no pointer, so building one
// never allocates.
//
// exportaudit:keep — the type of every Cfg spacing field; callers name
// its values (SpacingMedium, SpacingPx) more often than the type.
type Spacing struct {
	px   float32
	role spacingRole
	set  bool
}

// spacingRole names a step of the theme's gap ladder. The zero value
// means "no role": the Spacing holds a fixed px value.
type spacingRole uint8

const (
	spacingRoleNone spacingRole = iota
	spacingRoleTight
	spacingRoleSmall
	spacingRoleMedium
	spacingRoleLarge
)

// Seed values of the gap ladder. Each step names a gap between
// things, and the steps differ in how closely related the things are
// (audit §4, issue #344):
//
//	gapTight   (2)  — inside one composite control, between parts
//	                   that read as a single unit: calendar cells,
//	                   the tab strip, submenu items.
//	gapSmall   (6)  — members of one visual group that share a
//	                   container: a control and its readout, the
//	                   ColorFields channel row.
//	gapMedium (14)  — sibling controls in a stack or row: dialog
//	                   rows, the toasts in a stack.
//	gapLarge  (28)  — unrelated sections of a surface: distinct
//	                   groups, panels stacked on one page.
//
// They seed ThemeCfg.Spacing*. Code reads the live step from the
// theme (or through a Spacing role), never these seeds, so a custom
// theme is not bypassed. Theme.PaddingField and the padding steps
// answer a different question — how much inset a control puts around
// its own content — and are not steps of this ladder.
const (
	gapTight  float32 = 2
	gapSmall  float32 = 6
	gapMedium float32 = 14
	gapLarge  float32 = 28
)

// Spacing roles. Each one resolves to the matching Theme.Spacing*
// step of the theme that is active when the widget is built.
var (
	// SpacingTight is the gap between the parts of one composite
	// control. It follows Theme.SpacingTight.
	SpacingTight = Spacing{role: spacingRoleTight, set: true}
	// SpacingSmall is the gap between members of one visual group. It
	// follows Theme.SpacingSmall.
	SpacingSmall = Spacing{role: spacingRoleSmall, set: true}
	// SpacingMedium is the gap between sibling controls. It follows
	// Theme.SpacingMedium.
	SpacingMedium = Spacing{role: spacingRoleMedium, set: true}
	// SpacingLarge is the gap between unrelated sections. It follows
	// Theme.SpacingLarge.
	SpacingLarge = Spacing{role: spacingRoleLarge, set: true}

	// NoSpacing is an explicit zero gap. It is set, so it does NOT
	// fall through to the theme default like Spacing{} (unset) does.
	NoSpacing = Spacing{set: true}
)

// SpacingPx returns a fixed gap of px logical pixels. It does not
// follow the theme. Prefer a role; ergonomics-audit -mode spacing
// flags a literal px > 0.
func SpacingPx(px float32) Spacing {
	return Spacing{px: px, set: true}
}

// IsSet reports whether the spacing was set (a role, SpacingPx or
// NoSpacing) as opposed to being the zero value.
func (s Spacing) IsSet() bool { return s.set }

// Or returns the gap in logical pixels: the role's step from the
// active theme, the fixed px value, or def when s is unset.
//
// Widgets call Or while they build their layout, inside the frame
// pass of the window being generated. At that point guiTheme holds
// that window's theme (see theme_install.go), so a role resolves
// against the right theme with no lock and no copy.
func (s Spacing) Or(def float32) float32 {
	if !s.set {
		return def
	}
	return s.resolve(&guiTheme)
}

// resolve returns the gap of a set Spacing against theme t. A role
// reads t's step; any other value is its fixed px.
func (s Spacing) resolve(t *Theme) float32 {
	switch s.role {
	case spacingRoleTight:
		return t.SpacingTight
	case spacingRoleSmall:
		return t.SpacingSmall
	case spacingRoleMedium:
		return t.SpacingMedium
	case spacingRoleLarge:
		return t.SpacingLarge
	default:
		return s.px
	}
}
