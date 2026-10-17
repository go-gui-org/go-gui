package main

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// keepLocale puts the active locale back when the test ends. The
// locale is global, so a test that switches it would leak into the
// tests that run after it.
func keepLocale(t *testing.T) {
	t.Helper()
	saved := gui.CurrentLocale()
	t.Cleanup(func() { gui.SetLocale(saved) })
}

// localePageWindow renders the showcase at the Locale page, with the
// showcase locale files registered.
func localePageWindow(t *testing.T) *gui.Window {
	t.Helper()
	loadEmbeddedLocales()
	app := newShowcaseApp()
	app.SelectedGroup = groupApp
	app.SelectedComponent = "locale"
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 1400})
	w.TestRender(mainView)
	w.TestRender(nil)
	return w
}

// The page lays out clean in every locale the showcase cycles,
// right-to-left Arabic included.
func TestLocalePageNoFindings(t *testing.T) {
	keepLocale(t)
	for idx, id := range showcaseLocaleIDs {
		w := localePageWindow(t)
		setShowcaseLocale(w, idx)
		w.TestRender(mainView)
		w.TestRender(nil)
		if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
			t.Fatalf("%s findings:\n%v", id, found)
		}
	}
}

// A pick in the page's own switch sets the locale and moves the
// catalog button with it: both read LocaleIndex.
func TestLocalePageSwitchSetsLocale(t *testing.T) {
	keepLocale(t)
	w := localePageWindow(t)
	for idx, id := range showcaseLocaleIDs {
		opt := gui.ScopeIDN(gui.ScopeID("detail", localeSwitchID), "opt", idx)
		if err := w.TestClick(opt); err != nil {
			t.Fatalf("TestClick %s: %v", opt, err)
		}
		if got := appState(w).LocaleIndex; got != idx {
			t.Fatalf("LocaleIndex = %d after a click on %s, want %d", got, id, idx)
		}
		if got := gui.CurrentLocale().ID; got != id {
			t.Fatalf("locale = %q after a click on %s, want %q", got, id, id)
		}
		w.TestRender(mainView)
		w.TestRender(nil)
	}
}

// The catalog button steps to the next locale and wraps at the end.
func TestCatalogLocaleButtonCycles(t *testing.T) {
	keepLocale(t)
	w := localePageWindow(t)
	button := gui.ScopeID(catalogScopeID, localeButtonID)
	for step := 1; step <= len(showcaseLocaleIDs); step++ {
		if err := w.TestClick(button); err != nil {
			t.Fatalf("TestClick %s: %v", button, err)
		}
		want := step % len(showcaseLocaleIDs)
		if got := appState(w).LocaleIndex; got != want {
			t.Fatalf("LocaleIndex = %d after %d clicks, want %d", got, step, want)
		}
		if got, wantID := gui.CurrentLocale().ID, showcaseLocaleIDs[want]; got != wantID {
			t.Fatalf("locale = %q after %d clicks, want %q", got, step, wantID)
		}
		w.TestRender(mainView)
		w.TestRender(nil)
	}
}

// An index outside the list changes nothing.
func TestSetShowcaseLocaleOutOfRange(t *testing.T) {
	keepLocale(t)
	w := localePageWindow(t)
	before := gui.CurrentLocale().ID
	for _, idx := range []int{-1, len(showcaseLocaleIDs)} {
		setShowcaseLocale(w, idx)
		if got := appState(w).LocaleIndex; got != 0 {
			t.Fatalf("LocaleIndex = %d after index %d, want 0", got, idx)
		}
		if got := gui.CurrentLocale().ID; got != before {
			t.Fatalf("locale = %q after index %d, want %q", got, idx, before)
		}
	}
}

// Every translated locale has every key the page looks up. A key
// missing from a file shows as English, which looks like a bug in
// the page and not in the file.
func TestLocaleTranslationsComplete(t *testing.T) {
	loadEmbeddedLocales()
	for _, id := range showcaseLocaleIDs {
		if id == "en-US" {
			// The keys are the English text: en-US needs no block.
			continue
		}
		locale, ok := gui.LocaleGet(id)
		if !ok {
			t.Fatalf("locale %s is not registered", id)
		}
		for _, key := range localeTranslationKeys {
			if locale.Translations[key] == "" {
				t.Errorf("%s has no translation for %q", id, key)
			}
		}
	}
}

// The button's accessible label names the locale it is on now, and
// its tooltip points at the page that shows what it changes.
func TestLocaleButtonLabels(t *testing.T) {
	for idx, id := range showcaseLocaleIDs {
		if got := localeButtonA11YLabel(idx); !strings.Contains(got, id) {
			t.Errorf("A11Y label %q does not name %s", got, id)
		}
	}
	// An index outside the list falls back to en-US, the same as
	// localeLabel, and does not panic.
	for _, idx := range []int{-1, len(showcaseLocaleIDs)} {
		if got := localeButtonA11YLabel(idx); !strings.Contains(got, "en-US") {
			t.Errorf("A11Y label %q for index %d, want en-US", got, idx)
		}
	}
	if !strings.Contains(localeButtonTooltip, "App & Window") {
		t.Errorf("tooltip %q does not point at the Locale page", localeButtonTooltip)
	}
}

// A switch formats the page's number input in the new locale. Text
// left in the old format would parse wrong: "1,234,567.50" is not a
// de-DE number.
func TestSetShowcaseLocaleReformatsNumber(t *testing.T) {
	keepLocale(t)
	w := localePageWindow(t)
	setShowcaseLocale(w, 1) // de-DE
	if got, want := appState(w).LocaleNumText, "1.234.567,50"; got != want {
		t.Fatalf("LocaleNumText = %q in de-DE, want %q", got, want)
	}
	setShowcaseLocale(w, 0) // en-US
	if got, want := appState(w).LocaleNumText, "1,234,567.50"; got != want {
		t.Fatalf("LocaleNumText = %q in en-US, want %q", got, want)
	}
}

// The switch marks the locale the page shows, even when the locale
// was set some other way and LocaleIndex is stale.
func TestLocalePageSwitchMarksActiveLocale(t *testing.T) {
	keepLocale(t)
	w := localePageWindow(t)
	de, ok := gui.LocaleGet("de-DE")
	if !ok {
		t.Fatal("de-DE is not registered")
	}
	w.SetLocale(de) // LocaleIndex stays 0 (en-US)
	layout := w.TestRender(mainView)
	for idx, id := range showcaseLocaleIDs {
		opt := gui.ScopeIDN(gui.ScopeID("detail", localeSwitchID), "opt", idx)
		seg, found := layout.FindByID(opt)
		if !found {
			t.Fatalf("segment %s not found", opt)
		}
		selected := seg.Shape.A11YState == gui.AccessStateSelected
		if selected != (id == "de-DE") {
			t.Errorf("%s selected = %v, want %v", id, selected, id == "de-DE")
		}
	}
}

// With no committed value there is nothing to reformat: the text stays
// as the reader left it.
func TestSetShowcaseLocaleKeepsTextWithoutValue(t *testing.T) {
	keepLocale(t)
	w := localePageWindow(t)
	app := appState(w)
	app.LocaleNumValue = gui.Opt[float64]{}
	app.LocaleNumText = "12"
	setShowcaseLocale(w, 1)
	if app.LocaleNumText != "12" {
		t.Fatalf("LocaleNumText = %q, want %q", app.LocaleNumText, "12")
	}
}

// The page's number input reads the active locale's separators: in
// de-DE, "2.000,5" is two thousand and a half.
func TestLocalePageNumberParsesActiveLocale(t *testing.T) {
	keepLocale(t)
	w := localePageWindow(t)
	setShowcaseLocale(w, 1) // de-DE
	app := appState(w)
	app.LocaleNumValue = gui.Opt[float64]{}
	app.LocaleNumText = ""
	w.TestRender(mainView)
	w.TestRender(nil)

	ids := w.ResolveID("showcase-locale-number")
	if len(ids) == 0 {
		t.Fatal("number input not found")
	}
	if err := w.TestType(ids[0], "2.000,5"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if err := w.TestKey(ids[0], gui.KeyEnter, gui.ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	v, ok := app.LocaleNumValue.Value()
	if !ok || v != 2000.5 {
		t.Fatalf("LocaleNumValue = %v (set %v), want 2000.5", v, ok)
	}
}
