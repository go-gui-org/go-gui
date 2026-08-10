//go:build !darwin || !cgo

package gui

import "os"

// LocaleDetect returns the BCP 47 locale ID from environment
// variables.
func localeDetect() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return normalizeLocaleEnv(v)
		}
	}
	return "en-US"
}
