package svg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestParserReleaseParsedRemovesEntry(t *testing.T) {
	p := New()
	parsed, err := p.ParseSvg(`<svg viewBox="0 0 10 10"><rect x="0" y="0" width="10" height="10"/></svg>`)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	p.mu.Lock()
	_, ok := p.byParsed[parsed]
	p.mu.Unlock()
	if !ok {
		t.Fatal("expected parser cache entry after parse")
	}
	p.ReleaseParsed(parsed)
	p.mu.Lock()
	_, ok = p.byParsed[parsed]
	p.mu.Unlock()
	if ok {
		t.Fatal("expected parser cache entry to be removed")
	}
}

func TestParserClearSvgParserCacheEmptiesMap(t *testing.T) {
	p := New()
	parsedA, err := p.ParseSvg(`<svg viewBox="0 0 10 10"><rect x="0" y="0" width="10" height="10"/></svg>`)
	if err != nil {
		t.Fatalf("parse A failed: %v", err)
	}
	parsedB, err := p.ParseSvg(`<svg viewBox="0 0 20 20"><rect x="0" y="0" width="20" height="20"/></svg>`)
	if err != nil {
		t.Fatalf("parse B failed: %v", err)
	}
	p.ClearSvgParserCache()
	p.mu.Lock()
	_, okA := p.byParsed[parsedA]
	_, okB := p.byParsed[parsedB]
	p.mu.Unlock()
	if okA {
		t.Fatal("expected A to be removed")
	}
	if okB {
		t.Fatal("expected B to be removed")
	}
}

func TestParserCacheHitReturnsSameParsed(t *testing.T) {
	p := New()
	src := `<svg viewBox="0 0 10 10"><rect x="0" y="0" width="10" height="10"/></svg>`
	a, err := p.ParseSvg(src)
	if err != nil {
		t.Fatalf("first parse failed: %v", err)
	}
	b, err := p.ParseSvg(src)
	if err != nil {
		t.Fatalf("second parse failed: %v", err)
	}
	if a != b {
		t.Fatal("expected parser cache hit to return same parsed pointer")
	}
}

func TestParserInvalidateSvgSourceRemovesOnlyTarget(t *testing.T) {
	p := New()
	srcA := `<svg viewBox="0 0 10 10"><rect x="0" y="0" width="10" height="10"/></svg>`
	srcB := `<svg viewBox="0 0 20 20"><rect x="0" y="0" width="20" height="20"/></svg>`
	a, err := p.ParseSvg(srcA)
	if err != nil {
		t.Fatalf("parse A failed: %v", err)
	}
	b, err := p.ParseSvg(srcB)
	if err != nil {
		t.Fatalf("parse B failed: %v", err)
	}
	p.InvalidateSvgSource(srcA)
	p.mu.Lock()
	_, okA := p.byParsed[a]
	_, okB := p.byParsed[b]
	p.mu.Unlock()
	if okA {
		t.Fatal("expected source A to be removed")
	}
	if !okB {
		t.Fatal("expected source B to remain")
	}
}

// Empty source string must be a no-op. Earlier impl let
// filepath.Clean("") collapse to ".", which absolutized to the CWD
// and could match unrelated cache entries.
func TestParserInvalidateSvgSourceEmptyIsNoop(t *testing.T) {
	p := New()
	src := `<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	parsed, err := p.ParseSvg(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p.InvalidateSvgSource("")
	p.mu.Lock()
	_, ok := p.byParsed[parsed]
	p.mu.Unlock()
	if !ok {
		t.Fatal("empty invalidate must not drop entries")
	}
}

// Invalidate must drop file-backed entries (cache key includes file
// contents, so the old hash-reconstruction path could not match).
func TestParserInvalidateSvgSourceDropsFileEntries(t *testing.T) {
	p := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(path,
		[]byte(`<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	parsed, err := p.ParseSvgFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p.mu.Lock()
	_, ok := p.byParsed[parsed]
	p.mu.Unlock()
	if !ok {
		t.Fatal("expected file entry in cache")
	}
	p.InvalidateSvgSource(path)
	p.mu.Lock()
	_, ok = p.byParsed[parsed]
	p.mu.Unlock()
	if ok {
		t.Fatal("file-backed entry must be invalidated")
	}
}

// Invalidate must drop every option-variant for a single source.
// Earlier impl only knew about {} and {PrefersReducedMotion:true},
// leaving FlatnessTolerance / Hovered / Focused variants stranded.
func TestParserInvalidateSvgSourceDropsAllOptionVariants(t *testing.T) {
	p := New()
	src := `<svg viewBox="0 0 10 10"><rect width="10" height="10" id="r"/></svg>`
	variants := []gui.SvgParseOpts{
		{},
		{PrefersReducedMotion: true},
		{FlatnessTolerance: 0.25},
		{HoveredElementID: "r"},
		{FocusedElementID: "r"},
		{PrefersReducedMotion: true, FlatnessTolerance: 0.5,
			HoveredElementID: "r", FocusedElementID: "r"},
	}
	parsedAll := make([]*gui.SvgParsed, 0, len(variants))
	for _, v := range variants {
		parsed, err := p.ParseSvgWithOpts(src, v)
		if err != nil {
			t.Fatalf("parse %+v: %v", v, err)
		}
		parsedAll = append(parsedAll, parsed)
	}
	p.mu.Lock()
	beforeCount := len(p.byHash)
	p.mu.Unlock()
	if beforeCount < len(variants) {
		t.Fatalf("expected at least %d cache entries, got %d",
			len(variants), beforeCount)
	}
	p.InvalidateSvgSource(src)
	p.mu.Lock()
	for _, parsed := range parsedAll {
		if _, ok := p.byParsed[parsed]; ok {
			p.mu.Unlock()
			t.Fatal("variant survived invalidation")
		}
	}
	p.mu.Unlock()
}

// Cache stored under the absolute-resolved path. Invalidation given
// the relative path (after chdir into the file's directory) must
// still match — candidateSourceKeys re-derives abs from clean.
func TestParserInvalidateSvgSourceMatchesAcrossPathForms(t *testing.T) {
	p := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(path,
		[]byte(`<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	parsed, err := p.ParseSvgFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	t.Chdir(dir)
	p.InvalidateSvgSource("icon.svg")
	p.mu.Lock()
	_, ok := p.byParsed[parsed]
	p.mu.Unlock()
	if ok {
		t.Fatal("relative-path invalidate must match abs-path cached entry")
	}
}

// Cache via the resolved canonical path; invalidate via a symlink
// that points at it. EvalSymlinks(linkPath) must resolve to the
// canonical key.
func TestParserInvalidateSvgSourceMatchesViaSymlink(t *testing.T) {
	p := New()
	dir := t.TempDir()
	real := filepath.Join(dir, "real.svg")
	link := filepath.Join(dir, "link.svg")
	if err := os.WriteFile(real,
		[]byte(`<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`),
		0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	parsed, err := p.ParseSvgFile(real)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p.InvalidateSvgSource(link)
	p.mu.Lock()
	_, ok := p.byParsed[parsed]
	p.mu.Unlock()
	if ok {
		t.Fatal("symlink-path invalidate must match canonical-path cached entry")
	}
}

// Reparse after invalidate must return a fresh pointer, proving the
// drop reached buildParsed and didn't leave a stale entry behind.
func TestParserInvalidateThenReparseReturnsFreshPointer(t *testing.T) {
	p := New()
	src := `<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	first, err := p.ParseSvg(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	p.InvalidateSvgSource(src)
	second, err := p.ParseSvg(src)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if first == second {
		t.Fatal("expected fresh parsed pointer after invalidate")
	}
}

func TestParserParseSvgFileCacheRefreshesWhenFileChanges(t *testing.T) {
	p := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(path,
		[]byte(`<svg width="10" height="10"></svg>`), 0o644); err != nil {
		t.Fatalf("write first file: %v", err)
	}
	first, err := p.ParseSvgFile(path)
	if err != nil {
		t.Fatalf("first parse failed: %v", err)
	}
	if err := os.WriteFile(path,
		[]byte(`<svg width="20" height="20"></svg>`), 0o644); err != nil {
		t.Fatalf("write second file: %v", err)
	}
	second, err := p.ParseSvgFile(path)
	if err != nil {
		t.Fatalf("second parse failed: %v", err)
	}
	if second.Width != 20 || second.Height != 20 {
		t.Fatalf("expected refreshed dimensions 20x20, got %vx%v",
			second.Width, second.Height)
	}
	if first == second {
		t.Fatal("expected file cache miss after file content change")
	}
}
