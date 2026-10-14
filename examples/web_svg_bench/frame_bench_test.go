package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// BenchmarkScrollFrame measures what one scrolled frame costs the gui
// package — view, layout, arrange and render-command emission — on
// the bench workload, without a backend. It is the layout half of the
// web numbers (goLayoutMs), profiled natively:
//
//	go test ./examples/web_svg_bench/ -run x -bench ScrollFrame -cpuprofile cpu.out
func BenchmarkScrollFrame(b *testing.B) {
	gui.SetTheme(gui.ThemeDark)
	for _, asset := range []string{"mix", "tiger", "icon"} {
		b.Run(asset, func(b *testing.B) {
			app := newBenchApp(func(k string) string {
				if k == "asset" {
					return asset
				}
				return ""
			})
			// The headless web runs' viewport.
			w := gui.NewWindow(gui.WindowCfg{State: app, Width: 982, Height: 604})
			w.SetSvgParser(svg.New())
			w.TestRender(mainView)
			// Warm the tessellation cache across the whole grid.
			for i := range 20 {
				w.ScrollVerticalToPct(scrollID, float32(i)/19)
				w.TestRender(nil)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				w.ScrollVerticalToPct(scrollID, float32(i%100)/99)
				w.TestRender(nil)
			}
			b.StopTimer()
			b.ReportMetric(float64(len(w.Renderers())), "cmds/frame")
		})
	}
}
