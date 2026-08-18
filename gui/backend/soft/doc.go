// Package soft renders a go-gui frame to an image on the CPU: no GPU, no
// window, no event loop.
//
// It is a third consumer of the same flat []gui.RenderCmd stream the GPU
// backends and the PDF printer replay, so it is purely additive — no other
// backend changes and none of them consult it.
//
// Typical use is a screenshot or a pixel-level regression test:
//
//	w := gui.SimpleWindow("Demo", 400, 300, &App{}, func(w *gui.Window) {
//		w.UpdateView(mainView)
//	})
//	err := soft.RenderToPNG(w, 2, "demo.png")
//
// Text is real, not approximated: the package builds a go-glyph TextSystem
// over a software glyph.DrawBackend and installs it as the window's
// gui.TextMeasurer, so shaping, metrics and glyph rasterization are the
// same code the GPU backends run.
//
// Phase 1 covers the core command kinds — clip, rect, stroke rect, circle,
// line, gradient, gradient border, image, and every text kind. SVG,
// shadow, blur, filter and stencil commands are accepted and skipped; see
// docs/specs/headless-software-rendering.md for the phase split.
package soft
