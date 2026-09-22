package gui

import "testing"

// Fake OS appearance platform for issue #752: reports a scripted
// setting and records the watcher callback and titlebar pushes.
type fakeAppearancePlatform struct {
	noopNativePlatform
	appearance  Appearance
	hasSetting  bool
	cb          func(Appearance)
	titlebar    []bool
	callbacks   int
	unregisters int
}

func (f *fakeAppearancePlatform) SystemAppearance() (Appearance, bool) {
	return f.appearance, f.hasSetting
}

func (f *fakeAppearancePlatform) SetSystemAppearanceCallback(cb func(Appearance)) {
	if cb == nil {
		f.unregisters++
	} else {
		f.callbacks++
	}
	f.cb = cb
}

func (f *fakeAppearancePlatform) TitlebarDark(dark bool) {
	f.titlebar = append(f.titlebar, dark)
}

func (f *fakeAppearancePlatform) fire(a Appearance) {
	if f.cb != nil {
		f.cb(a)
	}
}

func newAppearanceWindow(p *fakeAppearancePlatform) *Window {
	w := NewWindow(WindowCfg{State: new(int)})
	if p != nil {
		w.SetNativePlatform(p)
	}
	return w
}

func TestSystemAppearanceNilPlatform(t *testing.T) {
	w := newAppearanceWindow(nil)
	defer unregisterWindow(w)
	if _, ok := w.SystemAppearance(); ok {
		t.Error("nil platform: got ok=true, want false (keep app theme)")
	}
}

func TestFollowSystemAppliesCurrent(t *testing.T) {
	p := &fakeAppearancePlatform{appearance: AppearanceDark, hasSetting: true}
	w := newAppearanceWindow(p)
	defer unregisterWindow(w)

	w.FollowSystemAppearance(ThemeLight, ThemeDark)

	if got := w.Theme().id; got != ThemeDark.id {
		t.Errorf("theme id: got %d, want ThemeDark %d", got, ThemeDark.id)
	}
	if len(p.titlebar) != 1 || !p.titlebar[0] {
		t.Errorf("titlebar pushes: got %v, want [true]", p.titlebar)
	}
	if p.cb == nil {
		t.Error("watcher callback not registered")
	}
}

func TestFollowSystemChangeEvent(t *testing.T) {
	p := &fakeAppearancePlatform{appearance: AppearanceDark, hasSetting: true}
	w := newAppearanceWindow(p)
	defer unregisterWindow(w)
	w.FollowSystemAppearance(ThemeLight, ThemeDark)

	// OS event arrives on the watcher thread: only queues.
	p.fire(AppearanceLight)
	if got := w.Theme().id; got != ThemeDark.id {
		t.Fatalf("theme changed before flush: got %d", got)
	}
	w.flushCommands()
	if got := w.Theme().id; got != ThemeLight.id {
		t.Errorf("theme id after change: got %d, want ThemeLight %d", got, ThemeLight.id)
	}
	if len(p.titlebar) != 2 || p.titlebar[1] {
		t.Errorf("titlebar pushes: got %v, want [true false]", p.titlebar)
	}
}

func TestSetThemeUnfollows(t *testing.T) {
	p := &fakeAppearancePlatform{appearance: AppearanceDark, hasSetting: true}
	w := newAppearanceWindow(p)
	defer unregisterWindow(w)
	w.FollowSystemAppearance(ThemeLight, ThemeDark)

	custom := ThemeLight.WithBorders(false)
	w.SetTheme(custom)
	p.fire(AppearanceLight)
	w.flushCommands()

	if got := w.Theme().id; got != custom.id {
		t.Errorf("explicit SetTheme must end following: got %d, want %d", got, custom.id)
	}
	if p.cb != nil {
		t.Error("unfollowed window with no hook must release the watcher")
	}
}

func TestStopFollowSystemAppearance(t *testing.T) {
	p := &fakeAppearancePlatform{appearance: AppearanceDark, hasSetting: true}
	w := newAppearanceWindow(p)
	defer unregisterWindow(w)
	w.FollowSystemAppearance(ThemeLight, ThemeDark)

	w.StopFollowSystemAppearance()
	if p.cb != nil {
		t.Error("watcher callback still registered after stop")
	}
	p.fire(AppearanceLight)
	w.flushCommands()
	if got := w.Theme().id; got != ThemeDark.id {
		t.Errorf("theme changed after stop: got %d", got)
	}
}

func TestOnSystemAppearanceHook(t *testing.T) {
	p := &fakeAppearancePlatform{appearance: AppearanceLight, hasSetting: true}
	w := newAppearanceWindow(p)
	defer unregisterWindow(w)

	var got []Appearance
	w.OnSystemAppearance(func(a Appearance) { got = append(got, a) })
	// Hook alone (no follow) leaves the theme untouched.
	before := w.Theme().id
	p.fire(AppearanceDark)
	w.flushCommands()
	if w.Theme().id != before {
		t.Error("hook must not pin a theme on its own")
	}
	if len(got) != 1 || got[0] != AppearanceDark {
		t.Errorf("hook calls: got %v, want [dark]", got)
	}

	w.OnSystemAppearance(nil)
	if p.cb != nil {
		t.Error("nil hook must unregister the watcher")
	}
	if p.unregisters == 0 {
		t.Error("expected an unregister call")
	}
}

func TestFollowSystemNoSettingKeepsTheme(t *testing.T) {
	p := &fakeAppearancePlatform{hasSetting: false}
	w := newAppearanceWindow(p)
	defer unregisterWindow(w)
	before := w.Theme().id

	w.FollowSystemAppearance(ThemeLight, ThemeDark)
	if w.Theme().id != before {
		t.Error("no OS setting must keep the app theme")
	}
	// Staying following: attaching a platform with a setting applies it.
	q := &fakeAppearancePlatform{appearance: AppearanceDark, hasSetting: true}
	w.SetNativePlatform(q)
	if got := w.Theme().id; got != ThemeDark.id {
		t.Errorf("theme after attach: got %d, want ThemeDark %d", got, ThemeDark.id)
	}
}

func TestFollowSystemBeforeAttach(t *testing.T) {
	w := newAppearanceWindow(nil)
	defer unregisterWindow(w)

	// Follow reachable before a backend attaches: nothing to query,
	// nothing to subscribe, theme kept.
	w.FollowSystemAppearance(ThemeLight, ThemeDark)

	p := &fakeAppearancePlatform{appearance: AppearanceLight, hasSetting: true}
	w.SetNativePlatform(p)
	if got := w.Theme().id; got != ThemeLight.id {
		t.Errorf("theme after attach: got %d, want ThemeLight %d", got, ThemeLight.id)
	}
	if p.cb == nil {
		t.Error("watcher callback not registered on attach")
	}
}

func TestFollowSystemHookCombined(t *testing.T) {
	p := &fakeAppearancePlatform{appearance: AppearanceDark, hasSetting: true}
	w := newAppearanceWindow(p)
	defer unregisterWindow(w)

	var got []Appearance
	w.OnSystemAppearance(func(a Appearance) { got = append(got, a) })
	w.FollowSystemAppearance(ThemeLight, ThemeDark)
	// Initial apply runs the hook too, like a change event.
	if len(got) != 1 || got[0] != AppearanceDark {
		t.Fatalf("hook calls after follow: got %v, want [dark]", got)
	}
	p.fire(AppearanceLight)
	w.flushCommands()
	if w.Theme().id != ThemeLight.id {
		t.Error("follow did not re-pin on change")
	}
	if len(got) != 2 || got[1] != AppearanceLight {
		t.Errorf("hook calls after change: got %v, want [dark light]", got)
	}
}

func TestOnSystemAppearanceReplacesHook(t *testing.T) {
	p := &fakeAppearancePlatform{appearance: AppearanceLight, hasSetting: true}
	w := newAppearanceWindow(p)
	defer unregisterWindow(w)

	var first, second int
	w.OnSystemAppearance(func(Appearance) { first++ })
	w.OnSystemAppearance(func(Appearance) { second++ })
	p.fire(AppearanceDark)
	w.flushCommands()
	if first != 0 || second != 1 {
		t.Errorf("hook calls: first=%d second=%d, want 0 1", first, second)
	}
}

func TestStopFollowKeepsHookWatcher(t *testing.T) {
	p := &fakeAppearancePlatform{appearance: AppearanceDark, hasSetting: true}
	w := newAppearanceWindow(p)
	defer unregisterWindow(w)

	var got []Appearance
	w.OnSystemAppearance(func(a Appearance) { got = append(got, a) })
	w.FollowSystemAppearance(ThemeLight, ThemeDark)
	w.StopFollowSystemAppearance()
	// The hook still wants events: no unregister.
	if p.cb == nil {
		t.Fatal("hook watcher unregistered while hook active")
	}
	p.fire(AppearanceLight)
	w.flushCommands()
	if w.Theme().id != ThemeDark.id {
		t.Error("theme changed after stop")
	}
	if len(got) != 2 {
		t.Errorf("hook calls: got %v, want 2 (initial + change)", got)
	}
}

func TestWindowCleanupUnregistersWatcher(t *testing.T) {
	p := &fakeAppearancePlatform{appearance: AppearanceDark, hasSetting: true}
	w := newAppearanceWindow(p)
	w.FollowSystemAppearance(ThemeLight, ThemeDark)
	if p.cb == nil {
		t.Fatal("watcher callback not registered")
	}
	w.WindowCleanup()
	unregisterWindow(w)
	if p.cb != nil {
		t.Error("watcher callback still registered after cleanup")
	}
}
