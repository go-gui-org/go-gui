package svg

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

type parserCacheEntry struct {
	parsed *gui.SvgParsed
	vg     *vectorGraphic
	// sourceKey identifies the user-visible source independent of
	// parse options: "inline:<data>" for inline SVGs or
	// "file:<absPath>" for file-backed sources. InvalidateSvgSource
	// walks entries and drops every option-variant whose sourceKey
	// matches the request.
	sourceKey string
}

// ReleaseParsed drops parser-side references for a parsed SVG once
// callers are done tessellating it.
func (p *Parser) releaseParsed(parsed *gui.SvgParsed) {
	if parsed == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	hash, ok := p.byParsed[parsed]
	if ok {
		delete(p.byParsed, parsed)
		delete(p.byHash, hash)
		p.removeHashFromOrder(hash)
	}
}

// InvalidateSvgSource invalidates parser cache for one SVG source.
// All option-variants (PrefersReducedMotion, FlatnessTolerance,
// HoveredElementID, FocusedElementID, …) and — for file sources — all
// content variants are dropped together since the user-visible
// identity is the source string itself, not a particular content
// snapshot or option permutation.
//
// Implementation walks the entry table by sourceKey rather than
// reconstructing hashes. Reconstruction is unsafe: file entries hash
// the file contents and the option struct into the cache key, neither
// of which the caller knows at invalidation time.
func (p *Parser) InvalidateSvgSource(svgSrc string) {
	if svgSrc == "" {
		return
	}
	keys := candidateSourceKeys(svgSrc)
	if len(keys) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for hash, entry := range p.byHash {
		if !slices.Contains(keys, entry.sourceKey) {
			continue
		}
		delete(p.byParsed, entry.parsed)
		delete(p.byHash, hash)
		p.removeHashFromOrder(hash)
	}
}

// candidateSourceKeys enumerates every sourceKey form that could have
// been stored for svgSrc: the literal inline form, plus the
// abs-path/resolved-symlink forms used by fileSourceKey.
func candidateSourceKeys(svgSrc string) []string {
	if svgSrc == "" {
		return nil
	}
	if strings.HasPrefix(svgSrc, "<") {
		return []string{inlineSourceKey(svgSrc)}
	}
	clean := filepath.Clean(svgSrc)
	abs, absErr := filepath.Abs(clean)
	if absErr != nil {
		return []string{"file:" + clean}
	}
	resolved, resErr := filepath.EvalSymlinks(abs)
	if resErr != nil {
		return []string{"file:" + clean, "file:" + abs}
	}
	return []string{"file:" + clean, "file:" + abs, "file:" + resolved}
}

// ClearSvgParserCache removes all retained parsed entries.
func (p *Parser) ClearSvgParserCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	clear(p.byHash)
	clear(p.byParsed)
	p.order = nil
}

func (p *Parser) buildParsed(hash uint64, sourceKey string, vg *vectorGraphic, scale float32) *gui.SvgParsed {
	resolveAnimationTargets(vg)
	tpaths := vg.getTriangles(scale)
	result := &gui.SvgParsed{
		Paths:          tpaths,
		Texts:          vg.Texts,
		TextPaths:      vg.TextPaths,
		DefsPaths:      vg.DefsPaths,
		Gradients:      vg.Gradients,
		FilteredGroups: tessellateFilteredGroups(vg, scale),
		Animations:     vg.Animations,
		Width:          vg.Width,
		Height:         vg.Height,
		ViewBoxX:       vg.ViewBoxX,
		ViewBoxY:       vg.ViewBoxY,
		A11y:           vg.A11y,
		PreserveAlign:  vg.PreserveAlign,
		PreserveSlice:  vg.PreserveSlice,
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.byHash[hash] = parserCacheEntry{parsed: result, vg: vg, sourceKey: sourceKey}
	p.byParsed[result] = hash
	p.order = append(p.order, hash)
	if len(p.order) > maxParsedRetained {
		evictHash := p.order[0]
		p.order = p.order[1:]
		if len(p.order) < cap(p.order)/2 {
			p.order = append([]uint64(nil), p.order...)
		}
		if entry, ok := p.byHash[evictHash]; ok {
			delete(p.byParsed, entry.parsed)
			delete(p.byHash, evictHash)
		}
	}
	return result
}

func (p *Parser) cachedParsed(hash uint64) *gui.SvgParsed {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.byHash[hash]
	if !ok {
		return nil
	}
	p.touchHash(hash)
	return entry.parsed
}

func (p *Parser) touchHash(hash uint64) {
	for i := range slices.Backward(p.order) {
		if p.order[i] == hash {
			copy(p.order[i:], p.order[i+1:])
			p.order[len(p.order)-1] = hash
			return
		}
	}
	p.order = append(p.order, hash)
}

func (p *Parser) removeHashFromOrder(hash uint64) {
	for i := range p.order {
		if p.order[i] == hash {
			p.order = append(p.order[:i], p.order[i+1:]...)
			return
		}
	}
}

func parserSourceHash(src string, inline bool) uint64 {
	const (
		fnvOffset = uint64(0xcbf29ce484222325)
		fnvPrime  = uint64(0x100000001b3)
	)
	h := fnvOffset
	prefix := "file:"
	if inline {
		prefix = "inline:"
	}
	for i := range len(prefix) {
		h ^= uint64(prefix[i])
		h *= fnvPrime
	}
	for i := range len(src) {
		h ^= uint64(src[i])
		h *= fnvPrime
	}
	return h
}

func parserFileHash(path string, data []byte) uint64 {
	h := parserSourceHash(string(data), true)
	clean := filepath.Clean(path)
	if abs, err := filepath.Abs(clean); err == nil {
		return mixHash(h, abs)
	}
	return mixHash(h, clean)
}

func mixHash(h uint64, extra string) uint64 {
	const fnvPrime = uint64(0x100000001b3)
	for i := range len(extra) {
		h ^= uint64(extra[i])
		h *= fnvPrime
	}
	return h
}

// maxGroupParentDepth caps GroupParent ancestor walks. Author-id
// collisions could in theory form a cycle; combined with the per-walk
// visited set this guards against pathological inputs.
const maxGroupParentDepth = 64
