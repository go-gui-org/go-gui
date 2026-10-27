package gui

// input_context_menu.go — the built-in Cut/Copy/Paste/Select All menu
// a right press opens on an Input (issue #1151).
//
// The menu is a window-level overlay, not a ContextMenu wrapped around
// each field. A window shows at most one context menu, so one state slot
// on the window is enough, and an Input pays nothing on a frame where
// the menu is closed: no wrapper container, no item slice, no clipboard
// read. Wrapping each field would also change every Input's layout tree,
// which composites and tests read by position.

// inputMenuFocusID is the menu's focus ID. Absolute (it contains ':'),
// because the menu is injected outside the main tree and the event
// handlers that move focus to it run with no scope in hand.
const inputMenuFocusID = "gui:input:context_menu"

// Item IDs. They are scoped under inputMenuFocusID by the menu build.
const (
	inputMenuCut       = "cut"
	inputMenuCopy      = "copy"
	inputMenuPaste     = "paste"
	inputMenuSelectAll = "select_all"
)

// inputMenuState is the one open (or just closed) input menu of a
// window. owner outlives open on purpose: dismissPopups clears open on
// a press or a window unfocus, and the arrange pass still needs owner
// to hand focus back to the field when nothing else took it.
type inputMenuState struct {
	// hcfg is the owning field's handler config from the frame the menu
	// opened in. The menu actions reuse its mask, validators and change
	// callback, so a paste through the menu obeys the same rules as
	// Ctrl+V.
	hcfg  *inputHandlerCfg
	owner string // effective ID of the field
	// fieldState is the field's caret and selection when the menu
	// opened. A real focus change clears input selections window-wide
	// (setFocusLocked), and focus moves to the menu, then on the press
	// that picks an item it is cleared (dismissPopups), then it comes
	// back. Each move would drop the selection Copy and Cut act on, so
	// the field gets this back whenever focus returns to it.
	fieldState inputState
	scrollID   string // effective scroll ID of the field, or ""
	x, y       float32
	// Item enable states, decided once when the menu opens. The
	// clipboard is read then and not on every frame: on some platforms
	// that read is a round trip to another process.
	canCut, canCopy, canPaste, canSelectAll bool
	open                                    bool
}

// inputMenuOnRightClick opens the menu for a right press on a field
// (inputOnClick). Caret and selection stay where they are, so Copy and
// Cut act on what the user selected.
func inputMenuOnRightClick(
	ctx EventCtx, leafID, leafScrollID string, hcfg *inputHandlerCfg,
) {
	// ctx.Layout is the field, which claims the ID. Its text is read
	// there, placeholder excluded.
	owner := ctx.EffID(leafID)
	if owner == "" {
		return
	}
	text := inputTextFromLayout(ctx.Layout)
	// MouseX/Y are local to the field; the menu is placed in window
	// coordinates.
	x := ctx.Layout.Shape.X + ctx.Event.MouseX
	y := ctx.Layout.Shape.Y + ctx.Event.MouseY
	openInputMenu(ctx.Window, hcfg, owner, ctx.EffID(leafScrollID), text, x, y)
	ctx.Consume()
}

// inputMenuReturnFocus gives focus back to the menu's field with the
// caret and selection it had when the menu opened. locked is true under
// the frame lock, where SetFocus would deadlock.
func inputMenuReturnFocus(w *Window, st *inputMenuState, locked bool) {
	if locked {
		w.setFocusLocked(st.owner)
	} else {
		w.SetFocus(st.owner)
	}
	inputStoreState(st.owner, w, st.fieldState)
}

// openInputMenu opens the menu for the field owner at window position
// (x, y) and moves focus to it. text is the field's current text.
func openInputMenu(
	w *Window, hcfg *inputHandlerCfg, owner, scrollID, text string,
	x, y float32,
) {
	is := inputStateOrDefault(owner, w)
	// A right press while this field's menu is open reopens it. That
	// press closed the menu and cleared focus first (dismissPopups keeps
	// owner), which cleared the selection, so start from the state the
	// menu saved. The field took no keys while its menu was open, so that
	// state is still current.
	if prev := &w.viewState.inputMenu; prev.owner == owner {
		is = prev.fieldState
	}
	hasSel := is.selectBeg != is.selectEnd
	// Copy follows inputCopy: a password field never hands out its text.
	canCopy := hasSel && !hcfg.IsPassword
	w.viewState.inputMenu = inputMenuState{
		hcfg:         hcfg,
		owner:        owner,
		fieldState:   is,
		scrollID:     scrollID,
		x:            x,
		y:            y,
		canCut:       canCopy && !hcfg.ReadOnly,
		canCopy:      canCopy,
		canPaste:     !hcfg.ReadOnly && w.GetClipboard() != "",
		canSelectAll: text != "",
		open:         true,
	}
	// The menu takes the keys while open. inputMenuHolds keeps the
	// field looking focused and holds off its blur meanwhile.
	w.SetFocus(inputMenuFocusID)
	inputStoreState(owner, w, is)
}

// inputMenuHolds reports whether the input menu is open over the field
// key. The field treats that as still focused: opening the menu must
// not fire the field's blur commit, as it does not in a native toolkit.
func inputMenuHolds(w *Window, key string) bool {
	st := &w.viewState.inputMenu
	return st.open && st.owner == key && w.IsFocus(inputMenuFocusID)
}

// inputMenuOverlay returns the menu layer for layoutArrange to inject,
// or nil when no menu is open. It also settles focus for a menu that
// just closed. It runs under the frame lock, so it moves focus with
// setFocusLocked.
func inputMenuOverlay(w *Window) View {
	st := &w.viewState.inputMenu
	if st.owner == "" {
		return nil
	}
	focus := w.FocusID()
	if !st.open {
		// Closed by dismissPopups while the menu still held focus (the
		// window lost focus): give it back to the field. A press closes
		// the menu too, but it clears focus first, so it takes this
		// branch only as a plain cleanup.
		if focus == inputMenuFocusID {
			inputMenuReturnFocus(w, st, true)
		}
		*st = inputMenuState{}
		return nil
	}
	if focus != inputMenuFocusID {
		// Focus left the menu. Escape clears focus (menuOnKeyDown), and
		// then the field gets it back; Tab or a programmatic SetFocus
		// moved it somewhere on purpose, and it stays there.
		if focus == "" {
			inputMenuReturnFocus(w, st, true)
		}
		*st = inputMenuState{}
		return nil
	}
	// w.layout is the last frame's composed tree, every layer in it: a
	// field inside a dialog lives in the dialog layer, not the main tree
	// this frame is arranging. With no last frame there is nothing to
	// compare against.
	if _, ok := w.layout.findByID(st.owner); !ok && w.layout.Shape != nil {
		// The field is gone from the tree; the menu has nothing to act on.
		w.setFocusLocked("")
		*st = inputMenuState{}
		return nil
	}
	return inputMenuView(w, *st)
}

// inputMenuView builds the menu. It hangs from a zero-size anchor at the
// press point, because a top-level float is placed against the window
// rect and never auto-flips (floatAttachLayout). As a child of the
// anchor the menu flips back inside the window near an edge, like
// ContextMenu's popup.
func inputMenuView(w *Window, st inputMenuState) View {
	items := []MenuItemCfg{
		{ID: inputMenuCut, Text: "Cut", disabled: !st.canCut},
		{ID: inputMenuCopy, Text: "Copy", disabled: !st.canCopy},
		{ID: inputMenuPaste, Text: "Paste", disabled: !st.canPaste},
		MenuSeparator(),
		{ID: inputMenuSelectAll, Text: "Select All", disabled: !st.canSelectAll},
	}
	popup := menu(w, MenubarCfg{
		ID:            inputMenuFocusID,
		Items:         items,
		Action:        func(id string, ctx EventCtx) { inputMenuAction(st, id, ctx.Window) },
		Float:         true,
		FloatAutoFlip: true,
		FloatAnchor:   FloatTopLeft,
		FloatTieOff:   FloatTopLeft,
	})
	return Column(ContainerCfg{
		Float:        true,
		FloatAnchor:  FloatTopLeft,
		FloatTieOff:  FloatTopLeft,
		FloatOffsetX: st.x,
		FloatOffsetY: st.y,
		Sizing:       FixedFixed,
		Padding:      NoPadding,
		SizeBorder:   NoBorder,
		Spacing:      NoSpacing,
		Content:      []View{popup},
	})
}

// inputMenuAction runs one menu item against the owning field. st is
// the state the menu opened with: a press closes the menu state before
// the item's click handler runs (dismissPopups), so the action cannot
// read it back from the window.
func inputMenuAction(st inputMenuState, id string, w *Window) {
	// The field's handler config as of the last frame, not the one the
	// menu opened with: inputAmendLayout refreshes it while the menu is
	// open, and dismissPopups keeps it. Ctrl+X and Ctrl+V run the current
	// frame's config too.
	if cur := &w.viewState.inputMenu; cur.owner == st.owner && cur.hcfg != nil {
		st.hcfg = cur.hcfg
	}
	w.viewState.inputMenu = inputMenuState{}
	inputMenuReturnFocus(w, &st, false)
	ly, ok := w.layout.findByID(st.owner)
	if !ok {
		return
	}
	h := st.hcfg
	text := inputTextFromLayout(ly)
	next, changed := text, false
	switch id {
	case inputMenuCut:
		// The flags were decided at open; ReadOnly is read again in
		// case the field turned read-only while the menu was open.
		if !st.canCut || h.ReadOnly {
			return
		}
		if nt, copied, cut := inputCut(text, st.owner, h.IsPassword, w); cut {
			w.SetClipboard(copied)
			next, changed = nt, true
		}
	case inputMenuCopy:
		if !st.canCopy {
			return
		}
		if copied, ok := inputCopy(text, st.owner, h.IsPassword, w); ok {
			w.SetClipboard(copied)
		}
	case inputMenuPaste:
		if !st.canPaste || h.ReadOnly {
			return
		}
		// Read again: the clipboard may have changed while the menu
		// was open. inputKeyPaste applies the mask, PreTextChange and
		// the rune budget, as Ctrl+V does.
		next, changed = inputKeyPaste(
			text, w.GetClipboard(), st.owner, h.CompiledMask, *h, ly, w)
	case inputMenuSelectAll:
		if !st.canSelectAll {
			return
		}
		inputSelectAll(text, st.owner, w)
	default:
		return
	}
	resetBlinkCursorVisible(w)
	if changed {
		h.fireTextChanged(ly, next, w)
	}
	if st.scrollID != "" {
		inputScrollCursorIntoView(st.scrollID, next, ly, w)
	}
}
