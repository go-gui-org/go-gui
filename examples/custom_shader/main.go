// This example demonstrates animated fragment shaders inside ordinary containers (advanced: custom shaders).
// Custom_shader renders animated fragment shaders inside ordinary
// go-gui containers. The shaders read Shadertoy's built-in iTime and
// set Shader.Animate, so the app keeps no clock and adds no animation
// of its own.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark.WithBorders(false))

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Custom Shader Demo",
		Width:  600,
		Height: 400,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
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

func mainView(_ *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingLarge,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Custom Fragment Shader Demo"}),
			gui.Row(gui.ContainerCfg{
				Spacing: gui.SpacingLarge,
				Content: []gui.View{rainbowTile(), plasmaTile()},
			}),
		},
	})
}

// doc:snippet-begin custom-shader-rainbow

// rainbowTile is an animated shader. iTime counts seconds from the
// first frame that drew a shader. Animate keeps frames coming while
// the tile is on screen, and each frame rebuilds only the render
// commands: no view function runs and nothing is re-arranged.
func rainbowTile() gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:  180,
		Height: 180,
		Sizing: gui.FixedFixed,
		Radius: gui.RadiusPx(16), // ergonomics-audit:spacing — decorative radius, not a control corner
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Shader: &gui.Shader{
			Metal: `
				float2 st = in.uv * 0.5 + 0.5;
				float3 c = 0.5 + 0.5 * cos(iTime + st.xyx + float3(0,2,4));
				float4 frag_color = float4(c, 1.0);
			`,
			GLSL: `
				vec2 st = uv * 0.5 + 0.5;
				vec3 c = 0.5 + 0.5 * cos(iTime + st.xyx + vec3(0,2,4));
				vec4 frag_color = vec4(c, 1.0);
			`,
			Animate: true,
		},
		Content: []gui.View{gui.Text(gui.TextCfg{Text: "Rainbow"})},
	})
}

// doc:snippet-end custom-shader-rainbow

// plasmaTile layers sine waves over iTime.
func plasmaTile() gui.View {
	return gui.Column(gui.ContainerCfg{
		Width:  180,
		Height: 180,
		Sizing: gui.FixedFixed,
		Radius: gui.RadiusPx(16), // ergonomics-audit:spacing — decorative radius, not a control corner
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Shader: &gui.Shader{
			Metal: `
				float2 st = in.uv * 3.0;
				float v = sin(st.x + iTime) + sin(st.y + iTime)
					+ sin(st.x + st.y + iTime)
					+ sin(length(st) + 1.5 * iTime);
				v = v * 0.25 + 0.5;
				float3 c = float3(
					sin(v * 3.14159),
					sin(v * 3.14159 + 2.094),
					sin(v * 3.14159 + 4.188));
				c = c * 0.5 + 0.5;
				float4 frag_color = float4(c, 1.0);
			`,
			GLSL: `
				vec2 st = uv * 3.0;
				float v = sin(st.x + iTime) + sin(st.y + iTime)
					+ sin(st.x + st.y + iTime)
					+ sin(length(st) + 1.5 * iTime);
				v = v * 0.25 + 0.5;
				vec3 c = vec3(
					sin(v * 3.14159),
					sin(v * 3.14159 + 2.094),
					sin(v * 3.14159 + 4.188));
				c = c * 0.5 + 0.5;
				vec4 frag_color = vec4(c, 1.0);
			`,
			Animate: true,
		},
		Content: []gui.View{gui.Text(gui.TextCfg{Text: "Plasma"})},
	})
}
