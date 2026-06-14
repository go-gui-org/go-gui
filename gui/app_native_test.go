package gui

import (
	"strings"
	"testing"
)

// mockAppPlatform extends NoopNativePlatform to track App-level
// menubar and system-tray interactions for integration tests.
type mockAppPlatform struct {
	NoopNativePlatform
	menubarCfg NativeMenubarCfg
	menubarCB  func(string)
	menubarSet bool
	trayCfgs   map[int]SystemTrayCfg
	trayCBs    map[int]func(string)
	nextTrayID int
	failCreate bool
}

func (m *mockAppPlatform) SetNativeMenubar(cfg NativeMenubarCfg, cb func(string)) {
	m.menubarCfg = cfg
	m.menubarCB = cb
	m.menubarSet = true
}

func (m *mockAppPlatform) ClearNativeMenubar() {
	m.menubarSet = false
}

func (m *mockAppPlatform) CreateSystemTray(cfg SystemTrayCfg, cb func(string)) (int, error) {
	if m.failCreate {
		return 0, &testError{msg: "platform error"}
	}
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

func (m *mockAppPlatform) UpdateSystemTray(id int, cfg SystemTrayCfg) {
	if _, ok := m.trayCfgs[id]; ok {
		m.trayCfgs[id] = cfg
	}
}

func (m *mockAppPlatform) RemoveSystemTray(id int) {
	delete(m.trayCfgs, id)
	delete(m.trayCBs, id)
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// --- App.SetNativeMenubar ---

func TestAppSetNativeMenubar(t *testing.T) {
	app := NewApp()
	mp := &mockAppPlatform{}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(mp)
	app.Register(1, w)

	menu := NativeMenuCfg{
		Title: "File",
		Items: []NativeMenuItemCfg{
			{ID: "open", Text: "Open", CommandID: "cmd-open"},
		},
	}
	app.SetNativeMenubar(NativeMenubarCfg{
		Menus:   []NativeMenuCfg{menu},
		AppName: "TestApp",
	})

	if !mp.menubarSet {
		t.Error("menubar should be set")
	}
	if mp.menubarCfg.AppName != "TestApp" {
		t.Errorf("AppName = %q, want TestApp", mp.menubarCfg.AppName)
	}
	if len(mp.menubarCfg.Menus) != 1 {
		t.Fatalf("menus: got %d, want 1", len(mp.menubarCfg.Menus))
	}
	if mp.menubarCfg.Menus[0].Title != "File" {
		t.Errorf("menu title: got %q, want File",
			mp.menubarCfg.Menus[0].Title)
	}
}

func TestAppSetNativeMenubarNoMainWindow(t *testing.T) {
	app := NewApp()
	// No window registered — should not panic.
	app.SetNativeMenubar(NativeMenubarCfg{AppName: "Test"})
}

func TestAppSetNativeMenubarNilPlatform(t *testing.T) {
	app := NewApp()
	w := NewWindow(WindowCfg{State: new(struct{})})
	app.Register(1, w)
	// No native platform set — should not panic.
	app.SetNativeMenubar(NativeMenubarCfg{AppName: "Test"})
}

// --- App.ClearNativeMenubar ---

func TestAppClearNativeMenubar(t *testing.T) {
	app := NewApp()
	mp := &mockAppPlatform{}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(mp)
	app.Register(1, w)

	app.SetNativeMenubar(NativeMenubarCfg{AppName: "Test"})
	if !mp.menubarSet {
		t.Fatal("menubar should be set before clear")
	}

	app.ClearNativeMenubar()
	if mp.menubarSet {
		t.Error("menubar should be cleared")
	}
}

func TestAppClearNativeMenubarNoMainWindow(t *testing.T) {
	app := NewApp()
	// Should not panic.
	app.ClearNativeMenubar()
}

func TestAppClearNativeMenubarNilPlatform(t *testing.T) {
	app := NewApp()
	w := NewWindow(WindowCfg{State: new(struct{})})
	app.Register(1, w)
	// Should not panic.
	app.ClearNativeMenubar()
}

// --- App.SetSystemTray ---

func TestAppSetSystemTray(t *testing.T) {
	app := NewApp()
	mp := &mockAppPlatform{}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(mp)
	app.Register(1, w)

	h, err := app.SetSystemTray(SystemTrayCfg{
		Tooltip: "My App",
		Menu: []NativeMenuItemCfg{
			{ID: "show", Text: "Show"},
			{ID: "quit", Text: "Quit"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("handle should not be nil")
	}

	if len(mp.trayCfgs) != 1 {
		t.Fatalf("expected 1 tray, got %d", len(mp.trayCfgs))
	}
	cfg := mp.trayCfgs[1]
	if cfg.Tooltip != "My App" {
		t.Errorf("Tooltip = %q, want My App", cfg.Tooltip)
	}
	if len(cfg.Menu) != 2 {
		t.Errorf("menu items: got %d, want 2", len(cfg.Menu))
	}

	// Verify the tray is tracked in the app.
	app.mu.Lock()
	_, tracked := app.trays[1]
	app.mu.Unlock()
	if !tracked {
		t.Error("tray should be tracked in app.trays")
	}
}

func TestAppSetSystemTrayNoMainWindow(t *testing.T) {
	app := NewApp()
	_, err := app.SetSystemTray(SystemTrayCfg{Tooltip: "Test"})
	if err == nil {
		t.Error("expected error when no main window")
	} else if !strings.Contains(err.Error(), "no main window") {
		t.Errorf("error = %q, want 'no main window'", err.Error())
	}
}

func TestAppSetSystemTrayNilPlatform(t *testing.T) {
	app := NewApp()
	w := NewWindow(WindowCfg{State: new(struct{})})
	app.Register(1, w)
	_, err := app.SetSystemTray(SystemTrayCfg{Tooltip: "Test"})
	if err == nil {
		t.Error("expected error when no native platform")
	} else if !strings.Contains(err.Error(), "no native platform") {
		t.Errorf("error = %q, want 'no native platform'", err.Error())
	}
}

func TestAppSetSystemTrayPlatformError(t *testing.T) {
	app := NewApp()
	mp := &mockAppPlatform{failCreate: true}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(mp)
	app.Register(1, w)

	_, err := app.SetSystemTray(SystemTrayCfg{Tooltip: "Test"})
	if err == nil {
		t.Error("expected error from platform")
	}
}

func TestAppSetSystemTrayMultiple(t *testing.T) {
	app := NewApp()
	mp := &mockAppPlatform{}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(mp)
	app.Register(1, w)

	h1, err := app.SetSystemTray(SystemTrayCfg{Tooltip: "First"})
	if err != nil {
		t.Fatalf("first tray: %v", err)
	}
	h2, err := app.SetSystemTray(SystemTrayCfg{Tooltip: "Second"})
	if err != nil {
		t.Fatalf("second tray: %v", err)
	}
	if h1 == h2 {
		t.Error("handles should be distinct")
	}
	if len(mp.trayCfgs) != 2 {
		t.Errorf("expected 2 trays, got %d", len(mp.trayCfgs))
	}
}

// --- App.UpdateSystemTray ---

func TestAppUpdateSystemTray(t *testing.T) {
	app := NewApp()
	mp := &mockAppPlatform{}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(mp)
	app.Register(1, w)

	h, err := app.SetSystemTray(SystemTrayCfg{Tooltip: "Before"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	app.UpdateSystemTray(h, SystemTrayCfg{Tooltip: "After"})
	if mp.trayCfgs[h.id].Tooltip != "After" {
		t.Errorf("Tooltip = %q, want After",
			mp.trayCfgs[h.id].Tooltip)
	}
}

func TestAppUpdateSystemTrayNilHandle(t *testing.T) {
	app := NewApp()
	mp := &mockAppPlatform{}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(mp)
	app.Register(1, w)

	// Must not panic.
	app.UpdateSystemTray(nil, SystemTrayCfg{Tooltip: "Ghost"})
}

func TestAppUpdateSystemTrayNoMainWindow(t *testing.T) {
	app := NewApp()
	h := &SystemTrayHandle{id: 1}
	// Must not panic.
	app.UpdateSystemTray(h, SystemTrayCfg{Tooltip: "Orphan"})
}

func TestAppUpdateSystemTrayNilPlatform(t *testing.T) {
	app := NewApp()
	w := NewWindow(WindowCfg{State: new(struct{})})
	app.Register(1, w)
	h := &SystemTrayHandle{id: 1}
	// Must not panic.
	app.UpdateSystemTray(h, SystemTrayCfg{Tooltip: "Ghost"})
}

// --- App.RemoveSystemTray ---

func TestAppRemoveSystemTray(t *testing.T) {
	app := NewApp()
	mp := &mockAppPlatform{}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(mp)
	app.Register(1, w)

	h, err := app.SetSystemTray(SystemTrayCfg{Tooltip: "Temp"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	app.RemoveSystemTray(h)

	// Mock platform should have removed it.
	if len(mp.trayCfgs) != 0 {
		t.Errorf("expected 0 trays in mock, got %d",
			len(mp.trayCfgs))
	}

	// App should have removed it from tracking.
	app.mu.Lock()
	_, tracked := app.trays[h.id]
	app.mu.Unlock()
	if tracked {
		t.Error("tray should not be tracked after remove")
	}
}

func TestAppRemoveSystemTrayNilHandle(t *testing.T) {
	app := NewApp()
	// Must not panic.
	app.RemoveSystemTray(nil)
}

func TestAppRemoveSystemTrayNoMainWindow(t *testing.T) {
	app := NewApp()
	h := &SystemTrayHandle{id: 99}
	// Must not panic; removes from tracking even without window.
	app.mu.Lock()
	app.trays = map[int]*SystemTrayHandle{99: h}
	app.mu.Unlock()
	app.RemoveSystemTray(h)
}

func TestAppRemoveSystemTrayNilPlatform(t *testing.T) {
	app := NewApp()
	w := NewWindow(WindowCfg{State: new(struct{})})
	app.Register(1, w)
	h := &SystemTrayHandle{id: 1}
	// Must not panic.
	app.RemoveSystemTray(h)
}

// --- NoopNativePlatform safety ---

func TestAppNativeRoutingNoop(t *testing.T) {
	app := NewApp()
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(&NoopNativePlatform{})
	app.Register(1, w)

	// All App native methods should work safely with NoopNativePlatform.
	app.SetNativeMenubar(NativeMenubarCfg{
		AppName: "Test",
		Menus: []NativeMenuCfg{
			{Title: "File", Items: []NativeMenuItemCfg{
				{ID: "open", Text: "Open"},
			}},
		},
	})
	app.ClearNativeMenubar()

	h, err := app.SetSystemTray(SystemTrayCfg{Tooltip: "Test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("handle should not be nil")
	}

	app.UpdateSystemTray(h, SystemTrayCfg{Tooltip: "Updated"})
	app.RemoveSystemTray(h)
}

// --- ExitOnTrayRemoved with App native methods ---

func TestAppExitOnTrayRemoved_ActiveTrayBlocksExit(t *testing.T) {
	app := NewApp()
	app.ExitMode = ExitOnTrayRemoved

	w := NewWindow(WindowCfg{State: new(struct{})})
	app.Register(1, w)

	// Add a tray.
	app.mu.Lock()
	app.trays = map[int]*SystemTrayHandle{1: {id: 1}}
	app.mu.Unlock()

	// Removing last window should NOT exit (tray still active).
	if app.Unregister(1) {
		t.Error("should not exit while tray is active")
	}
}

func TestAppExitOnTrayRemoved_NoTraysAllowsExit(t *testing.T) {
	app := NewApp()
	app.ExitMode = ExitOnTrayRemoved

	w := NewWindow(WindowCfg{State: new(struct{})})
	app.Register(1, w)

	// No trays registered — removing last window should exit.
	if !app.Unregister(1) {
		t.Error("should exit: no windows and no trays")
	}
}
