package gui

import "testing"

func TestNativeMenuItemsFromMenuItems_Basic(t *testing.T) {
	items := []MenuItemCfg{
		{ID: "a", Text: "Alpha", CommandID: "cmd-a"},
		{ID: "b", Text: "Beta"},
	}
	got := NativeMenuItemsFromMenuItems(items)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "a" || got[0].Text != "Alpha" {
		t.Errorf("item 0: got %+v", got[0])
	}
	if got[0].CommandID != "cmd-a" {
		t.Errorf("CommandID = %q, want cmd-a", got[0].CommandID)
	}
	if got[1].ID != "b" || got[1].Text != "Beta" {
		t.Errorf("item 1: got %+v", got[1])
	}
}

func TestNativeMenuItemsFromMenuItems_Separator(t *testing.T) {
	items := []MenuItemCfg{
		MenuSeparator(),
	}
	got := NativeMenuItemsFromMenuItems(items)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if !got[0].Separator {
		t.Error("expected Separator=true")
	}
}

func TestNativeMenuItemsFromMenuItems_Submenu(t *testing.T) {
	items := []MenuItemCfg{
		MenuSubmenu("file", "File", []MenuItemCfg{
			{ID: "new", Text: "New"},
			{ID: "open", Text: "Open"},
		}),
	}
	got := NativeMenuItemsFromMenuItems(items)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if len(got[0].Submenu) != 2 {
		t.Fatalf("submenu len = %d, want 2",
			len(got[0].Submenu))
	}
	if got[0].Submenu[0].ID != "new" {
		t.Errorf("submenu[0].ID = %q", got[0].Submenu[0].ID)
	}
}

func TestNativeMenuItemsFromMenuItems_Empty(t *testing.T) {
	got := NativeMenuItemsFromMenuItems(nil)
	if len(got) != 0 {
		t.Fatalf("len = %d, want 0", len(got))
	}
}

func TestNativeMenuItemsFromMenuItems_Disabled(t *testing.T) {
	items := []MenuItemCfg{
		MenuSubtitle("Section"),
	}
	got := NativeMenuItemsFromMenuItems(items)
	if !got[0].Disabled {
		t.Error("expected Disabled=true for subtitle")
	}
}

func TestExitOnTrayRemoved(t *testing.T) {
	app := NewApp()
	app.ExitMode = ExitOnTrayRemoved

	w := NewWindow(WindowCfg{State: new(struct{})})
	app.Register(1, w)

	// Add a tray.
	app.mu.Lock()
	app.trays = map[int]*SystemTrayHandle{1: {id: 1}}
	app.mu.Unlock()

	// Unregister last window — should NOT exit (tray exists).
	if app.Unregister(1) {
		t.Error("should not exit: tray still active")
	}

	// Remove tray, register+unregister — should exit.
	app.mu.Lock()
	delete(app.trays, 1)
	app.mu.Unlock()

	w2 := NewWindow(WindowCfg{State: new(struct{})})
	app.Register(2, w2)
	if !app.Unregister(2) {
		t.Error("should exit: no windows and no trays")
	}
}

// mockMenuPlatform tracks menubar and tray state for interface-level
// testing of the NativeMenubar / NativeSystemTray contracts.
type mockMenuPlatform struct {
	NoopNativePlatform
	menubarCfg NativeMenubarCfg
	menubarCB  func(string)
	menubarSet bool
	trayCfgs   map[int]SystemTrayCfg
	trayCBs    map[int]func(string)
	nextTrayID int
}

func (m *mockMenuPlatform) SetNativeMenubar(cfg NativeMenubarCfg, cb func(string)) {
	m.menubarCfg = cfg
	m.menubarCB = cb
	m.menubarSet = true
}

func (m *mockMenuPlatform) ClearNativeMenubar() {
	m.menubarSet = false
}

func (m *mockMenuPlatform) CreateSystemTray(cfg SystemTrayCfg, cb func(string)) (int, error) {
	if m.trayCfgs == nil {
		m.trayCfgs = make(map[int]SystemTrayCfg)
		m.trayCBs = make(map[int]func(string))
	}
	m.nextTrayID++
	id := m.nextTrayID
	m.trayCfgs[id] = cfg
	m.trayCBs[id] = cb
	return id, nil
}

func (m *mockMenuPlatform) UpdateSystemTray(id int, cfg SystemTrayCfg) {
	if _, ok := m.trayCfgs[id]; ok {
		m.trayCfgs[id] = cfg
	}
}

func (m *mockMenuPlatform) RemoveSystemTray(id int) {
	delete(m.trayCfgs, id)
	delete(m.trayCBs, id)
}

func TestMockMenubarSetAndCallback(t *testing.T) {
	mp := &mockMenuPlatform{}
	menu := NativeMenuCfg{
		Title: "File",
		Items: []NativeMenuItemCfg{
			{ID: "open", Text: "Open"},
			{ID: "quit", Text: "Quit"},
		},
	}
	var dispatched []string
	mp.SetNativeMenubar(NativeMenubarCfg{
		Menus:   []NativeMenuCfg{menu},
		AppName: "TestApp",
	}, func(id string) {
		dispatched = append(dispatched, id)
	})
	if !mp.menubarSet {
		t.Error("menubar should be set")
	}
	if len(mp.menubarCfg.Menus) != 1 {
		t.Fatalf("menus: got %d, want 1", len(mp.menubarCfg.Menus))
	}
	if mp.menubarCfg.Menus[0].Title != "File" {
		t.Errorf("menu title: got %q", mp.menubarCfg.Menus[0].Title)
	}
	if mp.menubarCfg.AppName != "TestApp" {
		t.Errorf("app name: got %q", mp.menubarCfg.AppName)
	}
	// Dispatch callbacks.
	mp.menubarCB("open")
	mp.menubarCB("quit")
	if len(dispatched) != 2 || dispatched[0] != "open" || dispatched[1] != "quit" {
		t.Errorf("dispatched: got %v", dispatched)
	}
}

func TestMockMenubarClear(t *testing.T) {
	mp := &mockMenuPlatform{}
	mp.SetNativeMenubar(NativeMenubarCfg{}, nil)
	if !mp.menubarSet {
		t.Error("menubar should be set")
	}
	mp.ClearNativeMenubar()
	if mp.menubarSet {
		t.Error("menubar should be cleared")
	}
}

func TestMockMenubarEmptyMenus(t *testing.T) {
	mp := &mockMenuPlatform{}
	mp.SetNativeMenubar(NativeMenubarCfg{}, nil)
	if !mp.menubarSet {
		t.Error("menubar should be set even with empty config")
	}
}

func TestMockSystemTrayCreate(t *testing.T) {
	mp := &mockMenuPlatform{}
	var actionIDs []string
	id, err := mp.CreateSystemTray(SystemTrayCfg{
		Tooltip: "My Tray",
		Menu: []NativeMenuItemCfg{
			{ID: "show", Text: "Show Window"},
			{ID: "quit", Text: "Quit"},
		},
	}, func(id string) {
		actionIDs = append(actionIDs, id)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 1 {
		t.Errorf("first id: got %d, want 1", id)
	}
	if len(mp.trayCfgs) != 1 {
		t.Fatalf("expected 1 tray, got %d", len(mp.trayCfgs))
	}
	if mp.trayCfgs[id].Tooltip != "My Tray" {
		t.Errorf("tooltip: got %q", mp.trayCfgs[id].Tooltip)
	}
	// Invoke callback.
	mp.trayCBs[id]("show")
	mp.trayCBs[id]("quit")
	if len(actionIDs) != 2 || actionIDs[0] != "show" || actionIDs[1] != "quit" {
		t.Errorf("actionIDs: got %v", actionIDs)
	}
}

func TestMockSystemTrayMultipleCreate(t *testing.T) {
	mp := &mockMenuPlatform{}
	id1, _ := mp.CreateSystemTray(SystemTrayCfg{Tooltip: "First"}, nil)
	id2, _ := mp.CreateSystemTray(SystemTrayCfg{Tooltip: "Second"}, nil)
	if id1 != 1 || id2 != 2 {
		t.Errorf("ids: got %d, %d; want 1, 2", id1, id2)
	}
	if len(mp.trayCfgs) != 2 {
		t.Errorf("expected 2 trays, got %d", len(mp.trayCfgs))
	}
}

func TestMockSystemTrayUpdate(t *testing.T) {
	mp := &mockMenuPlatform{}
	id, _ := mp.CreateSystemTray(SystemTrayCfg{Tooltip: "Before"}, nil)
	mp.UpdateSystemTray(id, SystemTrayCfg{Tooltip: "After"})
	if mp.trayCfgs[id].Tooltip != "After" {
		t.Errorf("Tooltip: got %q, want After", mp.trayCfgs[id].Tooltip)
	}
}

func TestMockSystemTrayUpdateNonExistent(t *testing.T) {
	mp := &mockMenuPlatform{}
	// Must not panic.
	mp.UpdateSystemTray(99, SystemTrayCfg{Tooltip: "Ghost"})
}

func TestMockSystemTrayRemove(t *testing.T) {
	mp := &mockMenuPlatform{}
	id, _ := mp.CreateSystemTray(SystemTrayCfg{}, nil)
	mp.RemoveSystemTray(id)
	if len(mp.trayCfgs) != 0 {
		t.Errorf("expected 0 trays, got %d", len(mp.trayCfgs))
	}
}

func TestMockSystemTrayRemoveNonExistent(t *testing.T) {
	mp := &mockMenuPlatform{}
	// Must not panic.
	mp.RemoveSystemTray(99)
	mp.RemoveSystemTray(-1)
}

func TestMockSystemTrayDoubleRemove(t *testing.T) {
	mp := &mockMenuPlatform{}
	id, _ := mp.CreateSystemTray(SystemTrayCfg{}, nil)
	mp.RemoveSystemTray(id)
	// Double-remove must not panic.
	mp.RemoveSystemTray(id)
}
