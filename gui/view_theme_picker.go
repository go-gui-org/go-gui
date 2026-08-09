package gui

// ThemePickerCfg configures a theme picker view.
type ThemePickerCfg struct {
	OnSelect        func(string, EventCtx)
	ID              string
	A11YLabel       string
	A11YDescription string
	Focusable       bool
	FloatOffsetX    float32
	FloatOffsetY    float32
	Sizing          Sizing
	FloatAnchor     FloatAttach
	FloatTieOff     FloatAttach
}

// ThemePicker creates a palette icon that opens a dropdown of
// registered themes for selection.
func ThemePicker(cfg ThemePickerCfg) View {
	return &themePickerView{cfg: cfg}
}

type themePickerView struct {
	cfg ThemePickerCfg
}

func (tv *themePickerView) Content() []View { return nil }

func (tv *themePickerView) GenerateLayout(w *Window) Layout {
	cfg := &tv.cfg
	// Everything this widget keys on hangs off its effective ID, so a
	// second picker under a different ID-bearing panel gets its own
	// open flag, its own listbox focus, its own dropdown scroll. The
	// inner IDs stay absolute (they already contain IDSep), which is
	// what keeps them identical to the keys computed here.
	id := w.EffID(cfg.ID)
	isOpen := StateReadOr(w, nsSelect, id, false)
	currentName := guiTheme.Name
	focusID := id
	onSel := cfg.OnSelect
	lbID := ScopeID(id, "lb")

	content := make([]View, 0, 2)

	// Paint palette icon.
	content = append(content, Text(TextCfg{
		Text:      IconPalette,
		TextStyle: guiTheme.Icon3,
	}))

	if isOpen {
		names := ThemeRegisteredNames()
		data := make([]ListBoxOption, len(names))
		for i, name := range names {
			data[i] = NewListBoxOption(name, name, name)
		}
		content = append(content, Column(ContainerCfg{
			ID:            ScopeID(id, "dropdown"),
			Float:         true,
			FloatAutoFlip: true,
			FloatAnchor:   cfg.FloatAnchor,
			FloatTieOff:   cfg.FloatTieOff,
			FloatOffsetX:  cfg.FloatOffsetX,
			FloatOffsetY:  cfg.FloatOffsetY,
			Padding:       NoPadding,
			Content: []View{
				ListBox(ListBoxCfg{
					ID:          lbID,
					Scrollable:  true,
					MinWidth:    140,
					MaxHeight:   300,
					Data:        data,
					SelectedIDs: []string{currentName},
					OnSelect: func(ids []string, ctx EventCtx) {
						if len(ids) == 0 {
							return
						}
						name := ids[0]
						t, ok := ThemeGet(name)
						if !ok {
							return
						}
						ctx.Window.SetTheme(t)
						if onSel != nil {
							onSel(name, EventCtx{nil, ctx.Event, ctx.Window})
						}
						ctx.Consume()
					},
				}),
			},
		}))
	}

	colorFocus := guiTheme.ToggleStyle.ColorFocus
	colorBorderFocus := guiTheme.ToggleStyle.ColorBorderFocus

	return generateViewLayout(Row(ContainerCfg{
		ID:        cfg.ID,
		Focusable: cfg.Focusable,
		A11YRole:  AccessRoleButton,
		A11YLabel: a11yLabel(cfg.A11YLabel, "Theme Picker"),
		Sizing:    cfg.Sizing,
		Padding:   Some(PaddingSmall),
		OnClick: func(ctx EventCtx) {
			ss := StateMap[string, bool](ctx.Window, nsSelect, capModerate)
			ss.Clear()
			opening := !isOpen
			ss.Set(id, opening)
			if opening {
				themePickerSyncHighlight(lbID, ctx.Window)
			}
			// Toggling the popup is the whole click. Consume it so an
			// enclosing clickable (a menu item hosting the picker as a
			// CustomView, in menu_demo) does not also act on it — today
			// the consume-class pre-mark stops that implicitly, and
			// spec §4.3b would remove the pre-mark.
			ctx.Consume()
		},
		AmendLayout: func(ctx EventCtx) {
			if ctx.Window.IsFocus(focusID) {
				ctx.Layout.Shape.Color = colorFocus
				ctx.Layout.Shape.ColorBorder = colorBorderFocus
			}
		},
		OnKeyDown: func(ctx EventCtx) {
			wasOpen := StateReadOr(ctx.Window, nsSelect, id, false)
			if !wasOpen {
				if ctx.Event.KeyCode == KeySpace || ctx.Event.KeyCode == KeyEnter {
					ss := StateMap[string, bool](ctx.Window, nsSelect, capModerate)
					ss.Set(id, true)
					themePickerSyncHighlight(lbID, ctx.Window)
					ctx.Consume()
				}
				return
			}
			names := ThemeRegisteredNames()
			count := len(names)
			if count == 0 {
				return
			}
			lbf := StateMap[string, int](ctx.Window, nsListBoxFocus, capModerate)
			// Default 0: start at first item; bounds-checked below.
			currentIdx := lbf.GetOr(lbID, 0)
			action := listCoreNavigate(ctx.Event.KeyCode, count)

			nextIdx := -1
			switch action {
			case listCoreDismiss:
				ss := StateMap[string, bool](ctx.Window, nsSelect, capModerate)
				ss.Clear()
				ctx.Consume()
			case listCoreSelectItem:
				ctx.Consume()
				nextIdx = currentIdx
			case listCoreMoveUp:
				ctx.Consume()
				nextIdx = currentIdx - 1
				nextIdx = max(nextIdx, 0)
			case listCoreMoveDown:
				ctx.Consume()
				nextIdx = currentIdx + 1
				if nextIdx >= count {
					nextIdx = count - 1
				}
			case listCoreFirst:
				ctx.Consume()
				nextIdx = 0
			case listCoreLast:
				ctx.Consume()
				nextIdx = count - 1
			}

			if nextIdx >= 0 && nextIdx < count {
				lbf.Set(lbID, nextIdx)
				name := names[nextIdx]
				t, ok := ThemeGet(name)
				if !ok {
					return
				}
				ctx.Window.SetTheme(t)
				if onSel != nil {
					onSel(name, EventCtx{nil, ctx.Event, ctx.Window})
				}
			}
		},
		Content: content,
	}), w)
}

// themePickerSyncHighlight sets listbox focus index to match the current
// theme name.
func themePickerSyncHighlight(lbID string, w *Window) {
	names := ThemeRegisteredNames()
	current := guiTheme.Name
	idx := 0
	for i, n := range names {
		if n == current {
			idx = i
			break
		}
	}
	lbf := StateMap[string, int](w, nsListBoxFocus, capModerate)
	lbf.Set(lbID, idx)
}
