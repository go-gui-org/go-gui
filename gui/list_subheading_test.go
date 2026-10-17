package gui

import "testing"

// A list subheading names a group of rows, so it must read apart from
// them. The style guide records list subheadings as staying body
// weight, with the hierarchy from color: the default is the secondary
// role, not the row text. Its rule is a divider, so it takes the
// theme's separator color, not the text color, which drew a heavy
// full-contrast line across the list.
func TestListSubheadingDefaults(t *testing.T) {
	restore := CurrentTheme()
	defer applyTheme(&restore)

	for _, name := range ThemeRegisteredNames() {
		theme, ok := ThemeGet(name)
		if !ok {
			t.Fatalf("theme %q registered but not retrievable", name)
		}
		applyTheme(&theme)
		sep := defaultSeparatorStyle.Colors.Base

		for _, c := range []struct {
			widget string
			style  TextStyle
		}{
			{"listbox", theme.listBoxStyle.subheadingStyle},
			{"select", theme.selectStyle.subheadingStyle},
		} {
			if c.style.Color != theme.TextStyleSecondary.Color {
				t.Errorf("%s %s subheading color %v, want secondary %v",
					name, c.widget, c.style.Color, theme.TextStyleSecondary.Color)
			}
		}

		cfg := ListBoxCfg{}
		applyListBoxDefaults(&cfg)
		v := listBoxItemContent(ListBoxOption{Name: "Group", isSubheading: true}, cfg)
		ly := generateViewLayout(v, &Window{})
		if len(ly.Children) != 2 {
			t.Fatalf("%s: subheading children = %d, want text and rule", name, len(ly.Children))
		}
		if got := subheadingRuleColor(&ly.Children[1]); got != sep {
			t.Errorf("%s listbox: subheading rule %v, want separator %v", name, got, sep)
		}
	}
}

// subheadingRuleColor returns the fill of the first colored shape in
// a Separator's subtree (the rule itself).
func subheadingRuleColor(ly *Layout) Color {
	if ly.Shape != nil && ly.Shape.Color.A != 0 {
		return ly.Shape.Color
	}
	for i := range ly.Children {
		if c := subheadingRuleColor(&ly.Children[i]); c.A != 0 {
			return c
		}
	}
	return Color{}
}

// A recolor with WithColors moves the secondary role, so the default
// subheading, which sits on that role, must move with it. A stale
// subheading would keep the old page's gray on the new background.
func TestWithColorsSubheadingFollowsSecondary(t *testing.T) {
	theme := ThemeLight.WithColors(ColorOverrides{
		ColorBackground: RGB(20, 24, 32),
		ColorPanel:      RGB(30, 34, 44),
	})
	if theme.TextStyleSecondary.Color == ThemeLight.TextStyleSecondary.Color {
		t.Fatal("recolor left the secondary role unchanged; test proves nothing")
	}
	for _, c := range []struct {
		widget string
		style  TextStyle
	}{
		{"listbox", theme.listBoxStyle.subheadingStyle},
		{"select", theme.selectStyle.subheadingStyle},
	} {
		if c.style != theme.TextStyleSecondary {
			t.Errorf("%s subheading %v, want secondary %v",
				c.widget, c.style.Color, theme.TextStyleSecondary.Color)
		}
	}
}
