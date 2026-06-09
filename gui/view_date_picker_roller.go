package gui

import (
	"strconv"
	"time"
)

// DatePickerRollerDisplayMode controls which drums are shown.
type DatePickerRollerDisplayMode uint8

// DatePickerRollerDisplayMode constants.
const (
	RollerDayMonthYear DatePickerRollerDisplayMode = iota // DD MMM YYYY
	RollerMonthDayYear                                    // MMM DD YYYY
	RollerMonthYear                                       // MMM YYYY
	RollerYearOnly                                        // YYYY
)

// DatePickerRollerCfg configures a roller-style date picker.
type DatePickerRollerCfg struct {
	TextStyle        TextStyle
	SelectedDate     time.Time
	OnChange         func(time.Time, *Window)
	ID               string
	A11YLabel        string
	A11YDescription  string
	MinYear          int
	MaxYear          int
	VisibleItems     int // must be odd
	Padding          Opt[Padding]
	SizeBorder       Opt[float32]
	Radius           Opt[float32]
	IDFocus          uint32
	ItemHeight       float32
	MinWidth         float32
	MaxWidth         float32
	WidthDay         float32
	WidthMonth       float32
	WidthYear        float32
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color
	DisplayMode      DatePickerRollerDisplayMode
	LongMonths       bool // true = "January", false = "Jan"
	WrapYear         bool
}

type datePickerRollerView struct {
	cfg DatePickerRollerCfg
}

// DatePickerRoller creates a roller-style date picker view.
func DatePickerRoller(cfg DatePickerRollerCfg) View {
	applyRollerDefaults(&cfg)
	return &datePickerRollerView{cfg: cfg}
}

func (rv *datePickerRollerView) Content() []View { return nil }

func (rv *datePickerRollerView) GenerateLayout(w *Window) Layout {
	cfg := &rv.cfg
	sel := cfg.SelectedDate

	specs := rollerDrumSpecs(cfg, sel)
	drumNames := make([]string, len(specs))
	drums := make([]View, len(specs))
	for i, s := range specs {
		drumNames[i] = s.name
		drums[i] = rollerDrum(cfg, s.name, s.value,
			s.minVal, s.maxVal, s.format, s.width, s.wrap)
	}

	onChange := cfg.OnChange
	minYear := cfg.MinYear
	maxYear := cfg.MaxYear
	selectedDate := cfg.SelectedDate
	mode := cfg.DisplayMode
	wrapYear := cfg.WrapYear

	return generateViewLayout(container(ContainerCfg{
		ID:          cfg.ID,
		IDFocus:     cfg.IDFocus,
		A11YRole:    AccessRoleDateField,
		A11YLabel:   a11yLabel(cfg.A11YLabel, "Date Roller"),
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  cfg.SizeBorder,
		Radius:      cfg.Radius,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		Padding:     cfg.Padding,
		Spacing:     Some(SpacingSmall),
		HAlign:      HAlignCenter,
		VAlign:      VAlignMiddle,
		axis:        AxisLeftToRight,
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			rollerOnKeyDown(onChange, selectedDate,
				minYear, maxYear, e, w, mode, wrapYear)
		},
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if cfg.IDFocus > 0 {
				w.SetIDFocus(cfg.IDFocus)
				e.IsHandled = true
			}
		},
		AmendLayout: func(lo *Layout, w *Window) {
			if cfg.IDFocus > 0 && w.IsFocus(cfg.IDFocus) {
				lo.Shape.ColorBorder = cfg.ColorBorderFocus
			}
			lo.Shape.events.OnMouseScroll = func(
				_ *Layout, e *Event, w *Window,
			) {
				e.IsHandled = true
				if onChange == nil {
					return
				}
				delta := 1
				if e.ScrollY > 0 {
					delta = -1
				}
				for i, child := range lo.Children {
					if i < len(drumNames) &&
						child.Shape.PointInShape(
							e.MouseX, e.MouseY) {
						rollerDrumAdjust(drumNames[i],
							delta, selectedDate,
							minYear, maxYear, onChange, w, wrapYear)
						return
					}
				}
			}
		},
		Content: drums,
	}), w)
}

type drumSpec struct {
	name   string
	value  int
	minVal int
	maxVal int
	format func(int) string
	width  float32
	wrap   bool
}

func rollerDrumSpecs(
	cfg *DatePickerRollerCfg, sel time.Time,
) []drumSpec {
	day := drumSpec{"day", sel.Day(), 1,
		datePickerDaysInMonth(int(sel.Month()), sel.Year()),
		rollerDayFormat, cfg.WidthDay, true}
	month := drumSpec{"month", int(sel.Month()), 1, 12,
		rollerMonthFormat(cfg.LongMonths), cfg.WidthMonth, true}
	year := drumSpec{"year", sel.Year(),
		cfg.MinYear, cfg.MaxYear,
		rollerYearFormat, cfg.WidthYear, cfg.WrapYear}

	switch cfg.DisplayMode {
	case RollerMonthDayYear:
		return []drumSpec{month, day, year}
	case RollerMonthYear:
		return []drumSpec{month, year}
	case RollerYearOnly:
		return []drumSpec{year}
	default: // RollerDayMonthYear
		return []drumSpec{day, month, year}
	}
}

// rollerDrum builds a single drum column showing visibleItems.
func rollerDrum(
	cfg *DatePickerRollerCfg, name string,
	value, minVal, maxVal int,
	format func(int) string, drumWidth float32,
	wrap bool,
) View {
	vis := cfg.VisibleItems
	half := vis / 2
	ts := cfg.TextStyle
	onChange := cfg.OnChange
	selectedDate := cfg.SelectedDate
	minYear := cfg.MinYear
	maxYear := cfg.MaxYear

	items := make([]View, 0, vis)
	for i := range vis {
		offset := i - half
		v := value + offset

		if wrap {
			v = wrapRange(v, minVal, maxVal)
		}

		label := format(v)
		if v < minVal || v > maxVal {
			label = " "
		}

		itemTS := ts
		if offset == 0 {
			itemTS.Size = ts.Size + 2
		} else {
			dist := offset
			if dist < 0 {
				dist = -dist
			}
			alpha := uint8(150)
			if dist > 1 {
				alpha = 80
			}
			itemTS.Color = RGBA(
				ts.Color.R, ts.Color.G, ts.Color.B, alpha)
		}

		centeredTS := itemTS
		centeredTS.Align = TextAlignCenter
		itemCfg := ContainerCfg{
			Width:   drumWidth,
			Height:  cfg.ItemHeight,
			Padding: NoPadding,
			HAlign:  HAlignCenter,
			VAlign:  VAlignMiddle,
			Content: []View{
				Text(TextCfg{
					Text:      label,
					TextStyle: centeredTS,
					Sizing:    FillFit,
				}),
			},
		}
		itemClickable := offset != 0 && (v >= minVal && v <= maxVal || wrap)
		if itemClickable {
			clickDelta := offset
			itemCfg.OnClick = func(
				_ *Layout, _ *Event, w *Window,
			) {
				rollerDrumAdjust(name, clickDelta,
					selectedDate, minYear, maxYear,
					onChange, w, wrap)
			}
		}
		items = append(items, Row(itemCfg))
	}

	return Column(ContainerCfg{
		Width:   drumWidth,
		Padding: NoPadding,
		Content: items,
	})
}

func rollerDrumAdjust(
	name string, delta int, sel time.Time,
	minYear, maxYear int,
	onChange func(time.Time, *Window), w *Window,
	wrapYear bool,
) {
	switch name {
	case "day":
		rollerAdjustDay(delta, sel, minYear, maxYear, onChange, w)
	case "month":
		rollerAdjustMonth(delta, sel, minYear, maxYear, onChange, w)
	case "year":
		rollerAdjustYear(delta, sel, minYear, maxYear, onChange, w, wrapYear)
	}
}

// rollerOnKeyDown handles keyboard navigation for the roller.
func rollerOnKeyDown(
	onChange func(time.Time, *Window),
	sel time.Time, minYear, maxYear int,
	e *Event, w *Window,
	mode DatePickerRollerDisplayMode,
	wrapYear bool,
) {
	if onChange == nil {
		return
	}
	switch {
	case e.Modifiers == ModNone && e.KeyCode == KeyUp:
		switch mode {
		case RollerYearOnly:
			rollerAdjustYear(-1, sel, minYear, maxYear, onChange, w, wrapYear)
		case RollerMonthYear:
			rollerAdjustMonth(-1, sel, minYear, maxYear, onChange, w)
		default:
			rollerAdjustDay(-1, sel, minYear, maxYear, onChange, w)
		}
		e.IsHandled = true
	case e.Modifiers == ModNone && e.KeyCode == KeyDown:
		switch mode {
		case RollerYearOnly:
			rollerAdjustYear(1, sel, minYear, maxYear, onChange, w, wrapYear)
		case RollerMonthYear:
			rollerAdjustMonth(1, sel, minYear, maxYear, onChange, w)
		default:
			rollerAdjustDay(1, sel, minYear, maxYear, onChange, w)
		}
		e.IsHandled = true
	case e.Modifiers == ModShift && e.KeyCode == KeyUp:
		rollerAdjustYear(-1, sel, minYear, maxYear, onChange, w, wrapYear)
		e.IsHandled = true
	case e.Modifiers == ModShift && e.KeyCode == KeyDown:
		rollerAdjustYear(1, sel, minYear, maxYear, onChange, w, wrapYear)
		e.IsHandled = true
	case e.Modifiers == ModAlt && e.KeyCode == KeyUp:
		rollerAdjustMonth(-1, sel, minYear, maxYear, onChange, w)
		e.IsHandled = true
	case e.Modifiers == ModAlt && e.KeyCode == KeyDown:
		rollerAdjustMonth(1, sel, minYear, maxYear, onChange, w)
		e.IsHandled = true
	}
}

func rollerAdjustDay(
	delta int, sel time.Time,
	minYear, maxYear int,
	onChange func(time.Time, *Window), w *Window,
) {
	if onChange == nil {
		return
	}
	newDate := sel.AddDate(0, 0, delta)
	if newDate.Year() < minYear || newDate.Year() > maxYear {
		return
	}
	onChange(newDate, w)
}

func rollerAdjustMonth(
	delta int, sel time.Time,
	minYear, maxYear int,
	onChange func(time.Time, *Window), w *Window,
) {
	if onChange == nil {
		return
	}
	// Explicitly calculate to handle month-end clamping (e.g. Jan 31 -> Feb 28).
	y, m, d := sel.Date()
	nm := int(m) + delta
	ny := y
	for nm < 1 {
		nm += 12
		ny--
	}
	for nm > 12 {
		nm -= 12
		ny++
	}

	if ny < minYear || ny > maxYear {
		return
	}

	dim := datePickerDaysInMonth(nm, ny)
	nd := min(d, dim)
	newDate := time.Date(ny, time.Month(nm), nd,
		sel.Hour(), sel.Minute(), sel.Second(), sel.Nanosecond(),
		sel.Location())
	onChange(newDate, w)
}

func rollerAdjustYear(
	delta int, sel time.Time,
	minYear, maxYear int,
	onChange func(time.Time, *Window), w *Window,
	wrap bool,
) {
	if onChange == nil {
		return
	}
	newYear := sel.Year() + delta
	if wrap {
		newYear = wrapRange(newYear, minYear, maxYear)
	} else {
		if newYear < minYear || newYear > maxYear {
			return
		}
	}
	dim := datePickerDaysInMonth(int(sel.Month()), newYear)
	day := min(sel.Day(), dim)
	newDate := time.Date(newYear, sel.Month(), day,
		sel.Hour(), sel.Minute(), sel.Second(), sel.Nanosecond(),
		sel.Location())
	onChange(newDate, w)
}

func rollerDayFormat(v int) string {
	if v >= 1 && v <= 9 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}
func rollerYearFormat(v int) string { return strconv.Itoa(v) }

func rollerMonthFormat(long bool) func(int) string {
	return func(v int) string {
		idx := v - 1
		if idx < 0 || idx >= 12 {
			return ""
		}
		if long {
			return ActiveLocale.MonthsFull[idx]
		}
		return ActiveLocale.MonthsShort[idx]
	}
}

// wrapRange wraps v into [lo, hi] using modular arithmetic.
func wrapRange(v, lo, hi int) int {
	span := hi - lo + 1
	for v < lo {
		v += span
	}
	for v > hi {
		v -= span
	}
	return v
}

func applyRollerDefaults(cfg *DatePickerRollerCfg) {
	if cfg.MinYear == 0 {
		cfg.MinYear = 1900
	}
	if cfg.MaxYear == 0 {
		cfg.MaxYear = 2100
	}
	if cfg.MinYear > cfg.MaxYear {
		cfg.MinYear, cfg.MaxYear = cfg.MaxYear, cfg.MinYear
	}
	if cfg.ItemHeight == 0 {
		cfg.ItemHeight = 24
	}
	if cfg.VisibleItems == 0 {
		cfg.VisibleItems = 3
	}
	if cfg.VisibleItems%2 == 0 {
		cfg.VisibleItems++
	}
	if cfg.WidthDay == 0 {
		cfg.WidthDay = 40
	}
	if cfg.WidthMonth == 0 {
		if cfg.LongMonths {
			cfg.WidthMonth = 100
		} else {
			cfg.WidthMonth = 64
		}
	}
	if cfg.WidthYear == 0 {
		cfg.WidthYear = 52
	}
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.ColorBackground
	}
	d := &DefaultDatePickerStyle
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorBorderFocus.IsSet() {
		cfg.ColorBorderFocus = d.ColorBorderFocus
	}
	if !cfg.SizeBorder.IsSet() {
		cfg.SizeBorder = Some(d.SizeBorder)
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = Some(d.RadiusBorder)
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(PaddingSmall)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = DefaultTextStyle
	}
	if cfg.SelectedDate.IsZero() {
		cfg.SelectedDate = time.Now()
	}
}
