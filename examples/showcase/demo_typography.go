package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
)

func demoTypography(_ *gui.Window) gui.View {
	t := gui.CurrentTheme()
	content := []gui.View{
		gui.Text(gui.TextCfg{
			ID:        "typo-intro",
			Text:      "Every text style comes from the theme. Pick the role for the job the text does.",
			TextStyle: t.TextStyleCaption,
			Mode:      gui.TextModeWrap,
		}),
		textDemoCard("typo-scale", "Scale", 0, typographyScaleSection(t)),
		textDemoCard("typo-roles", "De-emphasis roles", 0, []gui.View{
			gui.Text(gui.TextCfg{Text: "Secondary supports primary text beside it.", TextStyle: t.TextStyleSecondary, Mode: gui.TextModeWrap}),
			gui.Text(gui.TextCfg{Text: "Label names a nearby value.", TextStyle: t.TextStyleLabel, Mode: gui.TextModeWrap}),
			gui.Text(gui.TextCfg{Text: "Disabled sits in a control the user cannot use.", TextStyle: t.TextStyleDisabled, Mode: gui.TextModeWrap}),
			gui.Text(gui.TextCfg{Text: "Placeholder stands in for a value not yet entered.", TextStyle: t.TextStylePlaceholder, Mode: gui.TextModeWrap}),
		}),
		textDemoCard("typo-modifiers", "Modifiers", 0, []gui.View{
			gui.Text(gui.TextCfg{Text: "Bold keeps size and color.", TextStyle: t.TextStyleBody.Bold()}),
			gui.Text(gui.TextCfg{Text: "Italic keeps size and color.", TextStyle: t.TextStyleBody.Italic()}),
			gui.Text(gui.TextCfg{Text: "Regular drops bold and italic.", TextStyle: t.TextStyleTitle.Regular()}),
			gui.Text(gui.TextCfg{Text: "Mono moves to the mono family.", TextStyle: t.Mono(t.TextStyleBody)}),
		}),
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.Some(t.SpacingSmall),
		Padding: gui.NoPadding,
		Content: content,
	})
}

func typographyScaleSection(t gui.Theme) []gui.View {
	return append(typographyScaleSpecimens(t), typographyScaleExtraRows(t)...)
}

func typographyScaleExtraRows(t gui.Theme) []gui.View {
	roles := []struct {
		name  string
		style gui.TextStyle
	}{
		{"TitleSmall", t.TextStyleTitleSmall},
		{"BodyLarge", t.TextStyleBodyLarge},
		{"BodySmall", t.TextStyleBodySmall},
		{"CaptionSmall", t.TextStyleCaptionSmall},
		{"CodeSmall", t.TextStyleCodeSmall},
		{"CodeTiny", t.TextStyleCodeTiny},
	}
	rows := make([]gui.View, 0, len(roles))
	for i, role := range roles {
		roleName := role.name
		roleStyle := role.style
		rows = append(rows, gui.Row(gui.ContainerCfg{
			ID:         gui.ScopeIDN("typo-scale", "row", i),
			Sizing:     gui.FillFit,
			Spacing:    gui.Some(t.SpacingMedium),
			Padding:    gui.NoPadding,
			VAlign:     gui.VAlignMiddle,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      fmt.Sprintf("%s (%.0fpt)", roleName, roleStyle.Size),
					TextStyle: t.TextStyleCaption,
				}),
				gui.Text(gui.TextCfg{
					Text:      "The quick brown fox",
					TextStyle: roleStyle,
				}),
			},
		}))
	}
	return rows
}

// doc:snippet-begin typography-scale
//
// The block below is quoted verbatim by docs/typography.md and by
// examples/showcase/docs/widget_typography.md. The snippet test
// fails when the guides drift, so keep this region self-contained:
// anything a reader would have to go looking for belongs above the
// marker, not inside it.

func typographyScaleSpecimens(t gui.Theme) []gui.View {
	return []gui.View{
		gui.Text(gui.TextCfg{Text: "Display", TextStyle: t.TextStyleDisplay}),
		gui.Text(gui.TextCfg{Text: "Title", TextStyle: t.TextStyleTitle}),
		gui.Text(gui.TextCfg{Text: "Body", TextStyle: t.TextStyleBody}),
		gui.Text(gui.TextCfg{Text: "Caption", TextStyle: t.TextStyleCaption}),
		gui.Text(gui.TextCfg{Text: "Code", TextStyle: t.TextStyleCode}),
	}
}

// doc:snippet-end typography-scale
