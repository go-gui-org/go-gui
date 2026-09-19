package gui

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// countingFileSvgParser counts file parses so cache-hit tests can
// tell a hit (no new parse) from a miss (one more parse).
type countingFileSvgParser struct {
	width, height float32
	fileParses    int
	filteredTris  []float32
}

func (m *countingFileSvgParser) ParseSvg(_ string) (*SvgParsed, error) {
	return &SvgParsed{Width: m.width, Height: m.height}, nil
}

func (m *countingFileSvgParser) ParseSvgFile(_ string) (*SvgParsed, error) {
	m.fileParses++
	parsed := &SvgParsed{Width: m.width, Height: m.height}
	if len(m.filteredTris) > 0 {
		parsed.FilteredGroups = []SvgParsedFilteredGroup{{
			Paths: []TessellatedPath{{
				Triangles: m.filteredTris,
				Color:     SvgColor{A: 255},
			}},
		}}
	}
	return parsed, nil
}

func (m *countingFileSvgParser) ParseSvgDimensions(_ string) (
	float32, float32, error,
) {
	return m.width, m.height, nil
}

func (m *countingFileSvgParser) Tessellate(
	_ *SvgParsed, _ float32,
) []TessellatedPath {
	return nil
}

// Rewriting the file must invalidate the cached entry. The old
// key hashed the path alone, so an edit served stale art.
func TestLoadSvg_FileEditInvalidatesCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	w := NewWindow(WindowCfg{})
	parser := &countingFileSvgParser{width: 10, height: 10}
	w.SetSvgParser(parser)
	if _, err := w.LoadSvg(path, 10, 10); err != nil {
		t.Fatalf("first LoadSvg: %v", err)
	}
	if _, err := w.LoadSvg(path, 10, 10); err != nil {
		t.Fatalf("second LoadSvg: %v", err)
	}
	if parser.fileParses != 1 {
		t.Fatalf("repeat load parsed %d times, want 1 (cache hit)",
			parser.fileParses)
	}
	// Rewrite with a new size so size and mtime both move.
	if err := os.WriteFile(path,
		[]byte("<svg><rect/></svg>"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	// Inside the recheck interval the load trusts the recorded
	// file state, so the edit is not seen yet.
	if _, err := w.LoadSvg(path, 10, 10); err != nil {
		t.Fatalf("throttled LoadSvg: %v", err)
	}
	if parser.fileParses != 1 {
		t.Fatalf("throttled load parsed %d times, want 1 (hit)",
			parser.fileParses)
	}
	expireSvgFileCheck(w, path)
	if _, err := w.LoadSvg(path, 10, 10); err != nil {
		t.Fatalf("post-edit LoadSvg: %v", err)
	}
	if parser.fileParses != 2 {
		t.Fatalf("post-edit load parsed %d times, want 2 (miss)",
			parser.fileParses)
	}
}

// expireSvgFileCheck ages the recorded file check for src past
// svgFileRecheckInterval, so the next load stats the file again.
func expireSvgFileCheck(w *Window, src string) {
	dc := StateMapRead[uint64, svgDimEntry](w, nsSvgDimCache)
	if dc == nil {
		return
	}
	srcHash := hashString(src)
	if entry, ok := dc.Get(srcHash); ok {
		entry.checkedAt = entry.checkedAt.Add(-2 * svgFileRecheckInterval)
		dc.Set(srcHash, entry)
	}
}

// A repeat load of a cached file source runs every frame from the
// render pass. It must not resolve or stat the file, and it must
// not allocate. Before the throttle each hit ran EvalSymlinks and
// two Stat calls.
func TestLoadSvg_FileCacheHitNoAllocs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	w := NewWindow(WindowCfg{})
	w.SetSvgParser(&countingFileSvgParser{width: 10, height: 10})
	if _, err := w.LoadSvg(path, 10, 10); err != nil {
		t.Fatalf("first LoadSvg: %v", err)
	}
	if _, _, err := w.getSvgDimensions(path); err != nil {
		t.Fatalf("first getSvgDimensions: %v", err)
	}
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = w.LoadSvg(path, 10, 10)
		_, _, _ = w.getSvgDimensions(path)
	})
	if allocs != 0 {
		t.Fatalf("cached file SVG hit allocated %v times, want 0", allocs)
	}
}

// The dim cache and the render cache evict on their own, so a
// render entry can outlive the dim entry that holds its file
// fingerprint. The slow-path hit must record the fingerprint
// again. Before the fix it did not, so every later load resolved
// and stat'ed the file, and allocated, with no way back.
func TestLoadSvg_EvictedDimEntryRestoredOnHit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	w := NewWindow(WindowCfg{})
	parser := &countingFileSvgParser{width: 10, height: 10}
	w.SetSvgParser(parser)
	if _, err := w.LoadSvg(path, 10, 10); err != nil {
		t.Fatalf("first LoadSvg: %v", err)
	}
	// Stand in for a FIFO eviction of the dim entry alone.
	StateMapRead[uint64, svgDimEntry](w, nsSvgDimCache).
		Delete(hashString(path))
	if _, err := w.LoadSvg(path, 10, 10); err != nil {
		t.Fatalf("slow-path LoadSvg: %v", err)
	}
	if parser.fileParses != 1 {
		t.Fatalf("slow-path load parsed %d times, want 1 (hit)",
			parser.fileParses)
	}
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = w.LoadSvg(path, 10, 10)
	})
	if allocs != 0 {
		t.Fatalf("hit after dim eviction allocated %v times, want 0",
			allocs)
	}
}

// A checkedAt later than now (restored snapshot, clock jump) must
// read as stale. Treated as fresh, the file is never checked again.
func TestFreshSvgFingerprintFutureCheckIsStale(t *testing.T) {
	w := NewWindow(WindowCfg{})
	now := time.Now()
	dc := StateMap[uint64, svgDimEntry](w, nsSvgDimCache, capModerate)
	dc.Set(1, svgDimEntry{contentHash: 7, checkedAt: now.Add(time.Hour)})
	if _, ok := w.freshSvgFingerprint(1, now); ok {
		t.Fatal("future checkedAt reported fresh")
	}
	dc.Set(2, svgDimEntry{contentHash: 7, checkedAt: now})
	if e, ok := w.freshSvgFingerprint(2, now.Add(time.Millisecond)); !ok ||
		e.contentHash != 7 {
		t.Fatalf("recent check: got (%v, %v), want (7, true)",
			e.contentHash, ok)
	}
	if _, ok := w.freshSvgFingerprint(2,
		now.Add(svgFileRecheckInterval)); ok {
		t.Fatal("check at the interval edge reported fresh")
	}
}

// readCappedSvgFile must reject a file over the cap and a missing
// file with an error, and read a small file whole.
func TestReadCappedSvgFileCapAndMissing(t *testing.T) {
	dir := t.TempDir()
	small := filepath.Join(dir, "small.svg")
	if err := os.WriteFile(small, []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if data, err := readCappedSvgFile(small); err != nil ||
		string(data) != "<svg/>" {
		t.Fatalf("small file: got (%q, %v)", data, err)
	}
	over := filepath.Join(dir, "over.svg")
	if err := os.WriteFile(over,
		make([]byte, maxSvgSourceBytes+1), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := readCappedSvgFile(over); err == nil {
		t.Fatal("file over the cap read, want error")
	}
	if _, err := readCappedSvgFile(filepath.Join(dir, "no.svg")); err == nil {
		t.Fatal("missing file read, want error")
	}
}

// After the recheck interval, getSvgDimensions must see an edit
// and parse again. Inside the interval it serves the cached dims.
func TestGetSvgDimensions_FileEditAfterRecheck(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "icon.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	w := NewWindow(WindowCfg{})
	parser := &countingFileSvgParser{width: 10, height: 10}
	w.SetSvgParser(parser)
	if gw, _, err := w.getSvgDimensions(path); err != nil || gw != 10 {
		t.Fatalf("first dims: got (%v, %v), want 10", gw, err)
	}
	parser.width = 20
	if err := os.WriteFile(path,
		[]byte("<svg><rect/></svg>"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	if gw, _, _ := w.getSvgDimensions(path); gw != 10 {
		t.Fatalf("throttled dims: got %v, want cached 10", gw)
	}
	expireSvgFileCheck(w, path)
	if gw, _, err := w.getSvgDimensions(path); err != nil || gw != 20 {
		t.Fatalf("post-edit dims: got (%v, %v), want 20", gw, err)
	}
}

// A zero-length vector has no angle. vecAngle must return 0, not
// NaN, and a non-finite polyline must give no arc table.
func TestArcMathDegenerateInputs(t *testing.T) {
	if got := vecAngle(0, 0, 1, 0); got != 0 {
		t.Errorf("vecAngle(zero, x) = %v, want 0", got)
	}
	if got := vecAngle(1, 0, 0, 0); got != 0 {
		t.Errorf("vecAngle(x, zero) = %v, want 0", got)
	}
	nan := float32(math.NaN())
	if table, total := buildArcLengthTable(
		[]float32{0, 0, nan, 1}); table != nil || total != 0 {
		t.Errorf("NaN polyline: got (%v, %v), want (nil, 0)",
			table, total)
	}
}

// Filtered-group vertices must count toward the cache cap. Before
// the fix only main paths counted, so a filter-heavy file entered
// the cache at any size.
func TestLoadSvg_FilteredVertsSkipCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.svg")
	if err := os.WriteFile(path, []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	w := NewWindow(WindowCfg{})
	// 1.3M floats beats the 1.25M cap with margin.
	big := make([]float32, 1_300_000)
	parser := &countingFileSvgParser{
		width: 10, height: 10, filteredTris: big,
	}
	w.SetSvgParser(parser)
	if _, err := w.LoadSvg(path, 10, 10); err != nil {
		t.Fatalf("first LoadSvg: %v", err)
	}
	if _, err := w.LoadSvg(path, 10, 10); err != nil {
		t.Fatalf("second LoadSvg: %v", err)
	}
	if parser.fileParses != 2 {
		t.Fatalf("oversize graphic parsed %d times, want 2 (no cache)",
			parser.fileParses)
	}
}

func TestQuantizeSvgDimHostileInputs(t *testing.T) {
	cases := []struct {
		name string
		in   float32
		want int32
	}{
		{"NaN", float32(math.NaN()), 0},
		{"+Inf", float32(math.Inf(1)), 0},
		{"-Inf", float32(math.Inf(-1)), 0},
		{"valid", 10, 100},
		{"overflow", 1e30, math.MaxInt32},
		{"underflow", -1e30, math.MinInt32},
	}
	for _, c := range cases {
		got := quantizeSvgDim(c.in)
		if got != c.want {
			t.Errorf("quantizeSvgDim(%v) = %v want %v",
				c.in, got, c.want)
		}
	}
}

// A multi-byte ID must truncate to valid UTF-8, the same cut the
// svg package makes, or both layers hash one ID differently. The
// euro sign takes 3 bytes, so a 256-byte cut splits a rune.
func TestClampSvgCacheIDKeepsRunesIntact(t *testing.T) {
	huge := strings.Repeat("€", 200) // 600 bytes
	got := clampSvgCacheID(huge)
	if len(got) > maxSvgCacheElementIDLen {
		t.Fatalf("expected len <= %d, got %d",
			maxSvgCacheElementIDLen, len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatal("truncated ID is not valid UTF-8")
	}
}

// A hostile path data string must flatten in bounded space and a
// bad scale must return nil instead of NaN geometry.
func TestFlattenDefsPathHostileBounds(t *testing.T) {
	huge := "M0,0" + strings.Repeat(" L1,1", 300_000)
	out := flattenDefsPath(huge, 1)
	if len(out) > maxDefsPathFloats+128 {
		t.Fatalf("flattened %d floats, want bounded output",
			len(out))
	}
	// One command letter with implicit repeats must hit the same
	// cap: the per-command check alone lets it run past.
	implicit := "M0,0 L" + strings.Repeat(" 1,1", 99_000)
	if out := flattenDefsPath(implicit, 1); len(out) > maxDefsPathFloats {
		t.Fatalf("implicit repeats flattened %d floats, want <= %d",
			len(out), maxDefsPathFloats)
	}
	if out := flattenDefsPath("M0,0 L10,10", float32(math.NaN())); out != nil {
		t.Fatal("NaN scale must return nil")
	}
	if out := flattenDefsPath("M0,0 L10,10", -1); out != nil {
		t.Fatal("negative scale must return nil")
	}
}

func TestParseFloatRejectsNonFinite(t *testing.T) {
	if got := parseFloat("1e999"); got != 0 {
		t.Errorf("overflow must map to 0, got %v", got)
	}
	if got := parseFloat("garbage"); got != 0 {
		t.Errorf("garbage must map to 0, got %v", got)
	}
	if got := parseFloat("2.5"); got != 2.5 {
		t.Errorf("valid input changed: got %v", got)
	}
}

// Bad sampler inputs must not produce NaN coordinates.
func TestSamplePathAtHostileInputs(t *testing.T) {
	poly := []float32{0, 0, 10, 0}
	if x, y, _ := samplePathAt(poly, []float32{0}, 5); x != 0 || y != 0 {
		t.Fatalf("short table must return zeros, got (%v,%v)", x, y)
	}
	table := []float32{0, 10}
	x, y, _ := samplePathAt(poly, table, float32(math.NaN()))
	if math.IsNaN(float64(x)) || math.IsNaN(float64(y)) {
		t.Fatal("NaN distance must not produce NaN output")
	}
	if x, y, _ := samplePathAt(nil, table, 5); x != 0 || y != 0 {
		t.Fatalf("short polyline must return zeros, got (%v,%v)", x, y)
	}
}

// A sliver triangle has no region and must not hit, even at
// its own centroid. The old zero-only test let near-zero areas
// through, and the huge inverse mapped the centroid inside.
func TestPointInTriSliver(t *testing.T) {
	if pointInTri(1, 1e-9, 0, 0, 1e20, 0, 2e20, 0) {
		t.Fatal("sliver triangle must not contain the point")
	}
	// Micro-triangle, area 5e-11: centroid must still miss.
	if pointInTri(1e-5/3, 1e-5/3, 0, 0, 1e-5, 0, 0, 1e-5) {
		t.Fatal("micro-triangle centroid must not hit")
	}
	if !pointInTri(0.25, 0.25, 0, 0, 1, 0, 0, 1) {
		t.Fatal("interior point of a unit triangle must hit")
	}
}
