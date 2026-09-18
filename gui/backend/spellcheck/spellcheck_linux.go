//go:build linux && !android && hunspell

// Package spellcheck provides native spell checking via Hunspell on Linux.
// Requires libhunspell-dev at build time.
package spellcheck

/*
#cgo pkg-config: hunspell
#include <hunspell/hunspell.h>
#include <stdlib.h>
*/
import "C"

import (
	"io"
	"os"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

var handle *C.Hunhandle

// mu serializes all Hunspell handle use. libhunspell documents no
// thread-safety guarantee, and Check/Suggest/Learn can arrive from
// the frame thread and event callbacks concurrently.
var mu sync.Mutex

var ensureInit = sync.OnceFunc(func() {
	lang := detectLang()
	aff, dic, ok := findDict(lang)
	if !ok {
		// Try language prefix (e.g. "en" from "en_US").
		if i := strings.IndexAny(lang, "_-"); i > 0 {
			aff, dic, ok = findDict(lang[:i])
		}
	}
	if !ok {
		return
	}
	cAff := C.CString(aff)
	defer C.free(unsafe.Pointer(cAff))
	cDic := C.CString(dic)
	defer C.free(unsafe.Pointer(cDic))

	handle = C.Hunspell_create(cAff, cDic)
	if handle != nil {
		loadPersonalDict()
	}
})

// loadPersonalDict reads the personal dictionary and adds each
// word to the hunspell session. Runs inside ensureInit, before the
// handle is visible to other goroutines, so it needs no lock.
func loadPersonalDict() {
	f, err := os.Open(personalDicPath())
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	capped := io.LimitReader(f, maxPersonalWords*(maxPersonalLine+1))
	for _, word := range parsePersonalWords(readPersonalLines(capped)) {
		cWord := C.CString(word)
		C.Hunspell_add(handle, cWord)
		C.free(unsafe.Pointer(cWord))
	}
}

// Check returns byte ranges of misspelled words in text.
func Check(text string) []gui.SpellRange {
	ensureInit()
	if handle == nil || len(text) == 0 {
		return nil
	}
	mu.Lock()
	defer mu.Unlock()
	return checkWords(text)
}

// checkWords tokenizes text into words and checks each against
// hunspell, returning ranges for misspelled words.
func checkWords(text string) []gui.SpellRange {
	var ranges []gui.SpellRange
	i := 0
	for i < len(text) {
		// Skip non-word characters.
		r, size := utf8.DecodeRuneInString(text[i:])
		if !unicode.IsLetter(r) {
			i += size
			continue
		}
		// Scan a word: letters + apostrophe/right-single-quote
		// mid-word.
		start := i
		for i < len(text) {
			r, size = utf8.DecodeRuneInString(text[i:])
			if unicode.IsLetter(r) {
				i += size
				continue
			}
			// Allow apostrophe or right single quotation mark
			// mid-word (e.g. "don't").
			if (r == '\'' || r == '\u2019') && i+size < len(text) {
				next, _ := utf8.DecodeRuneInString(text[i+size:])
				if unicode.IsLetter(next) {
					i += size
					continue
				}
			}
			break
		}
		word := text[start:i]
		cWord := C.CString(word)
		ok := C.Hunspell_spell(handle, cWord)
		C.free(unsafe.Pointer(cWord))
		if ok == 0 {
			ranges = append(ranges, gui.SpellRange{
				StartByte: start,
				LenBytes:  i - start,
			})
		}
	}
	return ranges
}

// Suggest returns spelling suggestions for a misspelled range.
// Out-of-range spans clamp to the text remainder; see clampRange.
func Suggest(text string, startByte, lenBytes int) []string {
	ensureInit()
	if handle == nil {
		return nil
	}
	startByte, lenBytes, ok := clampRange(text, startByte, lenBytes)
	if !ok {
		return nil
	}
	word := text[startByte : startByte+lenBytes]
	cWord := C.CString(word)
	defer C.free(unsafe.Pointer(cWord))

	mu.Lock()
	defer mu.Unlock()
	var cList **C.char
	n := C.Hunspell_suggest(handle, &cList, cWord)
	if n == 0 {
		return nil
	}
	defer C.Hunspell_free_list(handle, &cList, n)

	suggestions := make([]string, int(n))
	slice := unsafe.Slice(cList, int(n))
	for i := range suggestions {
		suggestions[i] = C.GoString(slice[i])
	}
	return suggestions
}

// Learn adds a word to the hunspell session and persists it to
// the personal dictionary file. Words failing validLearnWord are
// rejected: one Learn stores exactly one dictionary line, and the
// loader drops longer lines, so accepting them would leave the
// session and the file disagreeing after a restart.
func Learn(word string) {
	ensureInit()
	if handle == nil || !validLearnWord(word) {
		return
	}
	if strings.IndexFunc(word, unicode.IsControl) >= 0 {
		return
	}
	cWord := C.CString(word)
	defer C.free(unsafe.Pointer(cWord))
	// Unlock before the file write below: the session add is the
	// only part that needs the handle lock.
	mu.Lock()
	C.Hunspell_add(handle, cWord)
	mu.Unlock()
	persistWord(word)
}
