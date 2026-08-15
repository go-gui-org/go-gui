package gui

import (
	"strconv"
	"time"
)

// DatePickerRollerDisplayMode controls which drums are shown.
type datePickerRollerDisplayMode uint8

// DatePickerRollerDisplayMode constants.
const (
	rollerDayMonthYear datePickerRollerDisplayMode = iota // DD MMM YYYY
	rollerMonthDayYear                                    // MMM DD YYYY
	rollerMonthYear                                       // MMM YYYY
	rollerYearOnly                                        // YYYY
)

// DatePickerRollerCfg configures a roller-style date picker.
type DatePickerRollerCfg struct {
	TextStyle    TextStyle
	SelectedDate time.Time
	OnChange     func(time.Time, EventCtx)
	ID           string
	A11YCfg
	minYear          int
	maxYear          int
	visibleItems     int // must be odd
	Padding          Padding
	SizeBorder       Opt[float32]
	Radius           Opt[float32]
	Focusable        bool
	itemHeight       float32
	MinWidth         float32
	MaxWidth         float32
	widthDay         float32
	widthMonth       float32
	widthYear        float32
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color
	displayMode      datePickerRollerDisplayMode
	longMonths       bool // true = "January", false = "Jan"
	wrapYear         bool
}

type datePickerRollerView struct {
	cfg DatePickerRollerCfg
}

// DatePickerRoller creates a roller-style date picker view.
func DatePickerRoller(cfg DatePickerRollerCfg) View {
	applyRollerDefaults(&cfg)
	return &datePickerRollerView{cfg: cfg}
}

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
	minYear := cfg.minYear
	maxYear := cfg.maxYear
	selectedDate := cfg.SelectedDate
	mode := cfg.displayMode
	wrapYear := cfg.wrapYear

	return generateViewLayout(container(ContainerCfg{
		ID:        cfg.ID,
		Focusable: cfg.Focusable,
		A11YRole:  AccessRoleDateField,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, "Date Roller"),
			A11YDescription: cfg.A11YDescription,
		},
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
		axis:        axisLeftToRight,
		OnKeyDown: func(ctx EventCtx) {
			rollerOnKeyDown(onChange, selectedDate,
				minYear, maxYear, ctx.Event, ctx.Window, mode, wrapYear)
		},
		OnClick: func(ctx EventCtx) {
			if cfg.Focusable {
				ctx.Window.SetFocus(cfg.ID)
			}
		},
		AmendLayout: func(ctx EventCtx) {
			if cfg.Focusable && ctx.Window.IsFocus(cfg.ID) {
				ctx.Layout.Shape.ColorBorder = cfg.ColorBorderFocus
			}
			ctx.Layout.Shape.events.OnMouseScroll = func(ctx EventCtx) {
				ctx.Consume()
				if onChange == nil {
					return
				}
				delta := 1
				if ctx.Event.ScrollY > 0 {
					delta = -1
				}
				for i, child := range ctx.Layout.Children {
					if i < len(drumNames) &&
						child.Shape.PointInShape(
							ctx.Event.MouseX, ctx.Event.MouseY) {
						rollerDrumAdjust(drumNames[i],
							delta, selectedDate,
							minYear, maxYear, onChange, ctx.Window, wrapYear)
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
		rollerDayFormat, cfg.widthDay, true}
	month := drumSpec{"month", int(sel.Month()), 1, 12,
		rollerMonthFormat(cfg.longMonths), cfg.widthMonth, true}
	year := drumSpec{"year", sel.Year(),
		cfg.minYear, cfg.maxYear,
		rollerYearFormat, cfg.widthYear, cfg.wrapYear}

	switch cfg.displayMode {
	case rollerMonthDayYear:
		return []drumSpec{month, day, year}
	case rollerMonthYear:
		return []drumSpec{month, year}
	case rollerYearOnly:
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
	vis := cfg.visibleItems
	half := vis / 2
	ts := cfg.TextStyle
	onChange := cfg.OnChange
	selectedDate := cfg.SelectedDate
	minYear := cfg.minYear
	maxYear := cfg.maxYear

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
			Height:  cfg.itemHeight,
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
			itemCfg.OnClick = func(ctx EventCtx) {
				rollerDrumAdjust(name, clickDelta,
					selectedDate, minYear, maxYear,
					onChange, ctx.Window, wrap)
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
	onChange func(time.Time, EventCtx), w *Window,
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
	onChange func(time.Time, EventCtx),
	sel time.Time, minYear, maxYear int,
	e *Event, w *Window,
	mode datePickerRollerDisplayMode,
	wrapYear bool,
) {
	if onChange == nil {
		return
	}
	switch {
	case e.Modifiers == ModNone && e.KeyCode == KeyUp:
		switch mode {
		case rollerYearOnly:
			rollerAdjustYear(-1, sel, minYear, maxYear, onChange, w, wrapYear)
		case rollerMonthYear:
			rollerAdjustMonth(-1, sel, minYear, maxYear, onChange, w)
		default:
			rollerAdjustDay(-1, sel, minYear, maxYear, onChange, w)
		}
		e.IsHandled = true
	case e.Modifiers == ModNone && e.KeyCode == KeyDown:
		switch mode {
		case rollerYearOnly:
			rollerAdjustYear(1, sel, minYear, maxYear, onChange, w, wrapYear)
		case rollerMonthYear:
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
	onChange func(time.Time, EventCtx), w *Window,
) {
	if onChange == nil {
		return
	}
	newDate := sel.AddDate(0, 0, delta)
	if newDate.Year() < minYear || newDate.Year() > maxYear {
		return
	}
	onChange(newDate, EventCtx{nil, nil, w})
}

func rollerAdjustMonth(
	delta int, sel time.Time,
	minYear, maxYear int,
	onChange func(time.Time, EventCtx), w *Window,
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
	onChange(newDate, EventCtx{nil, nil, w})
}

func rollerAdjustYear(
	delta int, sel time.Time,
	minYear, maxYear int,
	onChange func(time.Time, EventCtx), w *Window,
	wrap bool,
) {
	if onChange == nil {
		return
	}
	newYear := sel.Year() + delta
	if wrap {
		newYear = wrapRange(newYear, minYear, maxYear)
	} else if newYear < minYear || newYear > maxYear {
		return
	}
	dim := datePickerDaysInMonth(int(sel.Month()), newYear)
	day := min(sel.Day(), dim)
	newDate := time.Date(newYear, sel.Month(), day,
		sel.Hour(), sel.Minute(), sel.Second(), sel.Nanosecond(),
		sel.Location())
	onChange(newDate, EventCtx{nil, nil, w})
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
	if cfg.minYear == 0 {
		cfg.minYear = 1900
	}
	if cfg.maxYear == 0 {
		cfg.maxYear = 2100
	}
	if cfg.minYear > cfg.maxYear {
		cfg.minYear, cfg.maxYear = cfg.maxYear, cfg.minYear
	}
	if cfg.itemHeight == 0 {
		cfg.itemHeight = 24
	}
	if cfg.visibleItems == 0 {
		cfg.visibleItems = 3
	}
	if cfg.visibleItems%2 == 0 {
		cfg.visibleItems++
	}
	if cfg.widthDay == 0 {
		cfg.widthDay = 40
	}
	if cfg.widthMonth == 0 {
		if cfg.longMonths {
			cfg.widthMonth = 100
		} else {
			cfg.widthMonth = 64
		}
	}
	if cfg.widthYear == 0 {
		cfg.widthYear = 52
	}
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.ColorBackground
	}
	d := &defaultDatePickerStyle
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
		cfg.Radius = Some(d.radiusBorder)
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = PaddingSmall
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = DefaultTextStyle
	}
	if cfg.SelectedDate.IsZero() {
		cfg.SelectedDate = time.Now()
	}
}
