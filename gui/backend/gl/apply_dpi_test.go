//go:build !js && !darwin && !android

package gl

import (
	"bytes"
	"log"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestApplyDPIScale_GL(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var b *Backend
		b.applyDPIScale(2) // must not panic
	})

	t.Run("valid update", func(t *testing.T) {
		b := &Backend{dpiScale: 1}
		b.glyphBack = &glyphBackend{dpiScale: 1}
		b.applyDPIScale(2)
		if b.dpiScale != 2 {
			t.Fatalf("dpiScale = %v, want 2", b.dpiScale)
		}
		if b.glyphBack.dpiScale != 2 {
			t.Fatalf("glyphBack.dpiScale = %v, want 2", b.glyphBack.dpiScale)
		}
	})

	t.Run("reject NaN", func(t *testing.T) {
		b := &Backend{dpiScale: 1}
		b.applyDPIScale(float32(math.NaN()))
		if b.dpiScale != 1 {
			t.Fatalf("NaN accepted: dpiScale = %v, want 1", b.dpiScale)
		}
	})

	t.Run("reject Inf", func(t *testing.T) {
		b := &Backend{dpiScale: 1}
		b.applyDPIScale(float32(math.Inf(1)))
		if b.dpiScale != 1 {
			t.Fatalf("Inf accepted: dpiScale = %v", b.dpiScale)
		}
	})

	t.Run("reject non-positive", func(t *testing.T) {
		for _, s := range []float32{0, -1} {
			b := &Backend{dpiScale: 1}
			b.applyDPIScale(s)
			if b.dpiScale != 1 {
				t.Fatalf("scale %v accepted: got %v", s, b.dpiScale)
			}
		}
	})

	t.Run("reject absurdly large", func(t *testing.T) {
		b := &Backend{dpiScale: 1}
		b.applyDPIScale(100)
		if b.dpiScale != 1 {
			t.Fatalf("large scale accepted: dpiScale = %v, want 1", b.dpiScale)
		}
	})

	t.Run("early return same scale", func(t *testing.T) {
		b := &Backend{dpiScale: 2}
		b.glyphBack = &glyphBackend{dpiScale: 2}
		b.applyDPIScale(2)
		if b.dpiScale != 2 {
			t.Fatalf("dpiScale changed on same-scale call: %v", b.dpiScale)
		}
	})

	t.Run("accept upper bound", func(t *testing.T) {
		b := &Backend{dpiScale: 1}
		b.glyphBack = &glyphBackend{dpiScale: 1}
		b.applyDPIScale(8)
		if b.dpiScale != 8 {
			t.Fatalf("scale 8 rejected: dpiScale = %v, want 8", b.dpiScale)
		}
	})

	// The reject path leaves the glyph backend alone as well: adopting a
	// broken scale in one half and not the other is worse than keeping
	// both at the old density.
	t.Run("reject leaves glyph backend untouched", func(t *testing.T) {
		b := &Backend{dpiScale: 2}
		b.glyphBack = &glyphBackend{dpiScale: 2}
		b.applyDPIScale(0)
		if b.glyphBack.dpiScale != 2 {
			t.Fatalf("glyphBack.dpiScale = %v, want 2", b.glyphBack.dpiScale)
		}
	})

	// With the dev gate on, a rejected scale must leave evidence — that
	// log line is the only trace of a failed platform DPI query.
	t.Run("reject logs under debug gate", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })
		gui.Debug(true)
		t.Cleanup(func() { gui.Debug(false) })

		b := &Backend{dpiScale: 1}
		b.applyDPIScale(float32(math.NaN()))
		if b.dpiScale != 1 {
			t.Fatalf("NaN accepted under debug: dpiScale = %v", b.dpiScale)
		}
		if !strings.Contains(buf.String(), "rejected implausible dpi scale") {
			t.Fatalf("no reject log emitted; got %q", buf.String())
		}
	})

}
