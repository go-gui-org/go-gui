package gui

import (
	"os"
	"strconv"
	"strings"
)

func init() {
	if mask := debugMaskFromEnv(); mask != 0 {
		debugMask.Store(uint32(mask))
		debugGen.Store(1)
	}
}

// debugMaskFromEnv returns the categories the environment turns on at
// startup. It is split from init so a test can drive it.
func debugMaskFromEnv() DebugCategory {
	var mask DebugCategory
	// GOGUI_DEBUG is the general gate. GOGUI_FOCUS_DEBUG is the
	// original focus-only spelling, still honoured so existing
	// workflows keep working. Either enables every category.
	if envTruthy("GOGUI_DEBUG") || envTruthy("GOGUI_FOCUS_DEBUG") {
		mask |= DebugAll
	}
	// DebugRebuilds is not in DebugAll because it logs normal
	// operation, so it has its own variable (#975). It combines with
	// GOGUI_DEBUG: both set turns on both.
	if envTruthy("GOGUI_DEBUG_REBUILDS") {
		mask |= DebugRebuilds
	}
	return mask
}

// envTruthy reports whether an environment variable is set to
// something a developer would read as "on".
func envTruthy(name string) bool {
	v, ok := os.LookupEnv(name)
	if !ok {
		return false
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	return err == nil && b
}
