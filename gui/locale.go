package gui

import (
	"fmt"
	"strings"
)

// NumberFormat defines locale-specific number formatting.
type NumberFormat struct {
	GroupSizes []int // default [3]
	DecimalSep rune  // default '.'
	GroupSep   rune  // default ','
	MinusSign  rune  // default '-'
	PlusSign   rune  // default '+'
}

// numberFormatDefaults returns en-US number format defaults.
func numberFormatDefaults() NumberFormat {
	return NumberFormat{
		DecimalSep: '.',
		GroupSep:   ',',
		GroupSizes: []int{3},
		MinusSign:  '-',
		PlusSign:   '+',
	}
}

// DateFormat defines locale-specific date formatting.
type DateFormat struct {
	ShortDate      string // "M/D/YYYY"
	LongDate       string // "MMMM D, YYYY"
	MonthYear      string // "MMMM YYYY"
	FirstDayOfWeek uint8  // 0=Sunday, 1=Monday
	Use24H         bool
}

// dateFormatDefaults returns en-US date format defaults.
func dateFormatDefaults() DateFormat {
	return DateFormat{
		ShortDate: "M/D/YYYY",
		LongDate:  "MMMM D, YYYY",
		MonthYear: "MMMM YYYY",
	}
}

// CurrencyFormat defines locale-specific currency formatting.
type CurrencyFormat struct {
	Symbol   string               // "$"
	Code     string               // "USD"
	Position NumericAffixPosition // AffixPrefix
	Spacing  bool
	Decimals int // 2
}

// currencyFormatDefaults returns en-US currency defaults.
func currencyFormatDefaults() CurrencyFormat {
	return CurrencyFormat{
		Symbol:   "$",
		Code:     "USD",
		Position: AffixPrefix,
		Decimals: 2,
	}
}

// Locale holds locale-specific settings for formatting,
// UI strings, and translations.
type Locale struct {

	// App-level translation keys
	Translations map[string]string

	// Month names (0=Jan..11=Dec)
	MonthsShort [12]string
	MonthsFull  [12]string

	// Weekday names (0=Sun..6=Sat)
	WeekdaysShort [7]string
	WeekdaysMed   [7]string
	WeekdaysFull  [7]string

	ID string // "en-US"

	// Dialog
	StrOK     string
	StrYes    string
	StrNo     string
	StrCancel string

	// CRUD / common actions
	StrSave   string
	StrDelete string
	StrAdd    string
	StrClear  string
	StrSearch string
	StrFilter string
	StrJump   string
	StrReset  string
	StrSubmit string

	// Status
	StrLoading        string
	StrLoadingDiagram string
	StrSaving         string
	StrSaveFailed     string
	StrSourceChanged  string
	StrLoadError      string
	StrError          string
	StrClean          string

	// Link context menu
	StrOpenLink   string
	StrGoToTarget string
	StrCopyLink   string
	StrCopied     string

	// Scrollbar
	StrHorizontalScrollbar string
	StrVerticalScrollbar   string

	// Color picker
	StrRed   string
	StrGreen string
	StrBlue  string
	StrAlpha string
	StrHue   string
	StrSat   string
	StrValue string

	// Data grid
	StrColumns  string
	StrSelected string
	StrDraft    string
	StrDirty    string
	StrMatches  string
	StrPage     string
	StrRows     string

	Date     DateFormat
	Currency CurrencyFormat

	Number  NumberFormat
	TextDir TextDirection // TextDirLTR

}

// localeDefaults returns the en-US locale with all defaults.
func localeDefaults() Locale {
	return Locale{
		ID:      "en-US",
		TextDir: TextDirLTR,

		Number:   numberFormatDefaults(),
		Date:     dateFormatDefaults(),
		Currency: currencyFormatDefaults(),

		StrOK:     "OK",
		StrYes:    "Yes",
		StrNo:     "No",
		StrCancel: "Cancel",

		StrSave:   "Save",
		StrDelete: "Delete",
		StrAdd:    "Add",
		StrClear:  "Clear",
		StrSearch: "Search",
		StrFilter: "Filter",
		StrJump:   "Jump",
		StrReset:  "Reset",
		StrSubmit: "Submit",

		StrLoading:        "Loading...",
		StrLoadingDiagram: "Loading diagram...",
		StrSaving:         "Saving...",
		StrSaveFailed:     "Save failed",
		StrSourceChanged:  "Source changed",
		StrLoadError:      "Load error",
		StrError:          "Error",
		StrClean:          "Clean",

		StrOpenLink:   "Open Link",
		StrGoToTarget: "Go to Target",
		StrCopyLink:   "Copy Link",
		StrCopied:     "Copied \u2713",

		StrHorizontalScrollbar: "Horizontal scrollbar",
		StrVerticalScrollbar:   "Vertical scrollbar",

		StrRed:   "Red",
		StrGreen: "Green",
		StrBlue:  "Blue",
		StrAlpha: "Alpha",
		StrHue:   "Hue",
		StrSat:   "Sat",
		StrValue: "Value",

		StrColumns:  "Columns",
		StrSelected: "Selected",
		StrDraft:    "Draft",
		StrDirty:    "Dirty",
		StrMatches:  "Matches",
		StrPage:     "Page",
		StrRows:     "Rows",

		WeekdaysShort: [7]string{"S", "M", "T", "W", "T", "F", "S"},
		WeekdaysMed:   [7]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
		WeekdaysFull: [7]string{
			"Sunday", "Monday", "Tuesday", "Wednesday",
			"Thursday", "Friday", "Saturday",
		},
		MonthsShort: [12]string{
			"Jan", "Feb", "Mar", "Apr", "May", "Jun",
			"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
		},
		MonthsFull: [12]string{
			"January", "February", "March", "April",
			"May", "June", "July", "August",
			"September", "October", "November", "December",
		},
	}
}

// ToNumericLocale converts locale number settings to
// NumericLocaleCfg for numeric input formatting.
func (l Locale) ToNumericLocale() NumericLocaleCfg {
	return NumericLocaleCfg{
		DecimalSep: l.Number.DecimalSep,
		GroupSep:   l.Number.GroupSep,
		GroupSizes: l.Number.GroupSizes,
		MinusSign:  l.Number.MinusSign,
		PlusSign:   l.Number.PlusSign,
	}
}

// ActiveLocale is the global locale setting.
var ActiveLocale = localeDefaults()

// effectiveTextDir resolves the text direction for a shape,
// falling back to the global locale when set to Auto.
func effectiveTextDir(shape *Shape) TextDirection {
	if shape.TextDir != TextDirAuto {
		return shape.TextDir
	}
	return ActiveLocale.TextDir
}

// SetLocale sets the active global locale.
func SetLocale(l Locale) {
	ActiveLocale = l
}

// CurrentLocale returns the active global locale.
func CurrentLocale() Locale {
	return ActiveLocale
}

// SetLocale sets the global locale and refreshes the window.
func (w *Window) SetLocale(l Locale) {
	SetLocale(l)
	w.UpdateWindow()
}

// SetLocaleID sets the global locale by registry ID and
// refreshes the window.
func (w *Window) SetLocaleID(id string) error {
	l, ok := LocaleGet(id)
	if !ok {
		return fmt.Errorf("locale not found: %s", id)
	}
	w.SetLocale(l)
	return nil
}

// LocaleAutoDetect detects the OS locale and sets the global
// locale to the best matching registered locale. Call before
// NewWindow. Falls back to language-prefix match if exact ID
// is not registered.
func LocaleAutoDetect() {
	id := LocaleDetect()
	if l, ok := LocaleGet(id); ok {
		SetLocale(l)
		return
	}
	// Try language-only prefix: "de-AT" → match "de-DE".
	if i := strings.IndexByte(id, '-'); i > 0 {
		prefix := id[:i]
		for _, name := range LocaleRegisteredNames() {
			if strings.HasPrefix(name, prefix+"-") {
				if l, ok := LocaleGet(name); ok {
					SetLocale(l)
					return
				}
			}
		}
	}
}

// normalizeLocaleEnv normalizes a POSIX locale value like
// "en_US.UTF-8" to BCP 47 "en-US".
func normalizeLocaleEnv(v string) string {
	// Strip encoding suffix (.UTF-8, .utf8, etc.).
	if i := strings.IndexByte(v, '.'); i > 0 {
		v = v[:i]
	}
	v = strings.ReplaceAll(v, "_", "-")
	if v == "" || v == "C" || v == "POSIX" {
		return "en-US"
	}
	return v
}
