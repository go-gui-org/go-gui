package gui

// view_dock_layout.go — DockLayoutCfg, DockPanelDef, recursive
// view generation (split -> splitter, panel_group -> tab header +
// content), and dock-drag integration.

// DockPanelDef defines a single panel that can appear in the
// dock layout. Maps panel_id to label and content views.
type DockPanelDef struct {
	ID       string
	Label    string
	Content  []View
	Closable bool
}

// DockLayoutCfg configures a dock layout component.
type DockLayoutCfg struct {
	Root           *DockNode
	OnLayoutChange func(*DockNode, EventCtx)
	OnPanelSelect  func(string, string, EventCtx) // (groupID, panelID)
	OnPanelClose   func(string, EventCtx)
	ID             string
	Panels         []DockPanelDef
	// ColorZonePreview paints the drop-zone highlight while dragging.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorZonePreview Color
	// ColorTab/ColorTabActive/ColorTabHover/ColorTabBar/
	// ColorTabSeparator/ColorContent theme the dock's tab strips and
	// panel background. Unset takes the theme defaults.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTab Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabActive Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabHover Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabBar Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabSeparator Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorContent Color
	Sizing       Sizing
	// HideSingleTab hides a group's tab strip when it holds one
	// panel.
	// exportaudit:keep — caller-facing config (issue #372)
	HideSingleTab bool

	// Sound overrides the theme's cue for the tab strip. A tab picks
	// one panel of several, so it takes the selection role; the close
	// button is a momentary activation, so it takes the click one.
	// SoundNone (the zero value) takes the theme's cue for whichever
	// applies, which is itself silent unless the app opted in
	// (issue #446).
	// exportaudit:keep — caller-facing config (issue #467)
	Sound SoundCue

	// SoundDisabled suppresses the tab strip's sounds regardless of
	// the theme and of Sound above.
	// exportaudit:keep — caller-facing config (issue #467)
	SoundDisabled bool
}

// dockLayoutCore holds callback-relevant fields without content
// arrays. Captured in closures to avoid GC false retention of the
// full DockLayoutCfg (which holds []DockPanelDef with []View).
type dockLayoutCore struct {
	root             *DockNode
	onLayoutChange   func(*DockNode, EventCtx)
	onPanelSelect    func(string, string, EventCtx)
	onPanelClose     func(string, EventCtx)
	id               string
	colorZonePreview Color
}

func newDockLayoutCore(cfg *DockLayoutCfg) *dockLayoutCore {
	return &dockLayoutCore{
		id:               cfg.ID,
		root:             cfg.Root,
		onLayoutChange:   cfg.OnLayoutChange,
		onPanelSelect:    cfg.OnPanelSelect,
		onPanelClose:     cfg.OnPanelClose,
		colorZonePreview: cfg.ColorZonePreview,
	}
}

// DockLayout creates a docking layout component. Renders a tree
// of splitters and tabbed panel groups. Supports drag-and-drop
// panel rearrangement.
func DockLayout(cfg DockLayoutCfg) View {
	applyDockLayoutDefaults(&cfg)
	return viewFunc(func(w *Window) View {
		// One resolved identity for every key below; see (*Window).EffID.
		// cfg is captured by this per-frame closure, so re-resolving it
		// each frame is fine: the join is idempotent.
		cfg.ID = w.EffID(cfg.ID)
		core := newDockLayoutCore(&cfg)
		drag := dockDragGet(w, cfg.ID)

		content := make([]View, 0, 3)
		content = append(content, dockNodeView(core, cfg.Root, &cfg, drag))
		content = append(content, dockDragZoneOverlayView(cfg.ColorZonePreview))

		if drag.active {
			ghostLabel := dockFindPanelLabel(cfg.Panels, drag.panelID)
			if len(ghostLabel) > 0 {
				content = append(content, dockDragGhostView(drag, ghostLabel))
			}
		}

		dockID := core.id
		colorZone := core.colorZonePreview

		return Canvas(ContainerCfg{
			ID:       cfg.ID,
			A11YRole: AccessRoleGroup,
			Sizing:   cfg.Sizing,
			Padding:  NoPadding,
			Spacing:  SomeF(0),
			Clip:     true,
			AmendLayout: func(ctx EventCtx) {
				dockLayoutAmend(dockID, colorZone, ctx.Layout, ctx.Window)
			},
			OnKeyDown: func(ctx EventCtx) {
				if ctx.Event.KeyCode == KeyEscape {
					state := dockDragGet(ctx.Window, dockID)
					if state.active {
						dockDragCancel(dockID, ctx.Window)
						ctx.Consume()
					}
				}
			},
			Content: content,
		})
	})
}

func applyDockLayoutDefaults(cfg *DockLayoutCfg) {
	cfg.Sizing = cfg.Sizing.Or(FillFill)
	if !cfg.ColorZonePreview.IsSet() {
		// Non-text fill, exempt from the dimming roles (audit §1.2).
		cfg.ColorZonePreview = RGBA(70, 130, 220, 80) // ergonomics-audit:visual
	}
	if !cfg.ColorTab.IsSet() {
		cfg.ColorTab = guiTheme.ColorPanel
	}
	if !cfg.ColorTabActive.IsSet() {
		cfg.ColorTabActive = guiTheme.ColorPanel
	}
	if !cfg.ColorTabHover.IsSet() {
		cfg.ColorTabHover = guiTheme.ColorHover
	}
	if !cfg.ColorTabBar.IsSet() {
		cfg.ColorTabBar = guiTheme.ColorPanel
	}
	if !cfg.ColorTabSeparator.IsSet() {
		cfg.ColorTabSeparator = guiTheme.ColorBorder
	}
	if !cfg.ColorContent.IsSet() {
		cfg.ColorContent = guiTheme.ColorBackground
	}
}

// dockLayoutAmend positions the tree view to fill the dock
// container, and positions the zone overlay.
func dockLayoutAmend(
	dockID string, colorZone Color,
	layout *Layout, w *Window,
) {
	if len(layout.Children) < 1 {
		return
	}
	// First child is the tree view — fill the entire dock area.
	splitterLayoutChild(
		&layout.Children[0],
		layout.Shape.X, layout.Shape.Y,
		layout.Shape.Width, layout.Shape.Height, w)
	// Zone overlay is positioned by dockDragAmendOverlay (found by id).
	dockDragAmendOverlay(dockID, colorZone, layout, w)
}

// dockNodeView recursively generates views for the dock tree.
func dockNodeView(
	core *dockLayoutCore, node *DockNode,
	cfg *DockLayoutCfg, drag dockDragState,
) View {
	if node == nil {
		return nil
	}
	if node.Kind == dockNodeSplit {
		return dockSplitView(core, node, cfg, drag)
	}
	return dockGroupView(core, node, cfg, drag)
}

// dockSplitView generates a splitter for a DockSplit node.
func dockSplitView(
	core *dockLayoutCore, node *DockNode,
	cfg *DockLayoutCfg, drag dockDragState,
) View {
	splitID := node.ID
	root := core.root
	onLayoutChange := core.onLayoutChange

	orientation := SplitterVertical
	if node.Dir == DockSplitHorizontal {
		orientation = SplitterHorizontal
	}

	var firstContent, secondContent []View
	if node.First != nil {
		firstContent = []View{dockNodeView(core, node.First, cfg, drag)}
	}
	if node.Second != nil {
		secondContent = []View{dockNodeView(core, node.Second, cfg, drag)}
	}

	return Splitter(SplitterCfg{
		// Absolute, and derived from the dock's own ID rather than the
		// splitter path that currently leads here: a rearrangement must
		// not move the ratio state of a splitter that did not move
		// (issue #389).
		ID:          ScopeID(core.id, "split", node.ID),
		Orientation: orientation,
		Ratio:       SomeF(node.Ratio),
		Sizing:      FillFill,
		OnChange: func(ratio float32, _ SplitterCollapsed, ctx EventCtx) {
			newRoot := dockTreeUpdateRatio(root, splitID, ratio)
			if onLayoutChange != nil {
				onLayoutChange(newRoot, ctx)
			}
		},
		First:  SplitterPaneCfg{Content: firstContent},
		Second: SplitterPaneCfg{Content: secondContent},
	})
}

// dockGroupView generates a tab header + content area for a
// DockPanelGroup node.
func dockGroupView(
	core *dockLayoutCore, group *DockNode,
	cfg *DockLayoutCfg, drag dockDragState,
) View {
	dragging := drag.active && drag.sourceGroup == group.ID

	tabButtons := make([]View, 0, len(group.PanelIDs))
	var activeContent []View

	colorSep := cfg.ColorTabSeparator
	for _, panelID := range group.PanelIDs {
		panelDef, ok := dockFindPanelDef(cfg.Panels, panelID)
		if !ok {
			continue
		}
		isSelected := panelID == group.SelectedID
		isDragged := dragging && drag.panelID == panelID

		if isDragged {
			continue
		}

		if isSelected {
			activeContent = panelDef.Content
		}

		if len(tabButtons) > 0 {
			tabButtons = append(tabButtons,
				Column(ContainerCfg{
					Width:      1,
					Sizing:     FixedFill,
					Padding:    NoPadding,
					SizeBorder: NoBorder,
					Color:      colorSep,
				}))
		}
		tabButtons = append(tabButtons,
			dockTabButton(core, group, panelDef, isSelected, cfg))
	}

	// If selected tab was dragged out, show first remaining.
	if len(activeContent) == 0 && len(group.PanelIDs) > 0 {
		for _, pid := range group.PanelIDs {
			if dragging && drag.panelID == pid {
				continue
			}
			if pd, ok := dockFindPanelDef(cfg.Panels, pid); ok {
				activeContent = pd.Content
				break
			}
		}
	}

	groupContent := make([]View, 0, 2)

	// Tab header row — hidden when HideSingleTab is set and the
	// group has only one panel.
	if !cfg.HideSingleTab || len(group.PanelIDs) > 1 {
		groupContent = append(groupContent, Row(ContainerCfg{
			Sizing:     FillFit,
			Padding:    NewPadding(2, 4, 0, 4),
			Spacing:    NoSpacing,
			SizeBorder: NoBorder,
			Color:      cfg.ColorTabBar,
			Content:    tabButtons,
		}))
	}

	// Content area.
	groupContent = append(groupContent, Column(ContainerCfg{
		Sizing:     FillFill,
		Padding:    NoPadding,
		Spacing:    NoSpacing,
		SizeBorder: NoBorder,
		Clip:       true,
		Color:      cfg.ColorContent,
		Content:    activeContent,
	}))

	return Column(ContainerCfg{
		// Absolute: dock ID + node ID, never the splitter path that leads
		// here. The group scopes its panel content, so a bare node ID
		// would re-key every widget inside the panel on each drop
		// elsewhere in the tree (issue #389).
		ID:         ScopeID(core.id, group.ID),
		Sizing:     FillFill,
		Padding:    NoPadding,
		Spacing:    NoSpacing,
		SizeBorder: NoBorder,
		Clip:       true,
		Content:    groupContent,
	})
}

// dockTabButton creates a single tab button in a panel group
// header.
func dockTabButton(
	core *dockLayoutCore, group *DockNode,
	panel DockPanelDef, isSelected bool, cfg *DockLayoutCfg,
) View {
	panelID := panel.ID
	groupID := group.ID
	dockID := core.id
	root := core.root
	onLayoutChange := core.onLayoutChange
	onPanelSelect := core.onPanelSelect
	onPanelClose := core.onPanelClose

	colorTab := cfg.ColorTab
	if isSelected {
		colorTab = cfg.ColorTabActive
	}
	colorHover := cfg.ColorTabHover

	closeSound := resolveSoundCue(
		guiTheme.Sounds.Click, cfg.Sound, cfg.SoundDisabled)
	tabSound := resolveSoundCue(
		guiTheme.Sounds.Selection, cfg.Sound, cfg.SoundDisabled)

	btnContent := make([]View, 0, 3)
	btnContent = append(btnContent, Text(TextCfg{Text: panel.Label}))

	if panel.Closable && onPanelClose != nil {
		btnContent = append(btnContent,
			Rectangle(RectangleCfg{Sizing: FillFill, Color: ColorTransparent}))
		btnContent = append(btnContent, Button(ButtonCfg{
			ID:         ScopeID(dockID, "close", panelID),
			Width:      18,
			Height:     18,
			Sizing:     FixedFixed,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Color:      colorTab,
			Colors:     ColorSet{Hover: guiTheme.ColorHover}.resolved(colorTab, themeButtonSet()),
			Radius:     SomeF(2),
			// SoundDisabled as well as Sound: a resolved SoundNone
			// reads as "unset" inside ButtonCfg (issue #467).
			Sound:         closeSound,
			SoundDisabled: closeSound == SoundNone,
			OnClick: func(ctx EventCtx) {
				onPanelClose(panelID, ctx)
				// The close button sits inside its own tab button, which
				// selects the panel on click. Closing must not also
				// select. The pre-mark stops that today; consume so it
				// still holds if spec §4.3b removes the pre-mark.
				ctx.Consume()
			},
			Content: []View{
				Text(TextCfg{
					Text: "×", // ×
					TextStyle: glyphStyle(mergeTextStyle(
						TextStyle{Size: guiTheme.SizeTextSmall},
						DefaultTextStyle)),
				}),
			},
		}))
	}

	return Button(ButtonCfg{
		ID:         ScopeID(dockID, "tab", groupID, panelID),
		Sizing:     FillFit,
		HAlign:     Some(HAlignLeft),
		Padding:    NewPadding(4, 8, 4, 8),
		Radius:     NoRadius,
		SizeBorder: NoBorder,
		Color:      colorTab,
		Colors:     ColorSet{Hover: colorHover}.resolved(colorTab, themeButtonSet()),
		// The cue marks selecting the panel, not the drag this handler
		// also starts: dragging is phase 3's question (issue #467).
		Sound:         tabSound,
		SoundDisabled: tabSound == SoundNone,
		OnClick: func(ctx EventCtx) {
			dockDragStart(dockID, panelID, groupID, root,
				onLayoutChange, ctx.Layout, ctx.Event, ctx.Window)
			if onPanelSelect != nil {
				onPanelSelect(groupID, panelID, ctx)
			}
		},
		Content: btnContent,
	})
}

// dockFindPanelDef looks up a panel definition by id.
func dockFindPanelDef(panels []DockPanelDef, panelID string) (DockPanelDef, bool) {
	for _, p := range panels {
		if p.ID == panelID {
			return p, true
		}
	}
	return DockPanelDef{}, false
}

// dockFindPanelLabel returns the label for a panel id.
func dockFindPanelLabel(panels []DockPanelDef, panelID string) string {
	for _, p := range panels {
		if p.ID == panelID {
			return p.Label
		}
	}
	return ""
}
