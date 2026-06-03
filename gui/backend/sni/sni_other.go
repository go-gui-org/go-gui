//go:build !linux

// Package sni provides StatusNotifierItem system tray on Linux.
// This file is a no-op stub for non-Linux platforms.
package sni

import "github.com/go-gui-org/go-gui/gui"

// Tray is a no-op on non-Linux platforms.
type Tray struct{}

// Create is a no-op on non-Linux platforms.
func (t *Tray) Create(_ gui.SystemTrayCfg, _ func(string)) (int, error) {
	return 0, nil
}

// Update is a no-op on non-Linux platforms.
func (t *Tray) Update(_ int, _ gui.SystemTrayCfg) {}

// Remove is a no-op on non-Linux platforms.
func (t *Tray) Remove(_ int) {}
