package gui

import (
	"slices"
	"testing"
)

// hintRecorder is a text measurer that also has the SetHinting capability.
// It records each call, so a test can see what the window asked for and when.
type hintRecorder struct {
	stubTextMeasurer
	calls []bool
}

func (m *hintRecorder) SetHinting(on bool) { m.calls = append(m.calls, on) }

func TestTextHintingDefaultIsOnAtInstall(t *testing.T) {
	w := NewWindow(WindowCfg{})
	m := &hintRecorder{}
	w.SetTextMeasurer(m)
	// The window states its setting on every install, so a text system
	// that had hinting turned off by an earlier window is reset too.
	if !slices.Equal(m.calls, []bool{true}) {
		t.Fatalf("SetHinting calls = %v, want [true]", m.calls)
	}
}

func TestTextHintingCfgDisableAppliesAtInstall(t *testing.T) {
	w := NewWindow(WindowCfg{DisableTextHinting: true})
	m := &hintRecorder{}
	w.SetTextMeasurer(m)
	if !slices.Equal(m.calls, []bool{false}) {
		t.Fatalf("SetHinting calls = %v, want [false]", m.calls)
	}
}

func TestSetTextHintingAppliesNowAndAfterSwap(t *testing.T) {
	w := NewWindow(WindowCfg{})
	first := &hintRecorder{}
	w.SetTextMeasurer(first)
	w.SetTextHinting(false)
	if !slices.Equal(first.calls, []bool{true, false}) {
		t.Fatalf("first SetHinting calls = %v, want [true false]", first.calls)
	}
	// A backend that replaces its measurer (the soft fallback does) must
	// get the setting the app chose, not the cfg value.
	second := &hintRecorder{}
	w.SetTextMeasurer(second)
	if !slices.Equal(second.calls, []bool{false}) {
		t.Fatalf("second SetHinting calls = %v, want [false]", second.calls)
	}
}

func TestSetTextHintingWithoutCapabilityIsSafe(t *testing.T) {
	w := NewWindow(WindowCfg{})
	// No measurer yet: the setting is kept for the install.
	w.SetTextHinting(false)
	m := &hintRecorder{}
	w.SetTextMeasurer(m)
	if !slices.Equal(m.calls, []bool{false}) {
		t.Fatalf("SetHinting calls = %v, want [false]", m.calls)
	}
	// A measurer without SetHinting is accepted and ignored.
	w.SetTextMeasurer(&stubTextMeasurer{})
	w.SetTextHinting(true)
	w.SetTextMeasurer(nil)
	w.SetTextHinting(false)
}

func TestSetTextHintingNilWindow(t *testing.T) {
	var w *Window
	w.SetTextHinting(false) // must not panic
}
