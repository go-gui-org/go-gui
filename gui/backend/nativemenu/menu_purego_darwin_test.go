//go:build darwin && !ios

package nativemenu

import (
	"bytes"
	"image"
	"image/png"
	"slices"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// These tests build menus but never install one: setMainMenu: and the
// status bar need the app on the main thread, and go test runs each
// test on its own goroutine. Building an NSMenu has no such limit.

// withPool runs fn inside an autorelease pool, on one OS thread.
func withPool(t *testing.T, fn func()) {
	t.Helper()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	fn()
}

func goStr(id objc.ID) string { return darwinffi.GoString(uintptr(id)) }

func titles(menu objc.ID) []string {
	n := objc.Send[int](menu, sel("numberOfItems"))
	out := make([]string, n)
	for i := range n {
		out[i] = itemTitle(menu, i)
	}
	return out
}

func item(menu objc.ID, i int) objc.ID { return menu.Send(sel("itemAtIndex:"), i) }

func TestBuildMenuItems(t *testing.T) {
	withPool(t, func() {
		items := []gui.NativeMenuItemCfg{
			{ID: "save", Text: "Save", Shortcut: gui.Shortcut{
				Key: gui.KeyS, Modifiers: gui.ModSuper | gui.ModShift,
			}},
			{Separator: true},
			{ID: "off", Text: "Off", Disabled: true},
			{ID: "on", Text: "On", Checked: true},
			{Text: "More", Submenu: []gui.NativeMenuItemCfg{{ID: "deep", Text: "Deep"}}},
			{ID: "bad\xff", Text: "Bad\xffText"},
		}
		menu := buildMenu("File", items, 7)
		if got := goStr(menu.Send(sel("title"))); got != "File" {
			t.Fatalf("title = %q, want File", got)
		}
		want := []string{"Save", "", "Off", "On", "More", "Bad\uFFFDText"}
		if got := titles(menu); !slices.Equal(got, want) {
			t.Fatalf("titles = %q, want %q", got, want)
		}
		if objc.Send[bool](menu, sel("autoenablesItems")) {
			t.Error("autoenablesItems = true, want false")
		}

		save := item(menu, 0)
		if k := goStr(save.Send(sel("keyEquivalent"))); k != "s" {
			t.Errorf("key equivalent = %q, want s", k)
		}
		if m := objc.Send[uint](save, sel("keyEquivalentModifierMask")); m != modCommand|modShift {
			t.Errorf("modifier mask = %#x, want %#x", m, modCommand|modShift)
		}
		if id := goStr(save.Send(sel("representedObject"))); id != "save" {
			t.Errorf("represented object = %q, want save", id)
		}
		if tag := objc.Send[int](save, sel("tag")); tag != 7 {
			t.Errorf("tag = %d, want 7", tag)
		}
		if a := objc.Send[objc.SEL](save, sel("action")); a != sel("menuItemClicked:") {
			t.Error("action is not menuItemClicked:")
		}
		if tg := save.Send(sel("target")); tg != menuTarget() {
			t.Error("target is not the menu target")
		}

		if !objc.Send[bool](item(menu, 1), sel("isSeparatorItem")) {
			t.Error("item 1 is not a separator")
		}
		if objc.Send[bool](item(menu, 2), sel("isEnabled")) {
			t.Error("disabled item is enabled")
		}
		if !objc.Send[bool](item(menu, 3), sel("isEnabled")) {
			t.Error("enabled item is disabled")
		}
		if s := objc.Send[int](item(menu, 3), sel("state")); s != controlStateOn {
			t.Errorf("checked state = %d, want %d", s, controlStateOn)
		}
		if s := objc.Send[int](item(menu, 2), sel("state")); s != 0 {
			t.Errorf("unchecked state = %d, want 0", s)
		}

		more := item(menu, 4)
		// setSubmenu: gives the parent AppKit's own submenuAction: and
		// target, so test that it does not fire the callback, not that
		// it has no action.
		if a := objc.Send[objc.SEL](more, sel("action")); a == sel("menuItemClicked:") {
			t.Error("submenu parent fires menuItemClicked:")
		}
		sub := more.Send(sel("submenu"))
		if got := titles(sub); !slices.Equal(got, []string{"Deep"}) {
			t.Errorf("submenu titles = %q, want [Deep]", got)
		}
		if tag := objc.Send[int](item(sub, 0), sel("tag")); tag != 7 {
			t.Errorf("submenu item tag = %d, want 7", tag)
		}

		// An ID that is not valid UTF-8 cannot be an NSString: no ID, so
		// no callback. The title shows U+FFFD (checked above).
		if id := item(menu, 5).Send(sel("representedObject")); id != 0 {
			t.Errorf("invalid ID gave represented object %q, want none", goStr(id))
		}
	})
}

// A click calls the menubar callback for tag 0 and the tray callback
// for the tray ID in the tag. With no NSApp there is no current event,
// so the key-equivalent record is cleared.
func TestMenuItemClickedRoutes(t *testing.T) {
	// Not parallel: the callbacks and the key record are package state.
	var menubarGot, trayGot []string
	mu.Lock()
	menubarActionCb = func(id string) { menubarGot = append(menubarGot, id) }
	trayActionCbs[5] = func(id string) { trayGot = append(trayGot, id) }
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		menubarActionCb = nil
		delete(trayActionCbs, 5)
		mu.Unlock()
	})

	withPool(t, func() {
		bar := buildMenu("", []gui.NativeMenuItemCfg{{ID: "open", Text: "Open"}, {Text: "NoID"}}, 0)
		tray := buildMenu("", []gui.NativeMenuItemCfg{{ID: "quit", Text: "Quit"}}, 5)

		noteKeyEquivalent(0x2C) // a stale record the click must clear
		click := func(mi objc.ID) { menuTarget().Send(sel("menuItemClicked:"), mi) }
		click(item(bar, 0))
		click(item(bar, 1))
		click(item(tray, 0))

		if !slices.Equal(menubarGot, []string{"open"}) {
			t.Errorf("menubar callback got %q, want [open]", menubarGot)
		}
		if !slices.Equal(trayGot, []string{"quit"}) {
			t.Errorf("tray callback got %q, want [quit]", trayGot)
		}
		if _, ok := TakeKeyEquivalent(); ok {
			t.Error("key record kept after a click with no key-down")
		}
	})
}

func TestBuildMenubarOrder(t *testing.T) {
	menus := []gui.NativeMenuCfg{
		{Title: "File"}, {Title: "View"}, {Title: "Help"},
	}
	tests := []struct {
		name string
		cfg  gui.NativeMenubarCfg
		want []string
	}{
		{"menus only", gui.NativeMenubarCfg{AppName: "App", Menus: menus},
			[]string{"App", "File", "View", "Help"}},
		{"edit and window", gui.NativeMenubarCfg{
			AppName: "App", Menus: menus, IncludeEditMenu: true, IncludeWindowMenu: true,
		}, []string{"App", "File", "Edit", "View", "Window", "Help"}},
		{"no menus", gui.NativeMenubarCfg{
			AppName: "App", IncludeEditMenu: true, IncludeWindowMenu: true,
		}, []string{"App", "Edit", "Window"}},
		{"window without help", gui.NativeMenubarCfg{
			AppName: "App", Menus: menus[:1], IncludeWindowMenu: true,
		}, []string{"App", "File", "Window"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withPool(t, func() {
				bar, windows := buildMenubar(tt.cfg)
				if got := titles(bar); !slices.Equal(got, tt.want) {
					t.Fatalf("menubar = %q, want %q", got, tt.want)
				}
				if (windows != 0) != tt.cfg.IncludeWindowMenu {
					t.Errorf("windows menu = %v, want %v", windows != 0, tt.cfg.IncludeWindowMenu)
				}
				if windows != 0 {
					want := []string{"Close", "Minimize", "Zoom", "", "Bring All to Front"}
					if got := titles(windows); !slices.Equal(got, want) {
						t.Errorf("window menu = %q, want %q", got, want)
					}
				}
			})
		})
	}
}

func TestAppMenu(t *testing.T) {
	withPool(t, func() {
		std := appMenu("App", "", false).Send(sel("submenu"))
		if got := titles(std); !slices.Equal(got, []string{"About App", "", "Quit App"}) {
			t.Fatalf("app menu = %q", got)
		}
		if a := objc.Send[objc.SEL](item(std, 0), sel("action")); a != sel("orderFrontStandardAboutPanel:") {
			t.Error("About without an action ID does not open the standard panel")
		}
		quit := item(std, 2)
		if a := objc.Send[objc.SEL](quit, sel("action")); a != sel("terminate:") {
			t.Error("Quit action is not terminate:")
		}
		if k := goStr(quit.Send(sel("keyEquivalent"))); k != "q" {
			t.Errorf("Quit key = %q, want q", k)
		}

		custom := appMenu("App", "about", false).Send(sel("submenu"))
		about := item(custom, 0)
		if a := objc.Send[objc.SEL](about, sel("action")); a != sel("menuItemClicked:") {
			t.Error("About with an action ID does not go to the callback")
		}
		if id := goStr(about.Send(sel("representedObject"))); id != "about" {
			t.Errorf("About ID = %q, want about", id)
		}

		omit := appMenu("App", "about", true).Send(sel("submenu"))
		if got := titles(omit); !slices.Equal(got, []string{"Quit App"}) {
			t.Errorf("omitted About: app menu = %q, want [Quit App]", got)
		}

		// The cgo form passed the name to stringWithUTF8String:, which
		// gave nil, and NSMenuItem raised on the nil title.
		bad := appMenu("A\xffB", "", true)
		if got := goStr(bad.Send(sel("title"))); got != "A\uFFFDB" {
			t.Errorf("invalid app name: title = %q, want %q", got, "A\uFFFDB")
		}
	})
}

func TestStripSystemEditItems(t *testing.T) {
	withPool(t, func() {
		edit := stdMenu("Edit", editItems)
		sub := edit.Send(sel("submenu"))
		sub.Send(sel("addItem:"), separator())
		for _, s := range []string{"Start Dictation…", "AutoFill", "Writing Tools"} {
			sub.Send(sel("addItem:"), newItem(s, 0, ""))
		}
		// A second "Edit" before it, with no submenu, is passed over.
		bar := newMenu("")
		bar.Send(sel("addItem:"), newItem("Edit", 0, ""))
		bar.Send(sel("addItem:"), edit)

		stripSystemEditItems(bar)

		want := []string{"Undo", "Redo", "", "Cut", "Copy", "Paste", "", "Select All"}
		if got := titles(sub); !slices.Equal(got, want) {
			t.Errorf("edit menu = %q, want %q", got, want)
		}
	})
}

func TestImageFromPNG(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 32, 32))); err != nil {
		t.Fatal(err)
	}
	withPool(t, func() {
		img := imageFromPNG(buf.Bytes())
		if img == 0 {
			t.Fatal("valid PNG gave no image")
		}
		if !objc.Send[bool](img, sel("isTemplate")) {
			t.Error("image is not a template")
		}
		if sz := objc.Send[nsSize](img, sel("size")); sz != (nsSize{18, 18}) {
			t.Errorf("size = %v, want 18x18", sz)
		}
		if imageFromPNG(nil) != 0 {
			t.Error("empty data gave an image")
		}
		if imageFromPNG([]byte("not a png")) != 0 {
			t.Error("bad data gave an image")
		}
	})
}

func TestIsSystemEditItem(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"Start Dictation…", "Start Dictation", "Writing Tools", "Writing Tools…", "AutoFill", "AutoFill Contact"} {
		if !isSystemEditItem(s) {
			t.Errorf("isSystemEditItem(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"Copy", "Paste", "Emoji & Symbols", "Start"} {
		if isSystemEditItem(s) {
			t.Errorf("isSystemEditItem(%q) = true, want false", s)
		}
	}
}
