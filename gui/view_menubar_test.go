package gui

import "testing"

func TestMenubarLayout(t *testing.T) {
	w := &Window{}
	cfg := MenubarCfg{
		ID: "mb",
		Items: []MenuItemCfg{
			MenuItemText("file", "File"),
			MenuItemText("edit", "Edit"),
		},
	}
	view := Menubar(w, cfg)
	layout := generateViewLayout(view, w)

	if layout.Shape == nil {
		t.Fatal("nil shape")
	}
	if layout.Shape.ID != "mb" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
	if layout.Shape.Axis != axisLeftToRight {
		t.Errorf("axis = %d, want LeftToRight", layout.Shape.Axis)
	}
	// Should have at least 2 children (one per item).
	if len(layout.Children) < 2 {
		t.Errorf("children = %d, want >= 2",
			len(layout.Children))
	}
}

func TestMenubarKeydownEscape(t *testing.T) {
	w := &Window{}
	w.viewState.focusID = "mb"
	sm := StateMap[string, string](w, nsMenu, capModerate)
	sm.Set("mb", "file")

	cfg := MenubarCfg{
		ID: "mb",
		Items: []MenuItemCfg{
			MenuItemText("file", "File"),
		},
	}

	e := &Event{Type: EventKeyDown, KeyCode: KeyEscape}
	menuOnKeyDown(cfg, menuMapper, e, w)

	if e.IsHandled != true {
		t.Error("escape should be handled")
	}
	if w.viewState.focusID != "" {
		t.Error("focus should be cleared")
	}
	sel, _ := sm.Get("mb")
	if sel != "" {
		t.Errorf("selection = %q, want empty", sel)
	}
}

func TestMenubarKeydownNavigation(t *testing.T) {
	w := &Window{}
	w.viewState.focusID = "mb"
	sm := StateMap[string, string](w, nsMenu, capModerate)
	sm.Set("mb", "file")

	cfg := MenubarCfg{
		ID: "mb",
		Items: []MenuItemCfg{
			MenuItemText("file", "File"),
			MenuItemText("edit", "Edit"),
			MenuItemText("view", "View"),
		},
	}

	// Right arrow: file -> edit.
	e := &Event{Type: EventKeyDown, KeyCode: KeyRight}
	menuOnKeyDown(cfg, menuMapper, e, w)
	sel, _ := sm.Get("mb")
	if sel != "edit" {
		t.Errorf("after Right: sel = %q, want edit", sel)
	}

	// Left arrow: edit -> file.
	e = &Event{Type: EventKeyDown, KeyCode: KeyLeft}
	menuOnKeyDown(cfg, menuMapper, e, w)
	sel, _ = sm.Get("mb")
	if sel != "file" {
		t.Errorf("after Left: sel = %q, want file", sel)
	}
}

func TestMenubarAmendLayoutClearOnDefocus(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, string](w, nsMenu, capModerate)
	sm.Set("mb", "file")

	amend := makeMenuAmendLayout("mb")
	layout := &Layout{Shape: &Shape{}}
	amend(EventCtx{layout, nil, w})

	sel, ok := sm.Get("mb")
	if ok && sel != "" {
		t.Errorf("should clear selection when defocused, got %q", sel)
	}
}

func TestApplyMenubarDefaultsSpacingSubmenu(t *testing.T) {
	cfg := MenubarCfg{}
	applyMenubarDefaults(&cfg)
	if !cfg.SpacingSubmenu.IsSet() {
		t.Fatal("SpacingSubmenu should be set after defaults")
	}
	got := cfg.SpacingSubmenu.Get(0)
	want := defaultMenubarStyle.spacingSubmenu
	if got != want {
		t.Errorf("SpacingSubmenu = %v, want %v", got, want)
	}
}

func TestFindMenuByID(t *testing.T) {
	items := []MenuItemCfg{
		MenuSubmenu("a", "A", []MenuItemCfg{
			MenuItemText("b", "B"),
		}),
		MenuItemText("c", "C"),
	}
	item, ok := findMenuItemCfg(items, "b")
	if !ok {
		t.Fatal("should find b")
	}
	if item.Text != "B" {
		t.Errorf("Text = %q", item.Text)
	}
	_, ok = findMenuItemCfg(items, "z")
	if ok {
		t.Error("should not find z")
	}
}

// menubarInPanels renders one menubar, all named "bar", inside each of
// the given ID-bearing panels. The bar's identity is therefore
// "<panel>:bar", which is what every assertion below is about.
func menubarInPanels(panelIDs ...string) *Window {
	w := NewTestWindow(WindowCfg{})
	w.SetView(func(vw *Window) View {
		panels := make([]View, 0, len(panelIDs))
		for _, id := range panelIDs {
			panels = append(panels, Column(ContainerCfg{
				ID: id,
				Content: []View{
					// Called while the panel's Content slice is built,
					// which is the eager-factory position issue #528 is
					// about.
					Menubar(vw, MenubarCfg{
						ID: "bar",
						Items: []MenuItemCfg{
							MenuItemText("file", "File"),
							MenuItemText("edit", "Edit"),
						},
					}),
				},
			}))
		}
		return Column(ContainerCfg{Sizing: FillFill, Content: panels})
	})
	return w
}

// A menubar inside an ID-bearing panel keys its selection on the
// identity its shape resolves to. Window-global is the common case, not
// the only one (issue #528).
func TestMenubarScopesSelectionToPanel(t *testing.T) {
	w := menubarInPanels("panel")
	// Focus drives the auto-select branch, which is the write this test
	// is about.
	w.SetFocus("panel:bar")
	root := w.TestRender(nil)

	if _, ok := root.FindByID("panel:bar"); !ok {
		t.Fatalf("FindByID(%q) = false", "panel:bar")
	}
	sm := StateMapRead[string, string](w, nsMenu)
	if sm == nil {
		t.Fatal("no menu state written")
	}
	if _, stale := sm.Get("bar"); stale {
		t.Error("selection keyed on the bare leaf \"bar\"")
	}
	sel, ok := sm.Get("panel:bar")
	if !ok || sel != "file" {
		t.Fatalf("selection under %q = (%q, %v), want (%q, true)",
			"panel:bar", sel, ok, "file")
	}
}

// The same leaf in two panels is two menubars, in one window. Pre-fix
// both keyed selection on the bare "bar", so the focused bar's
// auto-select and the unfocused bar's AmendLayout cleanup fought over
// one entry.
func TestMenubarSameIDInTwoPanelsIsTwoKeys(t *testing.T) {
	w := menubarInPanels("a", "b")
	w.SetFocus("a:bar")
	w.TestRender(nil)

	sm := StateMapRead[string, string](w, nsMenu)
	if sm == nil {
		t.Fatal("no menu state written")
	}
	if sel, ok := sm.Get("a:bar"); !ok || sel != "file" {
		t.Errorf("selection under %q = (%q, %v), want (%q, true)",
			"a:bar", sel, ok, "file")
	}
	// The unfocused twin must not have been given a selection, and
	// neither bar may key on the bare leaf.
	if _, ok := sm.Get("b:bar"); ok {
		t.Error("unfocused menubar in panel b holds a selection")
	}
	if _, stale := sm.Get("bar"); stale {
		t.Error("selection keyed on the bare leaf \"bar\"")
	}
}

// The dev-mode gate that reports an unresolved state key must stay quiet
// for the fixed widget.
func TestMenubarUnderPanelIsQuiet(t *testing.T) {
	w := menubarInPanels("panel")
	w.SetFocus("panel:bar")
	if found := w.TestFindings(DebugAll); len(found) != 0 {
		t.Fatalf("findings = %v, want none", found)
	}
}

// The build is deferred, but validation is not: a missing ID or a
// duplicate item ID must still fail where the app wrote the call, not a
// frame later inside generation. Asserted by never generating.
func TestMenubarValidatesAtCallSite(t *testing.T) {
	cases := []struct {
		name string
		cfg  MenubarCfg
	}{
		{"missing ID", MenubarCfg{
			Items: []MenuItemCfg{MenuItemText("file", "File")},
		}},
		{"duplicate item ID", MenubarCfg{
			ID: "bar",
			Items: []MenuItemCfg{
				MenuItemText("file", "File"),
				MenuItemText("file", "Again"),
			},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Fatal("no panic; validation is no longer eager")
				}
			}()
			// The window is unused by the eager half, so a nil one is
			// enough to show nothing was deferred.
			Menubar(nil, tc.cfg)
		})
	}
}
