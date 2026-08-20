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
	}
}
