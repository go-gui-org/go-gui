//go:build !linux

// Package atspi provides AT-SPI2 accessibility on Linux.
// This file is a no-op stub for non-Linux platforms.
package atspi

import "github.com/go-gui-org/go-gui/gui"

// Bridge is a no-op on non-Linux platforms.
type Bridge struct{}

// Init is a no-op on non-Linux platforms.
func (b *Bridge) Init(_ func(action, index int)) {}

// Sync is a no-op on non-Linux platforms.
func (b *Bridge) Sync(_ []gui.A11yNode, _, _ int) {}

// Destroy is a no-op on non-Linux platforms.
func (b *Bridge) Destroy() {}

// Announce is a no-op on non-Linux platforms.
func (b *Bridge) Announce(_ string) {}
