package gui

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	orbGoldenPath = "testdata/thinking_orbs_golden.json"
)

// orbGoldenResolved mirrors one entry of the golden "resolved"
// table.
type orbGoldenResolved struct {
	Mode  string             `json:"mode"`
	Speed float64            `json:"speed"`
	Opts  map[string]float64 `json:"opts"`
}

// orbGoldenCase mirrors one entry of the golden "cases" list.
// Dots pack 6 values per dot (x, y, z, r, white, a); lines pack
// 7 per line (x1, y1, x2, y2, white, a, w).
type orbGoldenCase struct {
	Key       string    `json:"key"`
	State     string    `json:"state"`
	Size      int       `json:"size"`
	Mode      string    `json:"mode"`
	T         float64   `json:"t"`
	DotCount  int       `json:"dotCount"`
	LineCount int       `json:"lineCount"`
	Dots      []float64 `json:"dots"`
	Lines     []float64 `json:"lines"`
}

// orbGoldenFile mirrors the golden document envelope.
type orbGoldenFile struct {
	Tolerance float64                      `json:"tolerance"`
	Resolved  map[string]orbGoldenResolved `json:"resolved"`
	Cases     []orbGoldenCase              `json:"cases"`
}

// orbDesignForState maps a golden state name to its design.
func orbDesignForState(state string) (ThinkingOrbDesign, bool) {
	switch state {
	case "working":
		return ThinkingOrbWorking, true
	case "searching":
		return ThinkingOrbSearching, true
	case "solving":
		return ThinkingOrbSolving, true
	case "listening":
		return ThinkingOrbListening, true
	case "connecting":
		return ThinkingOrbConnecting, true
	case "weaving":
		return ThinkingOrbWeaving, true
	case "composing":
		return ThinkingOrbComposing, true
	case "breathing":
		return ThinkingOrbBreathing, true
	case "shaping":
		return ThinkingOrbShaping, true
	default:
		return ThinkingOrbWorking, false
	}
}

// loadOrbGoldenFile reads the vendored golden vectors. It
// reports false when the file is absent; callers skip then.
// The file is vendored bytes from the upstream ThinkingOrbs
// repo, never fetched at test time, so the test stays hermetic.
func loadOrbGoldenFile(t *testing.T) (orbGoldenFile, bool) {
	t.Helper()
	var golden orbGoldenFile
	raw, readErr := os.ReadFile(filepath.Clean(orbGoldenPath))
	if readErr != nil {
		t.Skipf("golden vectors absent: %v", readErr)
		return golden, false
	}
	if unmarshalErr := json.Unmarshal(raw, &golden); unmarshalErr != nil {
		t.Fatalf("golden file corrupt: %v", unmarshalErr)
		return golden, false
	}
	return golden, true
}

// orbMatchDots matches want dots to got dots within tol on all
// six fields, letting dots whose depths differ by at most tol
// swap slots. It returns the worst abs diff seen and the count
// of want dots left without a match.
func orbMatchDots(want, got []orbDot, tol float64) (float64, int) {
	used := make([]bool, len(got))
	worst := 0.0
	unmatched := 0
	for ei := range want {
		bestIdx := -1
		bestErr := math.Inf(1)
		for aj := range got {
			if used[aj] {
				continue
			}
			if math.Abs(want[ei].z-got[aj].z) > tol {
				continue
			}
			err := math.Abs(want[ei].x - got[aj].x)
			for _, dd := range []float64{
				math.Abs(want[ei].y - got[aj].y),
				math.Abs(want[ei].z - got[aj].z),
				math.Abs(want[ei].r - got[aj].r),
				math.Abs(want[ei].white - got[aj].white),
				math.Abs(want[ei].a - got[aj].a),
			} {
				if dd > err {
					err = dd
				}
			}
			if err <= tol && err < bestErr {
				bestErr = err
				bestIdx = aj
			}
		}
		if bestIdx < 0 {
			unmatched++
			continue
		}
		used[bestIdx] = true
		if bestErr > worst {
			worst = bestErr
		}
	}
	return worst, unmatched
}

// orbUnpackDots converts flat stride-6 golden dots.
func orbUnpackDots(flat []float64) []orbDot {
	dots := make([]orbDot, 0, len(flat)/6)
	for i := 0; i+6 <= len(flat); i += 6 {
		dots = append(dots, orbDot{
			x: flat[i], y: flat[i+1], z: flat[i+2],
			r: flat[i+3], white: flat[i+4], a: flat[i+5],
		})
	}
	return dots
}

// orbUnpackLines converts flat stride-7 golden lines.
func orbUnpackLines(flat []float64) []orbLine {
	lines := make([]orbLine, 0, len(flat)/7)
	for i := 0; i+6 < len(flat); i += 7 {
		lines = append(lines, orbLine{
			x1: flat[i], y1: flat[i+1], x2: flat[i+2],
			y2: flat[i+3], white: flat[i+4], a: flat[i+5],
			w: flat[i+6],
		})
	}
	return lines
}

// orbGoldenSlabFlips lists golden cases with rubik dots sitting
// exactly on move-slab boundaries. The slab test `coord < lo`
// amplifies a 1-ulp trig difference into a quarter-turn: e.g. at
// solving-20-0.6 Go computes cosLat*cos(lon) as exactly 0.5
// (bit pattern ...912, dot joins the [0.5, 1) slab) while Swift
// computes 0.4999999999999999 (...910, dot skips it), verified
// bit-for-bit against the reference Swift sources. The formulas
// match term for term; only the platform trig differs, so these
// cases verify through orbVerifySlabFlips instead of strictly.
var orbGoldenSlabFlips = map[string]bool{
	"solving-20-0.6": true,
	"solving-20-1.7": true,
	"solving-20-3.3": true,
	"solving-20-5.1": true,
}

// orbRubikKnifeEdges enumerates the rubik lattice and reports,
// per (li, lj), whether the pre-move coordinate sits within
// 1e-15 of the slab edge of a move with nonzero amount. Only
// such dots can change slab membership across platforms.
func orbRubikKnifeEdges(tt float64, o orbOpts) map[[2]int]bool {
	edge := make(map[[2]int]bool)
	moveCount := int(orbCountOpt(o, "moveCount", 14))
	moves := orbMakeMoves(moveCount)
	amount, _ := orbSolveCycle(tt, moveCount, 0.42, 1.2)
	latRings := orbCountOpt(o, "latRings", 15)
	lonDensity := orbOpt(o, "lonDensity", 40)
	for li := 0; li < orbThrough(latRings); li++ {
		lat := -math.Pi/2 + (float64(li)/latRings)*math.Pi
		cosLat := math.Cos(lat)
		sinLat := math.Sin(lat)
		lonCount := math.Max(1, orbJsRound(math.Abs(cosLat)*lonDensity))
		for lj := 0; lj < int(lonCount); lj++ {
			lon := (float64(lj) / lonCount) * 2 * math.Pi
			prex := cosLat * math.Cos(lon)
			prey := sinLat
			prez := cosLat * math.Sin(lon)
			for mi := range moves {
				if amount[mi] <= 0 {
					continue
				}
				mv := moves[mi]
				var coord float64
				switch mv.axis {
				case 0:
					coord = prex
				case 1:
					coord = prey
				default:
					coord = prez
				}
				if math.Abs(coord-mv.lo) <= 1e-15 ||
					math.Abs(coord-mv.hi) <= 1e-15 {
					edge[[2]int{li, lj}] = true
				}
			}
		}
	}
	return edge
}

// orbVerifySlabFlips checks a slab-flip case: every lattice dot
// whose Go rendering diverges from golden must sit on a
// documented slab boundary, and the divergent count must equal
// the unmatched count, so no other divergence hides behind the
// flips. It reports false on any unexplained difference.
func orbVerifySlabFlips(t *testing.T, key string, size ThinkingOrbSize,
	tt float64, want []orbDot, got orbFrameResult, tol float64,
	unmatched int) bool {
	t.Helper()
	resolved := orbResolve(ThinkingOrbSolving, size)
	edges := orbRubikKnifeEdges(tt, resolved.opts)
	length := size.Length()
	rr := (length / 2) * 0.82
	pt := newOrbProjector(tt*0.55, 0.35+0.1*math.Sin(tt*0.9),
		length/2, length/2, rr)
	moveCount := int(orbCountOpt(resolved.opts, "moveCount", 14))
	moves := orbMakeMoves(moveCount)
	amount, active := orbSolveCycle(tt, moveCount, 0.42, 1.2)
	latRings := orbCountOpt(resolved.opts, "latRings", 15)
	lonDensity := orbOpt(resolved.opts, "lonDensity", 40)
	divergent := 0
	explained := true
	for li := 0; li < orbThrough(latRings); li++ {
		lat := -math.Pi/2 + (float64(li)/latRings)*math.Pi
		cosLat := math.Cos(lat)
		sinLat := math.Sin(lat)
		lonCount := math.Max(1, orbJsRound(math.Abs(cosLat)*lonDensity))
		for lj := 0; lj < int(lonCount); lj++ {
			lon := (float64(lj) / lonCount) * 2 * math.Pi
			mx, my, mz, _ := orbApplyMoves(
				cosLat*math.Cos(lon), sinLat, cosLat*math.Sin(lon),
				moves, amount, active)
			gx, gy, _ := pt.project(mx, my, mz)
			best := math.Inf(1)
			for wi := range want {
				dd := math.Abs(want[wi].x-gx) + math.Abs(want[wi].y-gy)
				if dd < best {
					best = dd
				}
			}
			if best <= tol {
				continue
			}
			divergent++
			if !edges[[2]int{li, lj}] {
				t.Errorf("case %s lattice (%d,%d) diverges off any slab edge",
					key, li, lj)
				explained = false
			}
		}
	}
	if divergent != unmatched {
		t.Errorf("case %s divergent %d != unmatched %d",
			key, divergent, unmatched)
		return false
	}
	return explained
}

// TestOrbEngineGolden replays the Swift library's golden
// vectors: 18 resolved (design, size) entries and 72 frames.
func TestOrbEngineGolden(t *testing.T) {
	golden, ok := loadOrbGoldenFile(t)
	if !ok {
		return
	}
	tol := golden.Tolerance
	if !(tol > 0) {
		tol = 1e-4
	}

	// Resolved table: mode, speed and every opt to 1e-12.
	designs := []ThinkingOrbDesign{
		ThinkingOrbWorking, ThinkingOrbSearching, ThinkingOrbSolving,
		ThinkingOrbListening, ThinkingOrbConnecting, ThinkingOrbWeaving,
		ThinkingOrbComposing, ThinkingOrbBreathing, ThinkingOrbShaping,
	}
	sizes := []ThinkingOrbSize{ThinkingOrbRegular, ThinkingOrbSmall}
	matchedResolved := 0
	for _, design := range designs {
		for _, size := range sizes {
			key := strings.ToLower(
				fmt.Sprintf("%s-%d", design.Title(), int(size.Length())))
			want, found := golden.Resolved[key]
			if !found {
				t.Errorf("resolved entry %q missing from golden file", key)
				continue
			}
			got := orbResolve(design, size)
			if got.mode.String() != want.Mode {
				t.Errorf("resolved %s mode = %s, want %s",
					key, got.mode.String(), want.Mode)
			}
			if math.Abs(got.speed-want.Speed) > 1e-12 {
				t.Errorf("resolved %s speed = %v, want %v",
					key, got.speed, want.Speed)
			}
			for k, wv := range want.Opts {
				gv, present := got.opts[k]
				if !present {
					t.Errorf("resolved %s opt %q missing", key, k)
					continue
				}
				if math.Abs(gv-wv) > 1e-12 {
					t.Errorf("resolved %s opt %s = %v, want %v",
						key, k, gv, wv)
				}
			}
			for k := range got.opts {
				if _, present := want.Opts[k]; !present {
					t.Errorf("resolved %s extra opt %q = %v",
						key, k, got.opts[k])
				}
			}
			matchedResolved++
		}
	}
	if matchedResolved != 18 {
		t.Errorf("resolved entries matched = %d, want 18", matchedResolved)
	}

	// Frames: counts plus values to the golden tolerance.
	if len(golden.Cases) != 72 {
		t.Errorf("golden cases = %d, want 72", len(golden.Cases))
	}
	matchedCases := 0
	flipCases := 0
	worstOverall := 0.0
	for _, c := range golden.Cases {
		design, validState := orbDesignForState(c.State)
		if !validState {
			t.Errorf("case %s has unknown state %q", c.Key, c.State)
			continue
		}
		var size ThinkingOrbSize
		switch c.Size {
		case 64:
			size = ThinkingOrbRegular
		case 20:
			size = ThinkingOrbSmall
		default:
			t.Errorf("case %s has unknown size %d", c.Key, c.Size)
			continue
		}
		fr := orbFrame(design, size, c.T)
		if len(fr.dots) != c.DotCount {
			t.Errorf("case %s dots = %d, want %d",
				c.Key, len(fr.dots), c.DotCount)
			continue
		}
		if len(fr.lines) != c.LineCount {
			t.Errorf("case %s lines = %d, want %d",
				c.Key, len(fr.lines), c.LineCount)
			continue
		}
		wantDots := orbUnpackDots(c.Dots)
		if len(wantDots) != c.DotCount {
			t.Errorf("case %s golden dots unpack to %d, want %d",
				c.Key, len(wantDots), c.DotCount)
			continue
		}
		worst, unmatched := orbMatchDots(wantDots, fr.dots, tol)
		isFlip := false
		if unmatched > 0 {
			if !orbGoldenSlabFlips[c.Key] {
				t.Errorf("case %s dots: %d of %d have no match within %g",
					c.Key, unmatched, len(wantDots), tol)
				continue
			}
			if !orbVerifySlabFlips(t, c.Key, size, c.T, wantDots, fr, tol, unmatched) {
				continue
			}
			isFlip = true
		}
		wantLines := orbUnpackLines(c.Lines)
		linesOK := true
		for li := range wantLines {
			diffs := []float64{
				math.Abs(wantLines[li].x1 - fr.lines[li].x1),
				math.Abs(wantLines[li].y1 - fr.lines[li].y1),
				math.Abs(wantLines[li].x2 - fr.lines[li].x2),
				math.Abs(wantLines[li].y2 - fr.lines[li].y2),
				math.Abs(wantLines[li].white - fr.lines[li].white),
				math.Abs(wantLines[li].a - fr.lines[li].a),
				math.Abs(wantLines[li].w - fr.lines[li].w),
			}
			for _, dd := range diffs {
				if dd > worst {
					worst = dd
				}
				if dd > tol {
					t.Errorf("case %s line %d diff %g over tol %g",
						c.Key, li, dd, tol)
					linesOK = false
					break
				}
			}
			if !linesOK {
				break
			}
		}
		if !linesOK {
			continue
		}
		if worst > worstOverall {
			worstOverall = worst
		}
		if isFlip {
			flipCases++
		} else {
			matchedCases++
		}
	}
	t.Logf("golden: %d/18 resolved, %d strict + %d slab-flip / 72 cases,"+
		" worst abs diff %g (tol %g)",
		matchedResolved, matchedCases, flipCases, worstOverall, tol)
	if matchedCases+flipCases != len(golden.Cases) {
		t.Errorf("cases matched = %d, want %d",
			matchedCases+flipCases, len(golden.Cases))
	}
}

// orbFramesEqual reports exact bitwise equality of two frames.
func orbFramesEqual(a, b orbFrameResult) bool {
	if len(a.dots) != len(b.dots) || len(a.lines) != len(b.lines) {
		return false
	}
	for i := range a.dots {
		if a.dots[i] != b.dots[i] {
			return false
		}
	}
	for i := range a.lines {
		if a.lines[i] != b.lines[i] {
			return false
		}
	}
	return true
}

// TestOrbFrameDeterministic replays fixed inputs twice and
// demands identical outputs.
func TestOrbFrameDeterministic(t *testing.T) {
	designs := []ThinkingOrbDesign{
		ThinkingOrbWorking, ThinkingOrbSolving,
		ThinkingOrbConnecting, ThinkingOrbShaping,
	}
	sizes := []ThinkingOrbSize{ThinkingOrbRegular, ThinkingOrbSmall}
	times := []float64{0.6, 5.1}
	for _, design := range designs {
		for _, size := range sizes {
			for _, tt := range times {
				first := orbFrame(design, size, tt)
				second := orbFrame(design, size, tt)
				if !orbFramesEqual(first, second) {
					t.Errorf("design %d size %d t %v not deterministic",
						design, size, tt)
				}
			}
		}
	}
}

// TestOrbFrameFinite sweeps every design and size and demands
// finite geometry with sane radii and alphas. The grid is dense
// (every 0.05 geometry-seconds over a full cycle, plus negative
// times) so a transient NaN at one phase cannot hide between
// samples.
func TestOrbFrameFinite(t *testing.T) {
	var times []float64
	times = append(times, -2.5, -0.01)
	for tt := 0.0; tt <= orbCycleLen; tt += 0.05 {
		times = append(times, tt)
	}
	for design := ThinkingOrbWorking; design <= ThinkingOrbShaping; design++ {
		for _, size := range []ThinkingOrbSize{
			ThinkingOrbRegular, ThinkingOrbSmall,
		} {
			for _, tt := range times {
				fr := orbFrame(design, size, tt)
				for i := range fr.dots {
					d := fr.dots[i]
					for _, v := range []float64{
						d.x, d.y, d.z, d.r, d.white, d.a,
					} {
						if math.IsNaN(v) || math.IsInf(v, 0) {
							t.Fatalf("design %d size %d t %v dot %d non-finite",
								design, size, tt, i)
						}
					}
					if d.r < 0 {
						t.Errorf("design %d size %d t %v dot %d r %v < 0",
							design, size, tt, i, d.r)
					}
					// The engine never clamps alpha above: braid
					// strands peak at ~1.0148 in the golden
					// vectors, so the ceiling is a sanity
					// bound, not [0, 1].
					if d.a < 0 || d.a > 1.05 {
						t.Errorf("design %d size %d t %v dot %d a %v outside [0,1.05]",
							design, size, tt, i, d.a)
					}
				}
				for i := range fr.lines {
					l := fr.lines[i]
					for _, v := range []float64{
						l.x1, l.y1, l.x2, l.y2, l.white, l.a, l.w,
					} {
						if math.IsNaN(v) || math.IsInf(v, 0) {
							t.Fatalf("design %d size %d t %v line %d non-finite",
								design, size, tt, i)
						}
					}
					if l.a < 0 || l.a > 1.05 {
						t.Errorf("design %d size %d t %v line %d a %v outside [0,1.05]",
							design, size, tt, i, l.a)
					}
				}
			}
		}
	}
}

// TestOrbInvalidDesignClamps demands an out-of-range design
// draw exactly like working.
func TestOrbInvalidDesignClamps(t *testing.T) {
	for _, bad := range []ThinkingOrbDesign{9, 99, 255} {
		got := orbFrame(bad, ThinkingOrbRegular, 1.0)
		want := orbFrame(ThinkingOrbWorking, ThinkingOrbRegular, 1.0)
		if !orbFramesEqual(got, want) {
			t.Errorf("design %d did not clamp to working", bad)
		}
	}
}

// TestOrbInvalidSizeClamps demands an out-of-range size draw
// exactly like regular.
func TestOrbInvalidSizeClamps(t *testing.T) {
	for _, bad := range []ThinkingOrbSize{7, 255} {
		got := orbFrame(ThinkingOrbSearching, bad, 1.0)
		want := orbFrame(ThinkingOrbSearching, ThinkingOrbRegular, 1.0)
		if !orbFramesEqual(got, want) {
			t.Errorf("size %d did not clamp to regular", bad)
		}
	}
}

// TestOrbFrameNonFiniteT demands a non-finite time draw time
// zero rather than poison the frame.
func TestOrbFrameNonFiniteT(t *testing.T) {
	want := orbFrame(ThinkingOrbWeaving, ThinkingOrbSmall, 0)
	for _, bad := range []float64{
		math.NaN(), math.Inf(1), math.Inf(-1),
	} {
		if got := orbFrame(ThinkingOrbWeaving, ThinkingOrbSmall, bad); !orbFramesEqual(got, want) {
			t.Errorf("t %v did not draw time zero", bad)
		}
	}
}

// TestThinkingOrbA11YLabels pins the VoiceOver strings:
// breathing idles, every other design names its state, and an
// invalid design falls back to working.
func TestThinkingOrbA11YLabels(t *testing.T) {
	if got := ThinkingOrbBreathing.A11YLabel(); got != "Thinking…" {
		t.Errorf("breathing label = %q, want Thinking…", got)
	}
	if got := ThinkingOrbSearching.A11YLabel(); got != "Searching…" {
		t.Errorf("searching label = %q, want Searching…", got)
	}
	if got := ThinkingOrbDesign(99).A11YLabel(); got != "Working…" {
		t.Errorf("invalid label = %q, want Working…", got)
	}
}
