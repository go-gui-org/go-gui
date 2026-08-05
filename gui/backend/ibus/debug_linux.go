//go:build linux

package ibus

import (
	"fmt"
	"os"
	"sync"
)

// Setting GOGUI_IBUS_DEBUG dumps every raw UpdatePreeditText body to
// stderr.
//
// Which IBusAttribute marks the focused clause is engine convention
// rather than specification, so the only way to know what an engine
// actually sends is to watch it compose. The attribute shapes in
// text_test.go were captured this way from ibus-mozc; the next engine
// that renders wrongly is diagnosed the same way, which is why this
// outlives the issue that prompted it.
var debugPreedit = sync.OnceValue(func() bool {
	return os.Getenv("GOGUI_IBUS_DEBUG") != ""
})

func debugDumpPreedit(body []any) {
	if !debugPreedit() {
		return
	}
	fmt.Fprintf(os.Stderr, "IBUS preedit body: %#v\n", body)
}
