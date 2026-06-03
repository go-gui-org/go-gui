//go:build linux

package atspi

import "github.com/go-gui-org/go-gui/gui"

// AT-SPI2 role constants.
const (
	roleInvalid     uint32 = 0
	rolePushButton  uint32 = 42
	roleCheckBox    uint32 = 7
	roleColorChsr   uint32 = 8
	roleComboBox    uint32 = 9
	roleDateEditor  uint32 = 15
	roleDialog      uint32 = 16
	roleToggleBtn   uint32 = 62
	rolePanel       uint32 = 38
	roleHeading     uint32 = 81
	roleImage       uint32 = 26
	roleLink        uint32 = 80
	roleList        uint32 = 30
	roleListItem    uint32 = 31
	roleMenu        uint32 = 32
	roleMenuBar     uint32 = 33
	roleMenuItem    uint32 = 34
	roleProgressBr  uint32 = 41
	roleRadioBtn    uint32 = 43
	roleRadioGroup  uint32 = 38 // panel
	roleScrollPane  uint32 = 47
	roleScrollBar   uint32 = 46
	roleSlider      uint32 = 48
	roleSeparator   uint32 = 49
	roleLabel       uint32 = 60
	roleSwitchTgl   uint32 = 62
	rolePageTab     uint32 = 36
	rolePageTabLst  uint32 = 37
	roleEntry       uint32 = 22
	roleTextArea    uint32 = 22
	roleToolbar     uint32 = 63
	roleTree        uint32 = 64
	roleTreeItem    uint32 = 65
	roleApplication uint32 = 75
	roleFrame       uint32 = 21
)

// atspiRole maps AccessRole to AT-SPI2 role uint32.
var atspiRole [35]uint32

// atspiRoleName maps AccessRole to AT-SPI2 role name strings
// used by screen readers.
var atspiRoleName [35]string

func init() {
	atspiRole[gui.AccessRoleNone] = roleInvalid
	atspiRole[gui.AccessRoleButton] = rolePushButton
	atspiRole[gui.AccessRoleCheckbox] = roleCheckBox
	atspiRole[gui.AccessRoleColorWell] = roleColorChsr
	atspiRole[gui.AccessRoleComboBox] = roleComboBox
	atspiRole[gui.AccessRoleDateField] = roleDateEditor
	atspiRole[gui.AccessRoleDialog] = roleDialog
	atspiRole[gui.AccessRoleDisclosure] = roleToggleBtn
	atspiRole[gui.AccessRoleGrid] = rolePanel
	atspiRole[gui.AccessRoleGridCell] = rolePanel
	atspiRole[gui.AccessRoleGroup] = rolePanel
	atspiRole[gui.AccessRoleHeading] = roleHeading
	atspiRole[gui.AccessRoleImage] = roleImage
	atspiRole[gui.AccessRoleLink] = roleLink
	atspiRole[gui.AccessRoleList] = roleList
	atspiRole[gui.AccessRoleListItem] = roleListItem
	atspiRole[gui.AccessRoleMenu] = roleMenu
	atspiRole[gui.AccessRoleMenuBar] = roleMenuBar
	atspiRole[gui.AccessRoleMenuItem] = roleMenuItem
	atspiRole[gui.AccessRoleProgressBar] = roleProgressBr
	atspiRole[gui.AccessRoleRadioButton] = roleRadioBtn
	atspiRole[gui.AccessRoleRadioGroup] = roleRadioGroup
	atspiRole[gui.AccessRoleScrollArea] = roleScrollPane
	atspiRole[gui.AccessRoleScrollBar] = roleScrollBar
	atspiRole[gui.AccessRoleSlider] = roleSlider
	atspiRole[gui.AccessRoleSplitter] = roleSeparator
	atspiRole[gui.AccessRoleStaticText] = roleLabel
	atspiRole[gui.AccessRoleSwitchToggle] = roleSwitchTgl
	atspiRole[gui.AccessRoleTab] = rolePageTabLst
	atspiRole[gui.AccessRoleTabItem] = rolePageTab
	atspiRole[gui.AccessRoleTextField] = roleEntry
	atspiRole[gui.AccessRoleTextArea] = roleTextArea
	atspiRole[gui.AccessRoleToolbar] = roleToolbar
	atspiRole[gui.AccessRoleTree] = roleTree
	atspiRole[gui.AccessRoleTreeItem] = roleTreeItem

	atspiRoleName[gui.AccessRoleNone] = "unknown"
	atspiRoleName[gui.AccessRoleButton] = "push button"
	atspiRoleName[gui.AccessRoleCheckbox] = "check box"
	atspiRoleName[gui.AccessRoleColorWell] = "color chooser"
	atspiRoleName[gui.AccessRoleComboBox] = "combo box"
	atspiRoleName[gui.AccessRoleDateField] = "date editor"
	atspiRoleName[gui.AccessRoleDialog] = "dialog"
	atspiRoleName[gui.AccessRoleDisclosure] = "toggle button"
	atspiRoleName[gui.AccessRoleGrid] = "panel"
	atspiRoleName[gui.AccessRoleGridCell] = "panel"
	atspiRoleName[gui.AccessRoleGroup] = "panel"
	atspiRoleName[gui.AccessRoleHeading] = "heading"
	atspiRoleName[gui.AccessRoleImage] = "image"
	atspiRoleName[gui.AccessRoleLink] = "link"
	atspiRoleName[gui.AccessRoleList] = "list"
	atspiRoleName[gui.AccessRoleListItem] = "list item"
	atspiRoleName[gui.AccessRoleMenu] = "menu"
	atspiRoleName[gui.AccessRoleMenuBar] = "menu bar"
	atspiRoleName[gui.AccessRoleMenuItem] = "menu item"
	atspiRoleName[gui.AccessRoleProgressBar] = "progress bar"
	atspiRoleName[gui.AccessRoleRadioButton] = "radio button"
	atspiRoleName[gui.AccessRoleRadioGroup] = "panel"
	atspiRoleName[gui.AccessRoleScrollArea] = "scroll pane"
	atspiRoleName[gui.AccessRoleScrollBar] = "scroll bar"
	atspiRoleName[gui.AccessRoleSlider] = "slider"
	atspiRoleName[gui.AccessRoleSplitter] = "separator"
	atspiRoleName[gui.AccessRoleStaticText] = "label"
	atspiRoleName[gui.AccessRoleSwitchToggle] = "toggle button"
	atspiRoleName[gui.AccessRoleTab] = "page tab list"
	atspiRoleName[gui.AccessRoleTabItem] = "page tab"
	atspiRoleName[gui.AccessRoleTextField] = "entry"
	atspiRoleName[gui.AccessRoleTextArea] = "entry"
	atspiRoleName[gui.AccessRoleToolbar] = "tool bar"
	atspiRoleName[gui.AccessRoleTree] = "tree"
	atspiRoleName[gui.AccessRoleTreeItem] = "tree item"
}
