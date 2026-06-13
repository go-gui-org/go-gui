//go:build !darwin || ios

package nativemenu

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestSetMenubarNoPanic(t *testing.T) {
	t.Parallel()
	SetMenubar(gui.NativeMenubarCfg{}, nil)
}

func TestClearMenubarNoPanic(t *testing.T) {
	t.Parallel()
	ClearMenubar()
}

func TestCreateSystemTrayNoPanic(t *testing.T) {
	t.Parallel()
	id, err := CreateSystemTray(gui.SystemTrayCfg{}, nil)
	if id != 0 {
		t.Errorf("id: got %d, want 0", id)
	}
	if err != nil {
		t.Errorf("err: got %v, want nil", err)
	}
}

func TestUpdateSystemTrayNoPanic(t *testing.T) {
	t.Parallel()
	UpdateSystemTray(0, gui.SystemTrayCfg{})
}

func TestRemoveSystemTrayNoPanic(t *testing.T) {
	t.Parallel()
	RemoveSystemTray(0)
}

func TestSetMenubarWithItems(t *testing.T) {
	t.Parallel()
	SetMenubar(gui.NativeMenubarCfg{
		Items: []gui.NativeMenuItemCfg{
			{ID: "file", Text: "File", Submenu: []gui.NativeMenuItemCfg{
				{ID: "quit", Text: "Quit"},
			}},
			{ID: "edit", Text: "Edit"},
		},
	}, func(s string) {})
}

func TestCreateSystemTrayWithConfig(t *testing.T) {
	t.Parallel()
	id, err := CreateSystemTray(gui.SystemTrayCfg{
		Tooltip: "My App",
		Menu: []gui.NativeMenuItemCfg{
			{ID: "show", Text: "Show Window"},
			{Separator: true},
			{ID: "quit", Text: "Quit"},
		},
	}, func(s string) {})
	if id != 0 {
		t.Errorf("id: got %d, want 0", id)
	}
	if err != nil {
		t.Errorf("err: got %v, want nil", err)
	}
}

func TestUpdateSystemTrayNonExistent(t *testing.T) {
	t.Parallel()
	// Update with non-existent ID should not panic.
	UpdateSystemTray(42, gui.SystemTrayCfg{
		Tooltip: "ghost",
	})
	UpdateSystemTray(-1, gui.SystemTrayCfg{})
}

func TestRemoveSystemTrayNonExistent(t *testing.T) {
	t.Parallel()
	// Remove with non-existent ID should not panic.
	RemoveSystemTray(99)
	RemoveSystemTray(-1)
}

func TestSystemTrayLifecycle(t *testing.T) {
	t.Parallel()
	id, err := CreateSystemTray(gui.SystemTrayCfg{
		Tooltip: "Lifecycle Test",
	}, nil)
	if id != 0 || err != nil {
		t.Fatalf("create: id=%d err=%v", id, err)
	}
	UpdateSystemTray(id, gui.SystemTrayCfg{Tooltip: "Updated"})
	RemoveSystemTray(id)
	// Double-remove should be safe.
	RemoveSystemTray(id)
}
