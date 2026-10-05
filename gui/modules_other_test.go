//go:build !windows

package gui

import "io"

// logLoadedModules is a no-op off Windows. The Windows version supports the
// #886 crash diagnosis; see modules_windows_test.go.
func logLoadedModules(io.Writer) {}
