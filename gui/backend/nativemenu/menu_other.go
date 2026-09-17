//go:build !darwin || ios

// Package nativemenu provides native macOS menubar and system tray.
// This file is a no-op stub for unsupported platforms.
package nativemenu

import (
	"sync/atomic"

	"github.com/go-gui-org/go-gui/gui"
)

// stubTrayIDs hands out unique positive tray IDs. The stub reports
// success, so handles must stay distinct for App bookkeeping.
var stubTrayIDs atomic.Int64

// SetMenubar is a no-op on non-macOS platforms.
func SetMenubar(_ gui.NativeMenubarCfg, _ func(string)) {}

// ClearMenubar is a no-op on non-macOS platforms.
func ClearMenubar() {}

// CreateSystemTray is a no-op on non-macOS platforms. It reports
// success with a unique handle so App bookkeeping stays consistent.
func CreateSystemTray(
	_ gui.SystemTrayCfg, _ func(string),
) (int, error) {
	return int(stubTrayIDs.Add(1)), nil
}

// UpdateSystemTray is a no-op on non-macOS platforms.
func UpdateSystemTray(_ int, _ gui.SystemTrayCfg) {}

// RemoveSystemTray is a no-op on non-macOS platforms.
func RemoveSystemTray(_ int) {}
