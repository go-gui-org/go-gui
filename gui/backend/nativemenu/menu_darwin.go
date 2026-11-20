//go:build darwin && !ios

// Package nativemenu provides native macOS menubar and system tray.
package nativemenu

import (
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// The menubar and the tray are built through purego, so they work with
// cgo off (#1314). A menu is built when the app sets it, not per frame,
// so the objc.Send form, which allocates per call, is fine.

// AppKit constants (NSEvent.h, NSCell.h, NSStatusBar.h).
const (
	modShift   = 1 << 17 // NSEventModifierFlagShift
	modControl = 1 << 18 // NSEventModifierFlagControl
	modOption  = 1 << 19 // NSEventModifierFlagOption
	modCommand = 1 << 20 // NSEventModifierFlagCommand

	controlStateOn   = 1  // NSControlStateValueOn
	eventTypeKeyDown = 10 // NSEventTypeKeyDown

	// variableStatusItemLength is NSVariableStatusItemLength, a CGFloat.
	variableStatusItemLength = -1.0
)

// nsSize is NSSize: two CGFloats. It goes in two float registers.
type nsSize struct {
	Width, Height float64
}

// Callback registries — set by Go, called from the menu target.
var (
	mu              sync.Mutex
	menubarActionCb func(string)
	trayActionCbs   = map[int]func(string){}

	// trays holds each tray's NSStatusItem, retained, by tray ID. IDs
	// start at 1: a menu item's tag is its tray ID, and tag 0 marks a
	// menubar item.
	trays      = map[int]objc.ID{}
	nextTrayID = 1
)

// keyEquivalent holds the virtual key code of the key-down that last fired a
// menubar item through its key equivalent, plus one. Zero means none: the
// offset keeps key code 0 (kVK_ANSI_A) distinct from "no key".
//
// Written by menubarAction during [NSApp sendEvent:] and read by the
// metal backend's event mapper right after that sendEvent: returns. Both run
// on the main thread; the atomic only keeps the race detector quiet and costs
// nothing.
var keyEquivalent atomic.Int32

// TakeKeyEquivalent reports whether a menubar item fired from a key
// equivalent since the last call, and the virtual key code of that key-down.
// It clears the record, so each key-down is claimed at most once.
//
// The metal backend stores every key-down for Go before AppKit sees it, so a
// key the menu consumed would otherwise also reach the window's command
// registry. A backend calls this for each event it maps and drops a key-down
// whose code matches.
func TakeKeyEquivalent() (keyCode uint16, ok bool) {
	v := keyEquivalent.Swap(0)
	if v == 0 {
		return 0, false
	}
	return uint16(v - 1), true
}

// noteKeyEquivalent records the key-down behind a menubar action. keyCode is
// -1 (or any negative) when the action did not come from a key equivalent;
// that clears any stale record rather than leaving it to eat a later key.
func noteKeyEquivalent(keyCode int) {
	if keyCode < 0 || keyCode > 0xFFFF {
		keyEquivalent.Store(0)
		return
	}
	keyEquivalent.Store(int32(keyCode) + 1)
}

// menubarAction runs the app callback for a menubar item.
func menubarAction(id string, keyCode int) {
	// Recorded before the callback runs, so the record exists even when the
	// callback is nil — the menu still consumed the key.
	noteKeyEquivalent(keyCode)
	mu.Lock()
	cb := menubarActionCb
	mu.Unlock()
	if cb != nil {
		cb(id)
	}
}

// trayAction runs the app callback for an item of tray trayID.
func trayAction(trayID int, id string) {
	mu.Lock()
	cb := trayActionCbs[trayID]
	mu.Unlock()
	if cb != nil {
		cb(id)
	}
}

func sel(name string) objc.SEL { return darwinffi.Sel(name) }

// class returns the AppKit or Foundation class name. AppKit loads
// first: before it loads, GetClass finds no NSMenu and gives 0, and
// every send to 0 does nothing. Open is a map read after the first call.
func class(name string) objc.ID {
	darwinffi.Open(darwinffi.PathAppKit)
	return darwinffi.Class(name)
}

// nsApp returns the NSApp global. It is 0 before the app exists; a send
// to 0 returns 0.
func nsApp() objc.ID {
	return objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathAppKit), "NSApp"))
}

// text returns an autoreleased NSString for text that the user sees: a
// title, the app name, a tooltip. Bytes that are not valid UTF-8 show
// as U+FFFD. The cgo form gave such text to stringWithUTF8String:,
// which returns nil, and NSMenuItem raises an exception on a nil title,
// so the app stopped.
func text(s string) objc.ID {
	return darwinffi.NSString(strings.ToValidUTF8(s, "\uFFFD"))
}

// menuTarget returns the one target object of every custom item, on the
// menubar and on each tray. A menu item does not retain its target, so
// the object lives for the process, as the cgo singleton did. A class
// can be registered only once per process, so tests share it too.
var menuTarget = sync.OnceValue(func() objc.ID {
	darwinffi.Open(darwinffi.PathAppKit)
	cls, err := objc.RegisterClass("GoGuiMenuTarget",
		objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
			{Cmd: sel("menuItemClicked:"), Fn: menuItemClicked},
			{Cmd: sel("stripSystemEditItems:"), Fn: stripMainMenuEditItems},
		})
	if err != nil {
		panic("nativemenu: " + err.Error())
	}
	return objc.ID(cls).Send(sel("new"))
})

// menuItemClicked is the action of every custom item. The item's tag
// tells a tray item (the tray ID) from a menubar item (0). The cgo form
// had one target class per kind to carry the tray ID instead.
func menuItemClicked(_ objc.ID, _ objc.SEL, sender objc.ID) {
	idObj := sender.Send(sel("representedObject"))
	if idObj == 0 {
		return
	}
	id := darwinffi.GoString(uintptr(idObj)) // NSString is a CFString
	if trayID := objc.Send[int](sender, sel("tag")); trayID != 0 {
		trayAction(trayID, id)
		return
	}
	// A key equivalent fires inside [NSApp sendEvent:] for its key-down,
	// so the current event is that key-down. The metal backend already
	// stored the same key-down for Go before it called sendEvent:, and
	// uses this code to drop it — otherwise a shortcut shared by the menu
	// item and a command fires twice (a toggle opens and closes at once).
	keyCode := -1
	ev := nsApp().Send(sel("currentEvent"))
	if ev != 0 && objc.Send[uint](ev, sel("type")) == eventTypeKeyDown {
		keyCode = int(objc.Send[uint16](ev, sel("keyCode")))
	}
	menubarAction(id, keyCode)
}

// newMenu returns an autoreleased NSMenu.
func newMenu(title string) objc.ID {
	return class("NSMenu").Send(sel("alloc")).
		Send(sel("initWithTitle:"), text(title)).Send(sel("autorelease"))
}

// newItem returns an autoreleased NSMenuItem. action 0 is no action.
func newItem(title string, action objc.SEL, key string) objc.ID {
	return class("NSMenuItem").Send(sel("alloc")).
		Send(sel("initWithTitle:action:keyEquivalent:"),
			text(title), action, darwinffi.NSString(key)).
		Send(sel("autorelease"))
}

func separator() objc.ID {
	return class("NSMenuItem").Send(sel("separatorItem"))
}

// modifierMask maps encodeShortcut's mask (1=Cmd, 2=Shift, 4=Alt,
// 8=Ctrl) to NSEventModifierFlags.
func modifierMask(mods int) uint {
	var flags uint
	if mods&1 != 0 {
		flags |= modCommand
	}
	if mods&2 != 0 {
		flags |= modShift
	}
	if mods&4 != 0 {
		flags |= modOption
	}
	if mods&8 != 0 {
		flags |= modControl
	}
	return flags
}

// buildMenu returns an autoreleased NSMenu that holds items. tag goes on
// every item: the tray ID for a tray menu, 0 for the menubar.
func buildMenu(title string, items []gui.NativeMenuItemCfg, tag int) objc.ID {
	menu := newMenu(title)
	menu.Send(sel("setAutoenablesItems:"), false)
	target := menuTarget()
	clicked := sel("menuItemClicked:")
	for i := range items {
		it := &items[i]
		if it.Separator {
			menu.Send(sel("addItem:"), separator())
			continue
		}
		ch, mods := encodeShortcut(it.Shortcut)
		key := ""
		if ch != 0 {
			// NSMenuItem key equivalents are lowercase.
			key = strings.ToLower(string(rune(ch)))
		}
		// An item with a submenu opens it; it has no action of its own.
		hasSub := len(it.Submenu) > 0
		var action objc.SEL
		if !hasSub {
			action = clicked
		}
		mi := newItem(it.Text, action, key)
		mi.Send(sel("setKeyEquivalentModifierMask:"), modifierMask(mods))
		mi.Send(sel("setEnabled:"), !it.Disabled)
		state := 0
		if it.Checked {
			state = controlStateOn
		}
		mi.Send(sel("setState:"), state)
		mi.Send(sel("setTag:"), tag)
		if !hasSub {
			mi.Send(sel("setTarget:"), target)
		}
		if it.ID != "" {
			// An ID that is not valid UTF-8 gives 0: the item fires no
			// callback, as in the cgo form. A changed ID would name
			// nothing the app set.
			mi.Send(sel("setRepresentedObject:"), darwinffi.NSString(it.ID))
		}
		if hasSub {
			mi.Send(sel("setSubmenu:"), buildMenu(it.Text, it.Submenu, tag))
		}
		menu.Send(sel("addItem:"), mi)
	}
	return menu
}

// stdItem is one item of a standard menu. An empty title is a separator.
type stdItem struct {
	title, action, key string
	mods               uint
}

// editItems are the standard Edit menu items. They go to the responder
// chain, so a text view handles them.
var editItems = []stdItem{
	{"Undo", "undo:", "z", modCommand},
	{"Redo", "redo:", "Z", modCommand | modShift},
	{},
	{"Cut", "cut:", "x", modCommand},
	{"Copy", "copy:", "c", modCommand},
	{"Paste", "paste:", "v", modCommand},
	{},
	{"Select All", "selectAll:", "a", modCommand},
}

// windowItems mirror the menu the metal backend installs by default, so an
// app that replaces the default menubar with SetNativeMenubar doesn't silently
// lose Cmd+W / Cmd+M.
var windowItems = []stdItem{
	{"Close", "performClose:", "w", modCommand},
	{"Minimize", "performMiniaturize:", "m", modCommand},
	{"Zoom", "performZoom:", "", 0},
	{},
	{"Bring All to Front", "arrangeInFront:", "", 0},
}

// stdMenu returns a menubar item whose submenu holds items.
func stdMenu(title string, items []stdItem) objc.ID {
	menu := newMenu(title)
	for _, s := range items {
		if s.title == "" {
			menu.Send(sel("addItem:"), separator())
			continue
		}
		mi := newItem(s.title, sel(s.action), s.key)
		if s.key != "" {
			mi.Send(sel("setKeyEquivalentModifierMask:"), s.mods)
		}
		menu.Send(sel("addItem:"), mi)
	}
	item := newItem(title, 0, "")
	item.Send(sel("setSubmenu:"), menu)
	return item
}

// appMenu returns the app menu: About, then Quit.
func appMenu(name, aboutActionID string, omitAboutItem bool) objc.ID {
	menu := newMenu(name)
	// omitAboutItem leaves the app menu with Quit only — for apps that
	// place About under their own Help menu. The separator goes with it:
	// a leading separator above Quit reads as a rendering bug.
	if !omitAboutItem {
		title := "About " + name
		var about objc.ID
		if aboutActionID != "" {
			// Route through the user action callback so the app can show
			// its own About dialog instead of the system About panel.
			// Deliberate deviation from the AppKit-selector pattern used
			// by Quit: About content is app-defined, not OS-defined.
			about = newItem(title, sel("menuItemClicked:"), "")
			about.Send(sel("setTarget:"), menuTarget())
			about.Send(sel("setRepresentedObject:"), darwinffi.NSString(aboutActionID))
		} else {
			about = newItem(title, sel("orderFrontStandardAboutPanel:"), "")
		}
		menu.Send(sel("addItem:"), about)
		menu.Send(sel("addItem:"), separator())
	}
	quit := newItem("Quit "+name, sel("terminate:"), "q")
	quit.Send(sel("setKeyEquivalentModifierMask:"), uint(modCommand))
	menu.Send(sel("addItem:"), quit)

	item := newItem(name, 0, "")
	item.Send(sel("setSubmenu:"), menu)
	return item
}

// itemTitle returns the title of the item at index i of menu.
func itemTitle(menu objc.ID, i int) string {
	t := menu.Send(sel("itemAtIndex:"), i).Send(sel("title"))
	return darwinffi.GoString(uintptr(t))
}

// buildMenubar returns the autoreleased menubar for cfg, and the Window
// submenu (0 when cfg has none) for setWindowsMenu:.
func buildMenubar(cfg gui.NativeMenubarCfg) (bar, windows objc.ID) {
	bar = newMenu("")
	bar.Send(sel("addItem:"), appMenu(cfg.AppName, cfg.AboutActionID, cfg.OmitAboutItem))

	for i := range cfg.Menus {
		m := &cfg.Menus[i]
		top := newItem(m.Title, 0, "")
		top.Send(sel("setSubmenu:"), buildMenu(m.Title, m.Items, 0))
		bar.Send(sel("addItem:"), top)
	}

	if cfg.IncludeEditMenu {
		// Insert after File if present, or at index 1.
		idx := 1
		if objc.Send[int](bar, sel("numberOfItems")) > 1 {
			idx = 2
		}
		bar.Send(sel("insertItem:atIndex:"), stdMenu("Edit", editItems), idx)
	}

	// macOS convention puts the Window menu second to last, immediately
	// before Help — so insert before a user-supplied Help menu when there
	// is one, else append.
	if cfg.IncludeWindowMenu {
		n := objc.Send[int](bar, sel("numberOfItems"))
		idx := n
		for i := range n {
			if itemTitle(bar, i) == "Help" {
				idx = i
				break
			}
		}
		wm := stdMenu("Window", windowItems)
		bar.Send(sel("insertItem:atIndex:"), wm, idx)
		windows = wm.Send(sel("submenu"))
	}
	return bar, windows
}

// isSystemEditItem reports whether title names an item that macOS adds
// to the Edit menu by itself.
func isSystemEditItem(title string) bool {
	return title == "Start Dictation…" || title == "Start Dictation" ||
		strings.HasPrefix(title, "Writing Tools") ||
		strings.HasPrefix(title, "AutoFill")
}

// stripSystemEditItems removes the items that macOS adds (AutoFill,
// Writing Tools, Start Dictation) from the Edit menu of bar, and then a
// separator that the removal leaves at the end.
func stripSystemEditItems(bar objc.ID) {
	n := objc.Send[int](bar, sel("numberOfItems"))
	for i := range n {
		top := bar.Send(sel("itemAtIndex:"), i)
		edit := top.Send(sel("submenu"))
		if itemTitle(bar, i) != "Edit" || edit == 0 {
			continue
		}
		// Back to front, so a removal does not move the items still to
		// test.
		for j := objc.Send[int](edit, sel("numberOfItems")) - 1; j >= 0; j-- {
			if isSystemEditItem(itemTitle(edit, j)) {
				edit.Send(sel("removeItemAtIndex:"), j)
			}
		}
		last := objc.Send[int](edit, sel("numberOfItems")) - 1
		if last >= 0 && objc.Send[bool](edit.Send(sel("itemAtIndex:"), last),
			sel("isSeparatorItem")) {
			edit.Send(sel("removeItemAtIndex:"), last)
		}
		break
	}
}

// stripMainMenuEditItems is the deferred form of stripSystemEditItems,
// on the installed menubar.
func stripMainMenuEditItems(_ objc.ID, _ objc.SEL, _ objc.ID) {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	stripSystemEditItems(nsApp().Send(sel("mainMenu")))
}

func encodeShortcut(s gui.Shortcut) (byte, int) {
	if !s.IsSet() {
		return 0, 0
	}
	var ch byte
	k := s.Key
	switch {
	case k >= gui.KeyA && k <= gui.KeyZ:
		ch = byte('A' + (k - gui.KeyA))
	case k >= gui.Key0 && k <= gui.Key9:
		ch = byte('0' + (k - gui.Key0))
	// Punctuation keys pass straight through: each is a valid AppKit key
	// equivalent, and buildMenu's lowercase fold is a no-op for them.
	// Spelled out per key rather than by ASCII range — the KeyCode values
	// only coincide with ASCII by convention (see event.go).
	case k == gui.KeySpace:
		ch = ' '
	case k == gui.KeyApostrophe:
		ch = '\''
	case k == gui.KeyComma:
		ch = ','
	case k == gui.KeyMinus:
		ch = '-'
	case k == gui.KeyPeriod:
		ch = '.'
	case k == gui.KeySlash:
		ch = '/'
	case k == gui.KeySemicolon:
		ch = ';'
	case k == gui.KeyEqual:
		ch = '='
	case k == gui.KeyLeftBracket:
		ch = '['
	case k == gui.KeyBackslash:
		ch = '\\'
	case k == gui.KeyRightBracket:
		ch = ']'
	case k == gui.KeyGraveAccent:
		ch = '`'
	default:
		return 0, 0
	}
	var mods int
	if s.Modifiers.Has(gui.ModSuper) {
		mods |= 1
	}
	if s.Modifiers.Has(gui.ModShift) {
		mods |= 2
	}
	if s.Modifiers.Has(gui.ModAlt) {
		mods |= 4
	}
	if s.Modifiers.Has(gui.ModCtrl) {
		mods |= 8
	}
	return ch, mods
}

// SetMenubar installs a native macOS menubar.
func SetMenubar(
	cfg gui.NativeMenubarCfg, actionCb func(string),
) {
	mu.Lock()
	menubarActionCb = actionCb
	mu.Unlock()

	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	bar, windows := buildMenubar(cfg)
	app := nsApp()
	if windows != 0 {
		// AppKit appends the open-window list to this menu.
		app.Send(sel("setWindowsMenu:"), windows)
	}
	app.Send(sel("setMainMenu:"), bar)

	if cfg.SuppressSystemEditItems {
		// macOS adds its items after setMainMenu: returns, so remove them
		// on a later pass of the main run loop. The cgo form used
		// dispatch_async to the main queue, which the same run loop
		// drains.
		menuTarget().Send(sel("performSelectorOnMainThread:withObject:waitUntilDone:"),
			sel("stripSystemEditItems:"), objc.ID(0), false)
	}
}

// ClearMenubar removes the native menubar.
func ClearMenubar() {
	mu.Lock()
	menubarActionCb = nil
	mu.Unlock()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	nsApp().Send(sel("setMainMenu:"), newMenu(""))
}

// imageFromPNG returns an autoreleased template NSImage of the standard
// status item size, or 0 when b is empty or not an image.
func imageFromPNG(b []byte) objc.ID {
	if len(b) == 0 {
		return 0
	}
	// dataWithBytes:length: copies b before it returns.
	data := class("NSData").Send(sel("dataWithBytes:length:"),
		unsafe.Pointer(&b[0]), uint(len(b)))
	img := class("NSImage").Send(sel("alloc")).Send(sel("initWithData:"), data)
	if img == 0 {
		return 0
	}
	img.Send(sel("autorelease"))
	// Template images adapt to menu bar appearance.
	img.Send(sel("setTemplate:"), true)
	img.Send(sel("setSize:"), nsSize{Width: 18, Height: 18})
	return img
}

// configureTray sets the icon, tooltip and menu of status item si. An
// empty icon, tooltip or menu keeps the one si has, as in the cgo form.
func configureTray(si objc.ID, trayID int, cfg gui.SystemTrayCfg) {
	button := si.Send(sel("button"))
	if img := imageFromPNG(cfg.IconPNG); img != 0 {
		button.Send(sel("setImage:"), img)
	}
	if cfg.Tooltip != "" {
		button.Send(sel("setToolTip:"), text(cfg.Tooltip))
	}
	if len(cfg.Menu) > 0 {
		si.Send(sel("setMenu:"), buildMenu("", cfg.Menu, trayID))
	}
}

// CreateSystemTray creates a tray icon with menu.
func CreateSystemTray(
	cfg gui.SystemTrayCfg, actionCb func(string),
) (int, error) {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	mu.Lock()
	trayID := nextTrayID
	nextTrayID++
	mu.Unlock()

	si := class("NSStatusBar").Send(sel("systemStatusBar")).
		Send(sel("statusItemWithLength:"), variableStatusItemLength).
		Send(sel("retain")) // released by RemoveSystemTray
	configureTray(si, trayID, cfg)

	mu.Lock()
	trays[trayID] = si
	trayActionCbs[trayID] = actionCb
	mu.Unlock()
	return trayID, nil
}

// UpdateSystemTray updates an existing tray entry.
func UpdateSystemTray(id int, cfg gui.SystemTrayCfg) {
	mu.Lock()
	si := trays[id]
	mu.Unlock()
	if si == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	configureTray(si, id, cfg)
}

// RemoveSystemTray removes a tray icon.
func RemoveSystemTray(id int) {
	mu.Lock()
	si := trays[id]
	delete(trays, id)
	delete(trayActionCbs, id)
	mu.Unlock()
	if si == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	class("NSStatusBar").Send(sel("systemStatusBar")).Send(sel("removeStatusItem:"), si)
	si.Send(sel("release"))
}
