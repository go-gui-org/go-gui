package gui

import (
	"strings"
	"testing"
)

// FrameBackground is what every backend clears with, so the Transparent
// flag has to reach it: an app that sets only Transparent must not get
// the opaque theme background.
func TestFrameBackground(t *testing.T) {
	tests := []struct {
		name        string
		bg          Color
		transparent bool
		want        Color
	}{
		{
			name: "unset background takes the theme",
			want: ThemeDark.ColorBackground,
		},
		{
			name:        "transparent with no background is fully clear",
			transparent: true,
			want:        ColorTransparent,
		},
		{
			name: "an explicit background wins over both",
			bg:   RGBA(10, 20, 30, 128), transparent: true,
			want: RGBA(10, 20, 30, 128),
		},
		{
			name: "an explicit opaque background survives Transparent",
			bg:   RGB(10, 20, 30), transparent: true,
			want: RGB(10, 20, 30),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWindow(WindowCfg{
				BgColor:     tc.bg,
				Transparent: tc.transparent,
			})
			defer w.WindowCleanup()
			w.SetTheme(ThemeDark)
			if got := w.FrameBackground(); got != tc.want {
				t.Errorf("FrameBackground() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// BgColor is read every frame, so an app may assign it at runtime;
// Transparent, by contrast, is fixed when the window is made.
func TestFrameBackgroundRuntimeReassignment(t *testing.T) {
	w := NewWindow(WindowCfg{Transparent: true})
	defer w.WindowCleanup()
	w.SetTheme(ThemeDark)
	if got := w.FrameBackground(); got != ColorTransparent {
		t.Fatalf("FrameBackground() = %+v, want fully transparent", got)
	}
	w.Config.BgColor = RGB(10, 20, 30)
	if got := w.FrameBackground(); got != RGB(10, 20, 30) {
		t.Fatalf("FrameBackground() = %+v, want the runtime color", got)
	}
}

// The flag is opt-in: an existing window's clear color must not move.
func TestTransparentDefaultsOff(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.WindowCleanup()
	if w.Config.Transparent {
		t.Error("WindowCfg.Transparent defaults to true")
	}
}

// A transparent window that degraded is reported once per cause and
// only while its category is on. Two causes can hold at once (no ARGB
// visual and no compositor), so they must not collapse into one.
func TestDebugWindowTransparencyWarnOnce(t *testing.T) {
	buf := captureDebugMask(t, DebugWindowDegraded)
	w := &Window{}

	w.DebugWindowTransparency("no depth-32 visual")
	w.DebugWindowTransparency("no depth-32 visual")
	got := buf.String()
	if !strings.Contains(got, "Transparent requested but no depth-32 visual") {
		t.Fatalf("want the degrade finding, got %q", got)
	}
	if n := strings.Count(got, "Transparent requested"); n != 1 {
		t.Fatalf("warn-once: want 1 finding, got %d", n)
	}

	w.DebugWindowTransparency("no compositing manager")
	if n := strings.Count(buf.String(), "Transparent requested"); n != 2 {
		t.Fatalf("a second cause must report separately, got %d", n)
	}
}

// The check runs from backend window creation, outside the per-frame
// audit, so it has to consult the mask itself.
func TestDebugWindowTransparencyCategoryGate(t *testing.T) {
	buf := captureDebug(t)
	DebugCategories(DebugMissingIDs)
	w := &Window{}

	w.DebugWindowTransparency("no compositing manager")
	if got := buf.String(); got != "" {
		t.Fatalf("category off must be silent, got %q", got)
	}
}
