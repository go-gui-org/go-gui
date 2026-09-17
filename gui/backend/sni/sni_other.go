//go:build !linux && !windows

// Package sni provides system tray support.
// This file is a no-op stub for platforms without native tray support.
package sni

import (
	"sync/atomic"

	"github.com/go-gui-org/go-gui/gui"
)

// Tray is a no-op on non-Linux platforms.
type Tray struct{}

// stubTrayIDs hands out unique positive tray IDs. The stub reports
// success, so handles must stay distinct for App bookkeeping.
var stubTrayIDs atomic.Int64

// Create is a no-op on non-Linux platforms. It reports success with
// a unique handle so App bookkeeping stays consistent.
func (t *Tray) Create(_ gui.SystemTrayCfg, _ func(string)) (int, error) {
	return int(stubTrayIDs.Add(1)), nil
}

// Update is a no-op on non-Linux platforms.
func (t *Tray) Update(_ int, _ gui.SystemTrayCfg) {}

// Remove is a no-op on non-Linux platforms.
func (t *Tray) Remove(_ int) {}
