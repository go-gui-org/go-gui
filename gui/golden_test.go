package gui

// Golden-file harness for the render pipeline.
//
// Every visual claim in this package was previously verified by one
// person looking at a screen once. These tests make a visual change
// reviewable as a diff: build a widget, run the real pipeline to
// w.renderers, serialize the emitted []RenderCmd to a stable text
// form, and compare against a recorded file in testdata/.
//
// Re-record after reading the diff:
//
//	go test ./gui/ -run TestGolden -update
//
// Scope and limits:
//
//   - TextMeasurer is nil in tests, so text advances come from the
//     fallback measurement path. That is fine: these goldens exist to
//     pin colors, alphas, insets and geometry, not glyph metrics.
//   - Floats are rounded to two decimals so platform FP jitter does
//     not red the suite.
//   - FontName is not recorded: it resolves to a platform default
//     (Segoe UI on Windows) where a font is reachable and stays empty
//     where it is not, so it is noise against the geometry and colors
//     these files pin.
//   - Pointer fields (styles, gradients, layouts) are summarized by
//     presence, not dereferenced. A golden is a fingerprint, not a
//     serialization format.
//
// Each case is recorded in both ThemeDark and ThemeLight, which is
// what makes a per-theme styling decision checkable.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var updateGolden = flag.Bool("update", false,
	"re-record golden render-command files")

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
	RenderTermGrid:          "TermGrid",
}

func renderKindName(k renderKind) string {
	if n, ok := renderKindNames[k]; ok {
		return n
	}
	return fmt.Sprintf("Kind(%d)", k)
}

// goldenWindowSize is fixed so a golden never encodes the machine it
// was recorded on.
const (
	goldenWidth  = 320
	goldenHeight = 240
)

// f2 renders a float at two decimals, normalizing negative zero so a
// -0.00 never diffs against 0.00.
func f2(v float32) string {
	if v == 0 {
		v = 0
	}
	return fmt.Sprintf("%.2f", v)
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
	case RenderImage:
		fmt.Fprintf(&b, " res=%q", c.Resource)
	case RenderSvg:
		fmt.Fprintf(&b, " tris=%d", len(c.Triangles))
	case RenderLine:
		fmt.Fprintf(&b, " from=%s,%s", f2(c.OffsetX), f2(c.OffsetY))
	case RenderShadow:
		fmt.Fprintf(&b, " offset=%s,%s", f2(c.OffsetX), f2(c.OffsetY))
	}

	// Pointers are fingerprinted by presence. Dereferencing them
	// would couple the golden to struct layout without adding
	// signal about what a user sees.
	if c.Gradient != nil {
		b.WriteString(" +gradient")
	}
	if c.Shader != nil {
		b.WriteString(" +shader")
	}
	if c.LayoutPtr != nil {
		b.WriteString(" +glyphlayout")
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

// renderGolden drives the real frame pipeline for one view under one
// theme and returns the serialized command list.
//
// It goes through FrameFn rather than calling renderLayout directly so
// the golden covers what the app actually runs — including
// installTheme, which is what makes the theme argument mean anything.
func renderGolden(t *testing.T, theme Theme, c goldenCase) string {
	t.Helper()

	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  goldenWidth,
		Height: goldenHeight,
	})
	// Wrap in a filling root. A widget generated as the bare root
	// sizes Fit, and with a nil TextMeasurer several collapse to zero
	// and emit nothing. Real apps put widgets inside a layout, so the
	// wrapper is both what makes the case record and what the widget
	// actually sees.
	w.viewGenerator = func(win *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{c.build(win)},
		})
	}
	// Pin the clock. A widget that rings "today" — the date picker's
	// calendar — otherwise records a different cell every day, so the
	// recording rots overnight rather than when the widget changes.
	goldenNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	w.setVirtualNow(&goldenNow)
	w.SetTheme(theme)
	if c.focusID != "" {
		w.SetFocus(c.focusID)
	}
	w.refreshLayout = true
	w.FrameFn()

	if len(w.renderers) == 0 {
		t.Fatal("pipeline emitted no render commands")
	}
	return serializeCmds(w.renderers)
}

// checkGolden compares got against testdata/<name>.golden, or
// re-records it under -update.
func checkGolden(t *testing.T, name, got string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("recorded %s", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (run with -update to record)", path, err)
	}
	if string(want) != got {
		t.Errorf("golden mismatch for %s\n%s",
			name, goldenDiff(string(want), got))
	}
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

// goldenCase is one recorded widget. build receives the window so a
// case can reach window-scoped state; most ignore it.
type goldenCase struct {
	build func(*Window) View
	name  string
	// focusID, when set, is focused before the frame is rendered.
	// Focus resolves after layout, so a focus ring only appears in a
	// recording that actually holds focus — a case without this pins
	// the resting appearance and would not notice a ring regressing.
	focusID string
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

func TestGolden(t *testing.T) {
	for _, c := range goldenCases() {
		for _, th := range goldenThemes() {
			name := c.name + "." + th.name
			t.Run(name, func(t *testing.T) {
				got := renderGolden(t, th.theme, c)
				checkGolden(t, name, got)
			})
		}
	}
}
