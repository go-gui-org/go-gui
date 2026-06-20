//go:build !js && !darwin

package sdl2

import (
	"math"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/veandco/go-sdl2/sdl"
)

func (b *Backend) drawSvg(r *gui.RenderCmd) {
	if len(r.Triangles) == 0 || len(r.Triangles)%6 != 0 {
		return
	}
	s := b.dpiScale
	numVerts := len(r.Triangles) / 2
	if cap(b.svgVerts) < numVerts {
		b.svgVerts = make([]sdl.Vertex, numVerts)
	}
	verts := b.svgVerts[:numVerts]
	hasVCols := len(r.VertexColors) == numVerts
	vAlpha := float32(1)
	if r.HasVertexAlpha {
		vAlpha = r.VertexAlphaScale
		vAlpha = min(max(vAlpha, 0), 1)
	}

	hasXform := r.HasXform
	var sx, sy, tx, ty float32
	if hasXform {
		sx, sy, tx, ty = r.ScaleX, r.ScaleY, r.TransX, r.TransY
	}
	// Precompute rotation if needed.
	hasRot := r.RotAngle != 0
	var sinA, cosA, rcx, rcy float32
	if hasRot {
		rad := float64(r.RotAngle) * math.Pi / 180
		sinA = float32(math.Sin(rad))
		cosA = float32(math.Cos(rad))
		rcx, rcy = r.RotCX, r.RotCY
	}

	for i := range numVerts {
		vx := r.Triangles[i*2]
		vy := r.Triangles[i*2+1]
		if hasXform {
			vx = vx*sx + tx
			vy = vy*sy + ty
		}
		if hasRot {
			dx := vx - rcx
			dy := vy - rcy
			vx = rcx + dx*cosA - dy*sinA
			vy = rcy + dx*sinA + dy*cosA
		}
		verts[i].Position = sdl.FPoint{
			X: (r.X + vx*r.Scale) * s,
			Y: (r.Y + vy*r.Scale) * s,
		}
		if hasVCols {
			vc := r.VertexColors[i]
			alpha := vc.A
			if r.HasVertexAlpha {
				alpha = uint8(float32(alpha) * vAlpha)
			}
			verts[i].Color = sdl.Color{R: vc.R, G: vc.G, B: vc.B, A: vc.A}
			verts[i].Color.A = alpha
		} else {
			verts[i].Color = sdl.Color{
				R: r.Color.R, G: r.Color.G, B: r.Color.B, A: r.Color.A,
			}
		}
	}
	_ = b.renderer.RenderGeometry(nil, verts, nil)
	b.svgVerts = verts[:0]
}

// beginFilter starts rendering to a temporary texture for filter
// effects. All subsequent draw commands render to this texture
// until endFilter is called.
func (b *Backend) beginFilter(r *gui.RenderCmd) {
	prevTarget := b.renderer.GetRenderTarget()
	currClip, clipEnabled := b.currentClipState()
	outW, outH, err := b.currentTargetSize(prevTarget)
	if err != nil {
		return
	}
	if b.filterPool == nil || b.filterPoolW != outW || b.filterPoolH != outH {
		if b.filterPool != nil {
			_ = b.filterPool.Destroy()
			b.filterPool = nil
		}
		tex, err := b.renderer.CreateTexture(
			sdl.PIXELFORMAT_RGBA8888,
			sdl.TEXTUREACCESS_TARGET,
			outW, outH,
		)
		if err != nil {
			return
		}
		_ = tex.SetBlendMode(sdl.BLENDMODE_BLEND)
		b.filterPool = tex
		b.filterPoolW = outW
		b.filterPoolH = outH
	}
	b.filterTex = b.filterPool
	b.filterPrevTarget = prevTarget
	b.filterBlur = r.BlurRadius * b.dpiScale
	b.filterLayers = r.Layers
	b.filterColorMatrix = r.ColorMatrix
	_ = b.renderer.SetRenderTarget(b.filterTex)
	b.applyClipState(currClip, clipEnabled)
	_ = b.renderer.SetDrawColor(0, 0, 0, 0)
	_ = b.renderer.Clear()
}

// endFilter composites the filter texture back with a glow
// approximation: render blurred copies at reduced alpha, then
// the sharp source on top.
func (b *Backend) endFilter() {
	tex := b.filterTex
	if tex == nil {
		return
	}
	currClip, clipEnabled := b.currentClipState()
	prevTarget := b.filterPrevTarget
	b.filterTex = nil
	b.filterPrevTarget = nil

	// Apply color matrix in software before compositing.
	if b.filterColorMatrix != nil {
		b.applyColorMatrix(tex, prevTarget)
	}

	_ = b.renderer.SetRenderTarget(prevTarget)
	b.applyClipState(currClip, clipEnabled)

	blur := b.filterBlur
	blur = max(blur, 1)
	layers := b.filterLayers
	layers = max(layers, 1)

	outW, outH, err := b.currentTargetSize(prevTarget)
	if err != nil {
		return
	}

	// Simulate glow: render texture at increasing offsets with
	// Gaussian-like alpha falloff. 8 directions per ring for a
	// circular glow.
	steps := int(blur + 0.5)
	steps = min(max(steps, 1), 16)
	baseAlpha := float32(60) * float32(layers)
	diag := func(off int32) int32 {
		return off * 7 / 10 // ~0.707 approximation
	}
	for s := steps; s >= 1; s-- {
		t := float32(s) / float32(steps)
		a := baseAlpha * (1 - t*t)
		if a < 1 {
			continue
		}
		a = min(a, 255)
		_ = tex.SetAlphaMod(uint8(a))
		off := int32(s)
		d := diag(off)
		offsets := [8][2]int32{
			{-off, 0}, {off, 0}, {0, -off}, {0, off},
			{-d, -d}, {d, -d}, {-d, d}, {d, d},
		}
		for _, o := range offsets {
			_ = b.renderer.Copy(tex, nil,
				&sdl.Rect{X: o[0], Y: o[1], W: outW, H: outH})
		}
	}

	// Render source graphic on top at full alpha.
	_ = tex.SetAlphaMod(255)
	_ = b.renderer.Copy(tex, nil, nil)
}

// applyColorMatrix transforms filter texture pixels by the 4x4
// color matrix in software. Called with the render target still
// set to the filter texture.
func (b *Backend) applyColorMatrix(
	tex *sdl.Texture, restoreTarget *sdl.Texture,
) {
	_, _, outW, outH, err := tex.Query()
	if err != nil {
		return
	}
	nPixels := int(outW) * int(outH)
	if nPixels == 0 {
		return
	}
	if cap(b.filterPixels) < nPixels {
		b.filterPixels = make([]uint32, nPixels)
	} else {
		b.filterPixels = b.filterPixels[:nPixels]
	}

	pitch := int(outW) * 4
	_ = b.renderer.ReadPixels(nil, sdl.PIXELFORMAT_RGBA8888,
		unsafe.Pointer(&b.filterPixels[0]), pitch)

	cm := b.filterColorMatrix
	for i := range b.filterPixels {
		px := b.filterPixels[i]
		ri := float32((px>>24)&0xFF) / 255
		gi := float32((px>>16)&0xFF) / 255
		bi := float32((px>>8)&0xFF) / 255
		ai := float32(px&0xFF) / 255

		ro := cm[0]*ri + cm[4]*gi + cm[8]*bi + cm[12]*ai
		gOut := cm[1]*ri + cm[5]*gi + cm[9]*bi + cm[13]*ai
		bo := cm[2]*ri + cm[6]*gi + cm[10]*bi + cm[14]*ai
		ao := cm[3]*ri + cm[7]*gi + cm[11]*bi + cm[15]*ai

		ro = max(0, min(1, ro))
		gOut = max(0, min(1, gOut))
		bo = max(0, min(1, bo))
		ao = max(0, min(1, ao))

		b.filterPixels[i] = uint32(ro*255+0.5)<<24 |
			uint32(gOut*255+0.5)<<16 |
			uint32(bo*255+0.5)<<8 |
			uint32(ao*255+0.5)
	}

	_ = b.renderer.SetRenderTarget(restoreTarget)
	_ = tex.Update(nil, unsafe.Pointer(&b.filterPixels[0]), pitch)
}
