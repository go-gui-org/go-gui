package svg

import (
	_ "embed"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Embed a representative subset of spinners: pure-rotate, dash anim,
// group-inherited replace, motion path. Keeping the set small (and
// self-contained in this file) avoids depending on the gui package's
// embed directives and sidesteps the test-time import cycle.
const (
	benchSvgBarsRotateFade = `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><g><rect x="11" y="1" width="2" height="5" opacity=".14"/><rect x="11" y="1" width="2" height="5" transform="rotate(30 12 12)" opacity=".29"/><rect x="11" y="1" width="2" height="5" transform="rotate(60 12 12)" opacity=".43"/><rect x="11" y="1" width="2" height="5" transform="rotate(90 12 12)" opacity=".57"/><rect x="11" y="1" width="2" height="5" transform="rotate(120 12 12)" opacity=".71"/><rect x="11" y="1" width="2" height="5" transform="rotate(150 12 12)" opacity=".86"/><rect x="11" y="1" width="2" height="5" transform="rotate(180 12 12)"/><animateTransform attributeName="transform" type="rotate" calcMode="discrete" dur="0.75s" values="0 12 12;30 12 12;60 12 12;90 12 12;120 12 12;150 12 12;180 12 12;210 12 12;240 12 12;270 12 12;300 12 12;330 12 12;360 12 12" repeatCount="indefinite"/></g></svg>`

	benchSvg3DotsFade = `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><circle cx="4" cy="12" r="3"><animate id="a" begin="0;b.end-0.25s" attributeName="opacity" dur="0.75s" values="1;.2" fill="freeze"/></circle><circle cx="12" cy="12" r="3" opacity=".4"><animate begin="a.end-0.6s" attributeName="opacity" dur="0.75s" values="1;.2" fill="freeze"/></circle><circle cx="20" cy="12" r="3" opacity=".3"><animate id="b" begin="a.end-0.45s" attributeName="opacity" dur="0.75s" values="1;.2" fill="freeze"/></circle></svg>`

	benchSvgRingAnimR = `<svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><circle cx="12" cy="12" r="0" stroke="black" fill="none" stroke-width="2"><animate attributeName="r" dur="1.5s" values="0;11" repeatCount="indefinite"/></circle></svg>`
)

// BenchmarkTessellateAnimatedPerFrame exercises the per-frame attr-
// override tessellation path over a representative set of spinners.
// Post-Stage-3 the override map is keyed by PathID; this bench
// should report zero or near-zero allocs per iter when override
// shapes haven't changed (clone-into-scratch). Baseline captured
// before optimizing further.
func BenchmarkTessellateAnimatedPerFrame(b *testing.B) {
	sources := []string{
		benchSvgBarsRotateFade,
		benchSvg3DotsFade,
		benchSvgRingAnimR,
	}
	p := New()
	parseds := make([]*gui.SvgParsed, len(sources))
	for i, src := range sources {
		parsed, err := p.ParseSvg(src)
		if err != nil {
			b.Fatalf("parse[%d]: %v", i, err)
		}
		parseds[i] = parsed
	}

	// Synthetic override: pretend r is being animated on every path.
	overrides := map[uint32]gui.SvgAnimAttrOverride{}
	for _, parsed := range parseds {
		for i := range parsed.Paths {
			pid := parsed.Paths[i].PathID
			if pid == 0 {
				continue
			}
			overrides[pid] = gui.SvgAnimAttrOverride{
				Mask: gui.SvgAnimMaskR,
				R:    5,
			}
		}
	}

	var scratch []gui.TessellatedPath
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		for _, parsed := range parseds {
			scratch = p.TessellateAnimated(parsed, 1.0, overrides, scratch)
		}
	}
}

// BenchmarkParseSvgGallery parses a representative spinner set from
// cold. Catches regressions in the encoding/xml-based tree decoder
// introduced in Stage 2a.
func BenchmarkParseSvgGallery(b *testing.B) {
	sources := []string{
		benchSvgBarsRotateFade,
		benchSvg3DotsFade,
		benchSvgRingAnimR,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		p := New()
		for _, src := range sources {
			if _, err := p.ParseSvg(src); err != nil {
				b.Fatalf("parse: %v", err)
			}
		}
	}
}
