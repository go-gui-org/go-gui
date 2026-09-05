//go:build linux && !js && !android

package gl

import (
	"testing"

	"github.com/jezek/xgb/xproto"
)

// screenDepths builds the AllowedDepths shape an X screen reports, so
// the picker can be exercised with no X server.
func screenDepths(byDepth map[byte][]uint32) []xproto.DepthInfo {
	out := make([]xproto.DepthInfo, 0, len(byDepth))
	for depth, ids := range byDepth {
		vis := make([]xproto.VisualInfo, 0, len(ids))
		for _, id := range ids {
			vis = append(vis, xproto.VisualInfo{VisualId: xproto.Visualid(id)})
		}
		out = append(out, xproto.DepthInfo{Depth: depth, Visuals: vis})
	}
	return out
}

func TestVisualDepths(t *testing.T) {
	got := visualDepths(screenDepths(map[byte][]uint32{
		24: {0x21, 0x22},
		32: {0x9f},
	}))
	want := map[uint32]byte{0x21: 24, 0x22: 24, 0x9f: 32}
	if len(got) != len(want) {
		t.Fatalf("visualDepths returned %d entries, want %d", len(got), len(want))
	}
	for id, depth := range want {
		if got[id] != depth {
			t.Errorf("visual 0x%x: depth %d, want %d", id, got[id], depth)
		}
	}
}

func TestPickVisual(t *testing.T) {
	// The driver's own order: depth-24 first, which is why the opaque
	// path and the transparent path cannot share a choice.
	cands := []eglConfigVisual{
		{config: 1, visualID: 0x21},
		{config: 2, visualID: 0x22},
		{config: 3, visualID: 0x9f},
	}
	depths := visualDepths(screenDepths(map[byte][]uint32{
		24: {0x21, 0x22},
		32: {0x9f},
	}))
	noARGB := visualDepths(screenDepths(map[byte][]uint32{
		24: {0x21, 0x22, 0x9f},
	}))

	tests := []struct {
		name       string
		cands      []eglConfigVisual
		depths     map[uint32]byte
		wantAlpha  bool
		wantConfig uintptr
		wantDepth  byte
		wantOK     bool
	}{
		{
			name:  "opaque keeps the driver's first choice",
			cands: cands, depths: depths, wantAlpha: false,
			wantConfig: 1, wantDepth: 24, wantOK: true,
		},
		{
			name:  "transparent skips ahead to the ARGB visual",
			cands: cands, depths: depths, wantAlpha: true,
			wantConfig: 3, wantDepth: 32, wantOK: true,
		},
		{
			name:  "transparent with no ARGB visual degrades to opaque",
			cands: cands, depths: noARGB, wantAlpha: true,
			wantConfig: 1, wantDepth: 24, wantOK: false,
		},
		{
			name:   "visual absent from the screen reports depth 0",
			cands:  []eglConfigVisual{{config: 7, visualID: 0xdead}},
			depths: depths, wantAlpha: false,
			wantConfig: 7, wantDepth: 0, wantOK: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, depth, ok := pickVisual(tc.cands, tc.depths, tc.wantAlpha)
			if cfg.config != tc.wantConfig {
				t.Errorf("config = %d, want %d", cfg.config, tc.wantConfig)
			}
			if depth != tc.wantDepth {
				t.Errorf("depth = %d, want %d", depth, tc.wantDepth)
			}
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
		})
	}
}

// No candidates is not reachable through eglInitDisplayN, which errors
// instead, but the picker must not index an empty slice if that changes.
func TestPickVisualNoCandidates(t *testing.T) {
	if _, _, ok := pickVisual(nil, nil, true); ok {
		t.Error("pickVisual(nil) reported success")
	}
}
