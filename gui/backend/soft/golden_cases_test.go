//go:build !wasm

package soft

// The recorded view set for TestPixelGolden.
//
// Each case is deliberately text-free (the harness gate enforces it)
// and chosen for what the command goldens in gui/ cannot see: blend
// order and premultiplied compositing, antialiasing on rounded
// corners and circles, and the phase-2 rasterizer kinds — shadow,
// blur, filter, rotation, SVG, image scaling. A case here catches a
// rasterizer regression that no RenderCmd serialization ever could.
//
// The stencil bracket (ContainerCfg.clipContents) is the one phase-2
// kind with no public trigger — the field is unexported — so it stays
// covered by the direct pixel assertions in draw_phase2_test.go.

import (
	"github.com/go-gui-org/go-gui/gui"
)

// themeByName resolves a registered preset for a pixelTheme pair.
// A missing name is a test bug — presets are registered by package
// init before any test runs.
func themeByName(name string) pixelTheme {
	t, ok := gui.ThemeGet(name)
	if !ok {
		panic("pixel golden: registered theme " + name + " not found")
	}
	return pixelTheme{t, name}
}

func pixelCases() []pixelCase {
	return []pixelCase{
		{
			// Rounded corners + border: the mask rasterizer's
			// antialiasing, visible only as pixels. A corner that
			// squares off, a border that halves, a radius that
			// creeps — none of it appears in a command diff.
			name: "rounded_border",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Column(gui.ContainerCfg{
							Width:       120,
							Height:      80,
							Sizing:      gui.FixedFixed,
							Radius:      gui.SomeF(12),
							Color:       gui.RGB(70, 130, 220),
							SizeBorder:  gui.SomeF(2),
							ColorBorder: gui.RGB(230, 235, 240),
						}),
					},
				})
			},
		},
		{
			// A half-alpha swatch over its checkboard: premultiplied
			// compositing, blend order and the checker tiling are all
			// pixel-only claims. The swatch paints no text.
			name: "blend",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.ColorSwatch(gui.ColorSwatchCfg{
							ID:    "sw",
							Color: gui.RGBA(80, 120, 200, 128),
						}),
					},
				})
			},
		},
		{
			// Linear gradient across the whole window. The renderer
			// samples the full stop list (no five-stop resampling on
			// CPU), so this pins the gradient sampler end to end.
			name: "linear_gradient",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					Gradient: &gui.GradientDef{
						Direction: gui.GradientToRight,
						Stops: []gui.GradientStop{
							{Color: gui.RGB(220, 60, 40), Pos: 0},
							{Color: gui.RGB(40, 80, 220), Pos: 0.5},
							{Color: gui.RGB(60, 200, 120), Pos: 1},
						},
					},
				})
			},
		},
		{
			name: "radial_gradient",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					Gradient: &gui.GradientDef{
						Type: gui.GradientRadial,
						Stops: []gui.GradientStop{
							{Color: gui.RGB(255, 240, 160), Pos: 0},
							{Color: gui.RGB(200, 120, 40), Pos: 1},
						},
					},
				})
			},
		},
		{
			// A border that fades along its run: gradient sampling on
			// the stroke path, with the interior left as the theme
			// background so the border's edges stand out.
			name: "gradient_border",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Column(gui.ContainerCfg{
							Width:      160,
							Height:     100,
							Sizing:     gui.FixedFixed,
							Radius:     gui.SomeF(10),
							SizeBorder: gui.SomeF(3),
							BorderGradient: &gui.GradientDef{
								Direction: gui.GradientToBottom,
								Stops: []gui.GradientStop{
									{Color: gui.RGB(120, 220, 120), Pos: 0},
									{Color: gui.RGB(220, 120, 220), Pos: 1},
								},
							},
						}),
					},
				})
			},
		},
		{
			// A circular container: the disc's edge is pure
			// antialiasing, and the ring is a separate path.
			name: "circle",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Circle(gui.ContainerCfg{
							Width:       120,
							Height:      120,
							Sizing:      gui.FixedFixed,
							Color:       gui.RGB(230, 140, 40),
							SizeBorder:  gui.SomeF(3),
							ColorBorder: gui.RGB(250, 250, 250),
						}),
					},
				})
			},
		},
		{
			// Drop shadow: the coverage-mask blur and the caster's
			// complement both only exist after rasterization. The
			// blur's soft edge is where a regression shows first.
			name: "shadow",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Column(gui.ContainerCfg{
							Width:      120,
							Height:     80,
							Sizing:     gui.FixedFixed,
							Radius:     gui.SomeF(10),
							Color:      gui.RGB(90, 140, 200),
							SizeBorder: gui.NoBorder,
							Shadow: &gui.BoxShadow{
								Color:      gui.RGBA(0, 0, 0, 150),
								OffsetX:    0,
								OffsetY:    6,
								BlurRadius: 12,
							},
						}),
					},
				})
			},
		},
		{
			// Spread-only ring: zero blur and zero offsets, so the
			// ring exists only because Spread grew the shadow shape
			// beyond the caster. The caster's fill covers the middle.
			name: "shadow-spread-ring",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Column(gui.ContainerCfg{
							Width:      120,
							Height:     80,
							Sizing:     gui.FixedFixed,
							Radius:     gui.SomeF(10),
							Color:      gui.RGB(90, 140, 200),
							SizeBorder: gui.NoBorder,
							Shadow: &gui.BoxShadow{
								Color:  gui.RGBA(0, 0, 0, 150),
								Spread: 6,
							},
						}),
					},
				})
			},
		},
		{
			// Spread + blur: the ring is spread, then softened by
			// the blur band; both must contribute to the visible
			// extent or the ring measures wrong.
			name: "shadow-spread-blur",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Column(gui.ContainerCfg{
							Width:      120,
							Height:     80,
							Sizing:     gui.FixedFixed,
							Radius:     gui.SomeF(10),
							Color:      gui.RGB(90, 140, 200),
							SizeBorder: gui.NoBorder,
							Shadow: &gui.BoxShadow{
								Color:      gui.RGBA(0, 0, 0, 150),
								Spread:     6,
								BlurRadius: 8,
							},
						}),
					},
				})
			},
		},
		{
			// Spread on an already-rounded caster: the shadow corner
			// radius must grow by spread too, or the ring squares off
			// at the corners (the divergence softRoundRect has two
			// radius sites for).
			name: "shadow-spread-rounded",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Column(gui.ContainerCfg{
							Width:      120,
							Height:     80,
							Sizing:     gui.FixedFixed,
							Radius:     gui.SomeF(24),
							Color:      gui.RGB(90, 140, 200),
							SizeBorder: gui.NoBorder,
							Shadow: &gui.BoxShadow{
								Color:  gui.RGBA(0, 0, 0, 150),
								Spread: 8,
							},
						}),
					},
				})
			},
		},
		{
			// Spread + offset: the cut-out erases the shadow under
			// the caster, so the ring survives only on the offset
			// side (there it is offset+spread wide) and on the two
			// sides; the far side has no ring at all.
			name: "shadow-spread-offset",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Column(gui.ContainerCfg{
							Width:      120,
							Height:     80,
							Sizing:     gui.FixedFixed,
							Radius:     gui.SomeF(10),
							Color:      gui.RGB(90, 140, 200),
							SizeBorder: gui.NoBorder,
							Shadow: &gui.BoxShadow{
								Color:   gui.RGBA(0, 0, 0, 150),
								Spread:  4,
								OffsetY: 6,
							},
						}),
					},
				})
			},
		},
		{
			// SDF blur: one opaque rect, blurred by the renderer.
			// The radius maps onto a sigma and three box passes —
			// arithmetic a command golden cannot see.
			name: "blur",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Column(gui.ContainerCfg{
							Width:      160,
							Height:     100,
							Sizing:     gui.FixedFixed,
							Radius:     gui.SomeF(10),
							Color:      gui.RGB(80, 190, 140),
							SizeBorder: gui.NoBorder,
							BlurRadius: 10,
						}),
					},
				})
			},
		},
		{
			// The filter bracket: content composited into an
			// offscreen layer, then passed through a colour matrix
			// (grayscale here). The layer round-trip and the matrix
			// application are pure pixel behaviour.
			name: "filter",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:      gui.FillFill,
					SizeBorder:  gui.NoBorder,
					ColorFilter: gui.ColorFilterGrayscale(),
					Content: []gui.View{
						gui.Column(gui.ContainerCfg{
							Width:      120,
							Height:     80,
							Sizing:     gui.FixedFixed,
							Color:      gui.RGB(220, 60, 40),
							SizeBorder: gui.NoBorder,
						}),
						gui.Column(gui.ContainerCfg{
							Width:      120,
							Height:     80,
							Sizing:     gui.FixedFixed,
							Color:      gui.RGB(40, 80, 220),
							SizeBorder: gui.NoBorder,
						}),
					},
				})
			},
		},
		{
			// Quarter-turn rotation: the offscreen layer is resampled
			// by the inverse transform, so the rotated child's edges
			// exercise the resample filter.
			name: "rotation",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.RotatedBox(gui.RotatedBoxCfg{
							QuarterTurns: 1,
							Content: gui.Column(gui.ContainerCfg{
								Width:      140,
								Height:     60,
								Sizing:     gui.FixedFixed,
								Color:      gui.RGB(140, 100, 220),
								SizeBorder: gui.NoBorder,
							}),
						}),
					},
				})
			},
		},
		{
			// An inline SVG: tessellation, the vertex-coloured mesh
			// pass and the gradient fill over a shared coverage mask.
			// The data is a literal, so no file I/O and no asset
			// catalog joins the recording.
			name: "svg",
			build: func(_ *gui.Window) gui.View {
				const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
					`<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">` +
					`<stop offset="0" stop-color="#4a9eff"/>` +
					`<stop offset="1" stop-color="#ff4a9e"/>` +
					`</linearGradient></defs>` +
					`<circle cx="50" cy="50" r="42" fill="url(#g)"/>` +
					`</svg>`
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Svg(gui.SvgCfg{
							Width:   120,
							Height:  120,
							SvgData: svg,
						}),
					},
				})
			},
		},
		{
			// The radial half of the tessellator, which the "svg" case
			// above cannot reach: a two-stop linear gradient needs
			// neither the curvature pass nor an isoline cut. A
			// multi-stop radial one takes both.
			//
			// What this pins is that the curvature criterion is not
			// just cheaper but visually equivalent. The edge-length
			// rule it replaced spent 75,776 triangles on a fan the
			// curvature rule leaves at 74 (issue #399); recorded either
			// way the picture is the same to within a delta of 4.
			name: "svg_radial_gradient",
			build: func(_ *gui.Window) gui.View {
				const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
					`<defs><radialGradient id="g" cx="0.5" cy="0.5" r="0.5">` +
					`<stop offset="0" stop-color="#fff2a8"/>` +
					`<stop offset="0.35" stop-color="#ff9a3c"/>` +
					`<stop offset="0.7" stop-color="#c2410c"/>` +
					`<stop offset="1" stop-color="#4a1d05"/>` +
					`</radialGradient></defs>` +
					`<rect x="4" y="4" width="92" height="92" fill="url(#g)"/>` +
					`</svg>`
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Svg(gui.SvgCfg{
							Width:   120,
							Height:  120,
							SvgData: svg,
						}),
					},
				})
			},
		},
		{
			// A multi-stop *linear* gradient is the only shape that
			// reaches the isoline splitter's vertex sort, where an odd
			// permutation used to reverse a triangle's winding
			// (issue #399). The rasterizer takes the whole mesh as one
			// path and accumulates *signed* coverage, so mixed winding
			// cancels along a shared edge and the background shows
			// through as a crack across the fill — three of them here
			// before the fix.
			//
			// This case records the appearance. It is not the gate: the
			// cracks cover ~0.3% of the frame, inside this harness's
			// tolerance. The exact gate is
			// TestSvgGradientSubdivisionPreservesWinding in gui/svg,
			// which counts the emitted triangles' orientations.
			name: "svg_multistop_gradient",
			build: func(_ *gui.Window) gui.View {
				const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
					`<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">` +
					`<stop offset="0" stop-color="#fff2a8"/>` +
					`<stop offset="0.2" stop-color="#ff9a3c"/>` +
					`<stop offset="0.4" stop-color="#c2410c"/>` +
					`<stop offset="0.6" stop-color="#7c2d12"/>` +
					`<stop offset="0.8" stop-color="#312e81"/>` +
					`<stop offset="1" stop-color="#0f172a"/>` +
					`</linearGradient></defs>` +
					`<rect x="4" y="4" width="92" height="92" fill="url(#g)"/>` +
					`</svg>`
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Svg(gui.SvgCfg{
							Width:   120,
							Height:  120,
							SvgData: svg,
						}),
					},
				})
			},
		},
		{
			// spreadMethod through the whole pipeline. Reflect tiles
			// the ramp, and each fold is a break in it: subdividing on
			// the clamped projection, as this path did before
			// issue #399, put the cuts nowhere near the folds and the
			// bands came out smeared into a single wash.
			//
			// It also covers the resolve step. objectBoundingBox is the
			// default gradient units, and rewriting such a gradient
			// into user space used to drop its spread method, so every
			// gradient that took the default padded no matter what it
			// asked for.
			name: "svg_gradient_spread",
			build: func(_ *gui.Window) gui.View {
				const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
					`<defs><linearGradient id="g" x1="0.4" y1="0" x2="0.6" y2="0" spreadMethod="reflect">` +
					`<stop offset="0" stop-color="#fff2a8"/>` +
					`<stop offset="0.3" stop-color="#ff9a3c"/>` +
					`<stop offset="1" stop-color="#312e81"/>` +
					`</linearGradient></defs>` +
					`<rect x="4" y="4" width="92" height="92" fill="url(#g)"/>` +
					`</svg>`
				return gui.Column(gui.ContainerCfg{
					Sizing: gui.FillFill, SizeBorder: gui.NoBorder,
					HAlign: gui.HAlignCenter, VAlign: gui.VAlignMiddle,
					Content: []gui.View{gui.Svg(gui.SvgCfg{
						Width: 120, Height: 120, SvgData: svg})},
				})
			},
		},
		{
			// The repeat sibling of the case above, and deliberately
			// fold-aligned where that one is not. Repeat's sawtooth is
			// the one spread with a real jump in it: the ramp steps
			// from its end back to its start at every integer of the
			// raw parameter. The split pass places cut vertices exactly
			// on those integers, so a coloring pass that resolves one
			// vertex at a time can hand a triangle the color from the
			// far side of the step and Gouraud smears it across the
			// band (issue #417).
			//
			// userSpaceOnUse with x1=0 x2=30 over a 90-wide rect puts
			// the corners on t=0 and t=3 exactly and the folds on t=1
			// and t=2 — no float luck deciding which side a vertex
			// falls on, which is what the reflect case above lacks.
			name: "svg_gradient_repeat_fold",
			build: func(_ *gui.Window) gui.View {
				const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 90 90">` +
					`<defs><linearGradient id="g" gradientUnits="userSpaceOnUse" ` +
					`x1="0" y1="0" x2="30" y2="0" spreadMethod="repeat">` +
					`<stop offset="0" stop-color="#fff2a8"/>` +
					`<stop offset="1" stop-color="#312e81"/>` +
					`</linearGradient></defs>` +
					`<rect x="0" y="0" width="90" height="90" fill="url(#g)"/>` +
					`</svg>`
				return gui.Column(gui.ContainerCfg{
					Sizing: gui.FillFill, SizeBorder: gui.NoBorder,
					HAlign: gui.HAlignCenter, VAlign: gui.VAlignMiddle,
					Content: []gui.View{gui.Svg(gui.SvgCfg{
						Width: 120, Height: 120, SvgData: svg})},
				})
			},
		},
		{
			// A memory image scaled up 12x: the sampler's filter and
			// the tiling are pixel-only. The source is a deterministic
			// checker built in the test, never a file.
			name: "image_scaled",
			build: func(_ *gui.Window) gui.View {
				const src = "pixel-golden-img"
				pix := make([]byte, 8*8*4)
				for y := range 8 {
					for x := range 8 {
						k := (y*8 + x) * 4
						if (x/4+y/4)%2 == 0 {
							pix[k], pix[k+1], pix[k+2], pix[k+3] = 60, 120, 220, 255
						} else {
							pix[k], pix[k+1], pix[k+2], pix[k+3] = 240, 240, 240, 255
						}
					}
				}
				resource := gui.UseImage(src, 8, 8, pix)
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Image(gui.ImageCfg{
							ID:     "img",
							Src:    resource,
							Width:  96,
							Height: 96,
						}),
					},
				})
			},
		},
		{
			// A progress bar without its text label: rounded track and
			// fill, where the fill's corner antialiasing against the
			// track is the pixel claim.
			name: "progress_fill",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					Content: []gui.View{
						gui.ProgressBar(gui.ProgressBarCfg{
							ID:      "pb",
							Percent: 0.42,
						}),
					},
				})
			},
		},
		{
			// The focus ring is painted from AmendLayout as a stroke;
			// this is the recording that proves it lands on the
			// swatch's edge rather than insetting it (mirrors
			// color_swatch_focused in the command goldens).
			name:    "swatch_focused",
			focusID: "sw",
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.ColorSwatch(gui.ColorSwatchCfg{
							ID:        "sw",
							Color:     gui.RGBA(80, 120, 200, 128),
							Focusable: true,
						}),
					},
				})
			},
		},
		{
			// The phase-2 representative case (visual-refresh §10): one
			// form row and one button row, recorded under the dark
			// platform themes as well as dark/light. The platform
			// themes override the base ladder at exactly the points the
			// refresh moves (body size, padding ladder), so an
			// unrecorded platform theme is where a base-ladder change
			// silently breaks a hand-tuned override.
			//
			// Text-free per the harness gate: the controls carry no
			// content, which still pins the chrome — insets, spacing,
			// radii, heights — that the density and ladder changes
			// move.
			name: "form_row_button_row",
			themes: []pixelTheme{
				{gui.ThemeDark, "dark"},
				{gui.ThemeLight, "light"},
				themeByName("macos-dark"),
				themeByName("gnome-dark"),
				themeByName("windows-dark"),
			},
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					Spacing:    gui.SomeF(gui.SpacingMedium),
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Row(gui.ContainerCfg{
							Sizing:     gui.FillFit,
							Spacing:    gui.SomeF(gui.SpacingSmall),
							SizeBorder: gui.NoBorder,
							Content: []gui.View{
								gui.Input(gui.InputCfg{ID: "fn"}),
								gui.Select(gui.SelectCfg{ID: "role"}),
								gui.Input(gui.InputCfg{ID: "em"}),
							},
						}),
						gui.Row(gui.ContainerCfg{
							Sizing:     gui.FillFit,
							Spacing:    gui.SomeF(gui.SpacingSmall),
							SizeBorder: gui.NoBorder,
							Content: []gui.View{
								gui.Button(gui.ButtonCfg{ID: "cancel"}),
								gui.Button(gui.ButtonCfg{ID: "save"}),
							},
						}),
					},
				})
			},
		},
		{
			// The 5b representative case (visual-refresh §10): a
			// focused switch under the base pair and the three
			// platform darks. The focus ring is where the platform
			// themes diverge from the base on purpose — the default
			// presets carry a spread-free glow (blur 2, no spread),
			// macOS overrides it with its own soft glow, and
			// GNOME/Windows deliberately keep no ring at all (border
			// recolor only) — so an unrecorded platform theme is
			// where a base-ring change silently breaks a hand-tuned
			// override. The switch is the case because it is the
			// focusable whose geometry the macOS theme also overrides
			// (38×22).
			name:    "switch_focused",
			focusID: "swt",
			themes: []pixelTheme{
				{gui.ThemeDark, "dark"},
				{gui.ThemeLight, "light"},
				themeByName("macos-dark"),
				themeByName("gnome-dark"),
				themeByName("windows-dark"),
			},
			build: func(_ *gui.Window) gui.View {
				return gui.Column(gui.ContainerCfg{
					Sizing:     gui.FillFill,
					SizeBorder: gui.NoBorder,
					HAlign:     gui.HAlignCenter,
					VAlign:     gui.VAlignMiddle,
					Content: []gui.View{
						gui.Switch(gui.SwitchCfg{ID: "swt", Selected: true}),
					},
				})
			},
		},
	}
}
