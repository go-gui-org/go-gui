//go:build ios

package spellcheck

import "github.com/go-gui-org/go-gui/gui"

// Check returns nil on iOS.
func Check(_ string) []gui.SpellRange { return nil }

// Suggest returns nil on iOS.
func Suggest(_ string, _, _ int) []string { return nil }

// Learn is a no-op on iOS.
func Learn(_ string) {}
