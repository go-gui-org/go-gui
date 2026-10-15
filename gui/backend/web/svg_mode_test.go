//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"
)

// withPageGlobals sets the page globals initSvgMode reads for one test
// and restores them after. webgl says whether goGuiSvgGL.init returns a
// renderer; path2d whether the Path2D helper is there; mode is the
// page's goGuiSvgMode ("" = unset).
func withPageGlobals(t *testing.T, webgl, path2d bool, mode string) {
	t.Helper()
	g := js.Global()
	names := []string{"goGuiSvgGL", "goGuiBuildTrisPath", svgModeGlobal, "document"}
	saved := make([]js.Value, len(names))
	for i, n := range names {
		saved[i] = g.Get(n)
	}
	var funcs []js.Func
	fn := func(f func(args []js.Value) any) js.Func {
		jf := js.FuncOf(func(_ js.Value, args []js.Value) any { return f(args) })
		funcs = append(funcs, jf)
		return jf
	}
	t.Cleanup(func() {
		for i, n := range names {
			g.Set(n, saved[i])
		}
		for _, f := range funcs {
			f.Release()
		}
	})

	doc := g.Get("Object").New()
	doc.Set("createElement", fn(func([]js.Value) any {
		c := g.Get("Object").New()
		c.Set("addEventListener", fn(func([]js.Value) any { return nil }))
		return c
	}))
	g.Set("document", doc)

	api := g.Get("Object").New()
	api.Set("init", fn(func([]js.Value) any {
		if !webgl {
			return js.Null() // no WebGL2 context
		}
		return g.Get("Object").New()
	}))
	g.Set("goGuiSvgGL", api)

	if path2d {
		g.Set("goGuiBuildTrisPath", fn(func([]js.Value) any { return nil }))
	} else {
		g.Set("goGuiBuildTrisPath", js.Undefined())
	}
	if mode != "" {
		g.Set(svgModeGlobal, mode)
	} else {
		g.Set(svgModeGlobal, js.Undefined())
	}
}

// WebGL2 is the default; without it the Path2D cache; a page opts out
// with goGuiSvgMode = "canvas2d". The global reports the mode in use.
func TestInitSvgModeSelection(t *testing.T) {
	for _, tc := range []struct {
		name          string
		webgl, path2d bool
		mode          string
		want          svgMode
	}{
		{"default", true, true, "", svgModeWebGL},
		{"no webgl2", false, true, "", svgModePath2D},
		{"no helpers", false, false, "", svgModeCanvas2D},
		{"opt out", true, true, "canvas2d", svgModeCanvas2D},
		{"unknown value", true, true, "bogus", svgModeWebGL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withPageGlobals(t, tc.webgl, tc.path2d, tc.mode)
			b := &Backend{}
			b.initSvgMode()
			if b.svgMode != tc.want {
				t.Errorf("svgMode = %v, want %v", b.svgMode, tc.want)
			}
			if got := js.Global().Get(svgModeGlobal).String(); got != tc.want.String() {
				t.Errorf("%s = %q, want %q", svgModeGlobal, got, tc.want)
			}
			// WebGL batches that fall back to Canvas2D use the Path2D
			// cache when it exists.
			if tc.path2d && tc.want != svgModeCanvas2D && !b.paths.ok() {
				t.Error("Path2D cache not set up")
			}
		})
	}
}
