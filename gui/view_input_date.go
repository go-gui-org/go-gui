package gui

import (
	"fmt"
	"time"
)

// InputDateCfg configures a date input with dropdown calendar.
type InputDateCfg struct {
	// Label names this field. Empty renders exactly as before: no
	// wrapper and no extra shape. Set, it stacks above the field in
	// the theme's label role, and fills A11YLabel when that is unset.
	// See gui/field_label.go for the convention and why it is one.
	Label            string
	TextStyle        TextStyle
	PlaceholderStyle TextStyle
	Date             time.Time
	OnSelect         func([]time.Time, EventCtx)
	ID               string `gui:"required,focus"`
	Placeholder      string

	A11YCfg
	Dates           []time.Time
	AllowedWeekdays []DatePickerWeekdays
	AllowedMonths   []DatePickerMonths
	AllowedYears    []int
	AllowedDates    []time.Time
	Padding         Padding
	SizeBorder      Opt[float32]
	// CellSpacing gaps the day cells. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	CellSpacing Opt[float32]
	Radius      Opt[float32]
	// RadiusBorder rounds the calendar frame. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusBorder Opt[float32]
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Width         float32
	Height        float32
	MinWidth      float32
	MaxWidth      float32
	Color         Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it.
	Colors      ColorSet
	ColorSelect Color
	// ColorTextOnSelect is the text color drawn over the selected
	// day's fill. Unset takes the theme's.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTextOnSelect    Color
	Sizing               Sizing
	WeekdaysLen          DatePickerWeekdayLen
	Disabled             bool
	Invisible            bool
	SelectMultiple       bool
	HideTodayIndicator   bool
	MondayFirstDayOfWeek bool
	ShowAdjacentMonths   bool

	// ReadOnly blocks date edits while the field stays focusable and
	// selectable, mirroring InputCfg.ReadOnly. Typing is blocked on the
	// inner Input and the calendar popup is gated shut so it cannot
	// change the date. Distinct from Disabled, which removes interaction
	// entirely.
	ReadOnly bool
}

type inputDateView struct {
	cfg InputDateCfg
}

// InputDate creates a date input field with a dropdown calendar.
func InputDate(cfg InputDateCfg) View {
	applyInputDateDefaults(&cfg)
	requireFocusID("InputDate", cfg.FocusDisabled, cfg.ID)
	cfg.A11YLabel = a11yLabel(cfg.A11YLabel, cfg.Label)
	return labelledField(
		cfg.Label, cfg.TextStyle, HAlignLeft, cfg.Sizing,
		&inputDateView{cfg: cfg})
}

func (idv *inputDateView) GenerateLayout(w *Window) Layout {
	cfg := &idv.cfg

	// A read-only date field never opens the calendar popup, closing
	// the picker's OnSelect mutation path structurally regardless of any
	// stored open state.
	isOpen := StateReadOr(w, nsInputDate, cfg.ID, false) && !cfg.ReadOnly
	cfgID := cfg.ID

	// Format date for display.
	dates := cfg.Dates
	if len(dates) == 0 && !cfg.Date.IsZero() {
		dates = []time.Time{cfg.Date}
	}

	dateText := ""
	if len(dates) == 1 {
		dateText = LocaleFormatDate(dates[0],
			localeDatePadFormat(ActiveLocale.Date.ShortDate))
	} else if len(dates) > 1 {
		dateText = fmt.Sprintf("%d dates selected", len(dates))
	}

	// Sync editable text with external date. Only overwrite
	// user text when the external date actually changes.
	sm := StateMap[string, string](w, nsInputDateText, capModerate)
	// Default "": absent entry means no edit text cached yet.
	editText := sm.GetOr(cfgID, "")
	syncKey := ScopeID(cfgID, "sync")
	// Default "": absent entry means no prior sync occurred.
	lastSync := sm.GetOr(syncKey, "")
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
					// Namespaced by the field's ID: a form can hold
					// several date inputs.
					ID:         ScopeID(cfgID, "calendar"),
					Disabled:   cfg.Disabled || cfg.ReadOnly,
					Padding:    NoPadding,
					SizeBorder: NoBorder,
					Content: []View{Text(TextCfg{
						Text: "\U0001F4C5",
					})},
					OnClick: func(ctx EventCtx) {
						inputDateToggle(cfgID, ctx.Window)
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
			OnClick: func(ctx EventCtx) {
				inputDateClose(cfgID, ctx.Window)
			},
		}))
		content = append(content, Column(ContainerCfg{
			Float:       true,
			FloatAnchor: FloatBottomLeft,
			FloatTieOff: FloatTopLeft,
			Padding:     NoPadding,
			SizeBorder:  NoBorder,
			// Elevation rides the floating wrapper rather than
			// DatePicker itself, because a DatePicker placed inline in
			// a form is not a popover and must stay flat. The wrapper
			// exists only on the open-popup path, so it is the one
			// place the distinction is already made. Radius matches the
			// picker's so the shadow follows its corners.
			Shadow:       defaultDatePickerStyle.Shadow,
			Radius:       SomeF(defaultDatePickerStyle.Radius),
			FloatOffsetY: -cfg.SizeBorder.Get(0),
			// The popup floats over the form; a click inside it is the
			// popup's, not that of the field it is covering.
			OnClick: func(ctx EventCtx) {
				ctx.Consume()
			},
			Content: []View{
				DatePicker(DatePickerCfg{
					ID:                   ScopeID(cfgID, "picker"),
					Dates:                dates,
					AllowedWeekdays:      cfg.AllowedWeekdays,
					AllowedMonths:        cfg.AllowedMonths,
					AllowedYears:         cfg.AllowedYears,
					AllowedDates:         cfg.AllowedDates,
					WeekdaysLen:          cfg.WeekdaysLen,
					TextStyle:            cfg.TextStyle,
					Color:                cfg.Colors.Base,
					Colors:               ColorSet{Hover: cfg.Colors.Hover, Click: cfg.Colors.Click, Focus: cfg.Colors.Focus, Border: cfg.Colors.Border, BorderFocus: cfg.Colors.BorderFocus},
					ColorSelect:          cfg.ColorSelect,
					ColorTextOnSelect:    cfg.ColorTextOnSelect,
					SizeBorder:           cfg.SizeBorder,
					CellSpacing:          cfg.CellSpacing,
					Radius:               cfg.Radius,
					RadiusBorder:         cfg.RadiusBorder,
					SelectMultiple:       cfg.SelectMultiple,
					HideTodayIndicator:   cfg.HideTodayIndicator,
					MondayFirstDayOfWeek: cfg.MondayFirstDayOfWeek,
					ShowAdjacentMonths:   cfg.ShowAdjacentMonths,
					OnSelect: func(dates []time.Time, ctx EventCtx) {
						inputDateClose(cfgID, ctx.Window)
						if cfg.OnSelect != nil {
							cfg.OnSelect(dates, EventCtx{nil, ctx.Event, ctx.Window})
						}
					},
				}),
			},
		}))
	}

	col := Column(ContainerCfg{
		ID:        cfg.ID,
		Focusable: !cfg.FocusDisabled,
		A11YRole:  AccessRoleDateField,
		A11YState: a11yReadOnlyState(cfg.ReadOnly),
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, "Date Input"),
			A11YDescription: cfg.A11YDescription,
		},
		Color:       cfg.Colors.Base,
		ColorBorder: cfg.Colors.Border,
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
		AmendLayout: focusRingAmend(Color{}, cfg.Colors.BorderFocus),
	})
	return generateViewLayout(col, w)
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
		ID: ScopeID(cfgID, "input"),
		// InputDate focus is default-on; propagate FocusDisabled
		// intent to the inner Input.
		FocusDisabled: cfg.FocusDisabled,
		ReadOnly:      cfg.ReadOnly,
		Text:          dateText,
		Placeholder:   inputDatePlaceholder(cfg),
		Mask:          localeDateMaskPattern(ActiveLocale.Date.ShortDate),
		// The mask admits digits and separators only, so the reserved
		// descent is provably empty and the date can be centred on its
		// ink rather than on its line box (issue #346).
		opticalDigitCenter: true,
		TextStyle:          cfg.TextStyle,
		PlaceholderStyle:   cfg.PlaceholderStyle,
		Sizing:             FillFit,
		noMinWidthFloor:    true,
		SizeBorder:         NoBorder,
		Padding:            NoPadding,
		Color:              ColorTransparent,
		Disabled:           cfg.Disabled,
		OnTextChanged: func(s string, ctx EventCtx) {
			sm := StateMap[string, string](ctx.Window, nsInputDateText, capModerate)
			sm.Set(cfgID, s)
		},
		OnTextCommit: func(text string, _ InputCommitReason, ctx EventCtx) {
			// A read-only inner Input still fires OnTextCommit on Enter
			// (text unchanged); do not surface it as a date selection.
			if cfg.ReadOnly {
				return
			}
			if text == "" {
				if cfg.OnSelect != nil {
					cfg.OnSelect(nil, EventCtx{nil, &Event{}, ctx.Window})
				}
				ctx.Window.UpdateWindow()
				return
			}
			t, err := localeParseDate(text,
				localeDatePadFormat(ActiveLocale.Date.ShortDate))
			if err != nil {
				return
			}
			if cfg.OnSelect != nil {
				cfg.OnSelect([]time.Time{t}, EventCtx{nil, &Event{}, ctx.Window})
			}
			ctx.Window.UpdateWindow()
		},
		OnKeyDown: func(ctx EventCtx) {
			if isOpen && ctx.Event.KeyCode == KeyEscape {
				inputDateClose(cfgID, ctx.Window)
				ctx.Consume()
			}
		},
	})
}

func inputDatePlaceholder(cfg *InputDateCfg) string {
	if cfg.Placeholder != "" {
		return cfg.Placeholder
	}
	return localeDatePadFormat(ActiveLocale.Date.ShortDate)
}

func inputDateToggle(id string, w *Window) {
	sm := StateMap[string, bool](w, nsInputDate, capModerate)
	// Default false: absent entry means picker is closed.
	cur := sm.GetOr(id, false)
	sm.Set(id, !cur)
	w.UpdateWindow()
}

func inputDateClose(id string, w *Window) {
	sm := StateMap[string, bool](w, nsInputDate, capModerate)
	sm.Set(id, false)
	w.UpdateWindow()
}

func applyInputDateDefaults(cfg *InputDateCfg) {
	d := &defaultDatePickerStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.colorClick,
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = guiTheme.PaddingField
	}
	sizeBorder := cfg.SizeBorder.Get(d.SizeBorder)
	cellSpacing := cfg.CellSpacing.Get(d.cellSpacing)
	radius := cfg.Radius.Get(d.Radius)
	radiusBorder := cfg.RadiusBorder.Get(d.radiusBorder)
	cfg.SizeBorder = Some(sizeBorder)
	cfg.CellSpacing = Some(cellSpacing)
	cfg.Radius = Some(radius)
	cfg.RadiusBorder = Some(radiusBorder)
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	cfg.MinWidth = fieldMinWidth(cfg.MinWidth, cfg.Width)
	if cfg.PlaceholderStyle == (TextStyle{}) {
		cfg.PlaceholderStyle = withRoleAlpha(
			d.TextStyle, guiTheme.TextStylePlaceholder)
	}
}
