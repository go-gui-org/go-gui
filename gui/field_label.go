package gui

// Field labels.
//
// Eight form controls — Input, Select, Combobox, Slider, NumericInput,
// DatePicker, InputDate and ColorPicker — had no Label field at all
// (issue #335, audit section 3). An app that wanted a labelled form
// built the label itself, which meant every app invented its own form
// layout, and the three widgets that did carry a label (Switch, Toggle,
// Radio) each spelled it differently.
//
// This file holds the one convention, so a widget gets a label by
// asking for one rather than by re-deciding where labels go.
//
// The convention: above the field, left-aligned, in the theme's label
// role, separated by the small spacing tier.
//
//   - Above, not beside. A label beside its field makes the field's
//     left edge depend on the label's width, so a column of fields
//     cannot line up without the caller measuring text.
//   - Left, not centred. Centred reads as a heading over a narrow
//     column — right for ColorFields' channel stack, wrong for a
//     full-width field, where it detaches the name from the value.
//     labelledField takes the alignment so both are reachable; left is
//     the default because full-width is the common case.
//   - The label role, not a size or alpha chosen here. That is the
//     whole point of gui/theme_text_roles.go.

// labelledField stacks label above field.
//
// An empty label returns field untouched — no wrapper, no extra shape —
// so a Cfg that never sets Label renders exactly as it did before. That
// is what makes adding the field additive rather than a visual break.
//
// The label carries no ID. It is not focusable, not scrollable and has
// no OnMouseLeave, so it never joins the effective-ID space and cannot
// collide with anything (see the ID rules in CLAUDE.md).
//
// base supplies the label's family and color; only the de-emphasis and
// the size step come from the theme's label role. A caller that themed
// its field's text therefore keeps that color in the label naming it. A
// zero base takes the theme's default text style, which is what the two
// controls with no TextStyle field of their own (Slider, ColorPicker)
// pass.
func labelledField(
	label string, base TextStyle, align HorizontalAlign,
	sizing Sizing, field View,
) View {
	if label == "" {
		return field
	}
	if base == (TextStyle{}) {
		base = guiTheme.TextStyleDef
	}
	return Column(ContainerCfg{
		Padding: NoPadding,
		// Structural wrapper: an inherited border would both paint and
		// reserve space, which is the defect that made Input arrange
		// 3px taller than Select (audit section 7.1).
		SizeBorder: NoBorder,
		Sizing:     fitHeight(sizing),
		HAlign:     align,
		Spacing:    SomeF(guiTheme.SpacingSmall),
		Content: []View{
			Text(TextCfg{
				Text:      label,
				TextStyle: fieldLabelStyle(base),
			}),
			field,
		},
	})
}

// fitHeight keeps the caller's width mode and pins height to Fit.
//
// The wrapper must take the field's width -- a FillFit Input inside a
// labelled wrapper was capped by a hardcoded FitFit column, so no
// labelled field in any app on this toolkit could fill its row. The
// height stays Fit so the label/field stack still hugs its content.
//
// An unset sizing returns FitFit, which is the wrapper a Cfg that never
// set Sizing got before, so adding the pass-through is not itself a
// visual break.
//
// Built from the predefined vars, never a raw Sizing{...} literal:
// Sizing self-flags via an unexported set field, so a literal reads as
// unset (see gui/sizing.go).
func fitHeight(s Sizing) Sizing {
	if !s.IsSet() {
		return FitFit
	}
	switch s.Width {
	case sizingFill:
		return FillFit
	case sizingFixed:
		return FixedFit
	default:
		return FitFit
	}
}

// fieldLabelStyle is the label role applied to a base text style.
//
// Only the de-emphasis and the size step come from the role; the base's
// own color survives, so a caller that themed its field's text keeps
// that color in the label naming it.
func fieldLabelStyle(base TextStyle) TextStyle {
	role := guiTheme.TextStyleLabel
	ts := withRoleAlpha(base, role)
	ts.Size = role.Size
	return ts
}

// trailingLabel is the label a boolean control puts beside itself —
// Switch, Toggle and Radio.
//
// Distinct from labelledField on purpose. A field's label names a value
// the user will type or choose, and goes above so a column of fields
// lines up. A boolean control's label *is* the question, and reads
// beside the switch that answers it.
//
// It was spelled three ways: Switch emitted a bare Text, Toggle a bare
// Text in a separate themed style, and Radio a Text inside a padded Row
// — so only Radio left a gap between the control and its name, and the
// other two read cramped (issue #335, audit section 3). The gap is the
// better default, so all three take Radio's.
func trailingLabel(label string, style TextStyle) View {
	return Row(ContainerCfg{
		Padding:    NewPadding(0, PadXSmall, 0, 0),
		SizeBorder: NoBorder,
		Content: []View{
			Text(TextCfg{Text: label, TextStyle: style}),
		},
	})
}
