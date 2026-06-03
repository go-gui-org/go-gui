//go:build !darwin && (!linux || android)

// Package spellcheck provides native spell checking.
// This file is a no-op stub for unsupported platforms.
package spellcheck

import "github.com/go-gui-org/go-gui/gui"

// Check returns nil on unsupported platforms.
func Check(_ string) []gui.SpellRange { return nil }

// Suggest returns nil on unsupported platforms.
func Suggest(_ string, _, _ int) []string { return nil }

// Learn is a no-op on unsupported platforms.
func Learn(_ string) {}
