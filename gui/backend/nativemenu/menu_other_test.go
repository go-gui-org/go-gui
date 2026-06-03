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
