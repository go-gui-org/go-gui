//go:build linux && !android

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
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

var handle *C.Hunhandle

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

// detectLang returns the locale string from environment variables,
// stripped of encoding and modifier suffixes.
func detectLang() string {
	for _, env := range []string{"LC_ALL", "LANG", "LANGUAGE"} {
		if v := os.Getenv(env); v != "" && v != "C" && v != "POSIX" {
			// Strip .UTF-8 or other encoding suffix.
			if i := strings.IndexByte(v, '.'); i > 0 {
				v = v[:i]
			}
			// Strip @modifier.
			if i := strings.IndexByte(v, '@'); i > 0 {
				v = v[:i]
			}
			return v
		}
	}
	return "en_US"
}

// findDict searches standard paths for hunspell dictionary files.
func findDict(lang string) (aff, dic string, ok bool) {
	var dirs []string
	if p := os.Getenv("DICPATH"); p != "" {
		dirs = append(dirs, strings.Split(p, ":")...)
	}
	dirs = append(dirs,
		"/usr/share/hunspell",
		"/usr/share/myspell/dicts",
	)
	for _, dir := range dirs {
		a := filepath.Join(dir, lang+".aff")
		d := filepath.Join(dir, lang+".dic")
		if fileExists(a) && fileExists(d) {
			return a, d, true
		}
	}
	return "", "", false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// personalDicPath returns the path to the personal dictionary file.
func personalDicPath() string {
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		home, _ := os.UserHomeDir()
		cfg = filepath.Join(home, ".config")
	}
	return filepath.Join(cfg, "go-gui", "personal.dic")
}

// loadPersonalDict reads the personal dictionary and adds each
// word to the hunspell session.
func loadPersonalDict() {
	f, err := os.Open(personalDicPath())
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	first := true
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word == "" {
			continue
		}
		// Skip first line if it's a count (hunspell format).
		if first {
			first = false
			if _, err := strconv.Atoi(word); err == nil {
				continue
			}
		}
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
func Suggest(text string, startByte, lenBytes int) []string {
	ensureInit()
	if handle == nil || len(text) == 0 {
		return nil
	}
	if startByte < 0 || startByte+lenBytes > len(text) {
		return nil
	}
	word := text[startByte : startByte+lenBytes]
	cWord := C.CString(word)
	defer C.free(unsafe.Pointer(cWord))

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
// the personal dictionary file.
func Learn(word string) {
	ensureInit()
	if handle == nil || word == "" {
		return
	}
	cWord := C.CString(word)
	defer C.free(unsafe.Pointer(cWord))
	C.Hunspell_add(handle, cWord)
	persistWord(word)
}

// persistWord appends a word to the personal dictionary file.
func persistWord(word string) {
	path := personalDicPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.WriteString(word + "\n")
}
