//go:build !unix && !windows

package main

import "os"

// termSignal falls back to SIGINT where SIGTERM does not exist
// (js/wasm and other non-Unix targets). signal.Notify may never
// deliver it there; runparallel only ever runs on a developer host.
var termSignal = os.Interrupt
