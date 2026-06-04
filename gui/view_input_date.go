package gui

import (
	"fmt"
	"time"
)

// InputDateCfg configures a date input with dropdown calendar.
type InputDateCfg struct {
	TextStyle        TextStyle
	PlaceholderStyle TextStyle
	Date             time.Time
	OnSelect         func([]time.Time, *Event, *Window)
	ID               string
	Placeholder      string

	A11YLabel            string
	A11YDescription      string
	Dates                []time.Time
	AllowedWeekdays      []DatePickerWeekdays
	AllowedMonths        []DatePickerMonths
	AllowedYears         []int
	AllowedDates         []time.Time
	Padding              Opt[Padding]
	SizeBorder           Opt[float32]
	CellSpacing          Opt[float32]
	Radius               Opt[float32]
	RadiusBorder         Opt[float32]
	IDFocus              uint32
	Width                float32
	Height               float32
	MinWidth             float32
	MaxWidth             float32
	Color                Color
	ColorHover           Color
	ColorFocus           Color
	ColorClick           Color
	ColorBorder          Color
	ColorBorderFocus     Color
	ColorSelect          Color
	Sizing               Sizing
	WeekdaysLen          DatePickerWeekdayLen
	Disabled             bool
	Invisible            bool
	SelectMultiple       bool
	HideTodayIndicator   bool
	MondayFirstDayOfWeek bool
	ShowAdjacentMonths   bool
}

type inputDateView struct {
	cfg InputDateCfg
}

func (idv *inputDateView) Content() []View { return nil }

// InputDate creates a date input field with a dropdown calendar.
func InputDate(cfg InputDateCfg) View {
	applyInputDateDefaults(&cfg)
	return &inputDateView{cfg: cfg}
}

func (idv *inputDateView) GenerateLayout(w *Window) Layout {
	cfg := &idv.cfg

	isOpen := StateReadOr(w, nsInputDate, cfg.ID, false)
	cfgID := cfg.ID

	// Format date for display.
	dates := cfg.Dates
	if len(dates) == 0 && !cfg.Date.IsZero() {
		dates = []time.Time{cfg.Date}
	}

	dateText := ""
	if len(dates) == 1 {
		dateText = LocaleFormatDate(dates[0],
			localeDatePadFormat(guiLocale.Date.ShortDate))
	} else if len(dates) > 1 {
		dateText = fmt.Sprintf("%d dates selected", len(dates))
	}

	// Sync editable text with external date. Only overwrite
	// user text when the external date actually changes.
	sm := StateMap[string, string](w, nsInputDateText, capModerate)
	editText, _ := sm.Get(cfgID)
	syncKey := cfgID + ".sync"
	lastSync, _ := sm.Get(syncKey)
	if dateText != "" && dateText != lastSync {
		sm.Set(cfgID, dateText)
		sm.Set(syncKey, dateText)
		editText = dateText
	}

	var content []View

	// Date text + calendar icon button.
	content = append(content,
		Row(ContainerCfg{
			Sizing:     FillFit,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Spacing:    Some(SpacingSmall),
			VAlign:     VAlignMiddle,
			Content: []View{
				inputDateTextField(cfg, cfgID, isOpen, editText),
				Button(ButtonCfg{
					Disabled:   cfg.Disabled,
					Padding:    NoPadding,
					SizeBorder: NoBorder,
					Content: []View{Text(TextCfg{
						Text: "\U0001F4C5",
					})},
					OnClick: func(_ *Layout, e *Event, w *Window) {
						inputDateToggle(cfgID, w)
						e.IsHandled = true
					},
				}),
			},
		}),
	)

	// Floating date picker with click-outside-to-close backdrop.
	if isOpen {
		content = append(content, Column(ContainerCfg{
			Float:      true,
			Sizing:     FillFill,
			Color:      ColorTransparent,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			OnClick: func(_ *Layout, e *Event, w *Window) {
				inputDateClose(cfgID, w)
				e.IsHandled = true
			},
		}))
		content = append(content, Column(ContainerCfg{
			Float:        true,
			FloatAnchor:  FloatBottomLeft,
			FloatTieOff:  FloatTopLeft,
			Padding:      NoPadding,
			SizeBorder:   NoBorder,
			FloatOffsetY: -cfg.SizeBorder.Get(0),
			OnClick: func(_ *Layout, e *Event, _ *Window) {
				e.IsHandled = true
			},
			Content: []View{
				DatePicker(DatePickerCfg{
					ID:                   cfgID + ".picker",
					Dates:                dates,
					AllowedWeekdays:      cfg.AllowedWeekdays,
					AllowedMonths:        cfg.AllowedMonths,
					AllowedYears:         cfg.AllowedYears,
					AllowedDates:         cfg.AllowedDates,
					WeekdaysLen:          cfg.WeekdaysLen,
					TextStyle:            cfg.TextStyle,
					Color:                cfg.Color,
					ColorHover:           cfg.ColorHover,
					ColorFocus:           cfg.ColorFocus,
					ColorClick:           cfg.ColorClick,
					ColorBorder:          cfg.ColorBorder,
					ColorBorderFocus:     cfg.ColorBorderFocus,
					ColorSelect:          cfg.ColorSelect,
					SizeBorder:           cfg.SizeBorder,
					CellSpacing:          cfg.CellSpacing,
					Radius:               cfg.Radius,
					RadiusBorder:         cfg.RadiusBorder,
					SelectMultiple:       cfg.SelectMultiple,
					HideTodayIndicator:   cfg.HideTodayIndicator,
					MondayFirstDayOfWeek: cfg.MondayFirstDayOfWeek,
					ShowAdjacentMonths:   cfg.ShowAdjacentMonths,
					OnSelect: func(dates []time.Time, e *Event, w *Window) {
						inputDateClose(cfgID, w)
						if cfg.OnSelect != nil {
							cfg.OnSelect(dates, e, w)
						}
					},
				}),
			},
		}))
	}

	col := Column(ContainerCfg{
		ID:          cfg.ID,
		A11YRole:    AccessRoleDateField,
		A11YLabel:   a11yLabel(cfg.A11YLabel, "Date Input"),
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Radius:      cfg.RadiusBorder,
		Padding:     cfg.Padding,
		Sizing:      cfg.Sizing,
		Width:       cfg.Width,
		Height:      cfg.Height,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Content:     content,
		AmendLayout: func(layout *Layout, w *Window) {
			if w.IsFocus(cfg.IDFocus) {
				layout.Shape.ColorBorder = cfg.ColorBorderFocus
			}
		},
	})
	return GenerateViewLayout(col, w)
}

// inputDateTextField returns an Input for single/no dates (editable)
// or a Text for multi-select display ("N dates selected").
func inputDateTextField(
	cfg *InputDateCfg, cfgID string, isOpen bool,
	dateText string,
) View {
	if len(cfg.Dates) > 1 {
		return Text(TextCfg{
			Text:      dateText,
			TextStyle: cfg.TextStyle,
			Sizing:    FillFit,
		})
	}
	return Input(InputCfg{
		ID:               cfgID + ".input",
		IDFocus:          cfg.IDFocus,
		Text:             dateText,
		Placeholder:      inputDatePlaceholder(cfg),
		Mask:             localeDateMaskPattern(guiLocale.Date.ShortDate),
		TextStyle:        cfg.TextStyle,
		PlaceholderStyle: cfg.PlaceholderStyle,
		Sizing:           FillFit,
		SizeBorder:       NoBorder,
		Padding:          NoPadding,
		Color:            ColorTransparent,
		Disabled:         cfg.Disabled,
		OnTextChanged: func(_ *Layout, s string, w *Window) {
			sm := StateMap[string, string](w, nsInputDateText, capModerate)
			sm.Set(cfgID, s)
		},
		OnTextCommit: func(_ *Layout, text string, _ InputCommitReason, w *Window) {
			if text == "" {
				if cfg.OnSelect != nil {
					cfg.OnSelect(nil, &Event{}, w)
				}
				w.UpdateWindow()
				return
			}
			t, err := localeParseDate(text,
				localeDatePadFormat(guiLocale.Date.ShortDate))
			if err != nil {
				return
			}
			if cfg.OnSelect != nil {
				cfg.OnSelect([]time.Time{t}, &Event{}, w)
			}
			w.UpdateWindow()
		},
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			if isOpen && e.KeyCode == KeyEscape {
				inputDateClose(cfgID, w)
				e.IsHandled = true
			}
		},
	})
}

func inputDatePlaceholder(cfg *InputDateCfg) string {
	if cfg.Placeholder != "" {
		return cfg.Placeholder
	}
	return localeDatePadFormat(guiLocale.Date.ShortDate)
}

func inputDateToggle(id string, w *Window) {
	sm := StateMap[string, bool](w, nsInputDate, capModerate)
	cur, _ := sm.Get(id)
	sm.Set(id, !cur)
	w.UpdateWindow()
}

func inputDateClose(id string, w *Window) {
	sm := StateMap[string, bool](w, nsInputDate, capModerate)
	sm.Set(id, false)
	w.UpdateWindow()
}

func applyInputDateDefaults(cfg *InputDateCfg) {
	d := &DefaultDatePickerStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = d.ColorHover
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = d.ColorFocus
	}
	if !cfg.ColorClick.IsSet() {
		cfg.ColorClick = d.ColorClick
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorBorderFocus.IsSet() {
		cfg.ColorBorderFocus = d.ColorBorderFocus
	}
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(PaddingSmall)
	}
	sizeBorder := cfg.SizeBorder.Get(d.SizeBorder)
	cellSpacing := cfg.CellSpacing.Get(d.CellSpacing)
	radius := cfg.Radius.Get(d.Radius)
	radiusBorder := cfg.RadiusBorder.Get(d.RadiusBorder)
	cfg.SizeBorder = Some(sizeBorder)
	cfg.CellSpacing = Some(cellSpacing)
	cfg.Radius = Some(radius)
	cfg.RadiusBorder = Some(radiusBorder)
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if cfg.PlaceholderStyle == (TextStyle{}) {
		cfg.PlaceholderStyle = TextStyle{
			Color: RGBA(
				d.TextStyle.Color.R,
				d.TextStyle.Color.G,
				d.TextStyle.Color.B, 100),
			Size: d.TextStyle.Size,
		}
	}
}
