//go:build linux && !android

package spellcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setLocale(t *testing.T, all, lang, language string) {
	t.Helper()
	t.Setenv("LC_ALL", all)
	t.Setenv("LANG", lang)
	t.Setenv("LANGUAGE", language)
}

func TestDetectLang(t *testing.T) {
	tests := []struct {
		name     string
		all      string
		lang     string
		language string
		want     string
	}{
		{"empty falls back", "", "", "", "en_US"},
		{"plain lang", "", "en_US", "", "en_US"},
		{"encoding stripped", "", "en_US.UTF-8", "", "en_US"},
		{"modifier stripped", "", "fr_FR@euro", "", "fr_FR"},
		{"LC_ALL wins", "de_DE.UTF-8", "en_US", "", "de_DE"},
		{"C locale skipped", "C", "en_US", "", "en_US"},
		{"C.UTF-8 skipped", "C.UTF-8", "en_US", "", "en_US"},
		{"POSIX skipped", "", "POSIX", "", "en_US"},
		{"language list takes first", "", "", "de_DE:de", "de_DE"},
		{"C falls through to language", "C", "POSIX", "fr_FR", "fr_FR"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setLocale(t, tc.all, tc.lang, tc.language)
			if got := detectLang(); got != tc.want {
				t.Errorf("detectLang() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFindDictSkipsEmptyEntries(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".aff", ".dic"} {
		if err := os.WriteFile(filepath.Join(dir, "en_XX"+ext),
			[]byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// Empty entries around a valid dir must not break the search.
	t.Setenv("DICPATH", ":"+dir+":")
	aff, dic, ok := findDict("en_XX")
	if !ok {
		t.Fatal("findDict missed a dictionary beside empty DICPATH entries")
	}
	if !strings.HasSuffix(aff, ".aff") || !strings.HasSuffix(dic, ".dic") {
		t.Errorf("findDict = %q, %q, want .aff/.dic pair", aff, dic)
	}
}

func TestFindDictEmptyEntryProbesNothing(t *testing.T) {
	// A dictionary pair sitting in the working directory must not
	// match when DICPATH holds only empty entries: Join("", lang)
	// used to probe the CWD.
	dir := t.TempDir()
	for _, ext := range []string{".aff", ".dic"} {
		if err := os.WriteFile(filepath.Join(dir, "zz_CWDPROBE"+ext),
			[]byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	t.Setenv("DICPATH", ":")
	if _, _, ok := findDict("zz_CWDPROBE"); ok {
		t.Error("findDict matched via empty DICPATH entry probing the CWD")
	}
}

func TestParsePersonalWords(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"plain words", []string{"hello\n", "world\n"}, []string{"hello", "world"}},
		{"count header skipped", []string{"2\n", "hello\n", "world\n"},
			[]string{"hello", "world"}},
		{"lone number is a word", []string{"123\n"}, []string{"123"}},
		{"blanks dropped", []string{"\n", "  \n", "hello\n"},
			[]string{"hello"}},
		{"overlong dropped", []string{strings.Repeat("x", maxPersonalLine+1) + "\n", "ok\n"},
			[]string{"ok"}},
		{"empty", nil, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parsePersonalWords(tc.lines)
			if len(got) != len(tc.want) {
				t.Fatalf("parsePersonalWords = %q, want %q", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("parsePersonalWords = %q, want %q", got, tc.want)
				}
			}
		})
	}
}

func TestParsePersonalWordsCapsCount(t *testing.T) {
	var lines []string
	for range maxPersonalWords + 10 {
		lines = append(lines, "word\n")
	}
	if got := parsePersonalWords(lines); len(got) != maxPersonalWords {
		t.Errorf("parsePersonalWords capped at %d, want %d", len(got), maxPersonalWords)
	}
}

func TestReadPersonalLines(t *testing.T) {
	// A final line without a trailing newline is still a word;
	// readPersonalLines returns raw lines, trimming is parse's job.
	got := readPersonalLines(strings.NewReader("hello\nworld"))
	if len(got) != 2 || got[0] != "hello\n" || got[1] != "world" {
		t.Errorf("readPersonalLines = %q, want [hello world]", got)
	}
	if got := readPersonalLines(strings.NewReader("")); len(got) != 1 || got[0] != "" {
		t.Errorf("readPersonalLines empty = %q, want one empty line", got)
	}
}

func TestValidLearnWord(t *testing.T) {
	tests := []struct {
		name string
		word string
		want bool
	}{
		{"plain word", "hello", true},
		{"numeric word", "911", true},
		{"empty", "", false},
		{"newline", "a\nb", false},
		{"carriage return", "a\rb", false},
		{"tab", "a\tb", false},
		{"NUL", "a\x00b", false},
		{"at cap", strings.Repeat("x", maxPersonalLine), true},
		{"past cap", strings.Repeat("x", maxPersonalLine+1), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := validLearnWord(tc.word); got != tc.want {
				t.Errorf("validLearnWord(%q) = %v, want %v", tc.word, got, tc.want)
			}
		})
	}
}

func TestPersistWord(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	path := personalDicPath()

	persistWord("hello")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "hello\n" {
		t.Errorf("file = %q, want %q", content, "hello\n")
	}

	// One Learn stores one line: newline, control and overlong
	// words must leave the file untouched.
	before := string(content)
	for _, bad := range []string{"", "a\nb", "a\rb", "a\tb", "a\x00b",
		strings.Repeat("x", maxPersonalLine+1)} {
		persistWord(bad)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(after) != before {
		t.Errorf("file changed by rejected words: %q was %q", after, before)
	}
}
