package gui

import (
	"fmt"
	"strings"
)

// NumberFormat defines locale-specific number formatting.
type numberFormat struct {
	// exportaudit:keep — json-tagged or same-named member
	GroupSizes []int // default [3]
	// exportaudit:keep — shares a name with a json-tagged bundle field
	DecimalSep rune // default '.'
	GroupSep   rune // default ','
	// exportaudit:keep — shares a name with a json-tagged bundle field
	MinusSign rune // default '-'
	// exportaudit:keep — shares a name with a json-tagged bundle field
	PlusSign rune // default '+'
	// exportaudit:keep — shares a name with a json-tagged bundle field
}

// numberFormatDefaults returns en-US number format defaults.
func numberFormatDefaults() numberFormat {
	return numberFormat{
		DecimalSep: '.',
		GroupSep:   ',',
		GroupSizes: []int{3},
		MinusSign:  '-',
		PlusSign:   '+',
	}
}

// DateFormat defines locale-specific date formatting.
type dateFormat struct {
	ShortDate string // "M/D/YYYY"
	LongDate  string // "MMMM D, YYYY"
	MonthYear string // "MMMM YYYY"
	// exportaudit:keep — shares a name with a json-tagged bundle field
	FirstDayOfWeek uint8 // 0=Sunday, 1=Monday
	// exportaudit:keep — shares a name with a json-tagged bundle field
	Use24H bool
}

// dateFormatDefaults returns en-US date format defaults.
func dateFormatDefaults() dateFormat {
	return dateFormat{
		ShortDate: "M/D/YYYY",
		LongDate:  "MMMM D, YYYY",
		MonthYear: "MMMM YYYY",
	}
}

// CurrencyFormat defines locale-specific currency formatting.
type currencyFormat struct {
	Symbol string // "$"
	// exportaudit:keep — shares a name with a json-tagged bundle field
	Code string // "USD"
	// exportaudit:keep — shares a name with a json-tagged bundle field
	Position numericAffixPosition // AffixPrefix
	Spacing  bool
	Decimals int // 2
}

// currencyFormatDefaults returns en-US currency defaults.
func currencyFormatDefaults() currencyFormat {
	return currencyFormat{
		Symbol:   "$",
		Code:     "USD",
		Position: affixPrefix,
		Decimals: 2,
	}
}

// Locale holds locale-specific settings for formatting,
// UI strings, and translations.
type Locale struct {

	// App-level translation keys
	// exportaudit:keep — shares a name with a json-tagged bundle field
	Translations map[string]string

	// Month names (0=Jan..11=Dec)
	// exportaudit:keep — shares a name with a json-tagged bundle field
	MonthsShort [12]string
	// exportaudit:keep — shares a name with a json-tagged bundle field
	MonthsFull [12]string

	// Weekday names (0=Sun..6=Sat)
	// exportaudit:keep — shares a name with a json-tagged bundle field
	WeekdaysShort [7]string
	// exportaudit:keep — shares a name with a json-tagged bundle field
	WeekdaysMed [7]string
	// exportaudit:keep — shares a name with a json-tagged bundle field
	WeekdaysFull [7]string

	ID string // "en-US"

	// Dialog
	strOK     string
	strYes    string
	strNo     string
	StrCancel string

	// CRUD / common actions
	StrSave   string
	StrDelete string
	StrAdd    string
	StrClear  string
	strSearch string
	StrFilter string
	StrJump   string
	StrReset  string
	StrSubmit string

	// Status
	StrLoading        string
	strLoadingDiagram string
	StrSaving         string
	StrSaveFailed     string
	StrSourceChanged  string
	StrLoadError      string
	StrError          string
	StrClean          string

	// Link context menu
	strOpenLink   string
	strGoToTarget string
	strCopyLink   string
	strCopied     string

	// Scrollbar
	strHorizontalScrollbar string
	strVerticalScrollbar   string

	// Color picker
	strRed   string
	strGreen string
	strBlue  string
	strAlpha string
	strHue   string
	strSat   string
	strValue string
	// strLightness is HSL's L. Distinct from strValue, which is
	// HSV's V — the two are different quantities, not translations
	// of one another.
	strLightness string

	// Data grid
	StrColumns  string
	StrSelected string
	StrDraft    string
	StrDirty    string
	StrMatches  string
	strPage     string
	StrRows     string

	Date dateFormat
	// // exportaudit:keep — shares a name with a json-tagged bundle field
	Currency currencyFormat

	// exportaudit:keep — json-tagged or same-named member
	Number  numberFormat
	TextDir textDirection // TextDirLTR

}

// localeDefaults returns the en-US locale with all defaults.
func localeDefaults() Locale {
	return Locale{
		ID:      "en-US",
		TextDir: TextDirLTR,

		Number:   numberFormatDefaults(),
		Date:     dateFormatDefaults(),
		Currency: currencyFormatDefaults(),

		strOK:     "OK",
		strYes:    "Yes",
		strNo:     "No",
		StrCancel: "Cancel",

		StrSave:   "Save",
		StrDelete: "Delete",
		StrAdd:    "Add",
		StrClear:  "Clear",
		strSearch: "Search",
		StrFilter: "Filter",
		StrJump:   "Jump",
		StrReset:  "Reset",
		StrSubmit: "Submit",

		StrLoading:        "Loading...",
		strLoadingDiagram: "Loading diagram...",
		StrSaving:         "Saving...",
		StrSaveFailed:     "Save failed",
		StrSourceChanged:  "Source changed",
		StrLoadError:      "Load error",
		StrError:          "Error",
		StrClean:          "Clean",

		strOpenLink:   "Open Link",
		strGoToTarget: "Go to Target",
		strCopyLink:   "Copy Link",
		strCopied:     "Copied \u2713",

		strHorizontalScrollbar: "Horizontal scrollbar",
		strVerticalScrollbar:   "Vertical scrollbar",

		strRed:   "Red",
		strGreen: "Green",
		strBlue:  "Blue",
		strAlpha: "Alpha",
		strHue:   "Hue",
		strSat:   "Sat",
		strValue: "Value",

		strLightness: "Lightness",

		StrColumns:  "Columns",
		StrSelected: "Selected",
		StrDraft:    "Draft",
		StrDirty:    "Dirty",
		StrMatches:  "Matches",
		strPage:     "Page",
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
func (l Locale) toNumericLocale() NumericLocaleCfg {
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
func effectiveTextDir(shape *Shape) textDirection {
	if shape.TextDir != textDirAuto {
		return shape.TextDir
	}
	return ActiveLocale.TextDir
}

// SetLocale sets the active global locale.
func setLocale(l Locale) {
	ActiveLocale = l
}

// CurrentLocale returns the active global locale.
func CurrentLocale() Locale {
	return ActiveLocale
}

// SetLocale sets the global locale and refreshes the window.
func (w *Window) SetLocale(l Locale) {
	setLocale(l)
	w.UpdateWindow()
}

// SetLocaleID sets the global locale by registry ID and
// refreshes the window.
// exportaudit:keep — public seam for locale switching
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
func localeAutoDetect() {
	id := localeDetect()
	if l, ok := LocaleGet(id); ok {
		setLocale(l)
		return
	}
	// Try language-only prefix: "de-AT" → match "de-DE".
	if i := strings.IndexByte(id, '-'); i > 0 {
		prefix := id[:i]
		for _, name := range localeRegisteredNames() {
			if strings.HasPrefix(name, prefix+"-") {
				if l, ok := LocaleGet(name); ok {
					setLocale(l)
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
