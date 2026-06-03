package svg

import (
	"encoding/binary"
	"flag"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Phase 0 — regression fingerprint for the 15 SMIL spinners imported
// in commit 6b1b6cc. Locks parse + Tessellate + TessellateAnimated
// output before the CSS / @keyframes refactor begins. Any cascade
// rewrite (Phase A) that perturbs static geometry, animation parse,
// or the override-driven re-tessellation path will surface as a diff
// against testdata/phase0_smil_goldens.txt.
//
// Run `go test ./gui/svg/ -run TestPhase0 -phase0-update` to regen
// the goldens after a deliberate change.

var phase0Update = flag.Bool("phase0-update", false,
	"regenerate phase 0 SMIL goldens")

// phase0SmilSpinners is the canonical 15-asset fixture set. Order
// matters — the goldens file is written in this order.
var phase0SmilSpinners = []string{
	"180-ring-with-bg.svg",
	"270-ring-with-bg.svg",
	"6-dots-scale-middle.svg",
	"90-ring-with-bg.svg",
	"ball-triangle.svg",
	"clock.svg",
	"dot-revolve.svg",
	"loader2.svg",
	"oval.svg",
	"pulse-3.svg",
	"pulse-multiple.svg",
	"pulse-ring.svg",
	"pulse.svg",
	"rings.svg",
	"wifi-fade.svg",
}

func TestPhase0SmilSpinnerFingerprint(t *testing.T) {
	got := buildPhase0Fingerprints(t)
	goldenPath := filepath.Join("testdata", "phase0_smil_goldens.txt")
	if *phase0Update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write goldens: %v", err)
		}
		t.Logf("wrote %s (%d bytes)", goldenPath, len(got))
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read goldens: %v (run with -phase0-update to seed)", err)
	}
	if string(want) != got {
		// Surface a unified-style diff so the failure is actionable.
		t.Fatalf("phase 0 SMIL fingerprint drift:\n--- want\n%s\n--- got\n%s\n"+
			"re-run with -phase0-update if the change is intentional",
			string(want), got)
	}
}

// buildPhase0Fingerprints parses each fixture and emits four lines
// of fingerprint data per asset: viewbox/dims, static-tessellate
// hash, animation-list hash, and override-driven TessellateAnimated
// hash. Lines carry small human-readable counts so a regression diff
// shows what shifted at a glance.
func buildPhase0Fingerprints(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "assets", "svg-spinners")
	var out strings.Builder
	for _, name := range phase0SmilSpinners {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		p := New()
		parsed, err := p.ParseSvg(string(data))
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		fmt.Fprintf(&out, "%s vg     w=%g h=%g vb=%g,%g paths=%d anims=%d\n",
			name, parsed.Width, parsed.Height,
			parsed.ViewBoxX, parsed.ViewBoxY,
			len(parsed.Paths), len(parsed.Animations))

		staticTris := p.Tessellate(parsed, 1)
		fmt.Fprintf(&out, "%s static tris=%d hash=%s\n",
			name, len(staticTris), hashTessellated(staticTris))

		fmt.Fprintf(&out, "%s anims  hash=%s\n",
			name, hashAnimations(parsed.Animations))

		live := p.TessellateAnimated(parsed, 1,
			syntheticOverrides(parsed), nil)
		fmt.Fprintf(&out, "%s live   tris=%d hash=%s\n",
			name, len(live), hashTessellated(live))
	}
	return out.String()
}

// syntheticOverrides builds a deterministic override per animated
// PathID exercising every primitive attr field. Synthetic values
// chosen to be in-bounds for typical 0..24 viewBoxes; fingerprint
// only cares that the same numbers feed through to triangles each
// run, not that they correspond to a real animation moment.
func syntheticOverrides(parsed *gui.SvgParsed) map[uint32]gui.SvgAnimAttrOverride {
	pids := map[uint32]struct{}{}
	for i := range parsed.Paths {
		if !parsed.Paths[i].Animated {
			continue
		}
		if pid := parsed.Paths[i].PathID; pid != 0 {
			pids[pid] = struct{}{}
		}
	}
	if len(pids) == 0 {
		return nil
	}
	out := make(map[uint32]gui.SvgAnimAttrOverride, len(pids))
	for pid := range pids {
		out[pid] = gui.SvgAnimAttrOverride{
			Mask: gui.SvgAnimMaskCX | gui.SvgAnimMaskCY |
				gui.SvgAnimMaskR | gui.SvgAnimMaskRX |
				gui.SvgAnimMaskRY | gui.SvgAnimMaskX |
				gui.SvgAnimMaskY | gui.SvgAnimMaskWidth |
				gui.SvgAnimMaskHeight,
			CX: 12, CY: 12, R: 5,
			RX: 3, RY: 3,
			X: 2, Y: 4,
			Width: 10, Height: 10,
		}
	}
	return out
}

// hashTessellated returns an FNV-64a digest over a sorted projection
// of every TessellatedPath. Paths are sorted by (PathID, IsStroke,
// IsClipMask, ClipGroup) to make the hash insensitive to result-
// ordering tweaks while remaining sensitive to geometry, color,
// base-transform, and primitive metadata.
func hashTessellated(paths []gui.TessellatedPath) string {
	idx := make([]int, len(paths))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool {
		a, b := &paths[idx[i]], &paths[idx[j]]
		if a.PathID != b.PathID {
			return a.PathID < b.PathID
		}
		if a.IsStroke != b.IsStroke {
			return !a.IsStroke
		}
		if a.IsClipMask != b.IsClipMask {
			return !a.IsClipMask
		}
		return a.ClipGroup < b.ClipGroup
	})
	h := fnv.New64a()
	var buf [4]byte
	put := func(f float32) {
		// Normalize NaN to a single bit pattern so platform-specific
		// NaN payloads cannot perturb the digest. Quantize finite
		// values to 1e-3 so ULP-level drift between amd64 (asm
		// math.Sin/Cos) and arm64 (pure-Go) does not flip the digest.
		if math.IsNaN(float64(f)) {
			f = float32(math.NaN())
		} else if !math.IsInf(float64(f), 0) {
			f = float32(math.Round(float64(f)*1e3) / 1e3)
		}
		binary.LittleEndian.PutUint32(buf[:], math.Float32bits(f))
		h.Write(buf[:])
	}
	for _, i := range idx {
		p := &paths[i]
		fmt.Fprintf(h, "id=%d stroke=%v clip=%v cg=%d anim=%v ",
			p.PathID, p.IsStroke, p.IsClipMask, p.ClipGroup, p.Animated)
		fmt.Fprintf(h, "color=%d,%d,%d,%d ",
			p.Color.R, p.Color.G, p.Color.B, p.Color.A)
		fmt.Fprintf(h, "vc=%d ", len(p.VertexColors))
		for _, c := range p.VertexColors {
			fmt.Fprintf(h, "%d,%d,%d,%d|", c.R, c.G, c.B, c.A)
		}
		fmt.Fprintf(h, "tris=%d ", len(p.Triangles))
		for _, v := range p.Triangles {
			put(v)
		}
		fmt.Fprintf(h, "base=%v ", p.HasBaseXform)
		put(p.BaseTransX)
		put(p.BaseTransY)
		put(p.BaseScaleX)
		put(p.BaseScaleY)
		put(p.BaseRotAngle)
		put(p.BaseRotCX)
		put(p.BaseRotCY)
		fmt.Fprintf(h, "prim=%d ", p.Primitive.Kind)
		put(p.Primitive.CX)
		put(p.Primitive.CY)
		put(p.Primitive.R)
		put(p.Primitive.RX)
		put(p.Primitive.RY)
		put(p.Primitive.X)
		put(p.Primitive.Y)
		put(p.Primitive.W)
		put(p.Primitive.H)
		put(p.Primitive.X2)
		put(p.Primitive.Y2)
		fmt.Fprintln(h)
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// hashAnimations digests the parsed SvgAnimation slice. Targets are
// sorted before hashing so map-iteration order in the parser can't
// jitter the result.
func hashAnimations(anims []gui.SvgAnimation) string {
	h := fnv.New64a()
	var buf [4]byte
	put := func(f float32) {
		if math.IsNaN(float64(f)) {
			f = float32(math.NaN())
		} else if !math.IsInf(float64(f), 0) {
			f = float32(math.Round(float64(f)*1e3) / 1e3)
		}
		binary.LittleEndian.PutUint32(buf[:], math.Float32bits(f))
		h.Write(buf[:])
	}
	for i := range anims {
		a := &anims[i]
		fmt.Fprintf(h, "kind=%d gid=%q attr=%d target=%d calc=%d restart=%d ",
			a.Kind, a.GroupID, a.AttrName, a.Target, a.CalcMode, a.Restart)
		fmt.Fprintf(h, "set=%v add=%v acc=%v frz=%v ",
			a.IsSet, a.Additive, a.Accumulate, a.Freeze)
		put(a.DurSec)
		put(a.BeginSec)
		put(a.Cycle)
		put(a.CenterX)
		put(a.CenterY)
		fmt.Fprintf(h, "values=%d ", len(a.Values))
		for _, v := range a.Values {
			put(v)
		}
		fmt.Fprintf(h, "ksp=%d ", len(a.KeySplines))
		for _, v := range a.KeySplines {
			put(v)
		}
		fmt.Fprintf(h, "kt=%d ", len(a.KeyTimes))
		for _, v := range a.KeyTimes {
			put(v)
		}
		fmt.Fprintf(h, "mp=%d ml=%d mrot=%d dashk=%d ",
			len(a.MotionPath), len(a.MotionLengths),
			a.MotionRotate, a.DashKeyframeLen)
		for _, v := range a.MotionPath {
			put(v)
		}
		for _, v := range a.MotionLengths {
			put(v)
		}
		ids := slices.Clone(a.TargetPathIDs)
		slices.Sort(ids)
		fmt.Fprintf(h, "ids=%v\n", ids)
	}
	return fmt.Sprintf("%016x", h.Sum64())
}
