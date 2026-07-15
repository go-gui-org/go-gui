package gui

import "testing"

func TestCommandPaletteHidden(t *testing.T) {
	w := &Window{}
	v := CommandPalette(CommandPaletteCfg{
		ID: "cp-hidden",
		Items: []CommandPaletteItem{
			{ID: "save", Label: "Save"},
		},
		OnAction: func(_ string, _ *Event, _ *Window) {},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.shapeType == shapeDrawCanvas {
		t.Error("hidden palette should not be canvas")
	}
}

func TestCommandPaletteVisible(t *testing.T) {
	w := &Window{}
	CommandPaletteShow("__cmd_palette__", w)

	v := CommandPalette(CommandPaletteCfg{
		ID: "__cmd_palette__",
		Items: []CommandPaletteItem{
			{ID: "save", Label: "Save", Detail: "Ctrl+S"},
			{ID: "open", Label: "Open", Detail: "Ctrl+O"},
		},
		OnAction: func(_ string, _ *Event, _ *Window) {},
	})
	layout := generateViewLayout(v, w)
	if len(layout.Children) == 0 {
		t.Error("visible palette should have children")
	}
}

func TestCommandPaletteShowDismiss(t *testing.T) {
	w := &Window{}
	id := "cp-sd"
	CommandPaletteShow(id, w)
	if !CommandPaletteIsVisible(id, w) {
		t.Error("expected visible after show")
	}
	CommandPaletteDismiss(id, w)
	if CommandPaletteIsVisible(id, w) {
		t.Error("expected hidden after dismiss")
	}
}

func TestCommandPaletteToggle(t *testing.T) {
	w := &Window{}
	id := "cp-tog"
	CommandPaletteToggle(id, w)
	if !CommandPaletteIsVisible(id, w) {
		t.Error("first toggle should show")
	}
	CommandPaletteToggle(id, w)
	if CommandPaletteIsVisible(id, w) {
		t.Error("second toggle should hide")
	}
}

func TestPaletteOnKeyDownEscape(t *testing.T) {
	w := &Window{}
	CommandPaletteShow("cp-esc", w)
	dismissed := false
	e := &Event{KeyCode: KeyEscape}
	items := []listCoreItem{{ID: "a", Label: "A"}}
	paletteOnKeyDown("cp-esc", nil,
		func(_ *Window) { dismissed = true },
		items, []string{"a"}, e, w)
	if !dismissed {
		t.Error("escape should dismiss")
	}
	if CommandPaletteIsVisible("cp-esc", w) {
		t.Error("should be hidden after escape")
	}
}

func TestPaletteOnKeyDownSelect(t *testing.T) {
	w := &Window{}
	CommandPaletteShow("cp-sel", w)
	selected := ""
	e := &Event{KeyCode: KeyEnter}
	items := []listCoreItem{
		{ID: "cmd1", Label: "Cmd1"},
		{ID: "cmd2", Label: "Cmd2"},
	}
	paletteOnKeyDown("cp-sel",
		func(id string, _ *Event, _ *Window) { selected = id },
		nil,
		items, []string{"cmd1", "cmd2"}, e, w)
	if selected != "cmd1" {
		t.Errorf("selected = %q, want cmd1", selected)
	}
}

func TestPaletteOnKeyDownDisabledBlocked(t *testing.T) {
	w := &Window{}
	CommandPaletteShow("cp-dis", w)
	selected := ""
	e := &Event{KeyCode: KeyEnter}
	items := []listCoreItem{
		{ID: "cmd1", Label: "Cmd1", Disabled: true},
	}
	paletteOnKeyDown("cp-dis",
		func(id string, _ *Event, _ *Window) { selected = id },
		nil,
		items, []string{"cmd1"}, e, w)
	if selected != "" {
		t.Errorf("disabled item should not be selectable, got %q", selected)
	}
	if !e.IsHandled {
		t.Error("Enter on disabled item should still mark event handled")
	}
}

func TestPaletteOnKeyDownArrowCycle(t *testing.T) {
	w := &Window{}
	id := "cp-nav"
	CommandPaletteShow(id, w)
	items := []listCoreItem{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B"},
		{ID: "c", Label: "C"},
	}
	ids := []string{"a", "b", "c"}

	// Arrow down from 0 -> 1.
	e := &Event{KeyCode: KeyDown}
	paletteOnKeyDown(id, nil, nil, items, ids, e, w)
	sh := StateMap[string, int](w, nsCmdPaletteHighlight, capModerate)
	cur, _ := sh.Get(id)
	if cur != 1 {
		t.Errorf("after down: highlight = %d, want 1", cur)
	}

	// Arrow down again -> 2.
	e = &Event{KeyCode: KeyDown}
	paletteOnKeyDown(id, nil, nil, items, ids, e, w)
	cur, _ = sh.Get(id)
	if cur != 2 {
		t.Errorf("after second down: highlight = %d, want 2", cur)
	}

	// Arrow up -> 1.
	e = &Event{KeyCode: KeyUp}
	paletteOnKeyDown(id, nil, nil, items, ids, e, w)
	cur, _ = sh.Get(id)
	if cur != 1 {
		t.Errorf("after up: highlight = %d, want 1", cur)
	}
}

func TestPaletteQueryFiltering(t *testing.T) {
	w := &Window{}
	id := "cp-filter"
	CommandPaletteShow(id, w)

	makeView := func(query string) {
		StateMap[string, string](w, nsCmdPaletteQuery, capModerate).
			Set(id, query)
		v := CommandPalette(CommandPaletteCfg{
			ID: id,
			Items: []CommandPaletteItem{
				{ID: "save", Label: "Save"},
				{ID: "open", Label: "Open"},
				{ID: "search", Label: "Search"},
			},
			OnAction: func(_ string, _ *Event, _ *Window) {},
		})
		_ = generateViewLayout(v, w)
	}

	// Filter for "sa" should narrow results.
	makeView("sa")
	cm := StateMapRead[string, *cmdPaletteItemsCache](w, nsCmdPaletteItems)
	cache, _ := cm.Get(id)
	if len(cache.filtered) >= 3 {
		t.Errorf("filtering 'sa': got %d items, expected fewer than 3",
			len(cache.filtered))
	}
}

func TestCommandPaletteItemsCacheInvalidatesOnItemsChange(t *testing.T) {
	w := &Window{}
	id := "cp-cache"
	CommandPaletteShow(id, w)

	v := CommandPalette(CommandPaletteCfg{
		ID: id,
		Items: []CommandPaletteItem{
			{ID: "a", Label: "A"},
		},
		OnAction: func(_ string, _ *Event, _ *Window) {},
	})
	_ = generateViewLayout(v, w)

	cm := StateMapRead[string, *cmdPaletteItemsCache](w, nsCmdPaletteItems)
	if cm == nil {
		t.Fatal("expected command palette items cache map")
	}
	cache, ok := cm.Get(id)
	if !ok || cache == nil {
		t.Fatal("expected command palette cache entry")
	}
	if got := len(cache.items); got != 1 {
		t.Fatalf("cache items len = %d, want 1", got)
	}

	v = CommandPalette(CommandPaletteCfg{
		ID: id,
		Items: []CommandPaletteItem{
			{ID: "a", Label: "A"},
			{ID: "b", Label: "B"},
		},
		OnAction: func(_ string, _ *Event, _ *Window) {},
	})
	_ = generateViewLayout(v, w)
	cache, _ = cm.Get(id)
	if got := len(cache.items); got != 2 {
		t.Fatalf("cache items len = %d, want 2", got)
	}
}

func TestCommandPaletteCacheInvalidatesOnContentChange(t *testing.T) {
	w := &Window{}
	id := "cp-content"
	CommandPaletteShow(id, w)

	makeView := func(label string) uint64 {
		v := CommandPalette(CommandPaletteCfg{
			ID: id,
			Items: []CommandPaletteItem{
				{ID: "x", Label: label},
			},
			OnAction: func(_ string, _ *Event, _ *Window) {},
		})
		_ = generateViewLayout(v, w)
		cm := StateMapRead[string, *cmdPaletteItemsCache](
			w, nsCmdPaletteItems)
		c, _ := cm.Get(id)
		return c.sourceHash
	}

	h1 := makeView("Alpha")
	h2 := makeView("Beta")
	if h1 == h2 {
		t.Error("hash should change when label changes")
	}
}

func TestPaletteScrollResetOnShow(t *testing.T) {
	w := &Window{}
	id := "cp-scroll"
	idScroll := id + ":scroll"

	// Set scroll position to simulate previous scrolling.
	sy := w.scrollY()
	sy.Set(idScroll, 150)

	CommandPaletteShow(id, w)
	scrollY, _ := sy.Get(idScroll)
	if scrollY != 0 {
		t.Errorf("scroll should reset on show, got %v", scrollY)
	}
}

func TestPaletteBackdropDismiss(t *testing.T) {
	w := &Window{}
	id := "cp-backdrop"
	CommandPaletteShow(id, w)

	dismissed := false
	v := CommandPalette(CommandPaletteCfg{
		ID:       id,
		Items:    []CommandPaletteItem{{ID: "a", Label: "A"}},
		OnAction: func(_ string, _ *Event, _ *Window) {},
		OnDismiss: func(_ *Window) {
			dismissed = true
		},
	})
	layout := generateViewLayout(v, w)

	// The outermost child is the backdrop column with OnClick.
	if layout.Shape.events == nil || layout.Shape.events.OnClick == nil {
		t.Fatal("backdrop should have OnClick")
	}
	e := &Event{}
	layout.Shape.events.OnClick(&layout, e, w)
	if !dismissed {
		t.Error("backdrop click should trigger OnDismiss")
	}
	if !e.IsHandled {
		t.Error("backdrop click should mark event handled")
	}
	if CommandPaletteIsVisible(id, w) {
		t.Error("palette should be hidden after backdrop click")
	}
}

func TestPaletteOnEnterSelectsHighlighted(t *testing.T) {
	w := &Window{}
	id := "cp-enter"
	CommandPaletteShow(id, w)

	// Highlight index 1.
	sh := StateMap[string, int](w, nsCmdPaletteHighlight, capModerate)
	sh.Set(id, 1)

	selected := ""
	items := []listCoreItem{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B"},
	}
	ids := []string{"a", "b"}
	onEnter := makePaletteOnEnter(id,
		func(itemID string, _ *Event, _ *Window) { selected = itemID },
		nil, items, ids)

	e := &Event{KeyCode: KeyEnter}
	onEnter(nil, e, w)
	if selected != "b" {
		t.Errorf("OnEnter selected = %q, want b", selected)
	}
	if CommandPaletteIsVisible(id, w) {
		t.Error("palette should dismiss after Enter select")
	}
}

func TestPaletteOnEnterDisabledBlocked(t *testing.T) {
	w := &Window{}
	id := "cp-enter-dis"
	CommandPaletteShow(id, w)

	selected := ""
	items := []listCoreItem{
		{ID: "a", Label: "A", Disabled: true},
	}
	ids := []string{"a"}
	onEnter := makePaletteOnEnter(id,
		func(itemID string, _ *Event, _ *Window) { selected = itemID },
		nil, items, ids)

	e := &Event{KeyCode: KeyEnter}
	onEnter(nil, e, w)
	if selected != "" {
		t.Errorf("disabled item should not select, got %q", selected)
	}
}

func TestPaletteKeyboardDispatchIntegration(t *testing.T) {
	w := &Window{}
	id := "cp-integ"
	CommandPaletteShow(id, w)

	selected := ""
	v := CommandPalette(CommandPaletteCfg{
		ID:         id,
		Scrollable: true,
		Items: []CommandPaletteItem{
			{ID: "alpha", Label: "Alpha"},
			{ID: "beta", Label: "Beta"},
			{ID: "gamma", Label: "Gamma"},
		},
		OnAction: func(itemID string, _ *Event, _ *Window) {
			selected = itemID
		},
	})
	layout := generateViewLayout(v, w)

	// Arrow down through keydownHandler (full dispatch).
	e := &Event{KeyCode: KeyDown, Type: EventKeyDown}
	keydownHandler(&layout, e, w)
	if !e.IsHandled {
		t.Fatal("arrow down should be handled")
	}
	sh := StateMap[string, int](w, nsCmdPaletteHighlight, capModerate)
	cur, _ := sh.Get(id)
	if cur != 1 {
		t.Errorf("highlight after down = %d, want 1", cur)
	}

	// Re-generate layout after state change.
	layout = generateViewLayout(v, w)

	// Enter through keydownHandler to select.
	e = &Event{KeyCode: KeyEnter, Type: EventKeyDown}
	keydownHandler(&layout, e, w)
	if selected != "beta" {
		t.Errorf("selected = %q, want beta", selected)
	}

	// Palette should be dismissed.
	if CommandPaletteIsVisible(id, w) {
		t.Error("palette should be dismissed after Enter")
	}
}

func TestPaletteFloatZIndexDefault(t *testing.T) {
	cfg := CommandPaletteCfg{}
	applyCommandPaletteDefaults(&cfg)
	if cfg.FloatZIndex != 1000 {
		t.Errorf("FloatZIndex = %d, want 1000", cfg.FloatZIndex)
	}
}
