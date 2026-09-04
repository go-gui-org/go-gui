//go:build darwin && cgo && !ios

package metal

import (
	"bytes"
	"log"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestApplyDPIScale(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var ws *windowState
		ws.applyDPIScale(2) // must not panic
	})

	t.Run("valid update", func(t *testing.T) {
		ws := &windowState{dpiScale: 1}
		ws.glyphBack = &metalGlyphBackend{dpiScale: 1}
		ws.applyDPIScale(2)
		if ws.dpiScale != 2 {
			t.Fatalf("dpiScale = %v, want 2", ws.dpiScale)
		}
		if ws.glyphBack.dpiScale != 2 {
			t.Fatalf("glyphBack.dpiScale = %v, want 2", ws.glyphBack.dpiScale)
		}
	})

	t.Run("early return same scale", func(t *testing.T) {
		ws := &windowState{dpiScale: 2}
		ws.glyphBack = &metalGlyphBackend{dpiScale: 2}
		// Should not change; call with same scale is no-op
		ws.applyDPIScale(2)
		if ws.dpiScale != 2 {
			t.Fatalf("dpiScale changed on same-scale call: %v", ws.dpiScale)
		}
	})

	t.Run("reject NaN", func(t *testing.T) {
		ws := &windowState{dpiScale: 1}
		ws.applyDPIScale(float32(math.NaN()))
		if ws.dpiScale != 1 {
			t.Fatalf("NaN accepted: dpiScale = %v, want 1", ws.dpiScale)
		}
	})

	t.Run("reject Inf", func(t *testing.T) {
		ws := &windowState{dpiScale: 1}
		ws.applyDPIScale(float32(math.Inf(1)))
		if ws.dpiScale != 1 {
			t.Fatalf("Inf accepted: dpiScale = %v, want 1", ws.dpiScale)
		}
		ws.applyDPIScale(float32(math.Inf(-1)))
		if ws.dpiScale != 1 {
			t.Fatalf("-Inf accepted: dpiScale = %v, want 1", ws.dpiScale)
		}
	})

	t.Run("reject non-positive", func(t *testing.T) {
		for _, s := range []float32{0, -1, -0.5} {
			ws := &windowState{dpiScale: 1}
			ws.applyDPIScale(s)
			if ws.dpiScale != 1 {
				t.Fatalf("scale %v accepted: got %v, want 1", s, ws.dpiScale)
			}
		}
	})

	t.Run("reject absurdly large", func(t *testing.T) {
		ws := &windowState{dpiScale: 1}
		ws.applyDPIScale(100)
		if ws.dpiScale != 1 {
			t.Fatalf("large scale accepted: dpiScale = %v, want 1", ws.dpiScale)
		}
	})

	t.Run("accept upper bound", func(t *testing.T) {
		ws := &windowState{dpiScale: 1}
		ws.glyphBack = &metalGlyphBackend{dpiScale: 1}
		ws.applyDPIScale(5)
		if ws.dpiScale != 5 {
			t.Fatalf("scale 5 rejected: dpiScale = %v, want 5", ws.dpiScale)
		}
	})

	// The reject path leaves the glyph backend alone as well: adopting a
	// broken scale in one half and not the other is worse than keeping
	// both at the old density.
	t.Run("reject leaves glyph backend untouched", func(t *testing.T) {
		ws := &windowState{dpiScale: 2}
		ws.glyphBack = &metalGlyphBackend{dpiScale: 2}
		ws.applyDPIScale(0)
		if ws.glyphBack.dpiScale != 2 {
			t.Fatalf("glyphBack.dpiScale = %v, want 2", ws.glyphBack.dpiScale)
		}
	})

	// With the dev gate on, a rejected scale must leave evidence — that
	// log line is the only trace of a failed backing-scale query.
	t.Run("reject logs under debug gate", func(t *testing.T) {
		var buf bytes.Buffer
		log.SetOutput(&buf)
		t.Cleanup(func() { log.SetOutput(os.Stderr) })
		gui.Debug(true)
		t.Cleanup(func() { gui.Debug(false) })

		ws := &windowState{dpiScale: 1}
		ws.applyDPIScale(float32(math.NaN()))
		if ws.dpiScale != 1 {
			t.Fatalf("NaN accepted under debug: dpiScale = %v", ws.dpiScale)
		}
		if !strings.Contains(buf.String(), "rejected implausible dpi scale") {
			t.Fatalf("no reject log emitted; got %q", buf.String())
		}
	})

}
