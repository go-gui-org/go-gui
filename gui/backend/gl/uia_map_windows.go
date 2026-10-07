//go:build windows && !js && (amd64 || arm64)

package gl

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// UI Automation's view of an A11yNode: control type, patterns and the
// tree links it walks. Pure data, so the mapping is testable without
// COM. Adapted from mygo's provider (github.com/egoist/mygo,
// internal/windows/surface_access.go, MIT); see THIRD_PARTY_NOTICES.

// UIA_*ControlTypeId values.
const (
	uiaCtlButton      int32 = 50000
	uiaCtlCheckBox    int32 = 50002
	uiaCtlComboBox    int32 = 50003
	uiaCtlEdit        int32 = 50004
	uiaCtlHyperlink   int32 = 50005
	uiaCtlImage       int32 = 50006
	uiaCtlListItem    int32 = 50007
	uiaCtlList        int32 = 50008
	uiaCtlMenu        int32 = 50009
	uiaCtlMenuBar     int32 = 50010
	uiaCtlMenuItem    int32 = 50011
	uiaCtlProgressBar int32 = 50012
	uiaCtlRadioButton int32 = 50013
	uiaCtlScrollBar   int32 = 50014
	uiaCtlSlider      int32 = 50015
	uiaCtlTab         int32 = 50018
	uiaCtlTabItem     int32 = 50019
	uiaCtlText        int32 = 50020
	uiaCtlToolBar     int32 = 50021
	uiaCtlTree        int32 = 50023
	uiaCtlTreeItem    int32 = 50024
	uiaCtlCustom      int32 = 50025
	uiaCtlGroup       int32 = 50026
	uiaCtlDataGrid    int32 = 50028
	uiaCtlPane        int32 = 50033
	uiaCtlSeparator   int32 = 50038
)

// The interfaces an element can hand out, by index into uiaIIDs and
// the element's interface table.
const (
	ifaceSimple = iota
	ifaceFragment
	ifaceRoot
	ifaceInvoke
	ifaceToggle
	ifaceSelectionItem
	ifaceSelection
	ifaceRangeValue
	ifaceValue
	ifaceExpandCollapse
	uiaIfaces
)

// uiaControlType maps a role to its UIA control type. A dialog is a
// pane that reports IsDialog, as mygo's; a heading is text with a
// localized type, as there is no heading control type.
func uiaControlType(r gui.AccessRole) int32 {
	switch r {
	case gui.AccessRoleButton, gui.AccessRoleColorWell,
		gui.AccessRoleDisclosure, gui.AccessRoleSwitchToggle:
		return uiaCtlButton
	case gui.AccessRoleCheckbox:
		return uiaCtlCheckBox
	case gui.AccessRoleComboBox:
		return uiaCtlComboBox
	case gui.AccessRoleDateField, gui.AccessRoleTextField,
		gui.AccessRoleTextArea:
		return uiaCtlEdit
	case gui.AccessRoleDialog, gui.AccessRoleScrollArea:
		return uiaCtlPane
	case gui.AccessRoleGrid:
		return uiaCtlDataGrid
	case gui.AccessRoleGridCell:
		return uiaCtlCustom
	case gui.AccessRoleGroup, gui.AccessRoleRadioGroup:
		return uiaCtlGroup
	case gui.AccessRoleHeading, gui.AccessRoleStaticText:
		return uiaCtlText
	case gui.AccessRoleImage:
		return uiaCtlImage
	case gui.AccessRoleLink:
		return uiaCtlHyperlink
	case gui.AccessRoleList:
		return uiaCtlList
	case gui.AccessRoleListItem:
		return uiaCtlListItem
	case gui.AccessRoleMenu:
		return uiaCtlMenu
	case gui.AccessRoleMenuBar:
		return uiaCtlMenuBar
	case gui.AccessRoleMenuItem:
		return uiaCtlMenuItem
	case gui.AccessRoleProgressBar:
		return uiaCtlProgressBar
	case gui.AccessRoleRadioButton:
		return uiaCtlRadioButton
	case gui.AccessRoleScrollBar:
		return uiaCtlScrollBar
	case gui.AccessRoleSlider:
		return uiaCtlSlider
	case gui.AccessRoleSplitter:
		return uiaCtlSeparator
	case gui.AccessRoleTab:
		return uiaCtlTab
	case gui.AccessRoleTabItem:
		return uiaCtlTabItem
	case gui.AccessRoleToolbar:
		return uiaCtlToolBar
	case gui.AccessRoleTree:
		return uiaCtlTree
	case gui.AccessRoleTreeItem:
		return uiaCtlTreeItem
	}
	return uiaCtlCustom
}

// uiaLocalizedType names a role whose control type alone would be
// read wrong or not at all. Empty leaves UIA's name for the type.
func uiaLocalizedType(r gui.AccessRole) string {
	switch r {
	case gui.AccessRoleDialog:
		return "dialog"
	case gui.AccessRoleHeading:
		return "heading"
	case gui.AccessRoleSwitchToggle:
		return "toggle switch"
	case gui.AccessRoleRadioGroup:
		return "radio group"
	case gui.AccessRoleGridCell:
		return "cell"
	case gui.AccessRoleColorWell:
		return "color well"
	case gui.AccessRoleDateField:
		return "date field"
	case gui.AccessRoleSplitter:
		return "splitter"
	}
	return ""
}

// uiaFocusable reports whether a role takes keyboard focus. A node
// holding focus is focusable whatever its role.
func uiaFocusable(r gui.AccessRole) bool {
	switch r {
	case gui.AccessRoleButton, gui.AccessRoleCheckbox,
		gui.AccessRoleColorWell, gui.AccessRoleComboBox,
		gui.AccessRoleDateField, gui.AccessRoleDisclosure,
		gui.AccessRoleLink, gui.AccessRoleList, gui.AccessRoleListItem,
		gui.AccessRoleMenuItem, gui.AccessRoleRadioButton,
		gui.AccessRoleSlider, gui.AccessRoleSwitchToggle,
		gui.AccessRoleTabItem, gui.AccessRoleTextField,
		gui.AccessRoleTextArea, gui.AccessRoleTree,
		gui.AccessRoleTreeItem:
		return true
	}
	return false
}

// uiaSupports reports whether a node implements a control pattern.
// The fragment interfaces are the caller's: every element has them.
func uiaSupports(r gui.AccessRole, iface int) bool {
	switch iface {
	case ifaceInvoke:
		return r == gui.AccessRoleButton || r == gui.AccessRoleLink ||
			r == gui.AccessRoleMenuItem || r == gui.AccessRoleColorWell
	case ifaceToggle:
		return r == gui.AccessRoleCheckbox || r == gui.AccessRoleSwitchToggle
	case ifaceSelectionItem:
		return r == gui.AccessRoleRadioButton || r == gui.AccessRoleTabItem ||
			r == gui.AccessRoleListItem || r == gui.AccessRoleTreeItem
	case ifaceSelection:
		return uiaSelectionContainer(r)
	case ifaceRangeValue:
		return r == gui.AccessRoleSlider || r == gui.AccessRoleProgressBar ||
			r == gui.AccessRoleScrollBar
	case ifaceValue:
		return r == gui.AccessRoleTextField || r == gui.AccessRoleTextArea ||
			r == gui.AccessRoleComboBox || r == gui.AccessRoleDateField ||
			r == gui.AccessRoleColorWell
	case ifaceExpandCollapse:
		return r == gui.AccessRoleComboBox || r == gui.AccessRoleDisclosure ||
			r == gui.AccessRoleTreeItem
	}
	return false
}

// uiaSelectionContainer reports whether a role holds the Selection its
// SelectionItem descendants belong to.
func uiaSelectionContainer(r gui.AccessRole) bool {
	return r == gui.AccessRoleList || r == gui.AccessRoleTree ||
		r == gui.AccessRoleTab || r == gui.AccessRoleRadioGroup
}

// uiaToggleState is ToggleState_On or ToggleState_Off.
func uiaToggleState(n *gui.A11yNode) int32 {
	if n.State.Has(gui.AccessStateChecked) {
		return 1
	}
	return 0
}

// uiaIsSelected reads a radio button's check as its selection.
func uiaIsSelected(n *gui.A11yNode) bool {
	return n.State.Has(gui.AccessStateSelected) ||
		n.State.Has(gui.AccessStateChecked)
}

// uiaExpandState is ExpandCollapseState_Expanded or _Collapsed.
func uiaExpandState(n *gui.A11yNode) int32 {
	if n.State.Has(gui.AccessStateExpanded) {
		return 1
	}
	return 0
}

// uiaRangeReadOnly reports whether a range refuses SetValue: a
// progress or scroll bar always, a slider when read-only or disabled.
func uiaRangeReadOnly(n *gui.A11yNode) bool {
	return n.Role != gui.AccessRoleSlider ||
		n.State.Has(gui.AccessStateReadOnly) ||
		n.State.Has(gui.AccessStateDisabled)
}

// uiaLinks holds the tree links of a flat node array. A11yNode gives
// each node its parent; ChildrenStart/ChildrenCount span all
// descendants, not direct children, so sibling order is rebuilt from
// ParentIdx. Index len(nodes) stands for the root, -1 for none.
type uiaLinks struct {
	first, last, next, prev []int32
}

// build fills the links for nodes, reusing the slices' capacity. A
// ParentIdx that does not name an earlier node (the walk is pre-order,
// so a parent always precedes its children) hangs the node off the
// root rather than trusting it.
func (l *uiaLinks) build(nodes []gui.A11yNode) {
	n := len(nodes)
	l.first = fillInt32(l.first, n+1)
	l.last = fillInt32(l.last, n+1)
	l.next = fillInt32(l.next, n)
	l.prev = fillInt32(l.prev, n)
	for i := range nodes {
		p := l.parent(nodes, i)
		if last := l.last[p]; last < 0 {
			l.first[p] = int32(i)
		} else {
			l.next[last] = int32(i)
			l.prev[i] = last
		}
		l.last[p] = int32(i)
	}
}

// parent returns node i's parent index, len(nodes) for the root.
func (l *uiaLinks) parent(nodes []gui.A11yNode, i int) int {
	if p := nodes[i].ParentIdx; p >= 0 && p < i {
		return p
	}
	return len(nodes)
}

// fillInt32 returns s resized to n and set to -1.
func fillInt32(s []int32, n int) []int32 {
	if cap(s) < n {
		s = make([]int32, n)
	}
	s = s[:n]
	for i := range s {
		s[i] = -1
	}
	return s
}

// uiaNamedByContent reports whether a role with no label takes its
// name from the text inside it, as a button whose label is a child
// text node does.
func uiaNamedByContent(r gui.AccessRole) bool {
	switch r {
	case gui.AccessRoleButton, gui.AccessRoleCheckbox,
		gui.AccessRoleDisclosure, gui.AccessRoleGridCell,
		gui.AccessRoleHeading, gui.AccessRoleLink,
		gui.AccessRoleListItem, gui.AccessRoleMenuItem,
		gui.AccessRoleRadioButton, gui.AccessRoleSwitchToggle,
		gui.AccessRoleTabItem, gui.AccessRoleTreeItem:
		return true
	}
	return false
}

// uiaNameFromContent joins the labels of the static text under node
// idx, for a role uiaNamedByContent admits. Without it Narrator reads
// such a button as "button" and nothing else. Descendants are the
// pre-order span ChildrenStart..+ChildrenCount, clamped to the array.
func uiaNameFromContent(nodes []gui.A11yNode, idx int) string {
	n := &nodes[idx]
	if !uiaNamedByContent(n.Role) {
		return ""
	}
	start := max(n.ChildrenStart, idx+1)
	end := min(start+max(n.ChildrenCount, 0), len(nodes))
	var b strings.Builder
	for i := start; i < end && b.Len() < maxUIAText; i++ {
		d := &nodes[i]
		if d.Role != gui.AccessRoleStaticText || d.Label == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(d.Label)
	}
	return b.String()
}
