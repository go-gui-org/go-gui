package gui

import (
	"strings"
	"testing"
)

// AdjustFontSize's guard is minSize < 1, but its error said "> 0", so a
// caller passing 0.5 got a message denying the rule it had broken. And
// an inverted range was never checked: maxSize < minSize accepted
// nothing and blamed the font size for it.
func TestAdjustFontSizeRangeErrors(t *testing.T) {
	tests := []struct {
		name             string
		min, max         float32
		wantErrSubstring string
	}{
		{"min below one", 0.5, 100, "minSize must be >= 1"},
		{"min zero", 0, 100, "minSize must be >= 1"},
		{"inverted range", 20, 10, "maxSize must be >= minSize"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ThemeDark.AdjustFontSize(0, tt.min, tt.max)
			if err == nil {
				t.Fatalf("AdjustFontSize(0, %v, %v) = nil error",
					tt.min, tt.max)
			}
			if !strings.Contains(err.Error(), tt.wantErrSubstring) {
				t.Errorf("error = %q, want it to contain %q",
					err.Error(), tt.wantErrSubstring)
			}
		})
	}
}

// TestAdjustFontSizeEqualRange is the boundary the new guard must not
// take with it: min == max is a legal one-size range.
func TestAdjustFontSizeEqualRange(t *testing.T) {
	size := ThemeDark.Cfg.TextStyleDef.Size
	if _, err := ThemeDark.AdjustFontSize(0, size, size); err != nil {
		t.Errorf("AdjustFontSize(0, %v, %v) = %v, want nil",
			size, size, err)
	}
}

// The app-default theme used to be published as &ThemeDark — a pointer
// into an exported, mutable package var. currentDefaultThemeRef hands
// that pointer to callers and promises nothing writes through it, so an
// app assigning gui.ThemeDark = someTheme rewrote a theme other
// goroutines were mid-read on. It publishes a copy now.
func TestDefaultThemeDoesNotAliasThemeDark(t *testing.T) {
	defaultThemeMu.RLock()
	published := defaultTheme
	defaultThemeMu.RUnlock()
	if published == &ThemeDark {
		t.Error("defaultTheme points at the exported ThemeDark var; " +
			"assigning gui.ThemeDark would write through a pointer " +
			"readers hold")
	}
	if ref := currentDefaultThemeRef(); ref == &ThemeDark {
		t.Error("currentDefaultThemeRef returned &ThemeDark")
	}
}

// BenchmarkInstallThemeSteadyState measures the frame path's no-op:
// one window whose theme is already installed. It answered that
// question by copying the whole ~12 KB Theme out of the window first,
// at 705 ns/op; by reference it is the atomic load it should always
// have been.
func BenchmarkInstallThemeSteadyState(b *testing.B) {
	w := &Window{}
	w.installTheme()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		w.installTheme()
	}
}
