package gui

import "math"

// Named text roles.
//
// De-emphasized text used to be spelled as a literal alpha at each call
// site: seven distinct values covering four semantic roles, in three
// different notations, with the same "disabled text" state rendering at
// 65, 127 and 130 depending on the widget (issue #335; measurements in
// docs/specs/widget-visual-consistency-audit.md).
//
// A role names the *reason* text is quiet. A widget asks for the role;
// the theme decides what that looks like. Adding a literal alpha at a
// call site is the thing this replaces — if none of the four roles fits,
// the answer is a fifth role, not a local number.
//
//	TextStyleSecondary   supporting text beside primary text
//	TextStyleLabel       text naming a nearby value; the value is read
//	TextStyleDisabled    text in a control the user cannot act on
//	TextStylePlaceholder text standing in for a value not yet entered
//
// Roles are per-theme values, not one multiplier applied everywhere. See
// textRolesFor for why that distinction is load-bearing.

// textRoleAlphas is one theme's de-emphasis ladder, as alpha applied to
// the theme's own text color.
type textRoleAlphas struct {
	placeholder uint8
	disabled    uint8
	secondary   uint8
	label       uint8
}

// The two ladders are contrast-matched, not equal.
//
// Alpha blends text toward the background, and that is not perceptually
// symmetric between a dark and a light ground. Recorded from the render
// pipeline, every de-emphasis alpha in the package today is byte
// identical across ThemeDark and ThemeLight — which means the light
// theme has been running at materially lower contrast for every quiet
// role. Worked example, disabled text at alpha 128:
//
//	dark   225 over 48   -> #898989 on #303030, contrast 3.77
//	light   32 over 225  -> #808080 on #e1e1e1, contrast 3.11
//
// textRolesOnLight is the dark ladder solved back through the sRGB
// luminance curve to land on the *same* contrast ratio against a light
// ground. So a role means the same amount of de-emphasis in either
// theme, which is what lets a widget name a role and stop thinking.
var (
	textRolesOnDark = textRoleAlphas{
		placeholder: 100,
		disabled:    128,
		secondary:   160,
		label:       178,
	}
	textRolesOnLight = textRoleAlphas{
		placeholder: 124,
		disabled:    149,
		secondary:   174,
		label:       186,
	}
)

// srgbLuminance returns the WCAG relative luminance of c, ignoring
// alpha. Used only to classify a theme's polarity, so the channel
// coefficients matter more than exactness.
func srgbLuminance(c Color) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// textRolesFor picks the ladder matching a theme's polarity: text
// lighter than its background is a dark theme, and the reverse is a
// light one.
//
// Deriving this rather than requiring every ThemeCfg to state it means a
// theme an app builds itself gets correct de-emphasis without knowing
// these roles exist. A theme that wants different values still sets the
// ColorText* fields explicitly, which win.
func textRolesFor(text, background Color) textRoleAlphas {
	if srgbLuminance(text) < srgbLuminance(background) {
		return textRolesOnLight
	}
	return textRolesOnDark
}

// themeTextRoles builds the four role styles for a theme.
//
// base is the theme's default text style; every role inherits its family
// and size from it, so a theme that shifts its type scale carries the
// roles along. Only TextStyleLabel steps size, down to the N5 rung —
// naming a value should not compete with reading it.
//
// An explicitly configured ColorText* wins over the derived ladder,
// which is what makes the roles per-theme values rather than a fixed
// multiplier.
func themeTextRoles(cfg ThemeCfg, base TextStyle, labelSize float32) (
	secondary, label, disabled, placeholder TextStyle,
) {
	roles := textRolesFor(base.Color, cfg.ColorBackground)

	// Unset Color reads as "not specified" (Color carries its own set
	// flag), so an explicit fully-transparent role stays honorable.
	roleColor := func(explicit Color, alpha uint8) Color {
		if explicit.IsSet() {
			return explicit
		}
		return RGBA(base.Color.R, base.Color.G, base.Color.B, alpha)
	}

	secondary = base
	secondary.Color = roleColor(cfg.ColorTextSecondary, roles.secondary)

	label = base
	label.Color = roleColor(cfg.ColorTextLabel, roles.label)
	if labelSize > 0 {
		label.Size = labelSize
	}

	disabled = base
	disabled.Color = roleColor(cfg.ColorTextDisabled, roles.disabled)
	// This style already says "disabled"; renderText must not halve
	// it a second time. See TextStyle.disabledRole.
	disabled.disabledRole = true

	placeholder = base
	placeholder.Color = roleColor(
		cfg.ColorTextPlaceholder, roles.placeholder)

	return secondary, label, disabled, placeholder
}

// inspectorStyleFor returns the inspector palette for a theme, with its
// help text taken from the secondary role.
//
// defaultInspectorStyle stays a ThemeMaker input holding real literals
// (see gui/styles.go), because the inspector's panel and wireframe
// colors are deliberately theme-independent — a debug overlay that
// restyled itself per theme would be harder to recognize, not easier.
// Its help text is not in that category: it is ordinary supporting text
// and was drifting at its own alpha (issue #335).
func inspectorStyleFor(secondary TextStyle) InspectorStyle {
	s := defaultInspectorStyle
	s.colorTextHelp = secondary.Color
	return s
}

// withRoleAlpha de-emphasizes base by as much as role does, keeping
// base's own color.
//
// Widgets whose text color is caller-supplied cannot just adopt a role
// style outright — that would silently discard the caller's color. What
// should be shared between widgets is the *amount* of de-emphasis, not
// the hue, so this takes the role's alpha and leaves everything else on
// base. base's own alpha is replaced, not multiplied: a role is an
// absolute statement about how quiet the text is.
func withRoleAlpha(base, role TextStyle) TextStyle {
	out := base
	out.Color = RGBA(
		base.Color.R, base.Color.G, base.Color.B, role.Color.A)
	// Carry the role's disabled marker: the result expresses the same
	// state the role does, so it must be exempt from the same second
	// dim. See TextStyle.disabledRole.
	out.disabledRole = role.disabledRole
	return out
}
