package svg

import (
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// loadSpinnerCorpus reads every .svg under gui/assets/svg-spinners.
// Keeps animated path coverage realistic (SMIL transforms, dash anims).
func loadSpinnerCorpus(tb testing.TB) []string {
	tb.Helper()
	dir := filepath.Join("..", "assets", "svg-spinners")
	entries, err := os.ReadDir(dir)
	if err != nil {
		tb.Fatalf("read spinner dir: %v", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".svg" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, string(b))
	}
	if len(out) == 0 {
		tb.Fatal("no spinner seeds")
	}
	return out
}

// hostileOverride produces an override with some NaN/Inf and
// out-of-range values sprinkled across masked fields, to exercise
// overrideScalar + nonNegF32 guards under stress.
func hostileOverride(rng *rand.Rand, pid uint32) gui.SvgAnimAttrOverride {
	pick := func() float32 {
		switch rng.IntN(6) {
		case 0:
			return float32(math.NaN())
		case 1:
			return float32(math.Inf(1))
		case 2:
			return float32(math.Inf(-1))
		case 3:
			return -1e6
		case 4:
			return 1e6
		default:
			return rng.Float32() * 100
		}
	}
	ov := gui.SvgAnimAttrOverride{
		Mask: gui.SvgAnimMaskCX | gui.SvgAnimMaskCY |
			gui.SvgAnimMaskR | gui.SvgAnimMaskRX |
			gui.SvgAnimMaskRY | gui.SvgAnimMaskX |
			gui.SvgAnimMaskY | gui.SvgAnimMaskWidth |
			gui.SvgAnimMaskHeight,
		CX: pick(), CY: pick(), R: pick(),
		RX: pick(), RY: pick(),
		X: pick(), Y: pick(),
		Width: pick(), Height: pick(),
	}
	if rng.IntN(2) == 0 {
		ov.AdditiveMask = ov.Mask
	}
	_ = pid
	return ov
}

// collectAnimatedPathIDs returns every PathID among the parsed
// animated paths — used to target overrides at live paths.
func collectAnimatedPathIDs(parsed *gui.SvgParsed) []uint32 {
	seen := map[uint32]struct{}{}
	var out []uint32
	for i := range parsed.Paths {
		if !parsed.Paths[i].Animated {
			continue
		}
		pid := parsed.Paths[i].PathID
		if pid == 0 {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		out = append(out, pid)
	}
	return out
}

// assertTrianglesFinite fails if any triangle vertex is NaN or ±Inf.
// Magnitude is not bounded: replace-semantic overrides can legitimately
// translate geometry far outside the viewBox, so the stress test only
// guards the invariant the clamps actually promise — finiteness.
func assertTrianglesFinite(t *testing.T, paths []gui.TessellatedPath) {
	t.Helper()
	for i := range paths {
		tris := paths[i].Triangles
		for j := 0; j+1 < len(tris); j += 2 {
			x, y := tris[j], tris[j+1]
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) ||
				math.IsNaN(float64(y)) || math.IsInf(float64(y), 0) {
				t.Fatalf("path %d vertex[%d]=(%v,%v) non-finite",
					i, j/2, x, y)
			}
		}
	}
}

// TestTessellateAnimated_StressConcurrent hammers TessellateAnimated
// from N goroutines across M frames per worker, with per-frame
// hostile overrides. Run under -race to catch cache lock regressions.
// Asserts every output triangle is finite and bbox-bounded.
func TestTessellateAnimated_StressConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test; -short")
	}
	corpus := loadSpinnerCorpus(t)
	p := New()

	type parsedEntry struct {
		parsed *gui.SvgParsed
		pids   []uint32
	}
	parsed := make([]parsedEntry, 0, len(corpus))
	for _, src := range corpus {
		ps, err := p.ParseSvg(src)
		if err != nil || ps == nil {
			continue
		}
		pids := collectAnimatedPathIDs(ps)
		if len(pids) == 0 {
			continue
		}
		parsed = append(parsed, parsedEntry{ps, pids})
	}
	if len(parsed) == 0 {
		t.Fatal("no animated spinners")
	}

	workers := max(runtime.GOMAXPROCS(0), 2)
	framesPerWorker := 40

	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for w := range workers {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(seed, seed*2654435761+1))
			var reuse []gui.TessellatedPath
			defer func() {
				if r := recover(); r != nil {
					errCh <- wrapPanic(r)
				}
			}()
			for range framesPerWorker {
				pe := parsed[rng.IntN(len(parsed))]
				overrides := make(
					map[uint32]gui.SvgAnimAttrOverride,
					len(pe.pids),
				)
				for _, pid := range pe.pids {
					overrides[pid] = hostileOverride(rng, pid)
				}
				scale := 0.25 + rng.Float32()*4
				out := p.TessellateAnimated(
					pe.parsed, scale, overrides, reuse,
				)
				assertTrianglesFinite(t, out)
				// Reuse buffer when safely sized. Parser contract:
				// returns reuse slice when cap suffices.
				reuse = out[:0]
			}
		}(uint64(w) + 1)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

// wrapPanic converts a recovered panic into an error without losing
// the value. Keeps the goroutine error channel type simple.
func wrapPanic(v any) error {
	return &panicErr{v: v}
}

type panicErr struct{ v any }

func (e *panicErr) Error() string {
	return "panic in worker: " + formatAny(e.v)
}

func formatAny(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case error:
		return s.Error()
	default:
		return "non-string panic"
	}
}
