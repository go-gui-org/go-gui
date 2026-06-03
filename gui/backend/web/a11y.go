//go:build js && wasm

package web

import (
	"strconv"
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// a11yState holds the hidden ARIA DOM tree that mirrors the
// framework's A11yNode flat array for screen reader access.
type a11yState struct {
	doc        js.Value
	root       js.Value
	liveRegion js.Value
	elems      []js.Value
	actionCb   func(action, index int)
	clickCbs   []js.Func
}

// ariaRole maps AccessRole to ARIA role strings.
var ariaRole = [...]string{
	gui.AccessRoleNone:         "",
	gui.AccessRoleButton:       "button",
	gui.AccessRoleCheckbox:     "checkbox",
	gui.AccessRoleColorWell:    "textbox",
	gui.AccessRoleComboBox:     "combobox",
	gui.AccessRoleDateField:    "textbox",
	gui.AccessRoleDialog:       "dialog",
	gui.AccessRoleDisclosure:   "button",
	gui.AccessRoleGrid:         "grid",
	gui.AccessRoleGridCell:     "gridcell",
	gui.AccessRoleGroup:        "group",
	gui.AccessRoleHeading:      "heading",
	gui.AccessRoleImage:        "img",
	gui.AccessRoleLink:         "link",
	gui.AccessRoleList:         "list",
	gui.AccessRoleListItem:     "listitem",
	gui.AccessRoleMenu:         "menu",
	gui.AccessRoleMenuBar:      "menubar",
	gui.AccessRoleMenuItem:     "menuitem",
	gui.AccessRoleProgressBar:  "progressbar",
	gui.AccessRoleRadioButton:  "radio",
	gui.AccessRoleRadioGroup:   "radiogroup",
	gui.AccessRoleScrollArea:   "region",
	gui.AccessRoleScrollBar:    "scrollbar",
	gui.AccessRoleSlider:       "slider",
	gui.AccessRoleSplitter:     "separator",
	gui.AccessRoleStaticText:   "none",
	gui.AccessRoleSwitchToggle: "switch",
	gui.AccessRoleTab:          "tablist",
	gui.AccessRoleTabItem:      "tab",
	gui.AccessRoleTextField:    "textbox",
	gui.AccessRoleTextArea:     "textbox",
	gui.AccessRoleToolbar:      "toolbar",
	gui.AccessRoleTree:         "tree",
	gui.AccessRoleTreeItem:     "treeitem",
}

func (a *a11yState) init(doc js.Value, callback func(action, index int)) {
	a.doc = doc
	a.actionCb = callback

	// Visually-hidden root for the ARIA tree.
	root := doc.Call("createElement", "div")
	root.Call("setAttribute", "role", "application")
	root.Call("setAttribute", "aria-label", "go-gui")
	setScreenReaderOnly(root)
	a.root = root

	// Live region for announcements.
	live := doc.Call("createElement", "div")
	live.Call("setAttribute", "aria-live", "assertive")
	live.Call("setAttribute", "aria-atomic", "true")
	live.Call("setAttribute", "role", "status")
	setScreenReaderOnly(live)
	a.liveRegion = live

	doc.Get("body").Call("appendChild", root)
	doc.Get("body").Call("appendChild", live)
}

func (a *a11yState) sync(nodes []gui.A11yNode, count, focusedIdx int) {
	if !a.root.Truthy() {
		return
	}

	// Release previous click callbacks.
	for _, cb := range a.clickCbs {
		cb.Release()
	}
	a.clickCbs = a.clickCbs[:0]

	// Grow element pool as needed.
	for len(a.elems) < count {
		el := a.doc.Call("createElement", "div")
		el.Set("tabIndex", -1)
		a.elems = append(a.elems, el)
	}

	// Detach everything so the tree can be rebuilt.
	a.root.Set("innerHTML", "")
	for i := range count {
		a.elems[i].Set("innerHTML", "")
	}

	// Update each element and rebuild the DOM tree.
	for i := range count {
		node := &nodes[i]
		el := a.elems[i]

		// Role.
		role := ""
		if int(node.Role) < len(ariaRole) {
			role = ariaRole[node.Role]
		}
		if role != "" && role != "none" {
			el.Call("setAttribute", "role", role)
		} else {
			el.Call("removeAttribute", "role")
		}

		// ID for aria-activedescendant.
		el.Set("id", "a11y-"+itoa(i))

		// Label / description.
		setOrRemoveAttr(el, "aria-label", node.Label)
		setOrRemoveAttr(el, "aria-description", node.Description)

		// Numeric value (sliders, progress bars).
		if node.ValueMin != 0 || node.ValueMax != 0 ||
			node.ValueNum != 0 {
			el.Call("setAttribute", "aria-valuenow",
				strconv.FormatFloat(
					float64(node.ValueNum), 'g', -1, 32))
			el.Call("setAttribute", "aria-valuemin",
				strconv.FormatFloat(
					float64(node.ValueMin), 'g', -1, 32))
			el.Call("setAttribute", "aria-valuemax",
				strconv.FormatFloat(
					float64(node.ValueMax), 'g', -1, 32))
		} else {
			el.Call("removeAttribute", "aria-valuenow")
			el.Call("removeAttribute", "aria-valuemin")
			el.Call("removeAttribute", "aria-valuemax")
		}

		// State flags.
		setBoolAttr(el, "aria-expanded",
			node.State.Has(gui.AccessStateExpanded))
		setBoolAttr(el, "aria-selected",
			node.State.Has(gui.AccessStateSelected))
		setBoolAttr(el, "aria-checked",
			node.State.Has(gui.AccessStateChecked))
		setBoolAttr(el, "aria-required",
			node.State.Has(gui.AccessStateRequired))
		setBoolAttr(el, "aria-invalid",
			node.State.Has(gui.AccessStateInvalid))
		setBoolAttr(el, "aria-busy",
			node.State.Has(gui.AccessStateBusy))
		setBoolAttr(el, "aria-readonly",
			node.State.Has(gui.AccessStateReadOnly))
		setBoolAttr(el, "aria-modal",
			node.State.Has(gui.AccessStateModal))
		setBoolAttr(el, "aria-disabled",
			node.State.Has(gui.AccessStateDisabled))

		if node.State.Has(gui.AccessStateLive) {
			el.Call("setAttribute", "aria-live", "polite")
		} else {
			el.Call("removeAttribute", "aria-live")
		}

		// Click handler for actionable roles.
		if a.actionCb != nil && isActionable(node.Role) {
			idx := i
			cb := js.FuncOf(
				func(_ js.Value, _ []js.Value) any {
					a.actionCb(gui.A11yActionPress, idx)
					return nil
				})
			el.Call("addEventListener", "click", cb)
			a.clickCbs = append(a.clickCbs, cb)
		}

		// Attach to parent.
		if node.ParentIdx < 0 || node.ParentIdx >= count {
			a.root.Call("appendChild", el)
		} else {
			a.elems[node.ParentIdx].Call("appendChild", el)
		}
	}

	// Focus tracking.
	if focusedIdx >= 0 && focusedIdx < count {
		a.root.Call("setAttribute", "aria-activedescendant",
			"a11y-"+itoa(focusedIdx))
	} else {
		a.root.Call("removeAttribute", "aria-activedescendant")
	}
}

func (a *a11yState) destroy() {
	for _, cb := range a.clickCbs {
		cb.Release()
	}
	a.clickCbs = nil
	if a.root.Truthy() {
		a.root.Call("remove")
	}
	if a.liveRegion.Truthy() {
		a.liveRegion.Call("remove")
	}
}

func (a *a11yState) announce(text string) {
	if !a.liveRegion.Truthy() {
		return
	}
	// Clear first so repeated identical text still triggers
	// the aria-live change notification.
	a.liveRegion.Set("textContent", "")
	a.liveRegion.Set("textContent", text)
}

// setScreenReaderOnly applies CSS that hides an element
// visually while keeping it accessible to screen readers.
func setScreenReaderOnly(el js.Value) {
	st := el.Get("style")
	st.Set("position", "absolute")
	st.Set("width", "1px")
	st.Set("height", "1px")
	st.Set("overflow", "hidden")
	st.Set("clip", "rect(0,0,0,0)")
	st.Set("whiteSpace", "nowrap")
}

func setOrRemoveAttr(el js.Value, attr, value string) {
	if value != "" {
		el.Call("setAttribute", attr, value)
	} else {
		el.Call("removeAttribute", attr)
	}
}

func setBoolAttr(el js.Value, attr string, set bool) {
	if set {
		el.Call("setAttribute", attr, "true")
	} else {
		el.Call("removeAttribute", attr)
	}
}

func isActionable(role gui.AccessRole) bool {
	switch role {
	case gui.AccessRoleButton, gui.AccessRoleCheckbox,
		gui.AccessRoleLink, gui.AccessRoleMenuItem,
		gui.AccessRoleSwitchToggle, gui.AccessRoleTab,
		gui.AccessRoleTabItem, gui.AccessRoleRadioButton,
		gui.AccessRoleDisclosure:
		return true
	}
	return false
}
