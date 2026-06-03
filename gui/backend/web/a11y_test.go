//go:build js && wasm

package web

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// --- ariaRole mapping ---

func TestAriaRoleCompleteness(t *testing.T) {
	// Every AccessRole up to AccessRoleTreeItem must have
	// a mapping entry in ariaRole.
	roles := []gui.AccessRole{
		gui.AccessRoleNone,
		gui.AccessRoleButton,
		gui.AccessRoleCheckbox,
		gui.AccessRoleColorWell,
		gui.AccessRoleComboBox,
		gui.AccessRoleDateField,
		gui.AccessRoleDialog,
		gui.AccessRoleDisclosure,
		gui.AccessRoleGrid,
		gui.AccessRoleGridCell,
		gui.AccessRoleGroup,
		gui.AccessRoleHeading,
		gui.AccessRoleImage,
		gui.AccessRoleLink,
		gui.AccessRoleList,
		gui.AccessRoleListItem,
		gui.AccessRoleMenu,
		gui.AccessRoleMenuBar,
		gui.AccessRoleMenuItem,
		gui.AccessRoleProgressBar,
		gui.AccessRoleRadioButton,
		gui.AccessRoleRadioGroup,
		gui.AccessRoleScrollArea,
		gui.AccessRoleScrollBar,
		gui.AccessRoleSlider,
		gui.AccessRoleSplitter,
		gui.AccessRoleStaticText,
		gui.AccessRoleSwitchToggle,
		gui.AccessRoleTab,
		gui.AccessRoleTabItem,
		gui.AccessRoleTextField,
		gui.AccessRoleTextArea,
		gui.AccessRoleToolbar,
		gui.AccessRoleTree,
		gui.AccessRoleTreeItem,
	}
	for _, r := range roles {
		if int(r) >= len(ariaRole) {
			t.Errorf("AccessRole %d out of ariaRole bounds (len %d)",
				r, len(ariaRole))
		}
	}
}

func TestAriaRoleNonEmptyForVisibleRoles(t *testing.T) {
	// Every role except None should map to a non-empty string.
	for i := 1; i < len(ariaRole); i++ {
		if ariaRole[i] == "" {
			t.Errorf("ariaRole[%d] is empty", i)
		}
	}
}

// --- isActionable ---

func TestIsActionable(t *testing.T) {
	actionable := []gui.AccessRole{
		gui.AccessRoleButton,
		gui.AccessRoleCheckbox,
		gui.AccessRoleLink,
		gui.AccessRoleMenuItem,
		gui.AccessRoleSwitchToggle,
		gui.AccessRoleTab,
		gui.AccessRoleTabItem,
		gui.AccessRoleRadioButton,
		gui.AccessRoleDisclosure,
	}
	for _, r := range actionable {
		if !isActionable(r) {
			t.Errorf("isActionable(%d) = false, want true", r)
		}
	}

	notActionable := []gui.AccessRole{
		gui.AccessRoleNone,
		gui.AccessRoleStaticText,
		gui.AccessRoleImage,
		gui.AccessRoleGroup,
		gui.AccessRoleGrid,
		gui.AccessRoleSlider,
		gui.AccessRoleProgressBar,
		gui.AccessRoleToolbar,
	}
	for _, r := range notActionable {
		if isActionable(r) {
			t.Errorf("isActionable(%d) = true, want false", r)
		}
	}
}

// --- setBoolAttr / setOrRemoveAttr ---
// These require DOM (js.Value elements). Tested via the
// integration path in a11yState.sync which needs a browser.
// Covered by manual verification.
