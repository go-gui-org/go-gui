package gui

import (
	"fmt"
	"strconv"
	"strings"
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
	ID               string `gui:"auto"`
	Placeholder      string
	// DateFormat spells the date the field shows, masks and parses,
	// in the locale token language: YYYY, MM, M, DD, D and literal
	// separators — "DD.MM.YYYY", "YYYY-MM-DD". Unset takes the
	// active locale's short date, which is what every field did
	// before issue #578. Month-name tokens (MMM, MMMM), a 2-digit
	// year (YY) and time tokens (HH, mm, ss) are rejected: the
	// field is a masked numeric entry, so a text month could be
	// displayed but never typed back, and the parse only reads
	// YYYY, MM, M, DD, D.
	DateFormat string

	A11YCfg
	Dates           []time.Time
	AllowedWeekdays []DatePickerWeekdays
	AllowedMonths   []DatePickerMonths
	AllowedYears    []int
	AllowedDates    []time.Time
	Padding         Padding
	SizeBorder      Border
	// CellSpacing gaps the day cells. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	CellSpacing Spacing
	Radius      Radius
	// RadiusBorder rounds the calendar frame. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusBorder Radius
	// FocusDisabled opts out of the default-on focus.
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
	ColorTextOnSelect Color
	Sizing            Sizing
	WeekdaysLen       DatePickerWeekdayLen
	Disabled          bool
	Invisible         bool
	SelectMultiple    bool
	// HideTodayIndicator turns off the border drawn around today's cell,
	// which is otherwise on. Named for the opt-out because the indicator is
	// the default: a calendar that does not mark today is the rare choice
	// (issue #504).
	HideTodayIndicator   bool
	MondayFirstDayOfWeek bool
	// ShowAdjacentMonths fills the grid's leading and trailing cells with
	// the neighboring months' days, which is otherwise off. Named for the
	// opt-in because the plain grid is the default (issue #504).
	ShowAdjacentMonths bool

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
	// No ID: take a generated leaf at generation time (#881); see
	// id_auto.go. Also with FocusDisabled: the inner field and the
	// calendar button take IDs scoped by this one, so two ID-less
	// copies would otherwise both claim "input" and "calendar".
	if cfg.ID == "" {
		autoCfg := cfg
		return ViewFunc(func(vw *Window) View {
			autoCfg.ID = vw.autoLeaf("inputdate")
			return InputDate(autoCfg)
		})
	}
	applyInputDateDefaults(&cfg)
	requireDateFormat("InputDate", cfg.DateFormat)
	cfg.A11YLabel = a11yProseLabel(cfg.A11YLabel, cfg.Label)
	return labelledField(
		cfg.Label, cfg.TextStyle, HAlignLeft, cfg.Sizing,
		&inputDateView{cfg: cfg})
}

func (idv *inputDateView) GenerateLayout(w *Window) Layout {
	cfg := &idv.cfg

	// Resolve once, here: every state key and every inner ID below is
	// built from cfgID, and only generation time knows the ID scope the
	// field sits in. A resolve in the InputDate factory would be a
	// no-op — the factory runs before generateViewLayout descends the
	// View tree, so the scope stack is still empty there. See issue
	// #518.
	cfgID := w.EffID(cfg.ID)
	// One resolve per frame: display, mask, parse and placeholder all
	// read this, and re-deriving it at each site is how they drift.
	format := inputDateFormat(cfg)
	// A read-only date field never opens the calendar popup, closing
	// the picker's OnSelect mutation path structurally regardless of any
	// stored open state.
	isOpen := StateReadOr(w, nsInputDate, cfgID, false) && !cfg.ReadOnly

	// Format date for display.
	dates := cfg.Dates
	if len(dates) == 0 && !cfg.Date.IsZero() {
		dates = []time.Time{cfg.Date}
	}

	dateText := ""
	if len(dates) == 1 {
		dateText = LocaleFormatDate(dates[0], format)
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
			Spacing:    SpacingSmall,
			VAlign:     VAlignMiddle,
			Content: []View{
				inputDateTextField(cfg, cfgID, format, isOpen, editText),
				Button(ButtonCfg{
					// Namespaced by the field's ID: a form can hold
					// several date inputs.
					ID:       ScopeID(cfgID, "calendar"),
					Disabled: cfg.Disabled || cfg.ReadOnly,
					// The opt-out covers the whole control, not
					// only the text field.
					FocusDisabled: cfg.FocusDisabled,
					Padding:       NoPadding,
					SizeBorder:    NoBorder,
					// The icon font's calendar, not the 📅 emoji: a color
					// emoji ignores the theme's text color and was the only
					// one among the control icons. In the icon role, the
					// button centres it on its own ink like any glyph.
					Content: []View{Text(TextCfg{
						Text:      IconCalendar,
						TextStyle: inputDateIconStyle(cfg.TextStyle),
					})},
					OnClick: func(ctx EventCtx) {
						inputDateToggle(cfgID, ctx.Window)
						// Open with focus, as Select does. An
						// accessibility press runs OnClick without the
						// press walk's focus take, and the blur dismiss
						// would close an unfocused calendar the next
						// frame (#1212).
						if !cfg.FocusDisabled &&
							StateReadOr(ctx.Window, nsInputDate, cfgID, false) {
							ctx.Window.SetFocus(ScopeID(cfgID, "calendar"))
						}
						// Consumed for a FocusDisabled field, whose
						// button does not mark the press handled by
						// taking focus (as in Select, #1183).
						ctx.Consume()
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
			Radius:       RadiusPx(defaultDatePickerStyle.Radius),
			FloatOffsetY: -cfg.SizeBorder.Or(0),
			// The popup floats over the form; a click inside it is the
			// popup's, not that of the field it is covering.
			OnClick: func(ctx EventCtx) {
				ctx.Consume()
			},
			Content: []View{
				DatePicker(DatePickerCfg{
					ID: ScopeID(cfgID, "picker"),
					// The opt-out covers the calendar too: a picker
					// that took focus would hold it inside a field
					// that skips the blur dismiss, so the calendar
					// would stay open once focus left it (#1212).
					FocusDisabled:        cfg.FocusDisabled,
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
						// A FocusDisabled field leaves focus with the
						// widget that holds it, as Select does (#1183).
						if !cfg.FocusDisabled {
							inputDateFocusAfterPick(cfgID, ctx)
						}
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
		// An open calendar closes once focus leaves the field, so a
		// second date field's calendar does not open over this one
		// (#1212). The backdrop covers only the field, not the window.
		AmendLayout: dropdownBlurAmend(cfgID, isOpen,
			focusRingAmend(Color{}, cfg.Colors.BorderFocus), inputDateClose),
	})
	return generateViewLayout(col, w)
}

// inputDateTextField returns an Input for single/no dates (editable)
// or a Text for multi-select display ("N dates selected").
func inputDateTextField(
	cfg *InputDateCfg, cfgID, format string, isOpen bool,
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
		Placeholder:   inputDatePlaceholder(cfg, format),
		Mask:          localeDateMaskPattern(format),
		// The mask admits digits and separators only, so the reserved
		// descent is provably empty and the date can be centred on its
		// ink rather than on its line box (issue #346).
		opticalDigitCenter: true,
		Keyboard:           KeyboardNumber, // the mask inserts the separators
		TextStyle:          cfg.TextStyle,
		PlaceholderStyle:   cfg.PlaceholderStyle,
		Sizing:             FillFit,
		NoMinWidthFloor:    true,
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
					cfg.OnSelect(nil, EventCtx{nil, nil, ctx.Window})
				}
				ctx.Window.InvalidateLayout()
				return
			}
			t, err := localeParseDate(text, format)
			if err != nil {
				return
			}
			if cfg.OnSelect != nil {
				cfg.OnSelect([]time.Time{t}, EventCtx{nil, nil, ctx.Window})
			}
			ctx.Window.InvalidateLayout()
		},
		OnKeyDown: func(ctx EventCtx) {
			if isOpen && ctx.Event.KeyCode == KeyEscape {
				inputDateClose(cfgID, ctx.Window)
				ctx.Consume()
			}
		},
	})
}

func inputDatePlaceholder(cfg *InputDateCfg, format string) string {
	if cfg.Placeholder != "" {
		return cfg.Placeholder
	}
	return format
}

func inputDateToggle(id string, w *Window) {
	sm := StateMap[string, bool](w, nsInputDate, capModerate)
	// Default false: absent entry means picker is closed.
	cur := sm.GetOr(id, false)
	// One calendar open at a time, as Select keeps one list open. A
	// FocusDisabled field skips the blur dismiss, so its calendar
	// would otherwise stay open under the next one (#1212).
	inputDateSettleFocus(w)
	sm.Clear()
	if !cur {
		sm.Set(id, true)
		inputDateRollerReset(id, w)
	}
	w.InvalidateLayout()
}

// inputDateRollerReset puts the picker's month and year roller away,
// so a calendar always opens on its day grid. The picker keeps its
// state across a close, and a close from outside the picker (blur, an
// unhandled press, another calendar opening) never runs the roller's
// own dismiss, so a roller left up would come back with the calendar
// (#1212). Reset at the one open path rather than at the many close
// paths. The viewed month stays: that is navigation, not a popup.
func inputDateRollerReset(id string, w *Window) {
	pickerID := ScopeID(id, "picker")
	sm := StateMapRead[string, datePickerState](w, nsDatePicker)
	if sm == nil {
		return
	}
	if s, ok := sm.Get(pickerID); ok && s.ShowYearMonthPicker {
		s.ShowYearMonthPicker = false
		sm.Set(pickerID, s)
	}
}

func inputDateClose(id string, w *Window) {
	// Deleted, not set false, as selectClose does: an absent entry
	// reads as closed, and an empty map lets fixupPopupMaps skip its
	// per-frame walk.
	StateMap[string, bool](w, nsInputDate, capModerate).Delete(id)
	w.InvalidateLayout()
}

// inputDateEscape closes the open calendar that holds focus, on an
// Escape no widget handled, and puts focus back on its calendar
// button. Keys go only to the focused shape, and inside an open
// calendar focus can sit on the calendar button, the picker or any of
// its day, header and roller buttons, so a handler on each would be
// needed otherwise (#1212). The text field's own Escape and the
// roller's run first and handle the key.
func inputDateEscape(w *Window, e *Event) {
	sm := StateMapRead[string, bool](w, nsInputDate)
	if sm == nil || sm.Len() == 0 {
		return
	}
	focus := w.FocusID()
	openID := ""
	sm.Range(func(id string, isOpen bool) bool {
		if isOpen && targetWithin(focus, id) {
			openID = id
			return false
		}
		return true
	})
	if openID == "" {
		return
	}
	// With the roller up, focus can sit on its done button, which has
	// no Escape of its own: the first Escape closes only the roller,
	// as it does from the picker, and focus goes to the picker rather
	// than stay on the done button that leaves with it.
	if pickerID := ScopeID(openID, "picker"); datePickerRollerOpen(pickerID, w) {
		datePickerRollerDismiss(pickerID, w)
		e.IsHandled = true
		return
	}
	inputDateClose(openID, w)
	inputDateFocusButton(openID, w)
	e.IsHandled = true
}

// inputDateFocusAfterPick settles focus once a pick closes the
// calendar. The picked day held focus and goes away with the calendar;
// left there, the window's focus repair would move focus to the first
// tab stop in the window (#1212). A pointer pick clears focus. Any
// other pick returns it to the calendar button, so the user keeps their
// place in the form: Enter on the grid (key down), Space on a day
// button (its click fires on key up) and a screen reader's press,
// which arrives as a mouse down marked a11yPress.
func inputDateFocusAfterPick(id string, ctx EventCtx) {
	if inputDatePointerEvent(ctx.Event) {
		ctx.Window.ClearFocus()
		return
	}
	inputDateFocusButton(id, ctx.Window)
}

// inputDatePointerEvent reports whether e is a mouse or touch press. A
// screen reader's press arrives as a mouse down marked a11yPress and
// counts as a keyboard action, as do a key down (Enter) and a key up
// (a button's Space click fires on release).
func inputDatePointerEvent(e *Event) bool {
	if e == nil || e.a11yPress {
		return false
	}
	switch e.Type {
	case EventMouseDown, EventMouseUp,
		EventTouchesBegan, EventTouchesEnded:
		return true
	}
	return false
}

// inputDateFocusButton moves focus to the field's calendar button. A
// FocusDisabled field's picker still takes focus, but its calendar
// button does not: there, focus is cleared rather than left on an ID
// no focusable shape claims.
func inputDateFocusButton(id string, w *Window) {
	btnID := ScopeID(id, "calendar")
	if ly, ok := w.layout.findByID(btnID); ok && ly.Shape.canTakeFocus() {
		w.SetFocus(btnID)
		return
	}
	w.ClearFocus()
}

// inputDateSettleFocus settles focus before every open calendar
// closes: on the window's blur (dismissFieldPopups), and when another
// calendar opens (inputDateToggle) from a FocusDisabled field's button,
// which takes no focus of its own. Focus inside an open calendar's
// picker would be left on a shape that leaves the tree, and the
// window's focus repair would move it to the first tab stop in the
// window; it goes back to the calendar button instead (#1212). Focus
// on the text field or the button stays where it is. Event path only:
// it takes the window lock through SetFocus.
func inputDateSettleFocus(w *Window) {
	sm := StateMapRead[string, bool](w, nsInputDate)
	if sm == nil || sm.Len() == 0 {
		return
	}
	focus := w.FocusID()
	openID := ""
	sm.Range(func(id string, isOpen bool) bool {
		if isOpen && targetWithin(focus, ScopeID(id, "picker")) {
			openID = id
			return false
		}
		return true
	})
	if openID != "" {
		inputDateFocusButton(openID, w)
	}
}

// inputDatePressOutside closes the open calendar when a press lands
// outside its field. The blur dismiss covers a press that moves focus
// out, and the unhandled-press cleanup a press nothing takes. A press
// that another widget handles without moving focus out of the field
// falls between them: an Input pressed beside a FocusDisabled field,
// whose calendar never holds focus, or a FocusDisabled button pressed
// beside any field (#1212). The hit test maps a press in the floating
// calendar back to its field, as it does for hover and press state.
// Runs before the press walk, so a press that opens another calendar
// still opens it, and at touch-down, before a scrollable can hold the
// press and drop it on a pan. x and y are window coordinates. Event
// path only: settling focus takes the window lock through SetFocus.
func inputDatePressOutside(w *Window, x, y float32) {
	sm := StateMapRead[string, bool](w, nsInputDate)
	if sm == nil || sm.Len() == 0 {
		return
	}
	target := interactionTargetAt(w.layout.Children, x, y, w)
	openID := ""
	sm.Range(func(id string, isOpen bool) bool {
		if isOpen && !targetWithin(target, id) {
			openID = id
			return false
		}
		return true
	})
	if openID == "" {
		return
	}
	inputDateSettleFocus(w)
	inputDateClose(openID, w)
}

// inputDateFormat resolves the one format the field shows, masks and
// parses with. Every read goes through here so display, mask, parse
// and placeholder cannot drift apart; the padded form is what all four
// want, because a masked field always has both digits (issue #578).
func inputDateFormat(cfg *InputDateCfg) string {
	if cfg.DateFormat != "" {
		return localeDatePadFormat(cfg.DateFormat)
	}
	return localeDatePadFormat(activeLocaleShared().Date.ShortDate)
}

// requireDateFormat panics on a DateFormat the masked numeric field
// cannot honour. A month name renders but cannot be typed back, a
// 2-digit year and a time token display one thing and parse another
// (localeParseDate only reads YYYY, MM, M, DD, D), and a format with
// no date token at all masks to a row of literals, so each produces a
// field that looks right and refuses every keystroke. Fail at
// construction instead, the way RequireID does (issue #578).
func requireDateFormat(widget, format string) {
	if format == "" {
		return
	}
	if strings.Contains(format, "MMM") {
		panic("gui: " + widget + " DateFormat " + strconv.Quote(format) +
			" uses a month-name token; the field masks digits only")
	}
	if strings.Contains(format, "YY") &&
		!strings.Contains(format, "YYYY") {
		panic("gui: " + widget + " DateFormat " + strconv.Quote(format) +
			" uses a 2-digit year; spell YYYY")
	}
	if strings.Contains(format, "HH") ||
		strings.Contains(format, "mm") ||
		strings.Contains(format, "ss") {
		panic("gui: " + widget + " DateFormat " + strconv.Quote(format) +
			" uses a time token; the field is date-only")
	}
	padded := localeDatePadFormat(format)
	if !strings.Contains(padded, "YYYY") &&
		!strings.Contains(padded, "MM") &&
		!strings.Contains(padded, "DD") {
		panic("gui: " + widget + " DateFormat " + strconv.Quote(format) +
			" has no YYYY, MM or DD token")
	}
}

func applyInputDateDefaults(cfg *InputDateCfg) {
	d := &defaultDatePickerStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, d.Colors)
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = guiTheme.PaddingField
	}
	sizeBorder := cfg.SizeBorder.Or(d.SizeBorder)
	cellSpacing := cfg.CellSpacing.Or(d.cellSpacing)
	radius := cfg.Radius.Or(d.Radius)
	radiusBorder := cfg.RadiusBorder.Or(d.radiusBorder)
	cfg.SizeBorder = BorderPx(sizeBorder)
	cfg.CellSpacing = SpacingPx(cellSpacing)
	cfg.Radius = RadiusPx(radius)
	cfg.RadiusBorder = RadiusPx(radiusBorder)
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	cfg.MinWidth = fieldMinWidth(cfg.MinWidth, cfg.Width)
	if cfg.PlaceholderStyle == (TextStyle{}) {
		cfg.PlaceholderStyle = withRoleAlpha(
			d.TextStyle, guiTheme.TextStylePlaceholder)
	}
}

// inputDateIconStyle is the icon role at the field's text size and
// color, so the glyph matches the date beside it.
func inputDateIconStyle(field TextStyle) TextStyle {
	icon := guiTheme.TextStyleIconMedium
	icon.Size = field.Size
	icon.Color = field.Color
	return icon
}
