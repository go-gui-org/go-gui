//go:build darwin && arm64

package mtldraw

import "github.com/go-gui-org/go-gui/gui/backend/internal/msl"

// testNullBackends returns a Renderer and a glyph backend over a
// renderer with no device and no encoder. Every draw call returns at
// its encoder guard, so a test sees only the Go side of a draw
// path — what TestDrawPathsDoNotAllocate needs to count heap
// allocations without a GPU. The texture registered under glyph id 1
// lets the textured-quad paths get past their lookup. Call the
// returned func to free the context.
func testNullBackends() (*Renderer, *metalGlyphBackend, func()) {
	ctx := &metalCtx{nextTexID: 1}
	ws := &Renderer{ctx: ctx, dpiScale: 2}
	gb := newMetalGlyphBackend(ctx, 2)
	gb.textures[1] = metalTexInfo{cID: 1, w: 256, h: 256}
	return ws, gb, func() { ctx.destroy() }
}

// testCompileShaders compiles the built-in MSL library standalone at
// the pinned language version. 0 = compiled, -1 = compile failed,
// 1 = no Metal device. See TestBuiltinShadersCompile.
func testCompileShaders() int {
	return testCompileMSL(msl.Source)
}

// testCompileMSL compiles src through the same probe; return codes
// as testCompileShaders. See TestCustomWrapperCompiles.
func testCompileMSL(src string) int {
	return compileShadersProbe(src)
}

// testEdgeCoverage draws a diagonal triangle edge through the main
// render pass and returns how many edge pixels are partly covered.
// -1 = no Metal device, -2 = setup failed. See
// TestTriangleEdgesAntialiased.
func testEdgeCoverage() int {
	return edgeCoverageProbe(msl.Source)
}
