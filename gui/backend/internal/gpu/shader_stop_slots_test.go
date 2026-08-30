package gpu

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
	"github.com/go-gui-org/go-gui/gui/shader"
)

// The gradient shader is maintained in three hand-edited copies: the
// GLSL the GL backend compiles, the GLSL the Android backend holds as
// a C string literal, and the MSL the Metal and iOS backends compile.
// Nothing couples them but whoever remembers to edit all three, so a
// stop-limit raise applied to two of them skews the third silently:
// the packer writes GradientStopSlots stops, the stale shader reads
// fewer, and the extra stops vanish on that backend only.
//
// These tests read the stop count back out of each source and hold it
// to GradientStopSlots, so the next raise fails loudly here instead.
//
// Reading a sibling backend's source is the point rather than a
// shortcut: the Android copy is a C string literal that no Go symbol
// exposes, and its staleness is exactly what needs catching.
const androidShaderSrc = "../../android/gles_android.c"

var (
	// `vec4 stop_colors[12];` / `float4 stop_colors[12];`, and the
	// same for stop_positions. The element type differs per language,
	// so accept any of the three; requiring one is what separates the
	// declaration from an indexed read like `p1 = stop_positions[0];`.
	reStopArray = regexp.MustCompile(
		`\b(?:vec4|float4|float)\s+stop_(?:colors|positions)\[(\d+)\]\s*;`)

	// `for (int i = 1; i < 12; i++)` — the walk over the unpacked
	// stops. Identical in all three sources, but msl.Source and the
	// Android file hold every shader at once, so the blur weight
	// loops share the header shape. Requiring stop_positions[i] in
	// the body just below picks out the gradient walk.
	reStopLoop = regexp.MustCompile(
		`(?s)for\s*\(\s*int\s+i\s*=\s*1\s*;\s*i\s*<\s*(\d+)\s*;.{0,200}?stop_positions\[i\]`)
)

// unpackPair reports how many times src unpacks into slot i. Both
// spellings put the destination pair on one line, so one pattern
// covers the GLSL `unpack_gradient_data` and the MSL `unpack_stop`.
func unpackPair(src string, i int) int {
	pat := regexp.MustCompile(
		fmt.Sprintf(`stop_colors\[%d\]\s*,\s*stop_positions\[%d\]\s*\)`, i, i))
	return len(pat.FindAllString(src, -1))
}

// gradientSources returns each maintained copy of the gradient shader
// by name. The Android copy is read from disk; the other two are Go
// constants their backends compile directly.
func gradientSources(t *testing.T) map[string]string {
	t.Helper()
	android, err := os.ReadFile(androidShaderSrc)
	if err != nil {
		t.Fatalf("read %s: %v", androidShaderSrc, err)
	}
	return map[string]string{
		"shader.FsGradientGLSL": shader.FsGradientGLSL,
		"msl.Source":            msl.Source,
		androidShaderSrc:        string(android),
	}
}

// TestGradientShaderStopArraysMatchSlots checks the declared array
// size and the walk's loop bound in every copy. A raise that misses
// one source leaves it declaring the old size.
func TestGradientShaderStopArraysMatchSlots(t *testing.T) {
	t.Parallel()
	for name, src := range gradientSources(t) {
		arrays := reStopArray.FindAllStringSubmatch(src, -1)
		// Two arrays per copy: stop_colors and stop_positions.
		if len(arrays) != 2 {
			t.Errorf("%s: found %d stop arrays, want 2 (stop_colors and stop_positions)",
				name, len(arrays))
			continue
		}
		for _, m := range arrays {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				t.Errorf("%s: unparsable array size %q", name, m[1])
				continue
			}
			if n != GradientStopSlots {
				t.Errorf("%s: %s declares %d slots, want GradientStopSlots=%d",
					name, m[0], n, GradientStopSlots)
			}
		}

		loops := reStopLoop.FindAllStringSubmatch(src, -1)
		if len(loops) != 1 {
			t.Errorf("%s: found %d stop-walk loops, want 1", name, len(loops))
			continue
		}
		n, err := strconv.Atoi(loops[0][1])
		if err != nil {
			t.Errorf("%s: unparsable loop bound %q", name, loops[0][1])
			continue
		}
		if n != GradientStopSlots {
			t.Errorf("%s: stop walk bounded at %d, want GradientStopSlots=%d",
				name, n, GradientStopSlots)
		}
	}
}

// TestGradientShaderUnpacksEverySlot checks that each copy actually
// fills every slot it declares. Widening the arrays and the loop
// without adding the unpack calls reads slots nothing wrote, which
// the size check above cannot see.
func TestGradientShaderUnpacksEverySlot(t *testing.T) {
	t.Parallel()
	for name, src := range gradientSources(t) {
		for i := range GradientStopSlots {
			if got := unpackPair(src, i); got != 1 {
				t.Errorf("%s: slot %d unpacked %d times, want exactly 1",
					name, i, got)
			}
		}
		// Nothing may unpack past the last slot: a shrink that left a
		// stale call behind would index off the end of the arrays.
		if got := unpackPair(src, GradientStopSlots); got != 0 {
			t.Errorf("%s: unpacks slot %d, past GradientStopSlots=%d",
				name, GradientStopSlots, GradientStopSlots)
		}
	}
}
