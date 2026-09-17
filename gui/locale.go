package gui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
)

// NumberFormat defines locale-specific number formatting.
// exportaudit:keep — documented public API (widget_locale.md).
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
// exportaudit:keep — documented public API (widget_locale.md).
type DateFormat struct {
	ShortDate string // "M/D/YYYY"
	LongDate  string // "MMMM D, YYYY"
	// MonthYear is the month-year pattern.
	// exportaudit:keep — documented public API (widget_locale.md).
	MonthYear string
	// FirstDayOfWeek is 0=Sunday, 1=Monday. Valid range is 0-6;
	// bundles outside it are rejected by LocaleParse.
	// exportaudit:keep — documented public API (widget_locale.md).
	FirstDayOfWeek uint8
	// exportaudit:keep — documented public API (widget_locale.md).
	Use24H bool
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
// exportaudit:keep — documented public API (widget_locale.md).
type CurrencyFormat struct {
	Symbol string // "$"
	Code   string // "USD"
	// Position is AffixPrefix or AffixSuffix.
	Position NumericAffixPosition
	Spacing  bool
	// Decimals is the number of fraction digits. Valid range is
	// 0-20; bundles outside it are rejected by LocaleParse.
	Decimals int
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

	// App-level translation keys, looked up with LocaleT.
	// exportaudit:keep — documented public API (widget_locale.md).
	Translations map[string]string

	// Month names (0=Jan..11=Dec).
	// exportaudit:keep — documented public API (widget_locale.md).
	MonthsShort, MonthsFull [12]string

	// Weekday names (0=Sun..6=Sat).
	// exportaudit:keep — documented public API (widget_locale.md).
	WeekdaysShort, WeekdaysMed, WeekdaysFull [7]string

	ID string // "en-US"

	// Dialog button labels. Read by the dialog views; apps read
	// them through CurrentLocale for custom dialogs.
	// exportaudit:keep — documented public API (widget_locale.md).
	StrOK, StrYes, StrNo, StrCancel string

	// CRUD / common actions.
	// exportaudit:keep — documented public API (widget_locale.md).
	StrSave, StrDelete, StrAdd, StrClear, StrSearch string
	// exportaudit:keep — documented public API (widget_locale.md).
	StrFilter, StrJump, StrReset, StrSubmit string

	// Status strings.
	// exportaudit:keep — documented public API (widget_locale.md).
	StrLoading, StrLoadingDiagram, StrSaving, StrSaveFailed string
	// exportaudit:keep — documented public API (widget_locale.md).
	StrSourceChanged, StrLoadError, StrError, StrClean string

	// Link context menu strings.
	// exportaudit:keep — documented public API (widget_locale.md).
	StrOpenLink, StrGoToTarget, StrCopyLink, StrCopied string

	// Scrollbar accessibility labels.
	// exportaudit:keep — documented public API (widget_locale.md).
	StrHorizontalScrollbar, StrVerticalScrollbar string

	// Color picker channel labels. StrLightness is HSL's L,
	// distinct from StrValue (HSV's V): different quantities,
	// not translations of one another.
	// exportaudit:keep — documented public API (widget_locale.md).
	StrRed, StrGreen, StrBlue, StrAlpha string
	// exportaudit:keep — documented public API (widget_locale.md).
	StrHue, StrSat, StrValue, StrLightness string

	// Data grid strings.
	// exportaudit:keep — documented public API (widget_locale.md).
	StrColumns, StrSelected, StrDraft, StrDirty string
	// exportaudit:keep — documented public API (widget_locale.md).
	StrMatches, StrPage, StrRows string

	// exportaudit:keep — documented public API (widget_locale.md).
	Date DateFormat
	// exportaudit:keep — documented public API (widget_locale.md).
	Currency CurrencyFormat

	// exportaudit:keep — documented public API (widget_locale.md).
	Number NumberFormat
	// TextDir is TextDirLTR or TextDirRTL. Bundles without
	// text_dir load as LTR; Auto never escapes a bundle.
	TextDir TextDirection
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
		StrCopied:     "Copied ✓",

		StrHorizontalScrollbar: "Horizontal scrollbar",
		StrVerticalScrollbar:   "Vertical scrollbar",

		StrRed:   "Red",
		StrGreen: "Green",
		StrBlue:  "Blue",
		StrAlpha: "Alpha",
		StrHue:   "Hue",
		StrSat:   "Sat",
		StrValue: "Value",

		StrLightness: "Lightness",

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
// NumericLocaleCfg for numeric input formatting. The GroupSizes
// slice is cloned, so the caller cannot mutate the locale.
func (l Locale) toNumericLocale() NumericLocaleCfg {
	return NumericLocaleCfg{
		DecimalSep: l.Number.DecimalSep,
		GroupSep:   l.Number.GroupSep,
		GroupSizes: slices.Clone(l.Number.GroupSizes),
		MinusSign:  l.Number.MinusSign,
		PlusSign:   l.Number.PlusSign,
	}
}

// clone returns a deep copy: the Translations map and the
// GroupSizes slice are duplicated so a copy retrieved from the
// registry or the active locale cannot mutate the stored one.
func (l Locale) clone() Locale {
	out := l
	out.Translations = maps.Clone(l.Translations)
	out.Number.GroupSizes = slices.Clone(l.Number.GroupSizes)
	return out
}

// activeLocaleMu guards ActiveLocale. In-repo code reads through
// CurrentLocale or activeTextDir and writes through SetLocale.
var activeLocaleMu sync.RWMutex

// ActiveLocale is the global locale setting. Prefer
// CurrentLocale and SetLocale, which lock; direct access skips
// the mutex.
// exportaudit:keep — compat: direct global read; new code uses CurrentLocale.
var ActiveLocale = localeDefaults()

// effectiveTextDir resolves the text direction for a shape,
// falling back to the global locale when set to Auto.
func effectiveTextDir(shape *Shape) TextDirection {
	if shape.TextDir != textDirAuto {
		return shape.TextDir
	}
	return activeTextDir()
}

// activeTextDir returns the global locale's text direction under
// a read lock. A global TextDir of Auto (only possible from a
// hand-built Locale, never from a bundle) resolves to LTR here,
// so no caller handles a third case.
func activeTextDir() TextDirection {
	activeLocaleMu.RLock()
	dir := ActiveLocale.TextDir
	activeLocaleMu.RUnlock()
	if dir != TextDirRTL {
		return TextDirLTR
	}
	return dir
}

// SetLocale sets the active global locale. Safe to call from any
// goroutine; windows pick it up on their next frame.
func SetLocale(l Locale) {
	activeLocaleMu.Lock()
	ActiveLocale = l.clone()
	activeLocaleMu.Unlock()
}

// activeLocaleShared returns the active locale under a read lock
// without the deep copy CurrentLocale makes. The Translations
// map and GroupSizes slice are shared with the stored locale, so
// callers must only read them. SetLocale always stores a fresh
// clone and never mutates the old one, so a shared read stays
// valid after a later SetLocale. Views call this per frame (and
// per row or cell), where a deep copy would allocate each time.
func activeLocaleShared() Locale {
	activeLocaleMu.RLock()
	out := ActiveLocale
	activeLocaleMu.RUnlock()
	return out
}

// CurrentLocale returns a copy of the active global locale.
// Mutating the result does not affect the stored locale.
func CurrentLocale() Locale {
	activeLocaleMu.RLock()
	out := ActiveLocale.clone()
	activeLocaleMu.RUnlock()
	return out
}

// SetLocale sets the global locale and refreshes the window.
func (w *Window) SetLocale(l Locale) {
	SetLocale(l)
	w.InvalidateLayout()
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
// exportaudit:keep — documented public API (showcase docs)
func LocaleAutoDetect() {
	id := localeDetect()
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
	v = strings.TrimSpace(v)
	// Strip variant suffix ("de_DE@euro" → "de_DE").
	if i := strings.IndexByte(v, '@'); i >= 0 {
		v = v[:i]
	}
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
