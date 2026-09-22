//go:build !linux && !js

package gl

import "syscall"

// monitorSysProcAttr returns nil: only Linux runs the gsettings
// monitor, and Pdeathsig is a Linux-only field.
func monitorSysProcAttr() *syscall.SysProcAttr {
	return nil
}
