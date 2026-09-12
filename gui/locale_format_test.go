package gui

import (
	"testing"
	"time"
)

func TestLocaleFormatDate(t *testing.T) {
	saved := ActiveLocale
	t.Cleanup(func() { ActiveLocale = saved })

	tests := []struct {
		name   string
		locale Locale
		date   time.Time
		format string
		want   string
	}{
		{"short", localeEnUS,
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			"M/D/YYYY", "3/15/2025"},
		{"long_month", localeEnUS,
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			"MMMM D, YYYY", "March 15, 2025"},
		{"short_month", localeEnUS,
			time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
			"MMM YYYY", "Dec 2025"},
		{"german", localeDeDE,
			time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC),
			"D. MMMM YYYY", "5. M\u00E4rz 2025"},
		{"datetime", localeEnUS,
			time.Date(2025, 1, 2, 14, 5, 9, 0, time.UTC),
			"YYYY-MM-DD HH:mm:ss", "2025-01-02 14:05:09"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ActiveLocale = tt.locale
			got := LocaleFormatDate(tt.date, tt.format)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocaleFmt(t *testing.T) {
	saved := ActiveLocale
	t.Cleanup(func() { ActiveLocale = saved })
	ActiveLocale = localeEnUS

	t.Run("rows", func(t *testing.T) {
		got := LocaleRowsFmt(1, 50, 200)
		if got != "Rows 1-50/200" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("page", func(t *testing.T) {
		got := LocalePageFmt(3, 10)
		if got != "Page 3/10" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("matches", func(t *testing.T) {
		got := LocaleMatchesFmt(5, "100")
		if got != "Matches 5/100" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestLocaleDatePadFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		in, want string
	}{
		{"M/D/YYYY", "M/D/YYYY", "MM/DD/YYYY"},
		{"D.M.YYYY", "D.M.YYYY", "DD.MM.YYYY"},
		{"DD/MM/YYYY", "DD/MM/YYYY", "DD/MM/YYYY"},
		{"YYYY/M/D", "YYYY/M/D", "YYYY/MM/DD"},
		{"YYYY-M-D", "YYYY-M-D", "YYYY-MM-DD"},
		{"YYYY.M.D", "YYYY.M.D", "YYYY.MM.DD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := localeDatePadFormat(tt.in)
			if got != tt.want {
				t.Errorf("localeDatePadFormat(%q) = %q, want %q",
					tt.in, got, tt.want)
			}
		})
	}
}

func TestLocaleDateMaskPattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		in, want string
	}{
		{"M/D/YYYY", "M/D/YYYY", "99/99/9999"},
		{"D.M.YYYY", "D.M.YYYY", "99.99.9999"},
		{"DD/MM/YYYY", "DD/MM/YYYY", "99/99/9999"},
		{"YYYY/M/D", "YYYY/M/D", "9999/99/99"},
		{"YYYY-M-D", "YYYY-M-D", "9999-99-99"},
		{"YYYY.M.D", "YYYY.M.D", "9999.99.99"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := localeDateMaskPattern(tt.in)
			if got != tt.want {
				t.Errorf("localeDateMaskPattern(%q) = %q, want %q",
					tt.in, got, tt.want)
			}
		})
	}
}

func TestLocaleT(t *testing.T) {
	saved := ActiveLocale
	t.Cleanup(func() { ActiveLocale = saved })

	ActiveLocale = Locale{
		Translations: map[string]string{
			"greeting": "hello",
		},
	}
	if got := localeT("greeting"); got != "hello" {
		t.Fatalf("got %q, want hello", got)
	}
	if got := localeT("missing"); got != "missing" {
		t.Fatalf("got %q, want missing", got)
	}
}

// localeParseDate is the only parse in the date path and had no test
// before issue #578 gave callers control of the format.
func TestLocaleParseDate(t *testing.T) {
	tests := []struct {
		format  string
		text    string
		wantErr bool
		year    int
		month   time.Month
		day     int
	}{
		{"MM/DD/YYYY", "12/24/2026", false, 2026, time.December, 24},
		{"DD.MM.YYYY", "24.12.2026", false, 2026, time.December, 24},
		{"DD/MM/YYYY", "24/12/2026", false, 2026, time.December, 24},
		{"YYYY-MM-DD", "2026-12-24", false, 2026, time.December, 24},
		{"M/D/YYYY", "1/5/2026", false, 2026, time.January, 5},
		// The separators must match the format.
		{"DD.MM.YYYY", "24/12/2026", true, 0, 0, 0},
		// A day the month does not have.
		{"DD.MM.YYYY", "31.02.2026", true, 0, 0, 0},
	}
	for _, tt := range tests {
		got, err := localeParseDate(tt.text, tt.format)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s / %s: want error, got %v",
					tt.format, tt.text, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s / %s: %v", tt.format, tt.text, err)
			continue
		}
		if got.Year() != tt.year || got.Month() != tt.month ||
			got.Day() != tt.day {
			t.Errorf("%s / %s = %v, want %d-%02d-%02d",
				tt.format, tt.text, got, tt.year, tt.month, tt.day)
		}
	}
}
