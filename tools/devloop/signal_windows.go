//go:build windows

package main

import "os"

// termSignal falls back to SIGINT on Windows, which has no SIGTERM.
// signal.Notify delivers os.Interrupt for Ctrl-C and Ctrl-Break.
var termSignal = os.Interrupt
