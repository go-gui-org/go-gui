package soft

import (
	"image"

	"golang.org/x/image/vector"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
)

// renderer replays a []gui.RenderCmd stream into a buffer. It is the CPU
// analogue of gui/backend/gl's Backend.renderersDraw: same dispatch, same
// per-command scaling by the device pixel ratio.
type renderer struct {
	// buf is the current target: the window buffer, or the innermost
	// open bracket's layer. rootBuf is always the window buffer.
	buf     *buffer
	rootBuf *buffer
	scale   float32

	textSys   *glyph.TextSystem
	glyphBack *glyphBackend

	// textErrLogged warns once for a persistent DrawText failure
	// instead of spamming stderr every frame.
	textErrLogged bool

	// meshScoreBuf backs fillTriangleMesh's per-pixel fit scores; it is
	// grown, never freed, so a per-frame gradient does not allocate.
	meshScoreBuf []float32

	// images caches decoded sources by the RenderCmd.Resource string.
	images map[string]*image.NRGBA

	allowedImageRoots []string
	maxImageBytes     int64
	maxImagePixels    int64

	// warm marks the atlas-warming pass: only the text kinds run, so
	// shapes, gradients and images rasterize exactly once.
	warm bool

	// brackets holds the open begin/end pairs — filter, stencil and
	// rotate all scope later drawing by pushing an offscreen layer.
	brackets  []bracket
	layerPool []*buffer

	// Scratch buffers reused across commands.
	placements []glyph.GlyphPlacement
	normStops  []gui.GradientStop
	maskRast   vector.Rasterizer
	maskPix    []uint8
	maskPix2   []uint8
	blurTmp    []uint8
	svgBatch   []float32
	termRunes  []rune
	termCols   []int
}

// drawAll replays every command in cmds. With warm set it draws only the
// text kinds: that pass exists to rasterize glyphs into the atlas, and its
// quads are discarded against a nil glyph target.
func (r *renderer) drawAll(cmds []gui.RenderCmd) {
	for i := range cmds {
		cmd := &cmds[i]
		if r.warm {
			switch cmd.Kind {
			case gui.RenderText, gui.RenderLayout, gui.RenderRTF,
				gui.RenderLayoutTransformed, gui.RenderTextPath,
				gui.RenderTermGrid:
			default:
				continue
			}
		}
		switch cmd.Kind {
		case gui.RenderClip:
			r.buf.setClip(cmd.X*r.scale, cmd.Y*r.scale,
				cmd.W*r.scale, cmd.H*r.scale)
		case gui.RenderRect:
			r.drawRect(cmd)
		case gui.RenderStrokeRect:
			r.drawStrokeRect(cmd)
		case gui.RenderCircle:
			r.drawCircle(cmd)
		case gui.RenderLine:
			r.drawLine(cmd)
		case gui.RenderGradient:
			r.drawGradient(cmd)
		case gui.RenderGradientBorder:
			r.drawGradientBorder(cmd)
		case gui.RenderImage:
			r.drawImage(cmd)
		case gui.RenderText:
			r.drawText(cmd)
		case gui.RenderLayout, gui.RenderRTF:
			r.drawLayout(cmd)
		case gui.RenderLayoutTransformed:
			r.drawLayoutTransformed(cmd)
		case gui.RenderTextPath:
			r.drawTextPath(cmd)
		case gui.RenderSvg:
			r.drawSvg(cmd)
		case gui.RenderShadow:
			r.drawShadow(cmd)
		case gui.RenderBlur:
			r.drawBlur(cmd)
		case gui.RenderFilterBegin:
			r.beginFilter(cmd)
		case gui.RenderFilterEnd:
			r.endFilter()
		case gui.RenderStencilBegin:
			r.beginStencil(cmd)
		case gui.RenderStencilEnd:
			r.endStencil()
		case gui.RenderRotateBegin:
			r.beginRotation(cmd)
		case gui.RenderRotateEnd:
			r.endRotation()
		case gui.RenderTermGrid:
			r.drawTermGrid(cmd)

		// Unsupported by design, or never emitted by the render path.
		// Listed explicitly rather than caught by a default so a new
		// render kind is a compile-visible decision, following the
		// precedent in gui/print_pdf.go.
		//
		// RenderCustomShader is GLSL: there is no CPU equivalent to
		// compile. RenderFilterComposite is not emitted by the render
		// path — endFilter reads the layer count from the bracket's
		// RenderFilterBegin, as the GPU backends do.
		case gui.RenderNone,
			gui.RenderCustomShader,
			gui.RenderFilterComposite,
			gui.RenderLayoutPlaced:
		}
	}
}
