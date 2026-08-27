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
			w.UpdateView(mainView)
			w.AnimationAdd(&gui.Animate{
				// Keep the frame loop hot so the shader parameter updates continuously.
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
	elapsed := float32(time.Since(app.StartTime).Milliseconds()) / 1000.0

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
							Params: []float32{elapsed},
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
							Params: []float32{elapsed},
						},
						Content: []gui.View{gui.Text(gui.TextCfg{Text: "Plasma"})},
					}),
				},
			}),
		},
	})
}
