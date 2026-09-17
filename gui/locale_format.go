package gui

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// LocaleFormatDate formats a date using locale-aware month
// substitution. MMMM -> full month, MMM -> short month.
// Other tokens use a simple V-style token replacement:
//
//	YYYY->year, M->month, D->day, HH->hour, mm->minute,
//	ss->second.
func LocaleFormatDate(t time.Time, format string) string {
	return localeDateReplace(t, format, activeLocaleShared())
}

// localeDateReplace performs single-pass V-style date token
// substitution. Tokens: YYYY, MMMM (full month), MMM (short
// month), MM (zero-padded month), M (month), DD (zero-padded
// day), D (day), HH, mm, ss. A single left-to-right scan with
// longest-match takes MMMM before MMM before MM before M, so
// month names never see their letters re-scanned.
func localeDateReplace(t time.Time, format string, loc Locale) string {
	monthIdx := int(t.Month()) - 1
	monthValid := monthIdx >= 0 && monthIdx < 12
	var sb strings.Builder
	sb.Grow(len(format) + 8)
	for i := 0; i < len(format); {
		rest := format[i:]
		switch {
		case strings.HasPrefix(rest, "YYYY"):
			appendPadN(&sb, t.Year(), 4)
			i += 4
		case strings.HasPrefix(rest, "MMMM"):
			if monthValid {
				sb.WriteString(loc.MonthsFull[monthIdx])
			} else {
				sb.WriteString("MMMM")
			}
			i += 4
		case strings.HasPrefix(rest, "MMM"):
			if monthValid {
				sb.WriteString(loc.MonthsShort[monthIdx])
			} else {
				sb.WriteString("MMM")
			}
			i += 3
		case strings.HasPrefix(rest, "MM"):
			appendPad2(&sb, int(t.Month()))
			i += 2
		case rest[0] == 'M':
			sb.WriteString(strconv.Itoa(int(t.Month())))
			i++
		case strings.HasPrefix(rest, "DD"):
			appendPad2(&sb, t.Day())
			i += 2
		case rest[0] == 'D':
			sb.WriteString(strconv.Itoa(t.Day()))
			i++
		case strings.HasPrefix(rest, "HH"):
			appendPad2(&sb, t.Hour())
			i += 2
		case strings.HasPrefix(rest, "mm"):
			appendPad2(&sb, t.Minute())
			i += 2
		case strings.HasPrefix(rest, "ss"):
			appendPad2(&sb, t.Second())
			i += 2
		default:
			sb.WriteByte(rest[0])
			i++
		}
	}
	return sb.String()
}

// appendPad2 writes v zero-padded to two digits.
func appendPad2(sb *strings.Builder, v int) {
	appendPadN(sb, v, 2)
}

// appendPadN writes v zero-padded to width digits.
func appendPadN(sb *strings.Builder, v, width int) {
	s := strconv.Itoa(v)
	for i := len(s); i < width; i++ {
		sb.WriteByte('0')
	}
	sb.WriteString(s)
}

// localeDatePadFormat promotes M→MM, D→DD in a date format.
// Month-name tokens are sheltered first so MMMM never gains
// digits.
func localeDatePadFormat(format string) string {
	r := strings.ReplaceAll(format, "MMMM", "\x01")
	r = strings.ReplaceAll(r, "MMM", "\x02")
	if !strings.Contains(r, "MM") {
		r = strings.ReplaceAll(r, "M", "MM")
	}
	if !strings.Contains(r, "DD") {
		r = strings.ReplaceAll(r, "D", "DD")
	}
	r = strings.ReplaceAll(r, "\x01", "MMMM")
	r = strings.ReplaceAll(r, "\x02", "MMM")
	return r
}

// localeDateMaskPattern converts a numeric date format to a mask
// pattern: YYYY→9999, MM→99, DD→99, separators pass through.
// Month-name formats have no fixed-width digit mask; the names
// pass through as literals. Masked fields take numeric formats
// only (requireDateFormat enforces that); this stays total so
// previews and tests never corrupt the names.
func localeDateMaskPattern(format string) string {
	r := localeDatePadFormat(format)
	r = strings.ReplaceAll(r, "MMMM", "\x01")
	r = strings.ReplaceAll(r, "MMM", "\x02")
	r = strings.ReplaceAll(r, "YYYY", "9999")
	r = strings.ReplaceAll(r, "MM", "99")
	r = strings.ReplaceAll(r, "DD", "99")
	r = strings.ReplaceAll(r, "\x01", "MMMM")
	r = strings.ReplaceAll(r, "\x02", "MMM")
	return r
}

// localeParseDate parses a date string using locale format tokens.
// Converts locale tokens (YYYY or YY, MM, M, DD, D, HH, mm, ss)
// to a Go reference-time layout and parses. Month names (MMM,
// MMMM) have no fixed parse: they return an explicit error
// instead of a wrong date.
func localeParseDate(text, format string) (time.Time, error) {
	if strings.Contains(format, "MMM") {
		return time.Time{}, errors.New(
			"localeParseDate: month-name formats do not parse")
	}
	layout := format
	layout = strings.ReplaceAll(layout, "YYYY", "2006")
	layout = strings.ReplaceAll(layout, "YY", "06")
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
	layout = strings.ReplaceAll(layout, "HH", "15")
	layout = strings.ReplaceAll(layout, "mm", "04")
	layout = strings.ReplaceAll(layout, "ss", "05")
	return time.Parse(layout, text)
}

// LocaleRowsFmt formats "Rows start-end/total" with locale digit
// grouping.
func LocaleRowsFmt(start, end, total int) string {
	loc := activeLocaleShared()
	return fmt.Sprintf("%s %s-%s/%s",
		loc.StrRows,
		formatGroupedInt(start, loc),
		formatGroupedInt(end, loc),
		formatGroupedInt(total, loc))
}

// LocalePageFmt formats "Page current/total" with locale digit
// grouping.
func LocalePageFmt(page, total int) string {
	loc := activeLocaleShared()
	return fmt.Sprintf("%s %s/%s",
		loc.StrPage,
		formatGroupedInt(page, loc),
		formatGroupedInt(total, loc))
}

// LocaleMatchesFmt formats "Matches count/total" with locale
// digit grouping on the count. Total stays a string so callers
// can pass "?" for an unknown total.
func LocaleMatchesFmt(count int, total string) string {
	loc := activeLocaleShared()
	return fmt.Sprintf("%s %s/%s",
		loc.StrMatches, formatGroupedInt(count, loc), total)
}

// formatGroupedInt renders v with the locale's group separator
// and sizes. Sizes apply right to left with the last repeating
// ([3,2] groups 12345678 as 1,23,45,678); an empty size list
// groups by 3. The sign uses the locale's minus sign.
func formatGroupedInt(v int, loc Locale) string {
	neg := v < 0
	digits := strconv.Itoa(v)
	if neg {
		digits = digits[1:]
	}
	sizes := loc.Number.GroupSizes
	// Walk right to left to find where each separator goes, then
	// write left to right. Writing forward keeps a multi-byte
	// separator's UTF-8 bytes in order; reversing a byte buffer
	// would scramble them. An int has at most 19 digits, so 20
	// break slots always suffice and stay on the stack.
	var breaksBuf [20]int
	breaks := breaksBuf[:0]
	pos := len(digits)
	sizeIdx := 0
	for {
		size := 3
		if sizeIdx < len(sizes) && sizes[sizeIdx] > 0 {
			size = sizes[sizeIdx]
		}
		pos -= size
		if pos <= 0 {
			break
		}
		breaks = append(breaks, pos)
		if sizeIdx < len(sizes)-1 {
			sizeIdx++
		}
	}
	var sb strings.Builder
	sb.Grow(len(digits) + (len(breaks)+1)*utf8.UTFMax)
	if neg {
		sb.WriteRune(loc.Number.MinusSign)
	}
	prev := 0
	for _, brk := range slices.Backward(breaks) {
		sb.WriteString(digits[prev:brk])
		sb.WriteRune(loc.Number.GroupSep)
		prev = brk
	}
	sb.WriteString(digits[prev:])
	return sb.String()
}

// LocaleT looks up a translation key in the current locale.
// Returns the key itself when not found.
// exportaudit:keep — documented public API (showcase docs)
func LocaleT(key string) string {
	if v, ok := activeLocaleShared().Translations[key]; ok {
		return v
	}
	return key
}
