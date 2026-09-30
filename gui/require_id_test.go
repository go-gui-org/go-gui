package gui

import "testing"

// RequireID stays exported for sibling widgets that key state on an ID
// and cannot take a generated leaf. The gui widgets themselves no
// longer call it for their own Cfg.ID (#881); an ID-less call takes an
// auto leaf instead, which TestAutoIDEveryWidgetTwiceHasNoDuplicates
// covers.
func TestRequireIDPanics(t *testing.T) {
	defer func() {
		want := "gui: Widget requires a non-empty Cfg.ID"
		if r := recover(); r != want {
			t.Fatalf("panic = %v, want %q", r, want)
		}
	}()
	RequireID("Widget", "")
}
