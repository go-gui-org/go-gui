package gui

// ThemePickerCfg configures a theme picker view.
type ThemePickerCfg struct {
	OnSelect        func(string, *Event, *Window)
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
	isOpen := StateReadOr(w, nsSelect, cfg.ID, false)
	id := cfg.ID
	currentName := guiTheme.Name
	focusID := cfg.ID
	onSel := cfg.OnSelect
	lbID := cfg.ID + "lb"

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
			ID:            cfg.ID + "dropdown",
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
					OnSelect: func(ids []string, e *Event, w *Window) {
						if len(ids) == 0 {
							return
						}
						name := ids[0]
						t, ok := ThemeGet(name)
						if !ok {
							return
						}
						w.SetTheme(t)
						if onSel != nil {
							onSel(name, e, w)
						}
						e.IsHandled = true
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
		OnClick: func(_ *Layout, e *Event, w *Window) {
			ss := StateMap[string, bool](w, nsSelect, capModerate)
			ss.Clear()
			opening := !isOpen
			ss.Set(id, opening)
			if opening {
				themePickerSyncHighlight(lbID, w)
			}
			e.IsHandled = true
		},
		AmendLayout: func(layout *Layout, w *Window) {
			if w.IsFocus(focusID) {
				layout.Shape.Color = colorFocus
				layout.Shape.ColorBorder = colorBorderFocus
			}
		},
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			wasOpen := StateReadOr(w, nsSelect, id, false)
			if !wasOpen {
				if e.KeyCode == KeySpace || e.KeyCode == KeyEnter {
					ss := StateMap[string, bool](w, nsSelect, capModerate)
					ss.Set(id, true)
					themePickerSyncHighlight(lbID, w)
					e.IsHandled = true
				}
				return
			}
			names := ThemeRegisteredNames()
			count := len(names)
			if count == 0 {
				return
			}
			lbf := StateMap[string, int](w, nsListBoxFocus, capModerate)
			// Default 0: start at first item; bounds-checked below.
			currentIdx := lbf.GetOr(lbID, 0)
			action := listCoreNavigate(e.KeyCode, count)

			nextIdx := -1
			switch action {
			case listCoreDismiss:
				ss := StateMap[string, bool](w, nsSelect, capModerate)
				ss.Clear()
				e.IsHandled = true
			case listCoreSelectItem:
				e.IsHandled = true
				nextIdx = currentIdx
			case listCoreMoveUp:
				e.IsHandled = true
				nextIdx = currentIdx - 1
				nextIdx = max(nextIdx, 0)
			case listCoreMoveDown:
				e.IsHandled = true
				nextIdx = currentIdx + 1
				if nextIdx >= count {
					nextIdx = count - 1
				}
			case listCoreFirst:
				e.IsHandled = true
				nextIdx = 0
			case listCoreLast:
				e.IsHandled = true
				nextIdx = count - 1
			}

			if nextIdx >= 0 && nextIdx < count {
				lbf.Set(lbID, nextIdx)
				name := names[nextIdx]
				t, ok := ThemeGet(name)
				if !ok {
					return
				}
				w.SetTheme(t)
				if onSel != nil {
					onSel(name, e, w)
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
