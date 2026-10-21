//go:build android

package android

/*
#include <stdlib.h>

// libcTMPDIR is TMPDIR as libc sees it, or "" when unset. From Android
// 13 (API 33) ActivityThread sets it there, to the app's cache
// directory, but only after the Go runtime has copied its environment.
static const char* libcTMPDIR(void) {
	const char* v = getenv("TMPDIR");
	return v ? v : "";
}
*/
import "C"

import (
	"log"

	"github.com/go-gui-org/go-gui/gui/backend/internal/tempdir"
)

// init hands Go the TMPDIR Android set through libc. Without it
// os.TempDir falls back to /data/local/tmp, which an app cannot write:
// the icon font and every other temp file failed to load.
func init() {
	tempdir.Adopt(C.GoString(C.libcTMPDIR()))
}

// setTempDir gives Go a writable TMPDIR once the host has passed its
// files directory: the one libc has by now, or else the app's cache
// directory beside filesDir, the same directory, below API 33, where
// Android sets no TMPDIR at all.
func setTempDir(filesDir string) {
	tempdir.Adopt(C.GoString(C.libcTMPDIR()))
	if err := tempdir.FromFilesDir(filesDir); err != nil {
		log.Printf("android: temp directory beside %q: %v", filesDir, err)
	}
}
