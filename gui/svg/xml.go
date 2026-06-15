package svg

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/go-gui-org/go-gui/gui/svg/css"
)

// maxElementIDLen caps pseudo-state element IDs forwarded into the
// cascade. SVG ids in practice are short (icon-set IDs rarely exceed
// 64 bytes); a cap protects the cache key, hash mixer, and per-
// element ID compare from a hostile multi-MB id.
const maxElementIDLen = 256

// maxFlatnessTolerance caps the upper bound on user-supplied
// FlatnessTolerance. Beyond a few units the tessellator collapses
// curves to chords already; refusing absurd values keeps the floor
// math, cache keying, and hash mixing on safe ranges.
const maxFlatnessTolerance float32 = 64

// sanitizeFlatness drops NaN/Inf/negative inputs and caps the upper
// bound. Returning 0 disables the override and falls back to the
// built-in 0.15 floor.
func sanitizeFlatness(t float32) float32 {
	t64 := float64(t)
	if math.IsNaN(t64) || math.IsInf(t64, 0) || t <= 0 {
		return 0
	}
	if t > maxFlatnessTolerance {
		return maxFlatnessTolerance
	}
	return t
}

// clampElementID truncates s to maxElementIDLen UTF-8 bytes. Cheap
// byte slice; the cascade compares IDs by exact match so trimming a
// hostile string still produces correct (no-match) behavior.
func clampElementID(s string) string {
	if len(s) > maxElementIDLen {
		return s[:maxElementIDLen]
	}
	return s
}

// ParseOptions controls environment-dependent parsing behavior.
// PrefersReducedMotion is the snapshot fed to
// `@media (prefers-reduced-motion: reduce)` evaluation; see
// docs/svg-css-design.md "prefers-reduced-motion".
// FlatnessTolerance overrides the tessellation tolerance floor when
// > 0. HoveredElementID / FocusedElementID feed :hover / :focus
// pseudo-class matching during the cascade.
type ParseOptions struct {
	HoveredElementID     string
	FocusedElementID     string
	FlatnessTolerance    float32
	PrefersReducedMotion bool
}

// parseSvg parses an SVG string and returns a VectorGraphic.
func parseSvg(content string) (*VectorGraphic, error) {
	return parseSvgWith(content, ParseOptions{})
}

// parseSvgWith is the options-aware variant of parseSvg. opts is
// snapshotted into the cascade (e.g. for @media reduced-motion).
func parseSvgWith(content string, opts ParseOptions) (*VectorGraphic, error) {
	if len(content) > maxSvgFileSize {
		return nil, fmt.Errorf("svg: content too large: %d bytes", len(content))
	}
	root, err := decodeSvgTree(content)
	if err != nil {
		return nil, err
	}
	expandUseElements(root)

	vg := &VectorGraphic{
		Width:  defaultIconSize,
		Height: defaultIconSize,
	}

	// viewBox on root. Fall back to lowercase "viewbox" — SVG-in-HTML
	// authoring (and several svg-spinners assets) ship the attribute
	// lowercased; XHTML strict-mode is rare in the wild.
	vb, ok := root.AttrMap["viewBox"]
	if !ok {
		vb, ok = root.AttrMap["viewbox"]
	}
	if ok {
		nums := parseNumberList(vb)
		if len(nums) >= 4 {
			vg.ViewBoxX = nums[0]
			vg.ViewBoxY = nums[1]
			vg.Width = clampViewBoxDim(nums[2])
			vg.Height = clampViewBoxDim(nums[3])
		}
	} else {
		if w, ok := root.AttrMap["width"]; ok {
			vg.Width = clampViewBoxDim(parseLength(w))
		}
		if h, ok := root.AttrMap["height"]; ok {
			vg.Height = clampViewBoxDim(parseLength(h))
		}
	}

	vg.A11y = parseRootA11y(root)
	vg.PreserveAlign, vg.PreserveSlice = parsePreserveAspectRatio(
		root.AttrMap["preserveAspectRatio"])

	// Pre-pass: extract <defs>.
	vg.ClipPaths = parseDefsClipPaths(root)
	vg.Gradients = parseDefsGradients(root)
	vg.Filters = parseDefsFilters(root)
	vg.DefsPaths = parseDefsPaths(root)

	// viewBox offset is applied at render time; triangles, animation
	// centers, and motion paths all stay in raw viewBox space.
	sheet := css.ParseFull(collectStyleBlocks(root), css.ParseOptions{
		PrefersReducedMotion: opts.PrefersReducedMotion,
	})
	state := &parseState{
		defsPaths:    vg.DefsPaths,
		cssRules:     sheet.Rules,
		cssKeyframes: sheet.Keyframes,
		hoveredID:    clampElementID(opts.HoveredElementID),
		focusedID:    clampElementID(opts.FocusedElementID),
		curViewport: viewportRect{
			X: vg.ViewBoxX, Y: vg.ViewBoxY,
			W: vg.Width, H: vg.Height,
		},
	}
	state.vg = vg
	vg.FlatnessTolerance = sanitizeFlatness(opts.FlatnessTolerance)
	// Merge presentation attributes (and matched author rules, when
	// any) from the root <svg> tag so shapes that inherit e.g.
	// fill="currentColor" pick it up.
	rootInfo := makeElementInfo("svg", root.OpenTag, 1, true, root.AttrMap)
	applyPseudoState(&rootInfo, state)
	defStyle := computeStyle(root.OpenTag,
		defaultComputedStyle(identityTransform), state, rootInfo, nil, nil)
	ancestors := []css.ElementInfo{rootInfo}
	allPaths := parseSvgContent(root, defStyle, 0, state, ancestors)

	// Separate filtered paths from main paths.
	if len(vg.Filters) > 0 {
		// Bucket by per-occurrence key: non-contiguous filter uses must
		// composite separately or z-order against unfiltered siblings
		// between them is wrong. Map records first-seen index per key
		// so groups stay in document order across map iteration.
		idx := map[uint32]int{}
		for _, p := range allPaths {
			key := p.FilterGroupKey
			if key == 0 || p.FilterID == "" {
				vg.Paths = append(vg.Paths, p)
				continue
			}
			if _, ok := vg.Filters[p.FilterID]; !ok {
				vg.Paths = append(vg.Paths, p)
				continue
			}
			i, ok := idx[key]
			if !ok {
				i = len(vg.FilteredGroups)
				idx[key] = i
				vg.FilteredGroups = append(vg.FilteredGroups,
					svgFilteredGroup{FilterID: p.FilterID, GroupKey: key})
			}
			vg.FilteredGroups[i].Paths = append(
				vg.FilteredGroups[i].Paths, p)
		}
		for _, t := range state.texts {
			key := t.FilterGroupKey
			if key == 0 || t.FilterID == "" {
				vg.Texts = append(vg.Texts, t)
				continue
			}
			if _, ok := vg.Filters[t.FilterID]; !ok {
				vg.Texts = append(vg.Texts, t)
				continue
			}
			gi, ok := idx[key]
			if !ok {
				gi = len(vg.FilteredGroups)
				idx[key] = gi
				vg.FilteredGroups = append(vg.FilteredGroups,
					svgFilteredGroup{FilterID: t.FilterID, GroupKey: key})
			}
			vg.FilteredGroups[gi].Texts = append(
				vg.FilteredGroups[gi].Texts, t)
		}
		for _, tp := range state.textPaths {
			key := tp.FilterGroupKey
			if key == 0 || tp.FilterID == "" {
				vg.TextPaths = append(vg.TextPaths, tp)
				continue
			}
			if _, ok := vg.Filters[tp.FilterID]; !ok {
				vg.TextPaths = append(vg.TextPaths, tp)
				continue
			}
			gi, ok := idx[key]
			if !ok {
				gi = len(vg.FilteredGroups)
				idx[key] = gi
				vg.FilteredGroups = append(vg.FilteredGroups,
					svgFilteredGroup{FilterID: tp.FilterID, GroupKey: key})
			}
			vg.FilteredGroups[gi].TextPaths = append(
				vg.FilteredGroups[gi].TextPaths, tp)
		}
	} else {
		vg.Paths = allPaths
		vg.Texts = state.texts
		vg.TextPaths = state.textPaths
	}

	vg.Animations = state.animations
	vg.GroupParent = state.groupParent
	resolveBegins(vg.Animations, state.animBeginSpecs, state.animIDIndex)
	return vg, nil
}

const maxSvgFileSize = 4 << 20 // 4 MB

// parseSvgFile loads and parses an SVG file.
func parseSvgFile(path string) (*VectorGraphic, error) {
	data, err := loadSvgFile(path)
	if err != nil {
		return nil, err
	}
	return parseSvg(string(data))
}

func loadSvgFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read SVG file: %w", err)
	}
	if info.Size() > maxSvgFileSize {
		return nil, fmt.Errorf("SVG file too large: %d bytes", info.Size())
	}
	// #nosec G304 — path validated by caller through AllowedSvgRoots
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read SVG file: %w", err)
	}
	return data, nil
}

// parseSvgDimensions extracts only width/height without a full
// parse. Operates on the raw string so callers can probe dimensions
// on incomplete or fragment-only SVG content (no closing tag
// required).
func parseSvgDimensions(content string) (float32, float32) {
	// Cap probe input. Caller-supplied string of arbitrary length
	// would force extractRootSVGOpenTag/findAttr to scan unbounded
	// memory; the dimension probe never needs more than the root
	// open tag, well under maxSvgFileSize.
	if len(content) > maxSvgFileSize {
		content = content[:maxSvgFileSize]
	}
	openTag := extractRootSVGOpenTag(content)
	if openTag == "" {
		openTag = content
	}
	vb, ok := findAttr(openTag, "viewBox")
	if !ok {
		vb, ok = findAttr(openTag, "viewbox")
	}
	if ok {
		nums := parseNumberList(vb)
		if len(nums) >= 4 {
			return clampViewBoxDim(nums[2]), clampViewBoxDim(nums[3])
		}
	}
	w := float32(defaultIconSize)
	h := float32(defaultIconSize)
	if ws, ok := findAttr(openTag, "width"); ok {
		w = clampViewBoxDim(parseLength(ws))
	}
	if hs, ok := findAttr(openTag, "height"); ok {
		h = clampViewBoxDim(parseLength(hs))
	}
	return w, h
}

func extractRootSVGOpenTag(content string) string {
	start := strings.Index(content, "<svg")
	if start < 0 {
		return ""
	}
	nameEnd := start + len("<svg")
	if nameEnd < len(content) {
		switch content[nameEnd] {
		case '>', '/', ' ', '\t', '\n', '\r':
		default:
			return ""
		}
	}
	inQuote := byte(0)
	for i := nameEnd; i < len(content); i++ {
		switch c := content[i]; c {
		case '"', '\'':
			switch inQuote {
			case 0:
				inQuote = c
			case c:
				inQuote = 0
			}
		case '>':
			if inQuote == 0 {
				return content[start : i+1]
			}
		}
	}
	return content[start:]
}

// mintSynthID bumps n and returns prefix+N. Callers split counters
// per id namespace so concurrent prefixes never alias on the integer.
func mintSynthID(prefix string, n *int) string {
	*n++
	return prefix + strconv.Itoa(*n)
}

func (s *parseState) synthGroupID() string {
	return mintSynthID("__anim_", &s.synthID)
}

func (s *parseState) synthNestedClipID() string {
	return mintSynthID(synthNestedClipPrefix, &s.synthClipID)
}

// recordGroupParent registers child→parent in the GroupParent edge
// map, lazy-initing the map on first write.
func (s *parseState) recordGroupParent(child, parent string) {
	if s.groupParent == nil {
		s.groupParent = make(map[string]string, 16)
	}
	s.groupParent[child] = parent
}
