# Go-Gui Roadmap

This document outlines the planned future directions for the Go-Gui project.

## High Priority

- **WebGPU Backend**: Implement a WebGPU backend (via `GPUCanvasContext` / `navigator.gpu`) to provide lower GPU overhead and compute shader support on the web target.
- **SDL2 Deprecation (Renderer)**: Move toward OpenGL as the default renderer on all desktop platforms. SDL2 will be retained for windowing and input handling, but the software renderer will be phased out as the primary path.
- **Large-File Maintenance**: Continue the effort to reduce file sizes for better maintainability.
    - Split `gui/styles_widget.go` into widget-specific style files.
    - Split `gui/theme.go` into more cohesive groups.
    - Split `gui/fonts.go` into `fonts.go` and `icons.go`.
- **Trap Quit Requests**: When displaying a dialog, the request to quit the program (cmd+q on mac) should be ignored.

## Medium Priority

- **Performance Optimizations**:
    - Investigate releasing the `Window` mutex during user-provided View function execution.
    - Further optimization of scratch pools for transient layout data.
- **Expanded Test Coverage**:
    - Add dedicated unit tests for `view_input_keys.go` and `view_input_layout.go`.
    - Increase coverage for SVG animation rendering in `render_svg_animation.go`.

## Low Priority

- **Upstream Feature Integration**:
    - Support diagonal gradients in SVG once `go-glyph` adds angle support.
- **Improved Tooling**:
    - Enhance time-travel debugging with better state inspection.
