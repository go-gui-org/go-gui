// Thinking-orbs geometry engine, ported to Go from the Swift
// ThinkingOrbs library by Haplo LLC (MIT), itself hand-ported
// from Jakub Antalik's thinking-orbs (MIT). Every formula is
// transcribed term for term from the web engine, including its
// evaluation order, so output matches the golden vectors.
package gui

import (
	"maps"
	"math"
	"slices"
)

// ThinkingOrbDesign selects one of the nine hand-tuned spinner
// designs. Each is its own animation for one thing an AI or
// agent can be doing.
type ThinkingOrbDesign uint8

const (
	// ThinkingOrbWorking shows particles on tilted orbits.
	// General-purpose "busy".
	ThinkingOrbWorking ThinkingOrbDesign = iota
	// ThinkingOrbSearching sweeps a scan meridian across a
	// dotted globe. Web search, retrieval, lookup.
	ThinkingOrbSearching
	// ThinkingOrbSolving scrambles bands in quarter turns,
	// then clicks back solved. Reasoning, math, code.
	ThinkingOrbSolving
	// ThinkingOrbListening rolls a waveform through the
	// latitude rings. Voice input, transcription.
	ThinkingOrbListening
	// ThinkingOrbConnecting wires a constellation, packets
	// running the edges. Tool calls, APIs, sync.
	ThinkingOrbConnecting
	// ThinkingOrbWeaving plaits three strands around the
	// sphere. Planning, multi-step agents.
	ThinkingOrbWeaving
	// ThinkingOrbComposing waves an undulating multi-band
	// sash. Writing, generating a reply.
	ThinkingOrbComposing
	// ThinkingOrbBreathing slowly morphs a face-on ring.
	// Idle thinking, waiting on a model.
	ThinkingOrbBreathing
	// ThinkingOrbShaping morphs a dotted outline circle to
	// triangle to square. Design, image, layout work.
	ThinkingOrbShaping
)

// thinkingOrbTitles holds the display names in design order.
var thinkingOrbTitles = [9]string{
	"Working", "Searching", "Solving", "Listening",
	"Connecting", "Weaving", "Composing", "Breathing", "Shaping",
}

// thinkingOrbSummaries holds one line per design on what the
// animation shows, in design order.
var thinkingOrbSummaries = [9]string{
	"Particles on tilted orbits",
	"A scan meridian sweeps a dotted globe",
	"Bands scramble, then click back solved",
	"A waveform rolls through the rings",
	"A constellation wires itself",
	"Three strands plait around the sphere",
	"An undulating multi-band sash",
	"A ring slowly morphing",
	"Circle → triangle → square",
}

// Title returns the design's display name, such as "Searching".
func (d ThinkingOrbDesign) Title() string {
	if d > ThinkingOrbShaping {
		return thinkingOrbTitles[ThinkingOrbWorking]
	}
	return thinkingOrbTitles[d]
}

// Summary returns one line on what the animation shows.
func (d ThinkingOrbDesign) Summary() string {
	if d > ThinkingOrbShaping {
		return thinkingOrbSummaries[ThinkingOrbWorking]
	}
	return thinkingOrbSummaries[d]
}

// A11YLabel returns the VoiceOver label for the design.
func (d ThinkingOrbDesign) A11YLabel() string {
	if d == ThinkingOrbBreathing {
		return "Thinking…"
	}
	return d.Title() + "…"
}

// ThinkingOrbSize selects one of the two tuned sizes. They are
// separate designs, not one design scaled: each carries its own
// dot count, dot size and speed.
type ThinkingOrbSize uint8

const (
	// ThinkingOrbRegular is 64 pt, for chat avatars, empty
	// states and hero moments.
	ThinkingOrbRegular ThinkingOrbSize = iota
	// ThinkingOrbSmall is 20 pt, for sitting inline with
	// text: status chips, toolbars, list rows.
	ThinkingOrbSmall
)

// Length returns the size's side length in points.
func (s ThinkingOrbSize) Length() float64 {
	if s == ThinkingOrbSmall {
		return 20
	}
	return 64
}

// orbDot is one dot in a frame, in the frame's own point space
// (0…size on both axes). White is ink on paper, 0 (darkest) to
// 1 (white); a is opacity, 0 to 1.
type orbDot struct {
	x, y, z, r, white, a float64
}

// orbLine is a stroked edge between two dots (only connecting
// draws these). White and a match orbDot; w is the stroke width
// in points.
type orbLine struct {
	x1, y1, x2, y2, white, a, w float64
}

// orbFrameResult is one finished instant of an orb. Draw lines
// first, then dots in array order (far to near).
type orbFrameResult struct {
	dots  []orbDot
	lines []orbLine
}

// orbMode is the geometry builder a design runs on. Ring
// shares ribbon's builder.
type orbMode uint8

const (
	orbModeOrbits orbMode = iota
	orbModeGlobe
	orbModeRubik
	orbModeWave
	orbModeWeb
	orbModeBraid
	orbModeRibbon
	orbModeRing
	orbModeMorph
)

// orbModeNames holds the wire names in mode order.
var orbModeNames = [9]string{
	"orbits", "globe", "rubik", "wave", "web",
	"braid", "ribbon", "ring", "morph",
}

// String returns the mode's wire name, as in the golden file.
func (m orbMode) String() string {
	if m > orbModeMorph {
		return orbModeNames[orbModeOrbits]
	}
	return orbModeNames[m]
}

// mode maps a design to its geometry builder.
func (d ThinkingOrbDesign) mode() orbMode {
	switch d {
	case ThinkingOrbSearching:
		return orbModeGlobe
	case ThinkingOrbSolving:
		return orbModeRubik
	case ThinkingOrbListening:
		return orbModeWave
	case ThinkingOrbConnecting:
		return orbModeWeb
	case ThinkingOrbWeaving:
		return orbModeBraid
	case ThinkingOrbComposing:
		return orbModeRibbon
	case ThinkingOrbBreathing:
		return orbModeRing
	case ThinkingOrbShaping:
		return orbModeMorph
	default:
		return orbModeOrbits
	}
}

// orbOpts holds mode options, keyed exactly as the web engine
// keys them. A missing key falls back to the default written at
// its point of use.
type orbOpts map[string]float64

// orbOpt reads one option with its fallback default.
func orbOpt(o orbOpts, key string, fallback float64) float64 {
	if v, ok := o[key]; ok {
		return v
	}
	return fallback
}

// orbJsRound is JavaScript's Math.round: halves go toward +∞.
func orbJsRound(x float64) float64 { return math.Floor(x + 0.5) }

// orbFrac returns the fractional part of x.
func orbFrac(x float64) float64 { return x - math.Floor(x) }

// orbLerp blends a toward b by f.
func orbLerp(a, b, f float64) float64 { return a + (b-a)*f }

// orbHash is a deterministic hash in [0, 1).
func orbHash(a, b float64) float64 {
	h := math.Sin(a*12.9898+b*78.233) * 43758.5453
	return h - math.Floor(h)
}

// orbVnoise is value noise on a 2-D lattice: smooth,
// deterministic, cheap.
func orbVnoise(x, y float64) float64 {
	xi := math.Floor(x)
	yi := math.Floor(y)
	fx := x - xi
	fy := y - yi
	fx = fx * fx * (3 - 2*fx)
	fy = fy * fy * (3 - 2*fy)
	ha := orbHash(xi, yi)
	hb := orbHash(xi+1, yi)
	hc := orbHash(xi, yi+1)
	hd := orbHash(xi+1, yi+1)
	return ha + (hb-ha)*fx + (hc-ha)*fy + (ha-hb-hc+hd)*fx*fy
}

// orbFibDir returns stable directions on a unit sphere
// (Fibonacci lattice).
func orbFibDir(i int, n float64) (float64, float64, float64) {
	golden := math.Pi * (3 - math.Sqrt(5))
	yy := 1 - (2*(float64(i)+0.5))/n
	rad := math.Sqrt(1 - yy*yy)
	ang := float64(i) * golden
	return rad * math.Cos(ang), yy, rad * math.Sin(ang)
}

// orbAngleDelta is the shortest signed angular distance,
// wrapped to (-π, π].
func orbAngleDelta(a, b float64) float64 {
	return math.Atan2(math.Sin(a-b), math.Cos(a-b))
}

// orbRadiusScale scales dot radii, tuned for a 300-pt frame;
// sub-linear scaling keeps small spinners legible.
func orbRadiusScale(size, p float64) float64 {
	return math.Pow(size/300, p)
}

// orbProjector is the shared spin + tilt + orthographic
// projection.
type orbProjector struct {
	st, ct, sy, cyw, cx, cy, scale float64
}

func newOrbProjector(yaw, tilt, cx, cy, scale float64) orbProjector {
	return orbProjector{
		st: math.Sin(tilt), ct: math.Cos(tilt),
		sy: math.Sin(yaw), cyw: math.Cos(yaw),
		cx: cx, cy: cy, scale: scale,
	}
}

func (p orbProjector) project(x, y, z float64) (float64, float64, float64) {
	x1 := x*p.cyw + z*p.sy
	z1 := -x*p.sy + z*p.cyw
	y1 := y*p.ct - z1*p.st
	z2 := y*p.st + z1*p.ct
	return p.cx + x1*p.scale, p.cy - y1*p.scale, z2
}

// orbScratch holds the buffers a frame build writes into. A live
// orb rebuilds its frame on every tick, so the render path keeps
// one orbScratch per window (scratchPools.orb) and the builders
// append into it: after the first frames have grown the buffers,
// a rebuild allocates nothing. A frame built into a scratch
// aliases its buffers and is valid only until the next build.
type orbScratch struct {
	dots   []orbDot
	lines  []orbLine
	nodes  [][3]float64
	amount []float64
	moves  []orbMove
}

// orbFinalize drops invisible marks, clamps radii to the mode's
// floor, and z-sorts far → near (stable) into draw order. It
// filters in place and keeps the grown buffers in sc for the next
// frame.
func orbFinalize(sc *orbScratch, dots []orbDot, lines []orbLine,
	rMin float64) orbFrameResult {
	sc.dots = dots
	visible := dots[:0]
	for idx := range dots {
		mark := dots[idx]
		if mark.a < 0.02 {
			continue
		}
		if mark.r < rMin {
			mark.r = rMin
		}
		visible = append(visible, mark)
	}
	// Same strict order as a plain z < z test, so a NaN depth
	// sorts exactly as it did with sort.SliceStable. SortStableFunc
	// needs no reflection swapper and does not allocate.
	slices.SortStableFunc(visible, func(a, b orbDot) int {
		switch {
		case a.z < b.z:
			return -1
		case b.z < a.z:
			return 1
		}
		return 0
	})
	if lines != nil {
		sc.lines = lines
	}
	kept := lines[:0]
	for idx := range lines {
		if lines[idx].a >= 0.02 {
			kept = append(kept, lines[idx])
		}
	}
	return orbFrameResult{dots: visible, lines: kept}
}

// orbCountOpt keeps a count option as the raw number the web
// engine loops against, so even a fractional count behaves
// exactly as JS's `i < n` does.
func orbCountOpt(o orbOpts, key string, fallback float64) float64 {
	return orbOpt(o, key, fallback)
}

// orbBelow counts how many times `for (let i = 0; i < n; i++)`
// runs.
func orbBelow(n float64) int {
	if n > 0 {
		return int(math.Ceil(n))
	}
	return 0
}

// orbThrough counts how many times `for (let i = 0; i <= n; i++)`
// runs.
func orbThrough(n float64) int {
	if n >= 0 {
		return int(math.Floor(n)) + 1
	}
	return 0
}

// orbPreset is one shipped tuning: count and radii multiply the
// base profile, speed multiplies the shared clock.
type orbPreset struct {
	speed, count, radii float64
	extra               orbOpts
}

// orbPresetFor returns the shipped tuning for a mode and size.
func orbPresetFor(mode orbMode, size ThinkingOrbSize) orbPreset {
	small := size == ThinkingOrbSmall
	switch mode {
	case orbModeOrbits:
		if small {
			return orbPreset{speed: 3.9, count: 0.238, radii: 2.4}
		}
		return orbPreset{speed: 1.885, count: 1, radii: 1}
	case orbModeGlobe:
		if small {
			return orbPreset{speed: 2.665, count: 0.105, radii: 1.75,
				extra: orbOpts{"scanMul": 4.335, "dimBase": 0.45}}
		}
		return orbPreset{speed: 2.015, count: 0.42, radii: 1.15,
			extra: orbOpts{"scanMul": 4.08, "dimBase": 0.45}}
	case orbModeRubik:
		if small {
			return orbPreset{speed: 1.95, count: 0.088, radii: 1.9}
		}
		return orbPreset{speed: 1.82, count: 0.35, radii: 1.05}
	case orbModeWave:
		if small {
			return orbPreset{speed: 3.998, count: 0.105, radii: 1.6}
		}
		return orbPreset{speed: 4.388, count: 0.341, radii: 1}
	case orbModeWeb:
		if small {
			return orbPreset{speed: 6.63, count: 0.25, radii: 1.52}
		}
		return orbPreset{speed: 3.315, count: 1.35, radii: 0.95}
	case orbModeBraid:
		if small {
			return orbPreset{speed: 2.75, count: 0.1125, radii: 1.36}
		}
		return orbPreset{speed: 1.625, count: 0.5, radii: 1}
	case orbModeRibbon:
		if small {
			return orbPreset{speed: 3.12, count: 0.051, radii: 1.073,
				extra: orbOpts{"spin": 0, "bandMul": 4.94, "wobMul": 1}}
		}
		return orbPreset{speed: 2.34, count: 0.25, radii: 0.85,
			extra: orbOpts{"spin": 0, "bandMul": 3.9, "wobMul": 1}}
	case orbModeRing:
		if small {
			return orbPreset{speed: 3.78, count: 0.028, radii: 1.622,
				extra: orbOpts{"spin": 0, "bandMul": 3.968, "wobMul": 0.565}}
		}
		return orbPreset{speed: 3.24, count: 0.25, radii: 0.956,
			extra: orbOpts{"spin": 0, "bandMul": 3.627, "wobMul": 0.368}}
	default: // orbModeMorph
		if small {
			return orbPreset{speed: 2.08, count: 0.53, radii: 1.011,
				extra: orbOpts{"spread": 1.45}}
		}
		return orbPreset{speed: 2.405, count: 0.702, radii: 0.395,
			extra: orbOpts{"spread": 1.45}}
	}
}

// orbBaseOpts returns the base ("fine") profile per mode, before
// the preset multipliers.
func orbBaseOpts(mode orbMode) orbOpts {
	switch mode {
	case orbModeGlobe:
		return orbOpts{"latRings": 17, "lonDensity": 44, "rBase": 0.6,
			"rDepth": 1.7, "rBoost": 1.0, "inkFar": 0.62,
			"inkSpan": 0.54, "rsPow": 0.6, "rMin": 0.3}
	case orbModeOrbits:
		return orbOpts{"orbitN": 12, "ghostN": 40, "ghostR": 0.9,
			"ghostA": 0.5, "particles": 3, "partR": 1.2,
			"partRDepth": 1.6, "rsPow": 0.6, "rMin": 0.3}
	case orbModeRubik:
		return orbOpts{"latRings": 15, "lonDensity": 40, "moveCount": 14,
			"rBase": 0.6, "rDepth": 1.7, "rActive": 0.3,
			"inkFar": 0.62, "inkSpan": 0.54, "rsPow": 0.6, "rMin": 0.3}
	case orbModeWave:
		return orbOpts{"rings": 15, "lonDensity": 40, "rBase": 0.6,
			"rDepth": 1.7, "rsPow": 0.6, "rMin": 0.3}
	case orbModeWeb:
		return orbOpts{"nodeN": 30, "thr": 0.72, "signals": 5,
			"nodeR": 1.4, "nodeRDepth": 1.8, "lineW": 0.8,
			"rsPow": 0.6, "rMin": 0.3}
	case orbModeBraid:
		return orbOpts{"strandN": 52, "turns": 3.0, "ghostN": 150,
			"rBase": 1.2, "rDepth": 1.8, "rsPow": 0.6, "rMin": 0.3}
	case orbModeRibbon:
		return orbOpts{"lanes": 5, "segs": 88, "ghostN": 150,
			"rBase": 1.1, "rDepth": 1.7, "rsPow": 0.6, "rMin": 0.3}
	case orbModeRing:
		// Ribbon's builder; faceOn cancels the camera tilt and
		// moves the undulation onto the radius, with no ghost
		// sphere behind it.
		return orbOpts{"lanes": 5, "segs": 88, "ghostN": 0, "faceOn": 1,
			"rBase": 1.1, "rDepth": 1.7, "rsPow": 0.6, "rMin": 0.3}
	default: // orbModeMorph
		return orbOpts{"rDot": 0.021, "iconD": 1, "rMin": 0.25}
	}
}

// orbCountPairs scales 2-D lattices in pairs: each side takes
// √scale so the TOTAL count scales by scale. Flat lists scale
// linearly.
var orbCountPairs = [][2]string{
	{"latRings", "lonDensity"}, {"rings", "lonDensity"},
	{"lanes", "segs"},
}

var orbCountKeys = []string{
	"orbitN", "ghostN", "nodeN", "strandN", "signals",
}

var orbRadiusKeys = []string{
	"rBase", "rDepth", "rActive", "rDot", "ghostR", "partR",
	"partRDepth", "nodeR", "nodeRDepth",
}

func orbScaleCounts(opts orbOpts, scale float64) orbOpts {
	out := make(orbOpts, len(opts))
	maps.Copy(out, opts)
	done := make(map[string]bool, 4)
	rt := math.Sqrt(scale)
	for _, pair := range orbCountPairs {
		va, oka := out[pair[0]]
		vb, okb := out[pair[1]]
		if oka && okb && !done[pair[0]] && !done[pair[1]] {
			out[pair[0]] = math.Max(2, orbJsRound(va*rt))
			out[pair[1]] = math.Max(2, orbJsRound(vb*rt))
			done[pair[0]] = true
			done[pair[1]] = true
		}
	}
	for _, k := range orbCountKeys {
		// 0 opts a layer out entirely (ring has no ghost
		// sphere); scaling must not resurrect it as one
		// stray dot.
		if v, ok := out[k]; ok && v != 0 && !done[k] {
			out[k] = math.Max(1, orbJsRound(v*scale))
		}
	}
	if v, ok := out["iconD"]; ok {
		out["iconD"] = math.Max(0.02, v*scale)
	}
	return out
}

func orbScaleRadii(opts orbOpts, scale float64) orbOpts {
	out := make(orbOpts, len(opts)+1)
	maps.Copy(out, opts)
	for _, k := range orbRadiusKeys {
		if v, ok := out[k]; ok {
			out[k] = v * scale
		}
	}
	out["rSizeMul"] = orbOpt(out, "rSizeMul", 1) * scale
	return out
}

// orbResolved is a (design, size) pair resolved to its builder,
// baked speed and options.
type orbResolved struct {
	mode  orbMode
	speed float64
	opts  orbOpts
}

func orbComputeResolved(design ThinkingOrbDesign, size ThinkingOrbSize) orbResolved {
	mode := design.mode()
	preset := orbPresetFor(mode, size)
	opts := orbBaseOpts(mode)
	if preset.count != 1 {
		opts = orbScaleCounts(opts, preset.count)
	}
	if preset.radii != 1 {
		opts = orbScaleRadii(opts, preset.radii)
	}
	maps.Copy(opts, preset.extra)
	return orbResolved{mode: mode, speed: preset.speed, opts: opts}
}

// orbResolvedTable caches every (design, size) pair, so the
// render loop never rebuilds or rescales an option map. Builders
// still read options by key; a map read does not allocate.
var orbResolvedTable = buildOrbResolvedTable()

func buildOrbResolvedTable() [9][2]orbResolved {
	var table [9][2]orbResolved
	for di := ThinkingOrbWorking; di <= ThinkingOrbShaping; di++ {
		table[int(di)][int(ThinkingOrbRegular)] =
			orbComputeResolved(di, ThinkingOrbRegular)
		table[int(di)][int(ThinkingOrbSmall)] =
			orbComputeResolved(di, ThinkingOrbSmall)
	}
	return table
}

// orbResolve returns a (design, size) pair's builder and fully
// scaled options. Cached.
func orbResolve(design ThinkingOrbDesign, size ThinkingOrbSize) orbResolved {
	if design > ThinkingOrbShaping {
		design = ThinkingOrbWorking
	}
	if size != ThinkingOrbSmall {
		size = ThinkingOrbRegular
	}
	return orbResolvedTable[int(design)][int(size)]
}

// orbFrame returns the draw list for a design at geometry time t
// (seconds on the orb clock, already scaled by the resolved
// speed). Any time is safe, negative or not; a non-finite one
// draws time zero. An invalid design draws working. The result
// owns fresh buffers; the render path uses orbFrameInto.
func orbFrame(design ThinkingOrbDesign, size ThinkingOrbSize, t float64) orbFrameResult {
	var sc orbScratch
	return orbFrameInto(&sc, design, size, t)
}

// orbFrameInto is orbFrame building into sc. The result aliases
// sc's buffers and is valid until the next build into sc.
func orbFrameInto(sc *orbScratch, design ThinkingOrbDesign,
	size ThinkingOrbSize, t float64) orbFrameResult {
	resolved := orbResolve(design, size)
	tt := t
	if math.IsNaN(tt) || math.IsInf(tt, 0) {
		tt = 0
	}
	return orbModeFrame(sc, resolved.mode, size.Length(), tt, resolved.opts)
}

// orbModeFrame dispatches to the geometry builder.
func orbModeFrame(sc *orbScratch, mode orbMode, size, t float64,
	o orbOpts) orbFrameResult {
	switch mode {
	case orbModeOrbits:
		return orbOrbits(sc, size, t, o)
	case orbModeGlobe:
		return orbGlobe(sc, size, t, o)
	case orbModeRubik:
		return orbRubik(sc, size, t, o)
	case orbModeWave:
		return orbWave(sc, size, t, o)
	case orbModeWeb:
		return orbWeb(sc, size, t, o)
	case orbModeBraid:
		return orbBraid(sc, size, t, o)
	case orbModeRibbon, orbModeRing:
		return orbRibbon(sc, size, t, o)
	default:
		return orbMorph(sc, size, t, o)
	}
}
