//go:build windows

package sni

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	procGetMenuItemCount = user32.NewProc("GetMenuItemCount")
	procGetMenuItemID    = user32.NewProc("GetMenuItemID")
	procGetSubMenu       = user32.NewProc("GetSubMenu")
)

// noMenuItemID is what GetMenuItemID returns for a separator and for an
// item that opens a submenu.
const noMenuItemID = 0xFFFFFFFF

// clickEveryItem walks a real Win32 menu and sends each item's ID
// through fireMenuAction, as a click would.
func clickEveryItem(t *testing.T, tr *Tray, hMenu uintptr) {
	t.Helper()
	count, _, _ := procGetMenuItemCount.Call(hMenu)
	for pos := range count {
		if sub, _, _ := procGetSubMenu.Call(hMenu, pos); sub != 0 {
			clickEveryItem(t, tr, sub)
			continue
		}
		id, _, _ := procGetMenuItemID.Call(hMenu, pos)
		if uint32(id) == noMenuItemID {
			continue // separator
		}
		tr.fireMenuAction(uint32(id))
	}
}

// Issue #1273: a click on a tray menu item must fire that item's
// action. The menu's item IDs were one lower than the node indexes
// fireMenuAction reads, so "Quit" after a separator fired nothing and
// every other item fired the one above it.
func TestTrayMenuClickFiresTheClickedItem(t *testing.T) {
	menu := []gui.NativeMenuItemCfg{
		{ID: "show", Text: "Show Window"},
		{ID: "more", Text: "More", Submenu: []gui.NativeMenuItemCfg{
			{ID: "sub-a", Text: "A"},
			{Separator: true},
			{ID: "sub-b", Text: "B"},
		}},
		{ID: "prefs", Text: "Preferences"},
		{Separator: true},
		{ID: "quit", Text: "Quit"},
	}
	const entryID = 1
	nodes := buildMenuNodes(menu)
	hMenu := buildPopupMenu(nodes, entryID)
	if hMenu == 0 {
		t.Fatal("no menu was built")
	}
	t.Cleanup(func() { procDestroyMenu.Call(hMenu) })

	var got []string
	tr := &Tray{entries: map[int]*entry{entryID: {
		id:        entryID,
		menuNodes: nodes,
		actionCb:  func(id string) { got = append(got, id) },
	}}}
	clickEveryItem(t, tr, hMenu)

	want := []string{"show", "sub-a", "sub-b", "prefs", "quit"}
	if len(got) != len(want) {
		t.Fatalf("clicks fired %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("clicks fired %q, want %q", got, want)
		}
	}
}

// A tray with no menu items gets no menu.
func TestTrayMenuEmpty(t *testing.T) {
	if hMenu := buildPopupMenu(buildMenuNodes(nil), 1); hMenu != 0 {
		procDestroyMenu.Call(hMenu)
		t.Error("an empty item list built a menu")
	}
}
