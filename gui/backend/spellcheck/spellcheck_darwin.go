//go:build darwin && !ios

// Package spellcheck provides native spell checking via NSSpellChecker.
package spellcheck

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#include "spellcheck_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// Check returns byte ranges of misspelled words in text.
func Check(text string) []gui.SpellRange {
	if len(text) == 0 {
		return nil
	}
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	r := C.spellcheckCheck(cText, C.int(len(text)))
	defer C.spellcheckFreeResult(r)

	if r.count == 0 {
		return nil
	}
	cRanges := unsafe.Slice(r.ranges, int(r.count))
	ranges := make([]gui.SpellRange, int(r.count))
	for i := range ranges {
		ranges[i] = gui.SpellRange{
			StartByte: int(cRanges[i].startByte),
			LenBytes:  int(cRanges[i].lenBytes),
		}
	}
	return ranges
}

// Suggest returns spelling suggestions for a misspelled range.
func Suggest(text string, startByte, lenBytes int) []string {
	if len(text) == 0 {
		return nil
	}
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))

	r := C.spellcheckSuggest(cText, C.int(len(text)),
		C.int(startByte), C.int(lenBytes))
	defer C.spellcheckFreeSuggestResult(r)

	if r.count == 0 {
		return nil
	}
	cSuggestions := unsafe.Slice(r.suggestions, int(r.count))
	suggestions := make([]string, int(r.count))
	for i := range suggestions {
		suggestions[i] = C.GoString(cSuggestions[i])
	}
	return suggestions
}

// Learn adds a word to the user's dictionary.
func Learn(word string) {
	cWord := C.CString(word)
	defer C.free(unsafe.Pointer(cWord))
	C.spellcheckLearn(cWord)
}
