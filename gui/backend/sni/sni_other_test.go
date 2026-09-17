//go:build !linux && !windows

package sni

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestTrayCreateNoPanic(t *testing.T) {
	t.Parallel()
	var tr Tray
	id, err := tr.Create(gui.SystemTrayCfg{}, nil)
	// The stub reports success, so the handle must be positive —
	// a zero ID collides across trays in App bookkeeping.
	if id <= 0 {
		t.Errorf("id: got %d, want positive", id)
	}
	if err != nil {
		t.Errorf("err: got %v, want nil", err)
	}
}

func TestTrayCreateWithConfig(t *testing.T) {
	t.Parallel()
	var tr Tray
	cfg := gui.SystemTrayCfg{
		Tooltip: "test tooltip",
		IconPNG: []byte{0x89, 0x50, 0x4E, 0x47},
		Menu: []gui.NativeMenuItemCfg{
			{ID: "a", Text: "Alpha"},
		},
	}
	id, err := tr.Create(cfg, func(s string) {})
	if id <= 0 {
		t.Errorf("id: got %d, want positive", id)
	}
	if err != nil {
		t.Errorf("err: got %v, want nil", err)
	}
}

func TestTrayCreateMultiple(t *testing.T) {
	t.Parallel()
	var tr Tray
	seen := map[int]bool{}
	for i := range 3 {
		id, err := tr.Create(gui.SystemTrayCfg{}, nil)
		if id <= 0 {
			t.Errorf("iter %d: id: got %d, want positive", i, id)
		}
		if seen[id] {
			t.Errorf("iter %d: duplicate id %d", i, id)
		}
		seen[id] = true
		if err != nil {
			t.Errorf("iter %d: err: got %v, want nil", i, err)
		}
	}
}

func TestTrayUpdateNoPanic(t *testing.T) {
	t.Parallel()
	var tr Tray
	// Update with non-existent ID should not panic.
	tr.Update(0, gui.SystemTrayCfg{})
	tr.Update(42, gui.SystemTrayCfg{
		Tooltip: "updated",
		Menu: []gui.NativeMenuItemCfg{
			{ID: "b", Text: "Beta"},
		},
	})
}

func TestTrayUpdateAfterCreate(t *testing.T) {
	t.Parallel()
	var tr Tray
	id, _ := tr.Create(gui.SystemTrayCfg{Tooltip: "before"}, nil)
	tr.Update(id, gui.SystemTrayCfg{Tooltip: "after"})
	// No panic is the assertion.
}

func TestTrayRemoveNoPanic(t *testing.T) {
	t.Parallel()
	var tr Tray
	// Remove with non-existent ID should not panic.
	tr.Remove(0)
	tr.Remove(99)
	tr.Remove(-1)
}

func TestTrayRemoveAfterCreate(t *testing.T) {
	t.Parallel()
	var tr Tray
	id, _ := tr.Create(gui.SystemTrayCfg{}, nil)
	tr.Remove(id)
	// Remove-on-removed should be safe.
	tr.Remove(id)
}
