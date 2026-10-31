package gui

import (
	"slices"
	"testing"
)

func TestTitlebarDarkNilPlatform(_ *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	// Should not panic with nil nativePlatform.
	w.TitlebarDark(true)
	w.TitlebarDark(false)
}

// A TitlebarDark made before the backend attaches is applied on attach,
// once, with the last variant asked for. On Windows that is before the
// window shows, so a pinned dark app starts under a dark titlebar.
func TestTitlebarDarkBeforeAttachReplays(t *testing.T) {
	w := newAppearanceWindow(nil)
	defer unregisterWindow(w)
	w.TitlebarDark(false)
	w.TitlebarDark(true)

	p := &fakeAppearancePlatform{}
	w.SetNativePlatform(p)
	if want := []bool{true}; !slices.Equal(p.titlebar, want) {
		t.Errorf("titlebar pushes on attach: got %v, want %v", p.titlebar, want)
	}
}

// A window whose app never asked for a variant keeps the frame the
// system gives it: attaching pushes nothing.
func TestTitlebarDarkNotAskedNotReplayed(t *testing.T) {
	w := newAppearanceWindow(nil)
	defer unregisterWindow(w)

	p := &fakeAppearancePlatform{}
	w.SetNativePlatform(p)
	if len(p.titlebar) != 0 {
		t.Errorf("titlebar pushes on attach: got %v, want none", p.titlebar)
	}
}

// While following, the OS setting wins over a stored variant, in one
// push. With no OS setting to apply, the stored variant is replayed.
func TestTitlebarDarkReplayWhileFollowing(t *testing.T) {
	for _, c := range []struct {
		name       string
		hasSetting bool
		want       []bool
	}{
		{"setting known", true, []bool{false}},
		{"no setting", false, []bool{true}},
	} {
		w := newAppearanceWindow(nil)
		w.TitlebarDark(true)
		w.FollowSystemAppearance(ThemeLight, ThemeDark)

		p := &fakeAppearancePlatform{appearance: AppearanceLight, hasSetting: c.hasSetting}
		w.SetNativePlatform(p)
		if !slices.Equal(p.titlebar, c.want) {
			t.Errorf("%s: titlebar pushes on attach: got %v, want %v",
				c.name, p.titlebar, c.want)
		}
		unregisterWindow(w)
	}
}
