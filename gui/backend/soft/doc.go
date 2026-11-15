// Package soft renders a go-gui frame to an image on the CPU, with no GPU.
//
// It is a third consumer of the same flat []gui.RenderCmd stream the GPU
// backends and the PDF printer replay. Two entry points share one renderer:
//
//   - RenderToImage and RenderToPNG are headless: no window and no event
//     loop. They settle a frame themselves and return its pixels.
//   - Renderer draws a live window's frames for a backend that presents the
//     pixels itself. The Linux X11 backend uses it when GOGUI_RENDERER=soft
//     is set, or, with the variable unset, when GL would run on a software
//     rasterizer or has no context (#1265): the frame is drawn here and
//     uploaded with X11 PutImage, so a machine without a GPU does not need
//     Mesa's llvmpipe.
//
// Typical use is a screenshot or a pixel-level regression test:
//
//	w := gui.SimpleWindow("Demo", 400, 300, &App{}, func(w *gui.Window) {
//		w.SetView(mainView)
//	})
//	err := soft.RenderToPNG(w, 2, "demo.png")
//
// Text is real, not approximated: the package builds a go-glyph TextSystem
// over a software glyph.DrawBackend and installs it as the window's
// gui.TextMeasurer, so shaping, metrics and glyph rasterization are the
// same code the GPU backends run.
//
// Every render kind the pipeline emits is drawn: clip, rect, stroke
// rect, circle, line, gradient, gradient border, image, every text kind,
// SVG triangles, shadow and blur, filter brackets, stencil clipping,
// rotation brackets and the terminal grid. Filter, stencil and rotation
// brackets render into an offscreen layer that is composited back with
// the scoped effect applied, which is why they hold for text and images
// and not only for shapes.
//
// RenderCustomShader is unsupported by design: it is GLSL, and there is
// no CPU equivalent to compile. A shader container therefore renders
// nothing here, in screenshots and in golden tests alike: its children
// draw, its shader fill does not, so a new golden of a shader view
// silently lacks that content. SVG clip-mask geometry is skipped, as it
// is in every other backend. See
// docs/specs/headless-software-rendering.md.
package soft
