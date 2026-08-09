//go:build darwin

package gui

/*
#cgo LDFLAGS: -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <stdlib.h>

static const char* detectLocale() {
	CFLocaleRef locale = CFLocaleCopyCurrent();
	if (!locale) return NULL;
	CFStringRef ident = CFLocaleGetIdentifier(locale);
	if (!ident) {
		CFRelease(locale);
		return NULL;
	}
	CFIndex len = CFStringGetMaximumSizeForEncoding(
		CFStringGetLength(ident), kCFStringEncodingUTF8) + 1;
	char *buf = (char *)malloc(len);
	if (!CFStringGetCString(ident, buf, len, kCFStringEncodingUTF8)) {
		free(buf);
		CFRelease(locale);
		return NULL;
	}
	CFRelease(locale);
	return buf;
}
*/
import "C"

import (
	"os"
	"strings"
	"unsafe"
)

// LocaleDetect returns the BCP 47 locale ID from the OS.
func localeDetect() string {
	cStr := C.detectLocale()
	if cStr == nil {
		return localeFromEnv()
	}
	defer C.free(unsafe.Pointer(cStr))
	id := C.GoString(cStr)
	// Normalize en_US → en-US
	id = strings.ReplaceAll(id, "_", "-")
	if id == "" {
		return localeFromEnv()
	}
	return id
}

func localeFromEnv() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return normalizeLocaleEnv(v)
		}
	}
	return "en-US"
}
