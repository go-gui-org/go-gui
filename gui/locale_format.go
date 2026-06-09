package gui

import (
	"fmt"
	"strings"
	"time"
)

// LocaleFormatDate formats a date using locale-aware month
// substitution. MMMM -> full month, MMM -> short month.
// Other tokens use a simple V-style token replacement:
//
//	YYYY->year, M->month, D->day, HH->hour, mm->minute,
//	ss->second.
func LocaleFormatDate(t time.Time, format string) string {
	monthIdx := int(t.Month()) - 1
	if monthIdx < 0 || monthIdx >= 12 {
		return localeDateReplace(t, format)
	}
	result := format
	hasFull := strings.Contains(result, "MMMM")
	hasShort := !hasFull && strings.Contains(result, "MMM")
	if hasFull {
		result = strings.Replace(result, "MMMM", "\x01\x01\x01\x01", 1)
	} else if hasShort {
		result = strings.Replace(result, "MMM", "\x01\x01\x01", 1)
	}
	result = localeDateReplace(t, result)
	if hasFull {
		result = strings.Replace(result, "\x01\x01\x01\x01",
			ActiveLocale.MonthsFull[monthIdx], 1)
	} else if hasShort {
		result = strings.Replace(result, "\x01\x01\x01",
			ActiveLocale.MonthsShort[monthIdx], 1)
	}
	return result
}

// localeDateReplace performs simple V-style date token
// substitution. Tokens: YYYY, MM (zero-padded month),
// M (month), DD (zero-padded day), D (day),
// HH, mm, ss.
func localeDateReplace(t time.Time, format string) string {
	r := format
	r = strings.ReplaceAll(r, "YYYY", fmt.Sprintf("%04d", t.Year()))
	// MM must be replaced before M to avoid double-replace.
	// Use sentinel to protect.
	if strings.Contains(r, "MM") {
		r = strings.ReplaceAll(r, "MM",
			fmt.Sprintf("%02d", int(t.Month())))
	} else {
		r = strings.ReplaceAll(r, "M",
			fmt.Sprintf("%d", int(t.Month())))
	}
	if strings.Contains(r, "DD") {
		r = strings.ReplaceAll(r, "DD",
			fmt.Sprintf("%02d", t.Day()))
	} else {
		r = strings.ReplaceAll(r, "D",
			fmt.Sprintf("%d", t.Day()))
	}
	r = strings.ReplaceAll(r, "HH",
		fmt.Sprintf("%02d", t.Hour()))
	r = strings.ReplaceAll(r, "mm",
		fmt.Sprintf("%02d", t.Minute()))
	r = strings.ReplaceAll(r, "ss",
		fmt.Sprintf("%02d", t.Second()))
	return r
}

// localeDatePadFormat promotes M→MM, D→DD in a date format.
func localeDatePadFormat(format string) string {
	r := format
	if !strings.Contains(r, "MM") {
		r = strings.ReplaceAll(r, "M", "MM")
	}
	if !strings.Contains(r, "DD") {
		r = strings.ReplaceAll(r, "D", "DD")
	}
	return r
}

// localeDateMaskPattern converts a date format to a mask pattern:
// YYYY→9999, MM→99, DD→99, separators pass through.
func localeDateMaskPattern(format string) string {
	r := localeDatePadFormat(format)
	r = strings.ReplaceAll(r, "YYYY", "9999")
	r = strings.ReplaceAll(r, "MM", "99")
	r = strings.ReplaceAll(r, "DD", "99")
	return r
}

// localeParseDate parses a date string using locale format tokens.
// Converts locale tokens (YYYY, MM, M, DD, D) to Go reference
// time layout and parses.
func localeParseDate(text, format string) (time.Time, error) {
	layout := format
	layout = strings.ReplaceAll(layout, "YYYY", "2006")
	if strings.Contains(layout, "MM") {
		layout = strings.ReplaceAll(layout, "MM", "01")
	} else {
		layout = strings.ReplaceAll(layout, "M", "1")
	}
	if strings.Contains(layout, "DD") {
		layout = strings.ReplaceAll(layout, "DD", "02")
	} else {
		layout = strings.ReplaceAll(layout, "D", "2")
	}
	return time.Parse(layout, text)
}

// LocaleRowsFmt formats "Rows start-end/total".
// LocaleRowsFmt formats "Rows start-end/total".
func LocaleRowsFmt(start, end, total int) string {
	return fmt.Sprintf("%s %d-%d/%d",
		ActiveLocale.StrRows, start, end, total)
}

// LocalePageFmt formats "Page current/total".
// LocalePageFmt formats "Page current/total".
func LocalePageFmt(page, total int) string {
	return fmt.Sprintf("%s %d/%d",
		ActiveLocale.StrPage, page, total)
}

// LocaleMatchesFmt formats "Matches count/total".
func LocaleMatchesFmt(count int, total string) string {
	return fmt.Sprintf("%s %d/%s",
		ActiveLocale.StrMatches, count, total)
}

// LocaleT looks up a translation key in the current locale.
// Returns the key itself when not found.
func LocaleT(key string) string {
	if v, ok := ActiveLocale.Translations[key]; ok {
		return v
	}
	return key
}
