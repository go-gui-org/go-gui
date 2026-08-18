package soft

import (
	"image"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
)

// renderer replays a []gui.RenderCmd stream into a buffer. It is the CPU
// analogue of gui/backend/gl's Backend.renderersDraw: same dispatch, same
// per-command scaling by the device pixel ratio.
type renderer struct {
	buf   *buffer
	scale float32

	textSys   *glyph.TextSystem
	glyphBack *glyphBackend

	// images caches decoded sources by the RenderCmd.Resource string.
	images map[string]*image.NRGBA

	allowedImageRoots []string
	maxImageBytes     int64
	maxImagePixels    int64

	// warm marks the atlas-warming pass: only the text kinds run, so
	// shapes, gradients and images rasterize exactly once.
	warm bool

	// Scratch buffers reused across commands.
	placements []glyph.GlyphPlacement
	normStops  []gui.GradientStop
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
				gui.RenderLayoutTransformed, gui.RenderTextPath:
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

		// Phase 2 (see docs/specs/headless-software-rendering.md).
		// Listed explicitly rather than caught by a default so a new
		// render kind is a compile-visible decision, following the
		// precedent in gui/print_pdf.go.
		case gui.RenderNone,
			gui.RenderSvg,
			gui.RenderShadow,
			gui.RenderBlur,
			gui.RenderFilterBegin,
			gui.RenderFilterEnd,
			gui.RenderFilterComposite,
			gui.RenderStencilBegin,
			gui.RenderStencilEnd,
			gui.RenderRotateBegin,
			gui.RenderRotateEnd,
			gui.RenderCustomShader,
			gui.RenderTermGrid,
			gui.RenderLayoutPlaced:
		}
	}
}
