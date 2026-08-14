package svg

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"math"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

// svgTrace enables per-frame diagnostic logging of animated SVG
// primitive overrides and geometry bounds. Set GOGUI_SVG_TRACE=1 to
// activate. Off by default; evaluated once per process.
var svgTrace = os.Getenv("GOGUI_SVG_TRACE") == "1"

// Parser implements gui.SvgParser.
// exportaudit:keep — reachable from an exported signature
type Parser struct {
	animatedScratch sync.Pool
	byHash          map[uint64]parserCacheEntry
	byParsed        map[*gui.SvgParsed]uint64
	order           []uint64
	mu              sync.Mutex
}

const maxParsedRetained = 512

// maxAnimatedScratchCap bounds both the pool retention and the seed
// capacity in getAnimatedScratch. A hostile SVG with millions of
// synthesized animated paths would otherwise pin a giant backing
// array in the pool and force one giant make() per frame.
const maxAnimatedScratchCap = 4096

// New returns a new SVG parser.
func New() *Parser {
	return &Parser{
		byHash:   make(map[uint64]parserCacheEntry),
		byParsed: make(map[*gui.SvgParsed]uint64),
	}
}

// ParseSvg parses SVG string data.
func (p *Parser) ParseSvg(data string) (*gui.SvgParsed, error) {
	return p.ParseSvgWithOpts(data, gui.SvgParseOpts{})
}

// ParseSvgFile loads and parses an SVG file.
func (p *Parser) ParseSvgFile(path string) (*gui.SvgParsed, error) {
	return p.ParseSvgFileWithOpts(path, gui.SvgParseOpts{})
}

// ParseSvgWithOpts parses SVG string data, snapshotting environment
// flags (e.g. prefers-reduced-motion) into the cascade. Implements
// gui.SvgParserWithOpts.
func (p *Parser) ParseSvgWithOpts(
	data string, opts gui.SvgParseOpts,
) (*gui.SvgParsed, error) {
	hash := parserSourceHashWithOpts(data, true, opts)
	if parsed := p.cachedParsed(hash); parsed != nil {
		return parsed, nil
	}
	vg, err := parseSvgWith(data, optsToSvg(opts))
	if err != nil {
		return nil, err
	}
	return p.buildParsed(hash, inlineSourceKey(data), vg, 1), nil
}

// inlineSourceKey is the cache identity for an inline SVG. Hashes the
// data so retained sourceKeys are O(1) per entry instead of O(len(data)).
// Without this, 512 cache entries × 4 MB parse cap = 2 GB of source-string
// retention from sourceKey alone (hostile-caller DoS surface).
// candidateSourceKeys hashes the same way so InvalidateSvgSource matches.
func inlineSourceKey(data string) string {
	h := sha256.New()
	// sha256.digest.Write never errors — discard explicitly for the
	// linter and to avoid the []byte(data) copy that Sum256 would do.
	_, _ = io.WriteString(h, data)
	var sum [sha256.Size]byte
	h.Sum(sum[:0])
	var buf [7 + sha256.Size*2]byte
	copy(buf[:7], "inline:")
	hex.Encode(buf[7:], sum[:])
	return string(buf[:])
}

// ParseSvgFileWithOpts loads and parses an SVG file with options.
// Implements gui.SvgParserWithOpts.
func (p *Parser) ParseSvgFileWithOpts(
	path string, opts gui.SvgParseOpts,
) (*gui.SvgParsed, error) {
	fileData, err := loadSvgFile(path)
	if err != nil {
		return nil, err
	}
	hash := mixOptsHash(parserFileHash(path, fileData), opts)
	if parsed := p.cachedParsed(hash); parsed != nil {
		return parsed, nil
	}
	vg, err := parseSvgWith(string(fileData), optsToSvg(opts))
	if err != nil {
		return nil, err
	}
	return p.buildParsed(hash, fileSourceKey(path), vg, 1), nil
}

// fileSourceKey returns the canonical "file:<absPath>" identity used
// for cache invalidation. Resolves symlinks so two parses of the
// same underlying file via different alias paths share a sourceKey
// — matching the resolution candidateSourceKeys performs at
// invalidation time. Falls back to the abs (or cleaned) form when
// resolution fails.
func fileSourceKey(path string) string {
	clean := filepath.Clean(path)
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "file:" + clean
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return "file:" + resolved
	}
	return "file:" + abs
}

func optsToSvg(o gui.SvgParseOpts) parseOptions {
	return parseOptions{
		PrefersReducedMotion: o.PrefersReducedMotion,
		FlatnessTolerance:    o.FlatnessTolerance,
		HoveredElementID:     o.HoveredElementID,
		FocusedElementID:     o.FocusedElementID,
	}
}

func parserSourceHashWithOpts(
	src string, inline bool, opts gui.SvgParseOpts,
) uint64 {
	return mixOptsHash(parserSourceHash(src, inline), opts)
}

func mixOptsHash(h uint64, opts gui.SvgParseOpts) uint64 {
	const fnvPrime = uint64(0x100000001b3)
	var b byte
	if opts.PrefersReducedMotion {
		b = 1
	}
	h ^= uint64(b)
	h *= fnvPrime
	// FlatnessTolerance: hash bit pattern so cache invalidates on change.
	bits := math.Float32bits(opts.FlatnessTolerance)
	h ^= uint64(bits)
	h *= fnvPrime
	hovered := clampElementID(opts.HoveredElementID)
	for i := range len(hovered) {
		h ^= uint64(hovered[i])
		h *= fnvPrime
	}
	h ^= uint64('|')
	h *= fnvPrime
	focused := clampElementID(opts.FocusedElementID)
	for i := range len(focused) {
		h ^= uint64(focused[i])
		h *= fnvPrime
	}
	return h
}

// ParseSvgDimensions extracts width/height without full parse.
func (p *Parser) ParseSvgDimensions(data string) (float32, float32, error) {
	w, h := parseSvgDimensions(data)
	return w, h, nil
}

// Tessellate re-tessellates at a new scale. Also updates
// parsed.FilteredGroups with re-tessellated filter group paths.
func (p *Parser) Tessellate(parsed *gui.SvgParsed, scale float32) []gui.TessellatedPath {
	p.mu.Lock()
	hash, ok := p.byParsed[parsed]
	var entry parserCacheEntry
	if ok {
		entry, ok = p.byHash[hash]
	}
	p.mu.Unlock()
	if !ok {
		return nil
	}
	vg := entry.vg
	parsed.FilteredGroups = tessellateFilteredGroups(vg, scale)
	return vg.getTriangles(scale)
}

// resolveAnimationTargets populates each SvgAnimation.TargetPathIDs
// by scanning vg.Paths (and filtered-group paths) for VectorPaths
// whose GroupID matches the animation's binding. A single shape-id
// match yields one PathID; a group-id match yields every descendant
// primitive's PathID, enabling per-path animation routing without
// sibling-collision collapses.
func resolveAnimationTargets(vg *vectorGraphic) {
	if len(vg.Animations) == 0 {
		return
	}
	byGroup := make(map[string][]uint32, len(vg.groupParent)+8)
	visited := make(map[string]struct{}, 8)
	collect := func(paths []vectorPath) {
		for i := range paths {
			p := &paths[i]
			if p.GroupID == "" || p.PathID == 0 {
				continue
			}
			gid := p.GroupID
			clear(visited)
			for depth := 0; gid != "" && depth < maxGroupParentDepth; depth++ {
				if _, seen := visited[gid]; seen {
					break
				}
				visited[gid] = struct{}{}
				byGroup[gid] = append(byGroup[gid], p.PathID)
				parent, ok := vg.groupParent[gid]
				if !ok || parent == gid {
					break
				}
				gid = parent
			}
		}
	}
	collect(vg.Paths)
	for i := range vg.FilteredGroups {
		collect(vg.FilteredGroups[i].Paths)
	}
	for i := range vg.Animations {
		a := &vg.Animations[i]
		if a.GroupID == "" {
			continue
		}
		a.TargetPathIDs = byGroup[a.GroupID]
	}
}

func tessellateFilteredGroups(vg *vectorGraphic, scale float32) []gui.SvgParsedFilteredGroup {
	if len(vg.FilteredGroups) == 0 {
		return nil
	}
	groups := make([]gui.SvgParsedFilteredGroup, 0, len(vg.FilteredGroups))
	for _, fg := range vg.FilteredGroups {
		filter := vg.Filters[fg.FilterID]
		tpaths := vg.tessellatePaths(fg.Paths, scale)
		groups = append(groups, gui.SvgParsedFilteredGroup{
			Filter:    filter,
			Paths:     tpaths,
			Texts:     fg.Texts,
			TextPaths: fg.TextPaths,
		})
	}
	return groups
}
