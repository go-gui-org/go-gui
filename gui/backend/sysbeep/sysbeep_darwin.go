//go:build darwin && !ios

package sysbeep

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#include "sysbeep_darwin.h"
*/
import "C"

// Play plays the macOS system alert sound via NSBeep.
func Play() { C.sysbeepPlay() }

// Available reports that this platform has a system alert sound.
func Available() bool { return true }
