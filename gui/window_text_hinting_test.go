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

// Hinting is off by default (#1334): the go-glyph x-only stem snap makes
// straight and curved letters in one word look out of step (go-glyph#198).
func TestTextHintingDefaultIsOffAtInstall(t *testing.T) {
	w := NewWindow(WindowCfg{})
	m := &hintRecorder{}
	w.SetTextMeasurer(m)
	// The window states its setting on every install, so a text system
	// that had hinting turned on by an earlier window, or by the go-glyph
	// default, is reset too.
	if !slices.Equal(m.calls, []bool{false}) {
		t.Fatalf("SetHinting calls = %v, want [false]", m.calls)
	}
}

func TestTextHintingCfgEnableAppliesAtInstall(t *testing.T) {
	w := NewWindow(WindowCfg{EnableTextHinting: true})
	m := &hintRecorder{}
	w.SetTextMeasurer(m)
	if !slices.Equal(m.calls, []bool{true}) {
		t.Fatalf("SetHinting calls = %v, want [true]", m.calls)
	}
}

func TestSetTextHintingAppliesNowAndAfterSwap(t *testing.T) {
	w := NewWindow(WindowCfg{})
	first := &hintRecorder{}
	w.SetTextMeasurer(first)
	w.SetTextHinting(true)
	if !slices.Equal(first.calls, []bool{false, true}) {
		t.Fatalf("first SetHinting calls = %v, want [false true]", first.calls)
	}
	// A backend that replaces its measurer (the soft fallback does) must
	// get the setting the app chose, not the cfg value.
	second := &hintRecorder{}
	w.SetTextMeasurer(second)
	if !slices.Equal(second.calls, []bool{true}) {
		t.Fatalf("second SetHinting calls = %v, want [true]", second.calls)
	}
}

func TestSetTextHintingWithoutCapabilityIsSafe(t *testing.T) {
	w := NewWindow(WindowCfg{})
	// No measurer yet: the setting is kept for the install.
	w.SetTextHinting(true)
	m := &hintRecorder{}
	w.SetTextMeasurer(m)
	if !slices.Equal(m.calls, []bool{true}) {
		t.Fatalf("SetHinting calls = %v, want [true]", m.calls)
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
