package gui

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	glyph "github.com/go-gui-org/go-glyph"
)

// maxSvgCacheElementIDLen caps pseudo-state IDs participating in the
// SVG cache key. Same bound as svg/xml.go's maxElementIDLen — kept as
// a separate gui-package constant so the public LoadSvgWithOpts
// surface can reject hostile inputs without importing the internal
// svg package.
const maxSvgCacheElementIDLen = 256

const maxSvgSourceBytes = int64(4 * 1024 * 1024)

// svgCacheMaxMemory is the soft memory budget for the SVG render
// cache. When inserting a new entry would push the total estimated
// memory above this limit, oldest entries are evicted first
// (FIFO). The budget covers vertex and color data across all cached
// entries; a single entry exceeding the budget still enters the
// cache — the budget is a target, not a hard cap.
const svgCacheMaxMemory = 128 * 1024 * 1024 // 128 MB

type svgParserCacheInvalidator interface {
	InvalidateSvgSource(svgSrc string)
	ClearSvgParserCache()
}

// CachedSvgPath holds tessellated geometry with vertex colors. The
// MinX/MaxX/MinY/MaxY bbox carried by TessellatedPath is intentionally
// NOT mirrored here — ContainsPoint hit-testing operates on
// SvgParsed.Paths (TessellatedPath), not on render paths.
type cachedSvgPath struct {
	Triangles    []float32
	VertexColors []Color
	ClipGroup    int
	Primitive    SvgPrimitive
	PathID       uint32
	// Author's base transform, decomposed. Applied at render-time
	// when HasBaseXform is true. See TessellatedPath for details.
	BaseTransX   float32
	BaseTransY   float32
	BaseScaleX   float32
	BaseScaleY   float32
	BaseRotAngle float32
	BaseRotCX    float32
	BaseRotCY    float32
	Color        Color
	IsClipMask   bool
	Animated     bool
	// IsStroke marks the path as a stroke contribution; lets opacity
	// animations targeting fill-opacity / stroke-opacity scale only
	// the matching path.
	IsStroke     bool
	HasBaseXform bool
}

// CachedSvgTextDraw holds cached text rendering data.
type cachedSvgTextDraw struct {
	TextStyle TextStyle
	Gradient  *glyph.GradientConfig
	Text      string
	X, Y      float32
	TextWidth float32 // measured width including letter-spacing
}

// CachedSvgTextPathDraw holds precomputed textPath render data.
type cachedSvgTextPathDraw struct {
	Text      string
	TextStyle TextStyle
	Path      textPathData
}

// CachedFilteredGroup holds tessellated geometry for a filter group.
type cachedFilteredGroup struct {
	Gradients     map[string]SvgGradientDef
	renderPaths   []cachedSvgPath
	textDraws     []cachedSvgTextDraw
	textPathDraws []cachedSvgTextPathDraw
	Filter        SvgFilter
	bBox          [4]float32 // x, y, width, height
}

// svgBaseXform holds a decomposed author base transform, keyed by
// PathID. Used to seed svgAnimState at sandwich init so animations
// compose over the author's base.
type svgBaseXform struct {
	TransX, TransY float32
	ScaleX, ScaleY float32
	RotAngle       float32
	RotCX, RotCY   float32
}

// CachedSvg holds pre-tessellated SVG data for efficient rendering.
type CachedSvg struct {
	Gradients map[string]SvgGradientDef
	Parsed    *SvgParsed // retained for TessellateAnimated
	// BaseByPath maps PathID → decomposed author base transform.
	// Populated only for paths that have animations targeting them
	// AND whose base transform decomposed cleanly; used to seed
	// svgAnimState so animations compose over the author's base.
	baseByPath     map[uint32]svgBaseXform
	defsPathData   map[string]cachedDefsPathData
	animHash       string
	renderPaths    []cachedSvgPath
	textDraws      []cachedSvgTextDraw
	textPathDraws  []cachedSvgTextPathDraw
	FilteredGroups []cachedFilteredGroup
	Animations     []SvgAnimation
	animStartNs    int64
	Width          float32
	Height         float32
	Scale          float32
	// ViewBoxX / ViewBoxY are the authored viewBox origin. Applied at
	// render time as an outer translate on sx/sy so authored coords
	// stay in raw viewBox space throughout tessellation and animation.
	ViewBoxX         float32
	ViewBoxY         float32
	hasAnimations    bool
	hasAttrAnim      bool // any SvgAnimAttr present → try re-tessellation
	hasAnimatedPaths bool // any RenderPath has Animated=true
	// PreserveAlign / PreserveSlice mirror the parsed SVG's
	// preserveAspectRatio attribute so renderSvg can offset content
	// without re-loading the parser. PreserveSlice also drives the
	// scale picked above (max for slice, min for meet).
	PreserveAlign SvgAlign
	PreserveSlice bool
}

type svgCacheKey struct {
	// hoveredID / focusedID feed CSS :hover / :focus pseudo-class
	// matching. Cache invalidates on transition.
	hoveredID string
	focusedID string
	srcHash   uint64
	// contentHash marks the file state behind a file-backed
	// source. Size plus mtime is not a full content check, but
	// it catches each normal save while it costs one Stat call.
	// Inline sources set 0: srcHash already covers the content.
	contentHash uint64
	w10         int32
	h10         int32
	// flatness10000 is FlatnessTolerance × 10000 quantized into an
	// int. Zero (default) keeps fingerprint stable. Quantization
	// avoids float NaN/Inf collisions in map keys.
	flatness10000 int32
	// reducedMotion is the snapshotted prefers-reduced-motion flag.
	// Same SVG source rendered under different motion preferences
	// must cache separately so a user toggling the OS pref
	// invalidates the prior render naturally (Phase F).
	reducedMotion bool
}

// EstimateMemory returns a rough byte estimate for the cached SVG.
// Counts vertex data (float32 × 4 bytes), vertex colors, and a
// per-struct overhead for text draws, text paths, and animations.
func (c *CachedSvg) estimateMemory() int {
	n := 0
	for _, p := range c.renderPaths {
		n += len(p.Triangles) * 4    // float32 = 4 bytes
		n += len(p.VertexColors) * 4 // Color = 4 bytes (uint32)
	}
	for _, fg := range c.FilteredGroups {
		for _, p := range fg.renderPaths {
			n += len(p.Triangles) * 4
			n += len(p.VertexColors) * 4
		}
	}
	n += len(c.textDraws) * 384     // CachedSvgTextDraw + glyph.GradientConfig
	n += len(c.textPathDraws) * 640 // CachedSvgTextPathDraw + TextPathData
	n += len(c.Animations) * 256    // SvgAnimation
	n += len(c.Gradients) * 128     // SvgGradientDef
	return n
}

// validateSvgSource rejects file paths containing '..'.
func validateSvgSource(svgSrc string) error {
	return validateSvgSourceWithRoots(svgSrc, nil)
}

func validateSvgSourceWithRoots(svgSrc string, allowedRoots []string) error {
	if strings.HasPrefix(svgSrc, "<") {
		return nil
	}
	if strings.ContainsRune(svgSrc, 0) {
		return errors.New("invalid svg path: contains NUL")
	}
	cleanPath := filepath.Clean(svgSrc)
	if cleanPath == "." {
		return errors.New("invalid svg path")
	}
	for part := range strings.SplitSeq(filepath.ToSlash(cleanPath), "/") {
		if part == ".." {
			return errors.New("invalid svg path: contains parent directory reference")
		}
	}
	if ext := strings.ToLower(filepath.Ext(cleanPath)); ext != ".svg" {
		return fmt.Errorf("unsupported svg format: %s", ext)
	}
	if len(allowedRoots) > 0 {
		if err := validateSvgPathAllowed(cleanPath, allowedRoots); err != nil {
			return err
		}
	}
	return nil
}

func resolveValidatedSvgPath(svgSrc string, allowedRoots []string) (string, error) {
	if strings.HasPrefix(svgSrc, "<") {
		return svgSrc, nil
	}
	if err := validateSvgSourceWithRoots(svgSrc, allowedRoots); err != nil {
		return "", err
	}
	cleanPath := filepath.Clean(svgSrc)
	pathAbs, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid svg path: %w", err)
	}
	resolvedPath := resolvePathWithParentFallback(pathAbs)
	if len(allowedRoots) > 0 {
		if err := validateSvgPathAllowed(resolvedPath, allowedRoots); err != nil {
			return "", err
		}
	}
	return resolvedPath, nil
}

func validateSvgPathAllowed(cleanPath string, allowedRoots []string) error {
	pathAbs, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("invalid svg path: %w", err)
	}
	resolvedPath := resolvePathWithParentFallback(pathAbs)
	for i := range allowedRoots {
		root := strings.TrimSpace(allowedRoots[i])
		if root == "" {
			continue
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		resolvedRoot := resolvePathWithParentFallback(rootAbs)
		if pathWithinRoot(resolvedPath, resolvedRoot) {
			return nil
		}
	}
	return fmt.Errorf("svg path not allowed: %s", cleanPath)
}

func resolvePathWithParentFallback(path string) string {
	if p, err := filepath.EvalSymlinks(path); err == nil {
		return p
	}
	dir := filepath.Dir(path)
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		return filepath.Join(d, filepath.Base(path))
	}
	return path
}

func pathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// svgDimEntry pairs cached SVG dimensions with the file state
// they were read from. See svgCacheKey.contentHash.
type svgDimEntry struct {
	dims        [2]float32
	contentHash uint64
	// checkedAt is when contentHash was last confirmed against the
	// file. Within svgFileRecheckInterval of it, a load trusts
	// contentHash and makes no file system call.
	checkedAt time.Time
}

// svgFileRecheckInterval bounds how often a file-backed SVG is
// resolved and stat'ed to catch an edit. renderSvg and svgView
// load each visible SVG every frame, under the frame lock. A
// check per frame puts EvalSymlinks and two Stat calls (syscalls
// and allocations) in the render pass. An edit shows up within
// this interval instead.
const svgFileRecheckInterval = 500 * time.Millisecond

// freshSvgFingerprint returns the file fingerprint recorded for
// srcHash when it was confirmed less than svgFileRecheckInterval
// before now. It makes no file system call and no allocation, so
// a cache hit stays free.
func (w *Window) freshSvgFingerprint(srcHash uint64,
	now time.Time) (svgDimEntry, bool) {
	dc := StateMapRead[uint64, svgDimEntry](w, nsSvgDimCache)
	if dc == nil {
		return svgDimEntry{}, false
	}
	entry, ok := dc.Get(srcHash)
	if !ok || entry.checkedAt.IsZero() {
		return svgDimEntry{}, false
	}
	// A negative age means a checkedAt from the future: a restored
	// snapshot or a wall-clock jump with no monotonic reading.
	// Treat it as stale, or the file is never checked again.
	age := now.Sub(entry.checkedAt)
	if age < 0 || age >= svgFileRecheckInterval {
		return svgDimEntry{}, false
	}
	return entry, true
}

// confirmSvgFingerprint restarts the recheck interval for srcHash
// when the file still has the recorded contentHash. A changed file
// is left alone: its entry holds dims of the old content, and the
// parse that follows records the new state.
func (w *Window) confirmSvgFingerprint(srcHash, contentHash uint64,
	now time.Time) {
	dc := StateMapRead[uint64, svgDimEntry](w, nsSvgDimCache)
	if dc == nil {
		return
	}
	entry, ok := dc.Get(srcHash)
	if !ok || entry.contentHash != contentHash {
		return
	}
	entry.checkedAt = now
	dc.Set(srcHash, entry)
}

// restoreSvgFingerprint records the file state for srcHash after a
// slow-path render-cache hit, when the dim entry is missing or
// holds another contentHash. The dim cache and the render cache
// evict on their own, so a render entry can outlive its dim entry.
// With no entry, the fast path misses on every load, and each load
// then resolves and stats the file again. The dims come from the
// cached parse, which is the parse that the fingerprint matched.
func (w *Window) restoreSvgFingerprint(srcHash, contentHash uint64,
	cached *CachedSvg, now time.Time) {
	if cached == nil || cached.Parsed == nil {
		return
	}
	dc := StateMap[uint64, svgDimEntry](w, nsSvgDimCache, capModerate)
	if entry, ok := dc.Get(srcHash); ok && entry.contentHash == contentHash {
		return // confirmSvgFingerprint already refreshed it
	}
	dc.Set(srcHash, svgDimEntry{
		dims:        [2]float32{cached.Parsed.Width, cached.Parsed.Height},
		contentHash: contentHash,
		checkedAt:   now,
	})
}

// svgFileFingerprint reads the size and mtime of a resolved SVG
// file path into a cache key. A failed Stat returns 0, which
// disables the check for that load. The later parse then fails
// with the real error.
func svgFileFingerprint(resolvedSrc string) uint64 {
	info, err := os.Stat(resolvedSrc)
	if err != nil {
		return 0
	}
	mixed := fnvU64(Fnv64Offset, uint64(info.Size()))
	return fnvU64(mixed, uint64(info.ModTime().UnixNano()))
}

// readCappedSvgFile reads a resolved SVG file with a size cap.
// The cap applies at read time, so growth between the Stat probe
// in checkSvgSourceSize and this read cannot widen the buffer.
func readCappedSvgFile(resolvedSrc string) ([]byte, error) {
	// #nosec G304 — resolvedSrc validated through AllowedSvgRoots
	f, err := os.Open(resolvedSrc)
	if err != nil {
		return nil, fmt.Errorf("SVG not found: %s", resolvedSrc)
	}
	defer f.Close() //nolint:errcheck // read-only; no data in error
	data, err := io.ReadAll(io.LimitReader(f, maxSvgSourceBytes+1))
	if err != nil {
		return nil, fmt.Errorf("SVG not found: %s", resolvedSrc)
	}
	if int64(len(data)) > maxSvgSourceBytes {
		return nil, errors.New("SVG file too large")
	}
	return data, nil
}

// checkSvgSourceSize validates SVG source size.
func checkSvgSourceSize(svgSrc string) error {
	if strings.HasPrefix(svgSrc, "<") {
		if int64(len(svgSrc)) > maxSvgSourceBytes {
			return errors.New("SVG source too large")
		}
		return nil
	}
	info, err := os.Stat(svgSrc)
	if err != nil {
		return fmt.Errorf("SVG not found: %s", svgSrc)
	}
	if info.Size() > maxSvgSourceBytes {
		return errors.New("SVG file too large")
	}
	return nil
}

// svgParseOpts snapshots the SVG-relevant environment toggles from
// the window's NativePlatform via type-assertion adapters. Backends
// opt in by implementing PrefersReducedMotion(); when absent, the
// adapter returns the zero value (no preference).
func (w *Window) svgParseOpts() SvgParseOpts {
	var out SvgParseOpts
	if rm, ok := w.nativePlatform.(interface{ PrefersReducedMotion() bool }); ok {
		out.PrefersReducedMotion = rm.PrefersReducedMotion()
	}
	return out
}

// parseSvgWithOpts dispatches to ParseSvgWithOpts when the backend
// implements SvgParserWithOpts, falling back to plain ParseSvg /
// ParseSvgFile otherwise. Inline sources (svgSrc starts with '<')
// route to the string parser; everything else routes to the file
// parser using the resolved path.
func (w *Window) parseSvgWithOpts(
	svgSrc, resolvedSrc string, opts SvgParseOpts,
) (*SvgParsed, error) {
	inline := strings.HasPrefix(svgSrc, "<")
	if pwo, ok := w.svgParser.(svgParserWithOpts); ok {
		if inline {
			return pwo.ParseSvgWithOpts(svgSrc, opts)
		}
		return pwo.ParseSvgFileWithOpts(resolvedSrc, opts)
	}
	if inline {
		return w.svgParser.ParseSvg(svgSrc)
	}
	return w.svgParser.ParseSvgFile(resolvedSrc)
}

// resolveAndCheckSvgSource validates, resolves, and size-checks
// an SVG source path or inline data.
func (w *Window) resolveAndCheckSvgSource(svgSrc string) (string, error) {
	resolvedSrc, err := resolveValidatedSvgPath(svgSrc, w.Config.AllowedSvgRoots)
	if err != nil {
		return "", err
	}
	sizeSrc := svgSrc
	if !strings.HasPrefix(svgSrc, "<") {
		sizeSrc = resolvedSrc
	}
	if err := checkSvgSourceSize(sizeSrc); err != nil {
		return "", err
	}
	return resolvedSrc, nil
}

// LoadSvg loads and tessellates an SVG, caching the result.
// svgSrc can be a file path or inline SVG data (starting with '<').
func (w *Window) LoadSvg(svgSrc string, width, height float32) (*CachedSvg, error) {
	return w.loadSvgWithOpts(svgSrc, width, height, w.svgParseOpts())
}

// LoadSvgWithOpts is LoadSvg with caller-supplied per-render
// overrides. Window-derived flags (PrefersReducedMotion) are merged
// in; override fields take precedence on FlatnessTolerance,
// HoveredElementID, FocusedElementID.
// exportaudit:keep — collides with the loadSvgWithOpts helper method
func (w *Window) LoadSvgWithOpts(svgSrc string, width, height float32,
	override SvgParseOpts) (*CachedSvg, error) {
	opts := w.svgParseOpts()
	opts.FlatnessTolerance = override.FlatnessTolerance
	opts.HoveredElementID = override.HoveredElementID
	opts.FocusedElementID = override.FocusedElementID
	return w.loadSvgWithOpts(svgSrc, width, height, opts)
}

func (w *Window) loadSvgWithOpts(svgSrc string, width, height float32,
	opts SvgParseOpts) (*CachedSvg, error) {
	// Fast path: this runs per visible SVG per frame, from the
	// render pass. An inline source is its own content, and a file
	// confirmed within svgFileRecheckInterval keeps its recorded
	// fingerprint, so neither touches the file system on a hit.
	inline := strings.HasPrefix(svgSrc, "<")
	srcHash := hashString(svgSrc)
	now := time.Now()
	var contentHash uint64
	known := inline
	if !inline {
		var entry svgDimEntry
		entry, known = w.freshSvgFingerprint(srcHash, now)
		contentHash = entry.contentHash
	}
	sm := StateMapRead[svgCacheKey, *CachedSvg](w, nsSvgCache)
	if known && sm != nil {
		cacheKey := buildSvgCacheLookupKey(srcHash, contentHash,
			width, height, opts)
		if cached, ok := sm.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Slow path: resolve, then read the file state. An unchanged
	// file still hits the cache here.
	resolvedSrc, err := w.resolveAndCheckSvgSource(svgSrc)
	if err != nil {
		return nil, err
	}
	if !inline {
		contentHash = svgFileFingerprint(resolvedSrc)
		w.confirmSvgFingerprint(srcHash, contentHash, now)
		if sm != nil {
			cacheKey := buildSvgCacheLookupKey(srcHash, contentHash,
				width, height, opts)
			if cached, ok := sm.Get(cacheKey); ok {
				w.restoreSvgFingerprint(srcHash, contentHash, cached, now)
				return cached, nil
			}
		}
	}

	if w.svgParser == nil {
		return nil, errors.New("no SVG parser configured")
	}

	var parsed *SvgParsed
	parsed, err = w.parseSvgWithOpts(svgSrc, resolvedSrc, opts)
	if err != nil {
		return nil, err
	}

	// Cache dimensions with the file state they came from.
	dimCache := StateMap[uint64, svgDimEntry](w, nsSvgDimCache, capModerate)
	dimCache.Set(srcHash, svgDimEntry{
		dims:        [2]float32{parsed.Width, parsed.Height},
		contentHash: contentHash,
		checkedAt:   now,
	})

	// Compute scale. preserveAspectRatio="<align> meet" → fit
	// (min); "<align> slice" → fill (max). Alignment offset is
	// applied in renderSvg using PreserveAlign.
	scale := float32(1)
	if width > 0 && height > 0 {
		scaleX := float32(1)
		if parsed.Width > 0 {
			scaleX = width / parsed.Width
		}
		scaleY := float32(1)
		if parsed.Height > 0 {
			scaleY = height / parsed.Height
		}
		if parsed.PreserveSlice {
			scale = max(scaleX, scaleY)
		} else {
			scale = min(scaleX, scaleY)
		}
	}

	triangles := w.svgParser.Tessellate(parsed, scale)
	renderPaths := cachedSvgPaths(triangles)
	textDraws := cachedSvgTextDraws(parsed.Texts, scale, parsed.Gradients, w)
	defsPathData := buildDefsPathDataCache(parsed.TextPaths, parsed.FilteredGroups, parsed.DefsPaths, scale)
	textPathDraws := cachedSvgTextPathDraws(parsed.TextPaths, defsPathData, scale)

	// Build filtered groups.
	var filteredGroups []cachedFilteredGroup
	for _, fg := range parsed.FilteredGroups {
		fgPaths := cachedSvgPaths(fg.Paths)
		fgTextDraws := cachedSvgTextDraws(fg.Texts, scale, parsed.Gradients, w)
		fgTextPathDraws := cachedSvgTextPathDraws(fg.TextPaths, defsPathData, scale)
		filteredGroups = append(filteredGroups, cachedFilteredGroup{
			Filter:        fg.Filter,
			renderPaths:   fgPaths,
			textDraws:     fgTextDraws,
			textPathDraws: fgTextPathDraws,
			Gradients:     parsed.Gradients,
			bBox:          computeTriangleBBox(fg.Paths),
		})
	}

	hasAttrAnim := slices.ContainsFunc(parsed.Animations,
		func(a SvgAnimation) bool {
			return a.Kind == SvgAnimAttr ||
				a.Kind == SvgAnimDashArray ||
				a.Kind == SvgAnimDashOffset
		})
	isAnim := func(p cachedSvgPath) bool { return p.Animated }
	hasAnimatedPaths := slices.ContainsFunc(renderPaths, isAnim) ||
		slices.ContainsFunc(filteredGroups,
			func(g cachedFilteredGroup) bool {
				return slices.ContainsFunc(g.renderPaths, isAnim)
			})
	baseByPath := buildBaseByPath(renderPaths, filteredGroups,
		parsed.Animations)

	cached := &CachedSvg{
		renderPaths:      renderPaths,
		textDraws:        textDraws,
		textPathDraws:    textPathDraws,
		FilteredGroups:   filteredGroups,
		Gradients:        parsed.Gradients,
		Animations:       parsed.Animations,
		hasAnimations:    len(parsed.Animations) > 0,
		hasAttrAnim:      hasAttrAnim,
		hasAnimatedPaths: hasAnimatedPaths,
		Parsed:           parsed,
		animStartNs:      time.Now().UnixNano(),
		animHash:         strconv.FormatUint(srcHash, 16),
		Width:            parsed.Width,
		Height:           parsed.Height,
		Scale:            scale,
		ViewBoxX:         parsed.ViewBoxX,
		ViewBoxY:         parsed.ViewBoxY,
		PreserveAlign:    parsed.PreserveAlign,
		PreserveSlice:    parsed.PreserveSlice,
		baseByPath:       baseByPath,
		defsPathData:     defsPathData,
	}

	// Cache if vertex count is reasonable. Filtered groups
	// count too: without them a filter-heavy file skips the cap
	// while it still fills the vertex budget.
	totalVerts := 0
	for _, p := range renderPaths {
		totalVerts += len(p.Triangles)
	}
	for _, g := range filteredGroups {
		for _, p := range g.renderPaths {
			totalVerts += len(p.Triangles)
		}
	}
	const maxCachedVerts = 1_250_000
	if totalVerts <= maxCachedVerts {
		svgCache := StateMap[svgCacheKey, *CachedSvg](w, nsSvgCache, capModerate)
		svgCache.evictToBudget(svgCacheMaxMemory,
			cached.estimateMemory(),
			func(c *CachedSvg) int { return c.estimateMemory() })
		svgCache.Set(buildSvgCacheLookupKey(srcHash, contentHash,
			width, height, opts), cached)
	}
	return cached, nil
}

// GetSvgDimensions returns natural SVG dimensions without full
// parse+tessellate. Uses cached dimensions when available.
func (w *Window) getSvgDimensions(svgSrc string) (float32, float32, error) {
	// Fast path, as in loadSvgWithOpts: svgView asks every frame,
	// so a recently confirmed file makes no file system call.
	inline := strings.HasPrefix(svgSrc, "<")
	srcHash := hashString(svgSrc)
	now := time.Now()
	dimCache := StateMapRead[uint64, svgDimEntry](w, nsSvgDimCache)
	if inline {
		if dimCache != nil {
			if entry, ok := dimCache.Get(srcHash); ok {
				return entry.dims[0], entry.dims[1], nil
			}
		}
	} else if entry, ok := w.freshSvgFingerprint(srcHash, now); ok {
		return entry.dims[0], entry.dims[1], nil
	}

	// Slow path: resolve, then compare the file state with the
	// recorded one.
	resolvedSrc, err := w.resolveAndCheckSvgSource(svgSrc)
	if err != nil {
		return 0, 0, err
	}
	var contentHash uint64
	if !inline {
		contentHash = svgFileFingerprint(resolvedSrc)
		if dimCache != nil {
			if entry, ok := dimCache.Get(srcHash); ok &&
				entry.contentHash == contentHash {
				w.confirmSvgFingerprint(srcHash, contentHash, now)
				return entry.dims[0], entry.dims[1], nil
			}
		}
	}

	if w.svgParser == nil {
		return 0, 0, errors.New("no SVG parser configured")
	}

	var content string
	if inline {
		content = svgSrc
	} else {
		data, err := readCappedSvgFile(resolvedSrc)
		if err != nil {
			return 0, 0, err
		}
		content = string(data)
	}

	svgW, svgH, err := w.svgParser.ParseSvgDimensions(content)
	if err != nil {
		return 0, 0, err
	}

	dc := StateMap[uint64, svgDimEntry](w, nsSvgDimCache, capModerate)
	dc.Set(srcHash, svgDimEntry{
		dims:        [2]float32{svgW, svgH},
		contentHash: contentHash,
		checkedAt:   now,
	})
	return svgW, svgH, nil
}

// RemoveSvgFromCache removes all cached variants of an SVG.
func (w *Window) removeSvgFromCache(svgSrc string) {
	srcHash := hashString(svgSrc)

	svgCache := StateMapRead[svgCacheKey, *CachedSvg](w, nsSvgCache)
	if svgCache != nil {
		var keysToDelete []svgCacheKey
		for _, key := range svgCache.Keys() {
			if key.srcHash == srcHash {
				keysToDelete = append(keysToDelete, key)
			}
		}
		for _, key := range keysToDelete {
			svgCache.Delete(key)
		}
	}

	dimCache := StateMapRead[uint64, svgDimEntry](w, nsSvgDimCache)
	if dimCache != nil {
		dimCache.Delete(srcHash)
	}
	if inv, ok := w.svgParser.(svgParserCacheInvalidator); ok {
		inv.InvalidateSvgSource(svgSrc)
	}
}

// ClearSvgCache removes all cached SVGs.
func (w *Window) clearSvgCache() {
	svgCache := StateMapRead[svgCacheKey, *CachedSvg](w, nsSvgCache)
	if svgCache != nil {
		svgCache.Clear()
	}
	dimCache := StateMapRead[uint64, svgDimEntry](w, nsSvgDimCache)
	if dimCache != nil {
		dimCache.Clear()
	}
	if inv, ok := w.svgParser.(svgParserCacheInvalidator); ok {
		inv.ClearSvgParserCache()
	}
}

// buildBaseByPath collects decomposed author base transforms keyed
// by PathID, for paths that any animation targets. Seeding the
// per-frame svgAnimState with these lets SMIL additive / replace
// compose over the author's base (see CachedSvg.BaseByPath).
