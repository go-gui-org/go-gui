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
		{"short", LocaleEnUS,
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			"M/D/YYYY", "3/15/2025"},
		{"long_month", LocaleEnUS,
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			"MMMM D, YYYY", "March 15, 2025"},
		{"short_month", LocaleEnUS,
			time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
			"MMM YYYY", "Dec 2025"},
		{"german", LocaleDeDE,
			time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC),
			"D. MMMM YYYY", "5. M\u00E4rz 2025"},
		{"datetime", LocaleEnUS,
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
	ActiveLocale = LocaleEnUS

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
	if got := LocaleT("greeting"); got != "hello" {
		t.Fatalf("got %q, want hello", got)
	}
	if got := LocaleT("missing"); got != "missing" {
		t.Fatalf("got %q, want missing", got)
	}
}
