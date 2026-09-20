package datagrid

import (
	gg "github.com/go-gui-org/go-gui/gui"
)

// dataGridResolveSet fills unset slots of cs from the theme set.
// datagrid lives outside gui/ and cannot call ColorSet.resolved,
// so the merge is spelled here, slot by slot (issue #720).
// The caller's own fallbacks come first — Base backs the three
// interactive states and Border backs BorderFocus, mirroring
// ColorSet.resolve — then the theme for anything still unspecified.
func dataGridResolveSet(cs, theme gg.ColorSet) gg.ColorSet {
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
	if !cs.Base.IsSet() {
		cs.Base = theme.Base
	}
	if !cs.Hover.IsSet() {
		cs.Hover = theme.Hover
	}
	if !cs.Click.IsSet() {
		cs.Click = theme.Click
	}
	if !cs.Focus.IsSet() {
		cs.Focus = theme.Focus
	}
	if !cs.Border.IsSet() {
		cs.Border = theme.Border
	}
	if !cs.BorderFocus.IsSet() {
		cs.BorderFocus = theme.BorderFocus
	}
	return cs
}
