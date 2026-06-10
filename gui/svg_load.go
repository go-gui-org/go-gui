package gui

import (
	"fmt"
	"math"
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

// cachedSvgPaths converts TessellatedPath slices to CachedSvgPath.
func cachedSvgPaths(paths []TessellatedPath) []CachedSvgPath {
	out := make([]CachedSvgPath, len(paths))
	for i := range paths {
		p := &paths[i]
		var vcols []Color
		if len(p.VertexColors) != 0 {
			vcols = make([]Color, len(p.VertexColors))
			for j := range p.VertexColors {
				vcols[j] = svgToColor(p.VertexColors[j])
			}
		}
		out[i] = CachedSvgPath{
			Triangles:    p.Triangles,
			Color:        svgToColor(p.Color),
			VertexColors: vcols,
			IsClipMask:   p.IsClipMask,
			ClipGroup:    p.ClipGroup,
			PathID:       p.PathID,
			Animated:     p.Animated,
			IsStroke:     p.IsStroke,
			Primitive:    p.Primitive,
			BaseTransX:   p.BaseTransX,
			BaseTransY:   p.BaseTransY,
			BaseScaleX:   p.BaseScaleX,
			BaseScaleY:   p.BaseScaleY,
			BaseRotAngle: p.BaseRotAngle,
			BaseRotCX:    p.BaseRotCX,
			BaseRotCY:    p.BaseRotCY,
			HasBaseXform: p.HasBaseXform,
		}
	}
	return out
}

// buildSvgTextStyle builds a TextStyle from SVG text properties.
func buildSvgTextStyle(
	fontFamily string, fontWeight int, isBold, isItalic bool,
	fontSize, letterSpacing, strokeWidth float32,
	strokeColor, color SvgColor, opacity, scale float32,
) TextStyle {
	fontName := fontFamily
	if wn := pangoWeightName(fontWeight); wn != "" {
		fontName += " " + wn
	} else if isBold {
		fontName += " Bold"
	}
	typeface := glyph.TypefaceRegular
	if isItalic {
		typeface = glyph.TypefaceItalic
	}
	ts := TextStyle{
		Family:        fontName,
		Size:          fontSize * scale,
		LetterSpacing: letterSpacing * scale,
		Typeface:      typeface,
		StrokeWidth:   strokeWidth * scale,
		StrokeColor:   svgToColor(strokeColor),
	}
	if opacity < 1.0 {
		ts.Color = Color{color.R, color.G, color.B,
			uint8(float32(color.A)*opacity + 0.5), true}
	} else {
		ts.Color = svgToColor(color)
	}
	return ts
}

// cachedSvgTextDraws converts SvgText elements to CachedSvgTextDraw.
func cachedSvgTextDraws(texts []SvgText, scale float32,
	gradients map[string]SvgGradientDef, w *Window) []CachedSvgTextDraw {
	draws := make([]CachedSvgTextDraw, 0, len(texts))
	for _, t := range texts {
		if len(t.Text) == 0 {
			continue
		}
		ts := buildSvgTextStyle(t.FontFamily, t.FontWeight,
			t.IsBold, t.IsItalic, t.FontSize, t.LetterSpacing,
			t.StrokeWidth, t.StrokeColor, t.Color, t.Opacity, scale)
		ts.Underline = t.Underline
		ts.Strikethrough = t.Strikethrough

		// Stroke-only text: fill=none + stroke set → transparent fill.
		if ts.StrokeWidth > 0 && ts.Color.A == 0 {
			ts.Color = Color{0, 0, 0, 0, true}
		}

		// Build gradient config from SVG gradient def.
		var grad *glyph.GradientConfig
		if t.FillGradientID != "" && gradients != nil {
			if gdef, ok := gradients[t.FillGradientID]; ok {
				grad = svgGradientToGlyph(gdef)
			}
		}

		// Measure text width for anchor adjustment.
		var tw float32
		var fh float32
		if w.textMeasurer != nil {
			tw = w.textMeasurer.TextWidth(t.Text, ts)
			fh = w.textMeasurer.FontHeight(ts)
		} else {
			fh = t.FontSize * scale
		}
		ascent := fh * 0.8
		x := t.X * scale
		y := t.Y*scale - ascent
		switch t.Anchor {
		case SvgTextAnchorMiddle:
			x -= tw / 2
		case SvgTextAnchorEnd:
			x -= tw
		}
		draws = append(draws, CachedSvgTextDraw{
			Text:      t.Text,
			TextStyle: ts,
			X:         x,
			Y:         y,
			TextWidth: tw,
			Gradient:  grad,
		})
	}
	return draws
}

func cachedSvgTextPathDraws(textPaths []SvgTextPath,
	defsPathData map[string]cachedDefsPathData,
	scale float32,
) []CachedSvgTextPathDraw {
	if len(textPaths) == 0 {
		return nil
	}
	out := make([]CachedSvgTextPathDraw, 0, len(textPaths))
	for i := range textPaths {
		tp := textPaths[i]
		if tp.Text == "" {
			continue
		}
		cached, ok := defsPathData[tp.PathID]
		if !ok || len(cached.polyline) < 4 || cached.totalLen <= 0 {
			continue
		}
		ts := buildSvgTextStyle(tp.FontFamily, tp.FontWeight,
			tp.IsBold, tp.IsItalic, tp.FontSize, tp.LetterSpacing,
			tp.StrokeWidth, tp.StrokeColor, tp.Color, tp.Opacity, scale)

		offset := tp.StartOffset * scale
		if tp.IsPercent {
			offset = (tp.StartOffset / 100) * cached.totalLen
		}
		out = append(out, CachedSvgTextPathDraw{
			Text:      tp.Text,
			TextStyle: ts,
			Path: TextPathData{
				Polyline: cached.polyline,
				Table:    cached.table,
				TotalLen: cached.totalLen,
				Offset:   offset,
				Anchor:   tp.Anchor,
				Method:   tp.Method,
			},
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// pangoWeightName maps CSS font-weight (100-900) to a Pango
// weight descriptor. Returns "" for regular (400) or unset (0).
func pangoWeightName(w int) string {
	switch w {
	case 100:
		return "Thin"
	case 200:
		return "Ultra-Light"
	case 300:
		return "Light"
	case 500:
		return "Medium"
	case 600:
		return "Semi-Bold"
	case 700:
		return "Bold"
	case 800:
		return "Ultra-Bold"
	case 900:
		return "Heavy"
	default:
		return ""
	}
}

// svgGradientToGlyph converts an SvgGradientDef to a glyph
// GradientConfig.
func svgGradientToGlyph(g SvgGradientDef) *glyph.GradientConfig {
	if len(g.Stops) == 0 {
		return nil
	}
	stops := make([]glyph.GradientStop, len(g.Stops))
	for i, s := range g.Stops {
		stops[i] = glyph.GradientStop{
			Color:    glyph.Color{R: s.Color.R, G: s.Color.G, B: s.Color.B, A: s.Color.A},
			Position: s.Offset,
		}
	}
	dir := glyph.GradientHorizontal
	// Determine direction from gradient vector. Diagonal gradients
	// are forced to H or V — glyph does not support arbitrary
	// angles yet.
	// TODO: support diagonal gradients when glyph adds angle support
	dx := g.X2 - g.X1
	dy := g.Y2 - g.Y1
	if dy*dy > dx*dx {
		dir = glyph.GradientVertical
	}
	return &glyph.GradientConfig{
		Stops:     stops,
		Direction: dir,
	}
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
		return fmt.Errorf("invalid svg path: contains NUL")
	}
	cleanPath := filepath.Clean(svgSrc)
	if cleanPath == "." {
		return fmt.Errorf("invalid svg path")
	}
	for part := range strings.SplitSeq(filepath.ToSlash(cleanPath), "/") {
		if part == ".." {
			return fmt.Errorf("invalid svg path: contains parent directory reference")
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
			return fmt.Errorf("SVG source too large")
		}
		return nil
	}
	info, err := os.Stat(svgSrc)
	if err != nil {
		return fmt.Errorf("SVG not found: %s", svgSrc)
	}
	if info.Size() > maxSvgSourceBytes {
		return fmt.Errorf("SVG file too large")
	}
	return nil
}

func buildDefsPathDataCache(
	textPaths []SvgTextPath,
	filtered []SvgParsedFilteredGroup,
	defsPaths map[string]string,
	scale float32,
) map[string]cachedDefsPathData {
	if (len(textPaths) == 0 && len(filtered) == 0) || len(defsPaths) == 0 {
		return nil
	}
	pathIDs := make(map[string]struct{}, len(textPaths))
	for i := range textPaths {
		id := textPaths[i].PathID
		if id != "" {
			pathIDs[id] = struct{}{}
		}
	}
	for i := range filtered {
		for j := range filtered[i].TextPaths {
			id := filtered[i].TextPaths[j].PathID
			if id != "" {
				pathIDs[id] = struct{}{}
			}
		}
	}
	if len(pathIDs) == 0 {
		return nil
	}
	cached := make(map[string]cachedDefsPathData, len(pathIDs))
	for pathID := range pathIDs {
		d, ok := defsPaths[pathID]
		if !ok {
			continue
		}
		polyline := flattenDefsPath(d, scale)
		if len(polyline) < 4 {
			continue
		}
		table, totalLen := buildArcLengthTable(polyline)
		if totalLen <= 0 {
			continue
		}
		cached[pathID] = cachedDefsPathData{
			polyline: polyline,
			table:    table,
			totalLen: totalLen,
		}
	}
	if len(cached) == 0 {
		return nil
	}
	return cached
}

func buildSvgCacheLookupKey(
	srcHash uint64, width, height float32, opts SvgParseOpts,
) svgCacheKey {
	return svgCacheKey{
		srcHash:       srcHash,
		w10:           int32(width * 10),
		h10:           int32(height * 10),
		reducedMotion: opts.PrefersReducedMotion,
		flatness10000: quantizeFlatness(opts.FlatnessTolerance),
		hoveredID:     clampSvgCacheID(opts.HoveredElementID),
		focusedID:     clampSvgCacheID(opts.FocusedElementID),
	}
}

// quantizeFlatness maps FlatnessTolerance into the int32 cache key
// slot. NaN/Inf and out-of-range values collapse to 0, which matches
// the "no override" key used by callers that don't set the field.
// Without this guard, `int32(NaN*10000)` is implementation-defined
// and `int32(huge*10000)` overflows.
func quantizeFlatness(t float32) int32 {
	t64 := float64(t)
	if math.IsNaN(t64) || math.IsInf(t64, 0) || t <= 0 {
		return 0
	}
	scaled := t64 * 10000
	if scaled > float64(math.MaxInt32) {
		return math.MaxInt32
	}
	return int32(scaled)
}

func clampSvgCacheID(s string) string {
	if len(s) > maxSvgCacheElementIDLen {
		return s[:maxSvgCacheElementIDLen]
	}
	return s
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
		return nil, fmt.Errorf("no SVG parser configured")
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
		return 0, 0, fmt.Errorf("no SVG parser configured")
	}

	var content string
	if strings.HasPrefix(svgSrc, "<") {
		content = svgSrc
	} else {
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
func buildBaseByPath(
	paths []CachedSvgPath,
	filteredGroups []CachedFilteredGroup,
	anims []SvgAnimation,
) map[uint32]svgBaseXform {
	if len(anims) == 0 {
		return nil
	}
	targeted := make(map[uint32]struct{}, len(anims))
	for i := range anims {
		for _, pid := range anims[i].TargetPathIDs {
			targeted[pid] = struct{}{}
		}
	}
	if len(targeted) == 0 {
		return nil
	}
	out := make(map[uint32]svgBaseXform)
	collect := func(ps []CachedSvgPath) {
		for i := range ps {
			p := &ps[i]
			if !p.HasBaseXform || p.PathID == 0 {
				continue
			}
			if _, ok := targeted[p.PathID]; !ok {
				continue
			}
			if _, already := out[p.PathID]; already {
				continue
			}
			out[p.PathID] = svgBaseXform{
				TransX:   p.BaseTransX,
				TransY:   p.BaseTransY,
				ScaleX:   p.BaseScaleX,
				ScaleY:   p.BaseScaleY,
				RotAngle: p.BaseRotAngle,
				RotCX:    p.BaseRotCX,
				RotCY:    p.BaseRotCY,
			}
		}
	}
	collect(paths)
	for i := range filteredGroups {
		collect(filteredGroups[i].RenderPaths)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// computeTriangleBBox computes bounding box from tessellated paths.
// Returns [x, y, width, height].
func computeTriangleBBox(tpaths []TessellatedPath) [4]float32 {
	minX := float32(1e30)
	minY := float32(1e30)
	maxX := float32(-1e30)
	maxY := float32(-1e30)
	hasData := false

	for _, tp := range tpaths {
		for i := 0; i+1 < len(tp.Triangles); i += 2 {
			x := tp.Triangles[i]
			y := tp.Triangles[i+1]
			minX = min(minX, x)
			maxX = max(maxX, x)
			minY = min(minY, y)
			maxY = max(maxY, y)
			hasData = true
		}
	}

	if !hasData {
		return [4]float32{0, 0, 0, 0}
	}
	return [4]float32{minX, minY, maxX - minX, maxY - minY}
}
