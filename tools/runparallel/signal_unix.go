//go:build unix

package main

import "syscall"

// termSignal is SIGTERM on Unix. Windows has no SIGTERM; see
// signal_windows.go.
const termSignal = syscall.SIGTERM
