// This example demonstrates animated fragment shaders inside ordinary containers (advanced: custom shaders).
// Custom_shader renders animated fragment shaders inside ordinary
// go-gui containers.
package main

import (
	"time"

	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const shaderTickAnimationID = "shader_tick"

// rainbowParams and plasmaParams back the shaders' Params slices below.
// Reused every frame so the two shader uniforms cost no allocation;
// a []float32{elapsed} literal per shader would alloc twice per frame.
var rainbowParams [1]float32
var plasmaParams [1]float32

type App struct {
	StartTime time.Time
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark.WithBorders(false))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{StartTime: time.Now()},
		Title:  "Custom Shader Demo",
		Width:  600,
		Height: 400,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
			w.AnimationAdd(&gui.Animate{
				// Keep the frame loop hot so the shader parameter updates continuously.
				// WARNING: Repeat with an empty per-tick callback rebuilds
				// layout every animation tick (~16ms) forever. Fine for
				// this demo, but do not copy it as a default — an idle
				// app should have no repeating animations at all.
				AnimID:   shaderTickAnimationID,
				Repeat:   true,
				Callback: func(_ *gui.Animate, _ *gui.Window) {},
			})
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	// Wall-clock elapsed, truncated to milliseconds. The Animate API
	// passes no clock into its callback (Callback takes only the
	// animation and the window; the nominal dt reaches Update, not the
	// callback), so there is no animation-clock value to read here —
	// the view recomputes from StartTime every hot-loop tick instead.
	elapsed := float32(time.Since(app.StartTime).Milliseconds()) / 1000.0
	rainbowParams[0] = elapsed
	plasmaParams[0] = elapsed

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.Some[float32](20),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Custom Fragment Shader Demo"}),
			gui.Row(gui.ContainerCfg{
				Spacing: gui.Some[float32](20),
				Content: []gui.View{
					// Animated rainbow
					gui.Column(gui.ContainerCfg{
						Width:  200,
						Height: 200,
						Sizing: gui.FixedFixed,
						Radius: gui.Some[float32](16),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Shader: &gui.Shader{
							Metal: `
								float t = in.p0.x;
								float2 st = in.uv * 0.5 + 0.5;
								float3 c = 0.5 + 0.5 * cos(t + st.xyx + float3(0,2,4));
								float4 frag_color = float4(c, 1.0);
							`,
							GLSL: `
								float t = p0.x;
								vec2 st = uv * 0.5 + 0.5;
								vec3 c = 0.5 + 0.5 * cos(t + st.xyx + vec3(0,2,4));
								vec4 frag_color = vec4(c, 1.0);
							`,
							Params: rainbowParams[:],
						},
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Rainbow"})},
					}),
					// Plasma effect
					gui.Column(gui.ContainerCfg{
						Width:  200,
						Height: 200,
						Sizing: gui.FixedFixed,
						Radius: gui.Some[float32](16),
						HAlign: gui.HAlignCenter,
						VAlign: gui.VAlignMiddle,
						Shader: &gui.Shader{
							Metal: `
								float t = in.p0.x;
								float2 st = in.uv * 3.0;
									float v = sin(st.x + t) + sin(st.y + t)
										+ sin(st.x + st.y + t)
										+ sin(length(st) + 1.5 * t);
									v = v * 0.25 + 0.5;
								float3 c = float3(
										sin(v * 3.14159),
										sin(v * 3.14159 + 2.094),
										sin(v * 3.14159 + 4.188));
									c = c * 0.5 + 0.5;
								float4 frag_color = float4(c, 1.0);
							`,
							GLSL: `
								float t = p0.x;
								vec2 st = uv * 3.0;
								float v = sin(st.x + t) + sin(st.y + t)
									+ sin(st.x + st.y + t)
									+ sin(length(st) + 1.5 * t);
								v = v * 0.25 + 0.5;
								vec3 c = vec3(
									sin(v * 3.14159),
									sin(v * 3.14159 + 2.094),
									sin(v * 3.14159 + 4.188));
								c = c * 0.5 + 0.5;
								vec4 frag_color = vec4(c, 1.0);
							`,
							Params: plasmaParams[:],
						},
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Plasma"})},
					}),
				},
			}),
		},
	})
}
