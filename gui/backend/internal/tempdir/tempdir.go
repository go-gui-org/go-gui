// Package tempdir gives a backend a writable TMPDIR when the platform
// hands it over in a way Go's os.TempDir cannot see.
//
// Android is the case. A Go shared library reads its environment once,
// when it loads, so a TMPDIR the system sets later through libc is
// invisible to os.Getenv, and os.TempDir falls back to /data/local/tmp,
// which an app cannot write. ActivityThread sets TMPDIR to the app's
// cache directory from Android 13 (API 33). The AOSP change
// (d5ccb038f691, "Set $TMPDIR as well as java.io.tmpdir") dates from
// September 2021, but no Android 12 or 12L release carries it
// (android-12.0.0_r1 through android-12.1.0_r27); android-13.0.0_r1 is
// the first. Below API 33 nothing sets TMPDIR at all.
package tempdir

import "os"

// Adopt sets TMPDIR to v when Go sees none and v is not empty, and
// reports whether it did. A TMPDIR Go already sees is left alone.
func Adopt(v string) bool {
	if v == "" || os.Getenv("TMPDIR") != "" {
		return false
	}
	return os.Setenv("TMPDIR", v) == nil
}
