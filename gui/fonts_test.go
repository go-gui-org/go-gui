package gui

import (
	"errors"
	"io"
	"log"
	"os"
	"testing"
)

func TestIconLookupContainsAllConstants(t *testing.T) {
	if len(IconLookup) != 255 {
		t.Errorf("IconLookup: got %d entries, want 255", len(IconLookup))
	}
}

func TestIconLookupKnownKeys(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"icon_arrow_down", IconArrowDown},
		{"icon_check", IconCheck},
		{"icon_home", IconHome},
		{"icon_star", IconStar},
		{"icon_yaki_dango", IconYakiDango},
	}
	for _, tt := range tests {
		if got, ok := IconLookup[tt.key]; !ok {
			t.Errorf("IconLookup missing key %q", tt.key)
		} else if got != tt.want {
			t.Errorf("IconLookup[%q] = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestIconLookupMissingKey(t *testing.T) {
	if _, ok := IconLookup["nonexistent"]; ok {
		t.Error("expected missing key to return false")
	}
}

func TestFontVariantsStruct(t *testing.T) {
	fv := fontVariants{normal: "a.ttf", Bold: "b.ttf", Italic: "i.ttf", mono: "m.ttf"}
	if fv.normal != "a.ttf" || fv.mono != "m.ttf" {
		t.Error("FontVariants fields not set correctly")
	}
}

func TestIconFontName(t *testing.T) {
	if IconFontName != "feathericon" {
		t.Errorf("IconFontName = %q, want %q", IconFontName, "feathericon")
	}
}

func TestRegisterAppFont(t *testing.T) {
	// Registration lists are process globals; restore them so other
	// tests (and the backends) see the original state.
	saved := appFontPaths
	t.Cleanup(func() { appFontPaths = saved })
	appFontPaths = nil

	RegisterAppFont("/tmp/a.ttf")
	RegisterAppFont("/tmp/a.ttf")
	RegisterAppFont("/tmp/b.ttf")
	if len(appFontPaths) != 2 {
		t.Fatalf("AppFontPaths = %v, want 2 entries", appFontPaths)
	}
	if appFontPaths[0] != "/tmp/a.ttf" || appFontPaths[1] != "/tmp/b.ttf" {
		t.Errorf("AppFontPaths = %v, want [a b] in order", appFontPaths)
	}
}

func TestRegisterAppFontBytes(t *testing.T) {
	saved := appFontData
	t.Cleanup(func() { appFontData = saved })
	appFontData = nil

	registerAppFontBytes(nil)
	registerAppFontBytes([]byte{})
	if len(appFontData) != 0 {
		t.Fatalf("empty data registered: %v", appFontData)
	}

	registerAppFontBytes([]byte("font-a"))
	// Distinct slice, equal contents — must dedupe by value.
	registerAppFontBytes([]byte("font-a"))
	registerAppFontBytes([]byte("font-b"))
	if len(appFontData) != 2 {
		t.Fatalf("AppFontData has %d entries, want 2", len(appFontData))
	}
	if string(appFontData[0]) != "font-a" || string(appFontData[1]) != "font-b" {
		t.Errorf("AppFontData = %q, want [font-a font-b] in order",
			appFontData)
	}
}

// fakeRegistrar records what LoadAppFonts asked it to load and fails
// the entries named in failFiles / failBytes.
type fakeRegistrar struct {
	files     []string
	data      [][]byte
	failFiles map[string]bool
	failBytes map[string]bool
}

func (f *fakeRegistrar) AddFontFile(path string) error {
	f.files = append(f.files, path)
	if f.failFiles[path] {
		return errors.New("boom")
	}
	return nil
}

func (f *fakeRegistrar) AddFontBytes(data []byte) error {
	f.data = append(f.data, data)
	if f.failBytes[string(data)] {
		return errors.New("boom")
	}
	return nil
}

// setAppFonts points both registration globals at the given fixtures
// and restores them when the test ends.
func setAppFonts(t *testing.T, paths []string, data [][]byte) {
	t.Helper()
	savedPaths, savedData := appFontPaths, appFontData
	t.Cleanup(func() { appFontPaths, appFontData = savedPaths, savedData })
	appFontPaths, appFontData = paths, data
}

func TestLoadAppFontsRegistersBothLists(t *testing.T) {
	setAppFonts(t,
		[]string{"/tmp/a.ttf", "/tmp/b.ttf"},
		[][]byte{[]byte("font-c")})

	fake := &fakeRegistrar{}
	LoadAppFonts(fake, "test")

	if len(fake.files) != 2 ||
		fake.files[0] != "/tmp/a.ttf" || fake.files[1] != "/tmp/b.ttf" {
		t.Errorf("AddFontFile calls = %v, want both paths in order",
			fake.files)
	}
	if len(fake.data) != 1 || string(fake.data[0]) != "font-c" {
		t.Errorf("AddFontBytes calls = %q, want [font-c]", fake.data)
	}
}

// A font that fails to load must not stop the ones after it — an
// unreadable font should cost its own glyphs, not the whole window.
func TestLoadAppFontsContinuesPastFailures(t *testing.T) {
	setAppFonts(t,
		[]string{"/tmp/bad.ttf", "/tmp/good.ttf"},
		[][]byte{[]byte("bad"), []byte("good")})

	// Failures are logged; keep the test output clean.
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	fake := &fakeRegistrar{
		failFiles: map[string]bool{"/tmp/bad.ttf": true},
		failBytes: map[string]bool{"bad": true},
	}
	LoadAppFonts(fake, "test")

	if len(fake.files) != 2 || fake.files[1] != "/tmp/good.ttf" {
		t.Errorf("AddFontFile calls = %v, want the good path still attempted",
			fake.files)
	}
	if len(fake.data) != 2 || string(fake.data[1]) != "good" {
		t.Errorf("AddFontBytes calls = %q, want the good font still attempted",
			fake.data)
	}
}

func TestLoadAppFontsNilRegistrarNoPanic(t *testing.T) {
	setAppFonts(t, []string{"/tmp/a.ttf"}, [][]byte{[]byte("font-a")})
	LoadAppFonts(nil, "test") // must not panic
}

func TestLoadAppFontsEmptyListsNoCalls(t *testing.T) {
	setAppFonts(t, nil, nil)
	fake := &fakeRegistrar{}
	LoadAppFonts(fake, "test")
	if len(fake.files) != 0 || len(fake.data) != 0 {
		t.Errorf("registrar called with empty lists: %v %q",
			fake.files, fake.data)
	}
}
