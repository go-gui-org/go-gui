//go:build linux && !android

// Pure-Go dictionary helpers for Linux spell checking, without the
// Hunspell dependency. Split from spellcheck_linux.go so they
// compile — and their tests run — on every Linux build, including
// the ones without -tags hunspell that CI covers.
package spellcheck

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// detectLang returns the locale string from environment variables,
// stripped of encoding and modifier suffixes.
func detectLang() string {
	for _, env := range []string{"LC_ALL", "LANG", "LANGUAGE"} {
		v := os.Getenv(env)
		if v == "" {
			continue
		}
		// LANGUAGE is a priority list ("en_US:en"); only the
		// first entry names a dictionary.
		if i := strings.IndexByte(v, ':'); i >= 0 {
			v = v[:i]
		}
		// Strip .UTF-8 or other encoding suffix.
		if i := strings.IndexByte(v, '.'); i > 0 {
			v = v[:i]
		}
		// Strip @modifier.
		if i := strings.IndexByte(v, '@'); i > 0 {
			v = v[:i]
		}
		// Bare "C"/"POSIX" — including "C.UTF-8" reduced above —
		// names no dictionary; keep looking instead of returning it.
		if v == "" || v == "C" || v == "POSIX" {
			continue
		}
		return v
	}
	return "en_US"
}

// findDict searches standard paths for hunspell dictionary files.
func findDict(lang string) (aff, dic string, ok bool) {
	var dirs []string
	if p := os.Getenv("DICPATH"); p != "" {
		for dir := range strings.SplitSeq(p, ":") {
			// Skip empty entries: Join("", lang) probes the
			// working directory, which is never a dict dir.
			if dir == "" {
				continue
			}
			dirs = append(dirs, dir)
		}
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

// maxPersonalLine caps one personal-dictionary line. The capped
// Learn path never stores more than 256 bytes, so a longer line is
// foreign to this file; skipping it keeps one garbage line from
// hiding the words after it, which is what a fixed-buffer scanner
// would do on abort.
const maxPersonalLine = 1024

// maxPersonalWords caps loaded words. The file grows one line per
// Learn, so an unbounded load lets a runaway writer stall startup.
const maxPersonalWords = 10000

// readPersonalLines reads dictionary lines from r. The caller caps
// r (e.g. with io.LimitReader) so a giant line cannot balloon
// memory; overlong content is still returned here and filtered by
// parsePersonalWords, so one garbage line never hides the rest.
func readPersonalLines(r io.Reader) []string {
	var lines []string
	reader := bufio.NewReader(r)
	for {
		line, readErr := reader.ReadString('\n')
		lines = append(lines, line)
		if readErr != nil {
			break
		}
	}
	return lines
}

// parsePersonalWords drops blanks and overlong lines, then strips a
// leading hunspell count header: a number with words after it. A
// file holding only a number learned that number as a word, so the
// header needs a follower to count as one. The result is capped at
// maxPersonalWords, so callers can range over it directly.
// Lengths are bytes, the same unit readPersonalLines caps with, so a
// word Learn accepts always survives a load.
func parsePersonalWords(lines []string) []string {
	var words []string
	for _, line := range lines {
		if len(words) >= maxPersonalWords {
			break
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || len(trimmed) > maxPersonalLine {
			continue
		}
		words = append(words, trimmed)
	}
	if len(words) > 1 {
		if _, convErr := strconv.Atoi(words[0]); convErr == nil {
			words = words[1:]
		}
	}
	return words
}

// validLearnWord reports whether word may enter the session and the
// personal file. Control characters (including newline) and words
// past the line cap are rejected: one Learn stores exactly one
// dictionary line, and the loader drops longer lines, so anything
// else would leave the session and the file disagreeing after a
// restart. Shared by Learn and persistWord so the two gates cannot
// drift apart.
func validLearnWord(word string) bool {
	if word == "" || len(word) > maxPersonalLine {
		return false
	}
	return strings.IndexFunc(word, unicode.IsControl) < 0
}

// persistWord appends a word to the personal dictionary file. Words
// failing validLearnWord are rejected: one Learn stores exactly one
// line.
func persistWord(word string) {
	if !validLearnWord(word) {
		return
	}
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
