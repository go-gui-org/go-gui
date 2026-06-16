package gui

import (
	"errors"
	"fmt"
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
type CachedSvgPath struct {
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
type CachedSvgTextDraw struct {
	TextStyle TextStyle
	Gradient  *glyph.GradientConfig
	Text      string
	X, Y      float32
	TextWidth float32 // measured width including letter-spacing
}

// CachedSvgTextPathDraw holds precomputed textPath render data.
type CachedSvgTextPathDraw struct {
	Text      string
	TextStyle TextStyle
	Path      TextPathData
}

// CachedFilteredGroup holds tessellated geometry for a filter group.
type CachedFilteredGroup struct {
	Gradients     map[string]SvgGradientDef
	RenderPaths   []CachedSvgPath
	TextDraws     []CachedSvgTextDraw
	TextPathDraws []CachedSvgTextPathDraw
	Filter        SvgFilter
	BBox          [4]float32 // x, y, width, height
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
	BaseByPath     map[uint32]svgBaseXform
	defsPathData   map[string]cachedDefsPathData
	AnimHash       string
	RenderPaths    []CachedSvgPath
	TextDraws      []CachedSvgTextDraw
	TextPathDraws  []CachedSvgTextPathDraw
	FilteredGroups []CachedFilteredGroup
	Animations     []SvgAnimation
	AnimStartNs    int64
	Width          float32
	Height         float32
	Scale          float32
	// ViewBoxX / ViewBoxY are the authored viewBox origin. Applied at
	// render time as an outer translate on sx/sy so authored coords
	// stay in raw viewBox space throughout tessellation and animation.
	ViewBoxX         float32
	ViewBoxY         float32
	HasAnimations    bool
	HasAttrAnim      bool // any SvgAnimAttr present → try re-tessellation
	HasAnimatedPaths bool // any RenderPath has Animated=true
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
	w10       int32
	h10       int32
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
func (c *CachedSvg) EstimateMemory() int {
	n := 0
	for _, p := range c.RenderPaths {
		n += len(p.Triangles) * 4    // float32 = 4 bytes
		n += len(p.VertexColors) * 4 // Color = 4 bytes (uint32)
	}
	for _, fg := range c.FilteredGroups {
		for _, p := range fg.RenderPaths {
			n += len(p.Triangles) * 4
			n += len(p.VertexColors) * 4
		}
	}
	n += len(c.TextDraws) * 384     // CachedSvgTextDraw + glyph.GradientConfig
	n += len(c.TextPathDraws) * 640 // CachedSvgTextPathDraw + TextPathData
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
	if pwo, ok := w.svgParser.(SvgParserWithOpts); ok {
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
	srcHash := hashString(svgSrc)
	cacheKey := buildSvgCacheLookupKey(srcHash, width, height, opts)

	sm := StateMapRead[svgCacheKey, *CachedSvg](w, nsSvgCache)
	if sm != nil {
		if cached, ok := sm.Get(cacheKey); ok {
			return cached, nil
		}
	}

	resolvedSrc, err := w.resolveAndCheckSvgSource(svgSrc)
	if err != nil {
		return nil, err
	}

	if w.svgParser == nil {
		return nil, errors.New("no SVG parser configured")
	}

	var parsed *SvgParsed
	parsed, err = w.parseSvgWithOpts(svgSrc, resolvedSrc, opts)
	if err != nil {
		return nil, err
	}

	// Cache dimensions.
	dimCache := StateMap[uint64, [2]float32](w, nsSvgDimCache, capModerate)
	dimCache.Set(srcHash, [2]float32{parsed.Width, parsed.Height})

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
	var filteredGroups []CachedFilteredGroup
	for _, fg := range parsed.FilteredGroups {
		fgPaths := cachedSvgPaths(fg.Paths)
		fgTextDraws := cachedSvgTextDraws(fg.Texts, scale, parsed.Gradients, w)
		fgTextPathDraws := cachedSvgTextPathDraws(fg.TextPaths, defsPathData, scale)
		filteredGroups = append(filteredGroups, CachedFilteredGroup{
			Filter:        fg.Filter,
			RenderPaths:   fgPaths,
			TextDraws:     fgTextDraws,
			TextPathDraws: fgTextPathDraws,
			Gradients:     parsed.Gradients,
			BBox:          computeTriangleBBox(fg.Paths),
		})
	}

	hasAttrAnim := slices.ContainsFunc(parsed.Animations,
		func(a SvgAnimation) bool {
			return a.Kind == SvgAnimAttr ||
				a.Kind == SvgAnimDashArray ||
				a.Kind == SvgAnimDashOffset
		})
	isAnim := func(p CachedSvgPath) bool { return p.Animated }
	hasAnimatedPaths := slices.ContainsFunc(renderPaths, isAnim) ||
		slices.ContainsFunc(filteredGroups,
			func(g CachedFilteredGroup) bool {
				return slices.ContainsFunc(g.RenderPaths, isAnim)
			})
	baseByPath := buildBaseByPath(renderPaths, filteredGroups,
		parsed.Animations)

	cached := &CachedSvg{
		RenderPaths:      renderPaths,
		TextDraws:        textDraws,
		TextPathDraws:    textPathDraws,
		FilteredGroups:   filteredGroups,
		Gradients:        parsed.Gradients,
		Animations:       parsed.Animations,
		HasAnimations:    len(parsed.Animations) > 0,
		HasAttrAnim:      hasAttrAnim,
		HasAnimatedPaths: hasAnimatedPaths,
		Parsed:           parsed,
		AnimStartNs:      time.Now().UnixNano(),
		AnimHash:         strconv.FormatUint(srcHash, 16),
		Width:            parsed.Width,
		Height:           parsed.Height,
		Scale:            scale,
		ViewBoxX:         parsed.ViewBoxX,
		ViewBoxY:         parsed.ViewBoxY,
		PreserveAlign:    parsed.PreserveAlign,
		PreserveSlice:    parsed.PreserveSlice,
		BaseByPath:       baseByPath,
		defsPathData:     defsPathData,
	}

	// Cache if vertex count is reasonable.
	totalVerts := 0
	for _, p := range renderPaths {
		totalVerts += len(p.Triangles)
	}
	const maxCachedVerts = 1_250_000
	if totalVerts <= maxCachedVerts {
		svgCache := StateMap[svgCacheKey, *CachedSvg](w, nsSvgCache, capModerate)
		svgCache.EvictToBudget(svgCacheMaxMemory,
			cached.EstimateMemory(),
			func(c *CachedSvg) int { return c.EstimateMemory() })
		svgCache.Set(cacheKey, cached)
	}
	return cached, nil
}

// GetSvgDimensions returns natural SVG dimensions without full
// parse+tessellate. Uses cached dimensions when available.
func (w *Window) GetSvgDimensions(svgSrc string) (float32, float32, error) {
	srcHash := hashString(svgSrc)
	dimCache := StateMapRead[uint64, [2]float32](w, nsSvgDimCache)
	if dimCache != nil {
		if dims, ok := dimCache.Get(srcHash); ok {
			return dims[0], dims[1], nil
		}
	}

	resolvedSrc, err := w.resolveAndCheckSvgSource(svgSrc)
	if err != nil {
		return 0, 0, err
	}

	if w.svgParser == nil {
		return 0, 0, errors.New("no SVG parser configured")
	}

	var content string
	if strings.HasPrefix(svgSrc, "<") {
		content = svgSrc
	} else {
		// #nosec G304 — resolvedSrc validated through AllowedSvgRoots
		data, err := os.ReadFile(resolvedSrc)
		if err != nil {
			return 0, 0, fmt.Errorf("SVG not found: %s", resolvedSrc)
		}
		content = string(data)
	}

	svgW, svgH, err := w.svgParser.ParseSvgDimensions(content)
	if err != nil {
		return 0, 0, err
	}

	dc := StateMap[uint64, [2]float32](w, nsSvgDimCache, capModerate)
	dc.Set(srcHash, [2]float32{svgW, svgH})
	return svgW, svgH, nil
}

// RemoveSvgFromCache removes all cached variants of an SVG.
func (w *Window) RemoveSvgFromCache(svgSrc string) {
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

	dimCache := StateMapRead[uint64, [2]float32](w, nsSvgDimCache)
	if dimCache != nil {
		dimCache.Delete(srcHash)
	}
	if inv, ok := w.svgParser.(svgParserCacheInvalidator); ok {
		inv.InvalidateSvgSource(svgSrc)
	}
}

// ClearSvgCache removes all cached SVGs.
func (w *Window) ClearSvgCache() {
	svgCache := StateMapRead[svgCacheKey, *CachedSvg](w, nsSvgCache)
	if svgCache != nil {
		svgCache.Clear()
	}
	dimCache := StateMapRead[uint64, [2]float32](w, nsSvgDimCache)
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
