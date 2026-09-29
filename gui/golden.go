package gui

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui/internal/atomicfile"
)

// Public golden-file helpers for the render pipeline (issue #847).
//
// The in-repo suite pins every widget's appearance as text: build a
// view, run the real frame pipeline to w.renderers, serialize the
// emitted []RenderCmd to a stable form, and compare against a recorded
// file in testdata/. The serializer below used to live in a _test.go
// file, so apps and sibling repos could not call it. TestGolden is
// that same harness as a public method, so a consumer pins its own
// appearance the same way:
//
//	w.TestRender(myView)
//	w.TestClick("ok")
//	w.TestGolden(t, GoldenCfg{Name: "ok-clicked"})
//
// Scope and limits, same as the in-repo suite:
//
//   - TextMeasurer is nil in headless tests, so text advances come from
//     the fallback measurement path. Goldens pin colors, alphas,
//     insets and geometry, not glyph metrics.
//   - Floats are rounded to two decimals so platform FP jitter does
//     not red the suite.
//   - FontName is not recorded: it resolves to a platform default
//     where a font is reachable and stays empty where it is not.
//   - Pointer fields (styles, layouts) are summarized by presence, not
//     dereferenced. A golden is a fingerprint, not a serialization
//     format. The gradient pointer is the exception: it is recorded by
//     its stop list, because since the concentric radial fill lowered
//     to a shader quad (#462) that ramp is all the command carries.
//   - A triangle batch records a coordinate fingerprint (bbox,
//     centroid, digest) as well as its counts, and the digest hashes
//     values rounded to the same two decimals everything else prints.
//   - These pin commands, not pixels. A backend rasterizer bug is
//     invisible here; the soft backend's pixel goldens cover that.
//
// Re-record after reading the diff:
//
//	GOGUI_UPDATE_GOLDEN=1 go test ./... -run Golden
//
// The trigger is an env var on purpose: a flag.Bool in a library
// package collides with the consumer's own flags at link time.

// Format version of the text serializeCmds writes. A format change
// bumps this, so an old recording fails with a re-record hint naming
// both versions instead of a wall of line diffs.
const goldenFormatVersion = 1

// goldenUpdate reports whether goldens re-record instead of compare.
// The trigger is an env var on purpose: a flag.Bool in a library
// package collides with the consumer's own flags at link time.
func goldenUpdate() bool { return envTruthy("GOGUI_UPDATE_GOLDEN") }

// goldenHeader prefixes every golden file. Keep the version in sync
// with goldenFormatVersion: files without this header predate
// versioned goldens and fail with a re-record hint.
const goldenHeader = "# go-gui-golden v1\n"

// goldenInstant is the fixed clock goldens record under. A widget
// that rings "today" — the date picker's calendar — otherwise records
// a different cell every day, so the recording rots overnight rather
// than when the widget changes.
func goldenInstant() time.Time {
	return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
}

// GoldenTB is the part of testing.TB that TestGolden needs.
// *testing.T, *testing.B and *testing.F satisfy it. An interface
// rather than testing.TB keeps package testing out of the library's
// import graph, the same reason NewTestWindow takes a Cleanuper.
// exportaudit:keep — reachable from an exported signature
type GoldenTB interface {
	Helper()
	Logf(format string, args ...any)
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// GoldenCfg configures one TestGolden assertion. The zero value
// records the window's current state under Dir/Name in both themes;
// set the interaction fields to pin a non-resting appearance.
type GoldenCfg struct {
	// Dir holds the goldens; "testdata" when empty. Files are
	// <Name>.dark.golden and <Name>.light.golden inside it.
	Dir string
	// Name is the file base name, without theme or extension.
	Name string
	// FocusID, when set, is focused before the frame renders.
	// Focus resolves after layout, so a focus ring only appears in
	// a recording that actually holds focus.
	// exportaudit:keep — caller-facing config (issue #847)
	FocusID string
	// KeyPressID, when set, is held pressed by a Space key down
	// before the frame renders (#658). Set after FocusID, because
	// a focus change cancels a key press.
	// exportaudit:keep — caller-facing config (issue #847)
	KeyPressID string
	// HoverX, HoverY place the pointer before the frame renders,
	// so the recording pins the hovered appearance. Take the
	// coordinates from the case's own resting recording, not by
	// guessing: a point that misses fails the assertion rather
	// than recording the resting look under a hovered name.
	// exportaudit:keep — caller-facing config (issue #847)
	HoverX, HoverY float32
	// MousePressed holds the left button down for the hovered
	// frame, pinning the pressed-by-mouse appearance the way
	// KeyPressID pins the pressed-by-Space one (#658).
	//
	// A bool rather than a MouseButton because MouseLeft is 0: a
	// MouseButton field would mark every cfg that did not mention
	// it as pressed.
	// exportaudit:keep — caller-facing config (issue #847)
	MousePressed bool
	// HoverInert says the pointer is expected to land on nothing,
	// which is what a disabled widget does. Without this the hit
	// assert below would fail every disabled-hover case, which are
	// the ones worth recording.
	// exportaudit:keep — caller-facing config (issue #847)
	HoverInert bool
}

// TestGolden pins the window's appearance as text, in both themes.
// For each of ThemeDark and ThemeLight it installs the theme, runs
// one frame of the window's current view, serializes the emitted
// []RenderCmd, and compares against
// <Dir>/<Name>.<theme>.golden, recording it instead when
// GOGUI_UPDATE_GOLDEN is set.
//
// Drive state before calling: TestRender the view, then TestClick,
// TestType, SetFocus or whatever the case needs. The cfg's FocusID,
// KeyPressID, Hover and MousePressed cover the appearances those
// cannot reach (a held Space, a hover with no click).
//
// The clock is pinned to a fixed instant for the two frames, so a
// view reading Now records the same output every day, and the
// window's theme and pointer state are restored afterwards. Like
// SetTheme, this ends system-appearance following for the window.
func (w *Window) TestGolden(tb GoldenTB, cfg GoldenCfg) {
	tb.Helper()
	if cfg.Name == "" {
		tb.Fatalf("gui: TestGolden needs GoldenCfg.Name")
	}
	dir := cfg.Dir
	if dir == "" {
		dir = "testdata"
	}
	// Pin the clock (goldenInstant), so the two frames record the
	// same output every day. Restored below with the theme.
	now := goldenInstant()
	w.setVirtualNow(&now)
	defer w.setVirtualNow(nil)
	prevTheme := w.Theme()
	defer w.pinTheme(prevTheme)
	prevHeld := w.viewState.mouseButtonHeld
	defer func() { w.viewState.mouseButtonHeld = prevHeld }()
	prevKeyPress := w.viewState.keyPressTargetID
	defer func() { w.viewState.keyPressTargetID = prevKeyPress }()
	prevMouseX, prevMouseY := w.viewState.mousePosX, w.viewState.mousePosY
	prevPointerX, prevPointerY := w.viewState.pointerX, w.viewState.pointerY
	prevInWindow := w.viewState.pointerInWindow
	prevHoverTarget := w.viewState.hoverTargetID
	defer func() {
		w.viewState.mousePosX, w.viewState.mousePosY = prevMouseX, prevMouseY
		w.viewState.pointerX, w.viewState.pointerY = prevPointerX, prevPointerY
		w.viewState.pointerInWindow = prevInWindow
		w.viewState.hoverTargetID = prevHoverTarget
	}()

	for _, th := range goldenThemes() {
		name := cfg.Name + "." + th.name
		got := goldenFrame(tb, w, th.theme, cfg, name)
		checkGoldenFile(tb, dir, name, got)
	}
}

// goldenFrame runs one golden frame of w's current view under theme
// and returns the header-prefixed serialized command list. Shared by
// TestGolden (both themes) and the in-repo single-theme callers.
func goldenFrame(
	tb GoldenTB, w *Window, theme Theme, cfg GoldenCfg, name string,
) string {
	tb.Helper()
	if cfg.FocusID != "" {
		w.SetFocus(cfg.FocusID)
	}
	// Set after focus: a focus change cancels a key press.
	w.viewState.keyPressTargetID = cfg.KeyPressID
	if cfg.HoverX != 0 || cfg.HoverY != 0 {
		// Two positions, on purpose. mousePosX/Y is what
		// layoutHover dispatches OnHover from, which is what paints
		// a hover color today. pointerAt is what recordHoverTarget
		// reads, and it also sets pointerInWindow — without it the
		// frame records no hover target at all, because the pointer
		// is taken to have never entered the window
		// (docs/specs/build-time-interaction-state.md, rule 7).
		w.viewState.mousePosX, w.viewState.mousePosY = cfg.HoverX, cfg.HoverY
		w.pointerAt(cfg.HoverX, cfg.HoverY)
	}
	if cfg.MousePressed {
		w.viewState.mouseButtonHeld = MouseLeft
	}
	// Through FrameFn rather than TestRender so the golden covers
	// what the app actually runs — including installTheme, which is
	// what makes the theme argument mean anything, and the second
	// pass a deferred callback's state change needs.
	w.SetTheme(theme)
	w.refreshLayout.Store(true)
	w.FrameFn()

	if len(w.renderers) == 0 {
		tb.Fatalf("gui: golden %s: pipeline emitted no render commands", name)
	}
	// A hover case whose point misses its widget records the resting
	// appearance while claiming to be hovered, which is worse than no
	// case at all: it reds nothing and asserts nothing.
	// recordHoverTarget runs at the end of layoutArrange, so an empty
	// target here means the pointer landed on nothing.
	if (cfg.HoverX != 0 || cfg.HoverY != 0) && !cfg.HoverInert &&
		w.viewState.hoverTargetID == "" {
		tb.Fatalf("gui: golden %s: hover point (%v,%v) is over nothing; "+
			"read the resting golden for this case and use a point "+
			"inside the widget", name, cfg.HoverX, cfg.HoverY)
	}
	return goldenHeader + serializeCmds(w.renderers)
}

// checkGoldenFile compares a render-command golden against
// <dir>/<name>.golden, or re-records it when GOGUI_UPDATE_GOLDEN is
// set. The recording must carry the version header; one without it
// (or with a stale version) fails with a re-record hint instead of a
// wall of line diffs. On a mismatch the recording lands under
// <dir>/failures/ so a red run stays reviewable.
func checkGoldenFile(tb GoldenTB, dir, name, got string) {
	tb.Helper()
	checkGolden(tb, dir, name, got, true)
}

// checkGoldenText compares a non-command golden — the theme surface
// listing, which is a field inventory rather than render commands and
// so carries no version header. Otherwise identical to
// checkGoldenFile.
func checkGoldenText(tb GoldenTB, dir, name, got string) {
	tb.Helper()
	checkGolden(tb, dir, name, got, false)
}

// checkGolden compares got against <dir>/<name>.golden, or re-records
// it when GOGUI_UPDATE_GOLDEN is set. versioned enables the header
// and format-version gate for render-command goldens.
func checkGolden(tb GoldenTB, dir, name, got string, versioned bool) {
	tb.Helper()

	path := filepath.Join(dir, name+".golden")
	if goldenUpdate() {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			tb.Fatalf("gui: mkdir %s: %v", dir, err)
		}
		if err := atomicfile.WriteFile(path, []byte(got), 0o644); err != nil {
			tb.Fatalf("gui: write %s: %v", path, err)
		}
		tb.Logf("gui: recorded %s", path)
		return
	}

	want, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		writeGoldenArtifact(dir, name, got)
		tb.Fatalf("gui: read %s: %v "+
			"(run with GOGUI_UPDATE_GOLDEN=1 to record)", path, err)
	}
	if versioned {
		ver, ok := parseGoldenHeader(string(want))
		if !ok {
			writeGoldenArtifact(dir, name, got)
			tb.Fatalf("gui: %s has no golden header "+
				"(run with GOGUI_UPDATE_GOLDEN=1 to record)", path)
		}
		if ver != goldenFormatVersion {
			writeGoldenArtifact(dir, name, got)
			tb.Fatalf("gui: %s uses golden format v%d, this library writes v%d "+
				"(run with GOGUI_UPDATE_GOLDEN=1 to record)", path, ver,
				goldenFormatVersion)
		}
	}
	if string(want) != got {
		writeGoldenArtifact(dir, name, got)
		tb.Errorf("gui: golden mismatch for %s\n%s",
			name, goldenDiff(string(want), got))
	}
}

// parseGoldenHeader reads the "# go-gui-golden vN" first line. ok is
// false when the file predates versioned goldens.
func parseGoldenHeader(s string) (ver int, ok bool) {
	line, _, _ := strings.Cut(s, "\n")
	var v int
	if _, err := fmt.Sscanf(line, "# go-gui-golden v%d", &v); err != nil {
		return 0, false
	}
	return v, true
}

// writeGoldenArtifact saves a mismatched recording under
// <dir>/failures/ so a red run is reviewable, locally and as a CI
// artifact. Best effort: a write failure must not mask the mismatch
// it records.
func writeGoldenArtifact(dir, name, got string) {
	fdir := filepath.Join(dir, "failures")
	if err := os.MkdirAll(fdir, 0o750); err != nil {
		return
	}
	_ = atomicfile.WriteFile(
		filepath.Join(fdir, name+".actual.golden"), []byte(got), 0o644)
}

// goldenDiff reports the first differing line plus a little context.
// A full diff of a few hundred command lines buries the signal; the
// first divergence is almost always the whole story.
func goldenDiff(want, got string) string {
	wl := strings.Split(strings.TrimRight(want, "\n"), "\n")
	gl := strings.Split(strings.TrimRight(got, "\n"), "\n")

	var b strings.Builder
	n := max(len(wl), len(gl))
	shown := 0
	for i := range n {
		var wv, gv string
		if i < len(wl) {
			wv = wl[i]
		}
		if i < len(gl) {
			gv = gl[i]
		}
		if wv == gv {
			continue
		}
		fmt.Fprintf(&b, "line %d:\n  want: %s\n   got: %s\n", i+1, wv, gv)
		shown++
		if shown == 5 {
			b.WriteString("  (further differences elided)\n")
			break
		}
	}
	if shown == 0 {
		fmt.Fprintf(&b, "line counts differ: want %d, got %d\n",
			len(wl), len(gl))
	}
	return b.String()
}

// renderKindNames maps the render kinds a golden can contain to a
// stable label. A kind missing here serializes as its number, which
// is still diffable — add the name when a case starts emitting it.
var renderKindNames = map[renderKind]string{
	RenderNone:              "None",
	RenderClip:              "Clip",
	RenderRect:              "Rect",
	RenderStrokeRect:        "StrokeRect",
	RenderCircle:            "Circle",
	RenderImage:             "Image",
	RenderText:              "Text",
	RenderLine:              "Line",
	RenderShadow:            "Shadow",
	RenderBlur:              "Blur",
	RenderGradient:          "Gradient",
	RenderGradientBorder:    "GradientBorder",
	RenderSvg:               "Svg",
	RenderLayout:            "Layout",
	RenderLayoutTransformed: "LayoutTransformed",
	RenderLayoutPlaced:      "LayoutPlaced",
	RenderFilterBegin:       "FilterBegin",
	RenderFilterEnd:         "FilterEnd",
	RenderFilterComposite:   "FilterComposite",
	RenderCustomShader:      "CustomShader",
	RenderTextPath:          "TextPath",
	RenderRTF:               "RTF",
	RenderRotateBegin:       "RotateBegin",
	RenderRotateEnd:         "RotateEnd",
	RenderStencilBegin:      "StencilBegin",
	RenderStencilEnd:        "StencilEnd",
}

func renderKindName(k renderKind) string {
	if n, ok := renderKindNames[k]; ok {
		return n
	}
	return fmt.Sprintf("Kind(%d)", k)
}

// f2 renders a float at two decimals, normalizing negative zero so a
// -0.00 never diffs against 0.00.
func f2(v float32) string {
	if v == 0 {
		v = 0
	}
	return fmt.Sprintf("%.2f", v)
}

// roundF2 rounds to the precision f2 prints, normalizing negative zero
// so a value that prints 0.00 also hashes as 0.
func roundF2(v float32) float32 {
	r := float32(math.Round(float64(v)*100) / 100)
	if r == 0 {
		r = 0
	}
	return r
}

// triFingerprint summarizes a flat x,y triangle list: the bounding
// box, the centroid, and a digest of every coordinate.
//
// The three answer different questions. The bbox catches a batch drawn
// at the wrong size or place — the #449 fan band emitted at the full
// circle radius. The centroid separates a shape that moved from one
// that grew. The digest catches interior rearrangement that leaves
// both alone, which is exactly what a triangle count cannot see.
//
// Rounding before hashing is load bearing: raw float32 bits would red
// every golden on the first ULP of drift from an unrelated math
// change, and the suite would then be re-recorded without being read.
// Rounded, the digest is no more fragile than the numbers the file
// already records.
func triFingerprint(tris []float32) string {
	if len(tris) < 2 {
		return ""
	}
	minX, minY, maxX, maxY := triBounds(tris)

	var sumX, sumY float64
	n := 0
	for i := 0; i+1 < len(tris); i += 2 {
		sumX += float64(tris[i])
		sumY += float64(tris[i+1])
		n++
	}

	h := fnv.New32a()
	var buf [4]byte
	for _, v := range tris {
		binary.LittleEndian.PutUint32(buf[:], math.Float32bits(roundF2(v)))
		_, _ = h.Write(buf[:])
	}

	return fmt.Sprintf(" bbox=%s,%s..%s,%s centroid=%s,%s digest=%08x",
		f2(minX), f2(minY), f2(maxX), f2(maxY),
		f2(float32(sumX/float64(n))), f2(float32(sumY/float64(n))),
		h.Sum32())
}

// gradientStr fingerprints a gradient definition by its ramp. Presence
// alone stopped being enough when the concentric radial fill lowered to
// a shader quad: that command carries no triangles, so the stop list is
// the only record of what the fill paints. Every stop is printed, not a
// sample — the stop list is the ramp.
func gradientStr(g *GradientDef) string {
	var b strings.Builder
	b.WriteString(" +gradient(")
	switch {
	case g.Type == GradientRadial:
		b.WriteString("radial")
	case g.hasAngle:
		b.WriteString("linear angle=" + f2(g.angle))
	default:
		fmt.Fprintf(&b, "linear dir=%d", g.Direction)
	}
	fmt.Fprintf(&b, " stops=%d", len(g.Stops))
	if len(g.Stops) > 0 {
		b.WriteString(" [")
		for i, s := range g.Stops {
			if i > 0 {
				b.WriteString(" | ")
			}
			b.WriteString(f2(s.Pos) + " " + colorStr(s.Color))
		}
		b.WriteString("]")
	}
	b.WriteString(")")
	return b.String()
}

// colorStr renders a Color as a diffable token. Unset colors are
// distinguished from explicit transparent black, because that
// distinction is exactly what Color's set flag exists to carry.
func colorStr(c Color) string {
	if !c.IsSet() {
		return "unset"
	}
	return fmt.Sprintf("#%02x%02x%02x%02x", c.R, c.G, c.B, c.A)
}

// matrixStr renders a 4x4 color matrix as a diffable token. A filter
// bracket with the wrong matrix paints the wrong glow, and presence
// alone cannot see it.
func matrixStr(m *[16]float32) string {
	var b strings.Builder
	b.WriteString("[")
	for i, v := range m {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(f2(v))
	}
	b.WriteString("]")
	return b.String()
}

// serializeCmd renders one command as a single line. Only fields
// meaningful for the kind are emitted, so a golden stays readable and
// a diff points at what actually changed.
func serializeCmd(c RenderCmd) string {
	var b strings.Builder
	b.WriteString(renderKindName(c.Kind))
	fmt.Fprintf(&b, " xy=%s,%s wh=%s,%s",
		f2(c.X), f2(c.Y), f2(c.W), f2(c.H))

	if c.Color.IsSet() {
		b.WriteString(" color=" + colorStr(c.Color))
	}
	if c.Radius != 0 {
		b.WriteString(" radius=" + f2(c.Radius))
	}
	if c.Thickness != 0 {
		b.WriteString(" thickness=" + f2(c.Thickness))
	}
	if c.BlurRadius != 0 {
		b.WriteString(" blur=" + f2(c.BlurRadius))
	}
	if c.Fill {
		b.WriteString(" fill")
	}

	switch c.Kind {
	case RenderText, RenderRTF, RenderTextPath:
		fmt.Fprintf(&b, " text=%q", c.Text)
		if c.FontSize != 0 {
			b.WriteString(" size=" + f2(c.FontSize))
		}
		if c.TextWidth != 0 {
			b.WriteString(" tw=" + f2(c.TextWidth))
		}
		if c.LayoutTransform != nil {
			t := c.LayoutTransform
			fmt.Fprintf(&b, " affine=[%s,%s,%s,%s,%s,%s]",
				f2(t.XX), f2(t.XY), f2(t.YX), f2(t.YY),
				f2(t.X0), f2(t.Y0))
		}
	case RenderImage:
		fmt.Fprintf(&b, " res=%q", c.Resource)
		if c.ClipRadius != 0 {
			b.WriteString(" cradius=" + f2(c.ClipRadius))
		}
		if c.Opacity != 1 {
			b.WriteString(" opa=" + f2(c.Opacity))
		}
	case RenderSvg:
		fmt.Fprintf(&b, " tris=%d", len(c.Triangles))
		// A vertex-colored batch is a gradient fill. Record the count
		// and both ends of the ramp: the count pins the tessellation
		// the subdivision pass chose, and the endpoint colors pin the
		// shading, which is the part a reader would otherwise have to
		// take on trust.
		if len(c.VertexColors) > 0 {
			fmt.Fprintf(&b, " vcols=%d first=%s last=%s",
				len(c.VertexColors),
				colorStr(c.VertexColors[0]),
				colorStr(c.VertexColors[len(c.VertexColors)-1]))
		}
		// A canvas transform rides on the command rather than on the
		// vertices, so the fingerprint below is identical with and
		// without it. Record the matrix or a transform golden proves
		// nothing.
		if c.HasXform {
			fmt.Fprintf(&b, " xform=[%s,%s,%s,%s]",
				f2(c.ScaleX), f2(c.ScaleY), f2(c.TransX), f2(c.TransY))
		}
		// Counts and endpoint colors say how much was emitted and how
		// it was shaded; the fingerprint says where the vertices are.
		b.WriteString(triFingerprint(c.Triangles))
	case RenderLine:
		fmt.Fprintf(&b, " from=%s,%s", f2(c.OffsetX), f2(c.OffsetY))
	case RenderShadow:
		fmt.Fprintf(&b, " offset=%s,%s", f2(c.OffsetX), f2(c.OffsetY))
		if c.Spread != 0 {
			b.WriteString(" spread=" + f2(c.Spread))
		}
	case RenderRotateBegin:
		fmt.Fprintf(&b, " rot=%s@%s,%s",
			f2(c.RotAngle), f2(c.RotCX), f2(c.RotCY))
	case RenderStencilBegin, RenderStencilEnd:
		fmt.Fprintf(&b, " sdepth=%d", c.StencilDepth)
	case RenderFilterBegin, RenderFilterComposite:
		fmt.Fprintf(&b, " layers=%d", c.Layers)
		if c.ColorMatrix != nil {
			b.WriteString(" cmatrix=" + matrixStr(c.ColorMatrix))
		}
	}

	// Pointers are fingerprinted by presence. Dereferencing them
	// would couple the golden to struct layout without adding
	// signal about what a user sees.
	if c.Gradient != nil {
		b.WriteString(gradientStr(c.Gradient))
	}
	if c.Shader != nil {
		b.WriteString(" +shader")
	}
	if c.LayoutPtr != nil {
		fmt.Fprintf(&b, " +glyphlayout glyphs=%d items=%d",
			len(c.LayoutPtr.Glyphs), len(c.LayoutPtr.Items))
	}
	if c.LayoutTransform != nil && c.Kind != RenderText {
		t := c.LayoutTransform
		fmt.Fprintf(&b, " affine=[%s,%s,%s,%s,%s,%s]",
			f2(t.XX), f2(t.XY), f2(t.YX), f2(t.YY),
			f2(t.X0), f2(t.Y0))
	}
	return b.String()
}

func serializeCmds(cmds []RenderCmd) string {
	var b strings.Builder
	for _, c := range cmds {
		b.WriteString(serializeCmd(c))
		b.WriteByte('\n')
	}
	return b.String()
}

// goldenThemes are recorded for every case. Two themes is the point:
// a de-emphasis alpha that reads as "quiet" on dark can read as
// "nearly gone" on light, and only a side-by-side golden catches it.
//
// A function, not a package var: ThemeDark and ThemeLight are assigned
// inside init (gui/theme_defaults.go), and package-level variable
// initialization runs before init. A var here would capture two zero
// Themes, which install cleanly and then render almost nothing.
func goldenThemes() []struct {
	theme Theme
	name  string
} {
	return []struct {
		theme Theme
		name  string
	}{
		{ThemeDark, "dark"},
		{ThemeLight, "light"},
	}
}
