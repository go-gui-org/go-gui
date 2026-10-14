//go:build js && wasm

package web

import (
	"strconv"
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// a11yState holds the hidden ARIA DOM tree that mirrors the
// framework's A11yNode flat array for screen reader access.
//
// sync is incremental: each pooled element remembers what it last
// wrote, so an unchanged tree costs Go-side comparisons and no DOM
// calls, and the tree is re-linked only when its shape changes. A
// full rewrite every sync was ~1.4 ms per frame on an SVG-heavy page,
// paid as an ~8 ms burst every a11ySyncInterval.
type a11yState struct {
	doc        js.Value
	root       js.Value
	liveRegion js.Value
	elems      []a11yElem
	actionCb   func(action, index int)

	// linked is the node count the DOM tree was last built for, and
	// focused the index aria-activedescendant names (-1 = none).
	linked  int
	focused int
}

// a11yElem is one pooled element and the attributes it last wrote.
type a11yElem struct {
	el js.Value
	// click is created once per element; it depends only on the
	// index. listening says it is attached.
	click     js.Func
	listening bool

	written     bool // attributes below were applied at least once
	role        string
	label, desc string
	hasValue    bool
	vNow, vMin  float32
	vMax        float32
	state       gui.AccessState
	parent      int
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
	a.focused = -1

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

	// Grow the element pool as needed. The id names the index, so it
	// is set once.
	for len(a.elems) < count {
		el := a.doc.Call("createElement", "div")
		el.Set("tabIndex", -1)
		el.Set("id", "a11y-"+itoa(len(a.elems)))
		a.elems = append(a.elems, a11yElem{el: el, parent: -1})
	}

	// Children are appended in index order, which is the reading order.
	// A node already linked under an unchanged parent keeps its place;
	// a changed parent re-links the whole tree.
	relinkAll := false
	for i := range count {
		e := &a.elems[i]
		node := &nodes[i]
		parent := node.ParentIdx
		if parent < 0 || parent >= count {
			parent = -1
		}
		if i < a.linked && e.parent != parent {
			relinkAll = true
		}
		a.applyAttrs(i, e, node)
		e.parent = parent
	}

	// Detach what the previous tree used beyond count.
	for i := count; i < a.linked; i++ {
		a.elems[i].el.Call("remove")
	}
	// Append in index order: everything after a parent change, else
	// only the nodes new to the tree. New indices follow every linked
	// one, so appending keeps each parent's children in index order.
	from := min(a.linked, count)
	if relinkAll {
		from = 0
	}
	for i := from; i < count; i++ {
		e := &a.elems[i]
		if e.parent < 0 {
			a.root.Call("appendChild", e.el)
		} else {
			a.elems[e.parent].el.Call("appendChild", e.el)
		}
	}
	a.linked = count

	// Focus tracking.
	if focusedIdx < 0 || focusedIdx >= count {
		focusedIdx = -1
	}
	if focusedIdx != a.focused {
		if focusedIdx >= 0 {
			a.root.Call("setAttribute", "aria-activedescendant",
				"a11y-"+itoa(focusedIdx))
		} else {
			a.root.Call("removeAttribute", "aria-activedescendant")
		}
		a.focused = focusedIdx
	}
}

// applyAttrs writes the attributes of node that differ from what e
// last wrote, and attaches or detaches the click listener when
// actionability changes.
func (a *a11yState) applyAttrs(i int, e *a11yElem, node *gui.A11yNode) {
	el := e.el
	first := !e.written

	role := ""
	if int(node.Role) < len(ariaRole) {
		role = ariaRole[node.Role]
	}
	if role == "none" {
		role = ""
	}
	if first || role != e.role {
		setOrRemoveAttr(el, "role", role)
		e.role = role
	}

	if first || node.Label != e.label {
		setOrRemoveAttr(el, "aria-label", node.Label)
		e.label = node.Label
	}
	if first || node.Description != e.desc {
		setOrRemoveAttr(el, "aria-description", node.Description)
		e.desc = node.Description
	}

	// Numeric value (sliders, progress bars).
	hasValue := node.ValueMin != 0 || node.ValueMax != 0 || node.ValueNum != 0
	if first || hasValue != e.hasValue || (hasValue &&
		(node.ValueNum != e.vNow || node.ValueMin != e.vMin ||
			node.ValueMax != e.vMax)) {
		if hasValue {
			el.Call("setAttribute", "aria-valuenow",
				strconv.FormatFloat(float64(node.ValueNum), 'g', -1, 32))
			el.Call("setAttribute", "aria-valuemin",
				strconv.FormatFloat(float64(node.ValueMin), 'g', -1, 32))
			el.Call("setAttribute", "aria-valuemax",
				strconv.FormatFloat(float64(node.ValueMax), 'g', -1, 32))
		} else {
			el.Call("removeAttribute", "aria-valuenow")
			el.Call("removeAttribute", "aria-valuemin")
			el.Call("removeAttribute", "aria-valuemax")
		}
		e.hasValue = hasValue
		e.vNow, e.vMin, e.vMax = node.ValueNum, node.ValueMin, node.ValueMax
	}

	// State flags: only the bits that changed.
	if changed := node.State ^ e.state; first || changed != 0 {
		for _, f := range a11yStateAttrs {
			if first || changed.Has(f.state) {
				setBoolAttr(el, f.attr, node.State.Has(f.state))
			}
		}
		if first || changed.Has(gui.AccessStateLive) {
			if node.State.Has(gui.AccessStateLive) {
				el.Call("setAttribute", "aria-live", "polite")
			} else {
				el.Call("removeAttribute", "aria-live")
			}
		}
		e.state = node.State
	}

	// Click handler for actionable roles.
	want := a.actionCb != nil && isActionable(node.Role)
	if want != e.listening {
		if want {
			if e.click.IsUndefined() {
				idx := i
				e.click = js.FuncOf(func(_ js.Value, _ []js.Value) any {
					a.actionCb(gui.A11yActionPress, idx)
					return nil
				})
			}
			el.Call("addEventListener", "click", e.click)
		} else {
			el.Call("removeEventListener", "click", e.click)
		}
		e.listening = want
	}
	e.written = true
}

// a11yStateAttrs maps the boolean state flags to their ARIA attribute.
var a11yStateAttrs = [...]struct {
	state gui.AccessState
	attr  string
}{
	{gui.AccessStateExpanded, "aria-expanded"},
	{gui.AccessStateSelected, "aria-selected"},
	{gui.AccessStateChecked, "aria-checked"},
	{gui.AccessStateRequired, "aria-required"},
	{gui.AccessStateInvalid, "aria-invalid"},
	{gui.AccessStateBusy, "aria-busy"},
	{gui.AccessStateReadOnly, "aria-readonly"},
	{gui.AccessStateModal, "aria-modal"},
	{gui.AccessStateDisabled, "aria-disabled"},
}

func (a *a11yState) destroy() {
	for i := range a.elems {
		if e := &a.elems[i]; !e.click.IsUndefined() {
			if e.listening {
				e.el.Call("removeEventListener", "click", e.click)
			}
			e.click.Release()
		}
	}
	a.elems = nil
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
