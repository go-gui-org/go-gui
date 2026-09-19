package svg

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// A gradient keeps the first maxGradientStops stops. Extra stops
// add no visible detail and each one slows the per-vertex scan.
func TestParseGradientStopsCapsCount(t *testing.T) {
	grad := &xmlNode{Name: "linearGradient"}
	for range maxGradientStops + 50 {
		grad.Children = append(grad.Children, xmlNode{
			Name:    "stop",
			OpenTag: `<stop offset="0" stop-color="#ffffff"/>`,
		})
	}
	stops := parseGradientStops(grad)
	if len(stops) != maxGradientStops {
		t.Fatalf("kept %d stops, want %d",
			len(stops), maxGradientStops)
	}
}

// A missing or malformed opacity must leave the fallback, never
// turn the element transparent.
func TestParseOpacityAttrFallsBack(t *testing.T) {
	cases := []struct {
		name     string
		elem     string
		fallback float32
		want     float32
	}{
		{"missing", `<rect/>`, 1.0, 1.0},
		{"garbage", `<rect opacity="garbage"/>`, 1.0, 1.0},
		{"empty", `<rect opacity=""/>`, 1.0, 1.0},
		{"nan", `<rect opacity="NaN"/>`, 1.0, 1.0},
		{"valid", `<rect opacity="0.5"/>`, 1.0, 0.5},
		{"high clips", `<rect opacity="2"/>`, 1.0, 1.0},
		{"low clips", `<rect opacity="-1"/>`, 1.0, 0},
	}
	for _, c := range cases {
		got := parseOpacityAttr(c.elem, "opacity", c.fallback)
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// A bad scale must not change tessellation. NaN used to force
// full subdivision of each curve.
func TestSanitizeTessScale(t *testing.T) {
	cases := []struct {
		name string
		in   float32
		want float32
	}{
		{"NaN", float32(math.NaN()), 1},
		{"+Inf", float32(math.Inf(1)), 1},
		{"zero", 0, 1},
		{"negative", -2, 1},
		{"kept", 2, 2},
	}
	for _, c := range cases {
		if got := sanitizeTessScale(c.in); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

// A NaN scale tessellates exactly like scale 1 and terminates.
func TestTessellatePathsNaNScaleMatchesOne(t *testing.T) {
	path := func() vectorPath {
		return vectorPath{
			Transform: identityTransform,
			FillColor: gui.SvgColor{R: 255, G: 0, B: 0, A: 255},
			Segments: []pathSegment{
				{Cmd: cmdMoveTo, Points: []float32{0, 0}},
				{Cmd: cmdCubicTo, Points: []float32{
					10, 0, 10, 10, 20, 10,
				}},
			},
		}
	}
	nanTris := (&vectorGraphic{Paths: []vectorPath{path()}}).
		tessellatePaths([]vectorPath{path()},
			float32(math.NaN()))
	oneTris := (&vectorGraphic{Paths: []vectorPath{path()}}).
		tessellatePaths([]vectorPath{path()}, 1)
	if len(nanTris) != len(oneTris) {
		t.Fatalf("NaN scale gave %d paths, want %d",
			len(nanTris), len(oneTris))
	}
	for i := range nanTris {
		if len(nanTris[i].Triangles) != len(oneTris[i].Triangles) {
			t.Fatalf("path %d: NaN scale gave %d floats, want %d",
				i, len(nanTris[i].Triangles),
				len(oneTris[i].Triangles))
		}
	}
}

// A polygon with bad vertices produces no triangles. The old
// code copied a 3-vertex NaN polygon straight to the output.
func TestEarClipRejectsNonFinite(t *testing.T) {
	nan := float32(math.NaN())
	if out := earClip([]float32{0, 0, 1, 0, nan, 1}); out != nil {
		t.Fatalf("NaN vertex gave %d floats, want nil", len(out))
	}
	if out := earClip([]float32{0, 0, 4, 0, 4, 4, 0, 4}); out == nil {
		t.Fatal("valid quad gave nil, want triangles")
	}
}

// loadSvgFile reads with a cap, not Stat then ReadFile. A file one
// byte over the cap must fail; one at the cap must load; a missing
// file must return an error, not panic.
func TestLoadSvgFileCapsSize(t *testing.T) {
	dir := t.TempDir()
	atCap := filepath.Join(dir, "at.svg")
	if err := os.WriteFile(atCap, make([]byte, maxSvgFileSize), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if data, err := loadSvgFile(atCap); err != nil || len(data) != maxSvgFileSize {
		t.Fatalf("at cap: got (%d, %v), want full read", len(data), err)
	}
	over := filepath.Join(dir, "over.svg")
	if err := os.WriteFile(over, make([]byte, maxSvgFileSize+1), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := loadSvgFile(over); err == nil {
		t.Fatal("file over the cap loaded, want error")
	}
	if _, err := loadSvgFile(filepath.Join(dir, "missing.svg")); err == nil {
		t.Fatal("missing file loaded, want error")
	}
}
