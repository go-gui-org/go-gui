//go:build !darwin || ios

// Package nativemenu provides native macOS menubar and system tray.
// This file is a no-op stub for unsupported platforms.
package nativemenu

import "github.com/go-gui-org/go-gui/gui"

// SetMenubar is a no-op on non-macOS platforms.
func SetMenubar(_ gui.NativeMenubarCfg, _ func(string)) {}

// ClearMenubar is a no-op on non-macOS platforms.
func ClearMenubar() {}

// CreateSystemTray is a no-op on non-macOS platforms.
func CreateSystemTray(
	_ gui.SystemTrayCfg, _ func(string),
) (int, error) {
	return 0, nil
}

// UpdateSystemTray is a no-op on non-macOS platforms.
func UpdateSystemTray(_ int, _ gui.SystemTrayCfg) {}

// RemoveSystemTray is a no-op on non-macOS platforms.
func RemoveSystemTray(_ int) {}
