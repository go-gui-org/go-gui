package gui

import (
	"math"
	"reflect"
	"testing"
	"time"
)

// countThemeColors walks typ the same way a reader would count by
// hand: every Color stored by value, through nested structs and
// arrays, and nothing behind a pointer, slice, map or interface.
// It is a second, independent walk, so the offset table cannot pass
// by agreeing with itself.
func countThemeColors(typ reflect.Type) int {
	colorType := reflect.TypeFor[Color]()
	switch {
	case typ == colorType:
		return 1
	case typ.Kind() == reflect.Array:
		return typ.Len() * countThemeColors(typ.Elem())
	case typ.Kind() == reflect.Struct:
		n := 0
		for field := range typ.Fields() {
			n += countThemeColors(field.Type)
		}
		return n
	}
	return 0
}

func TestThemeFadeOffsetsCoverEveryColor(t *testing.T) {
	offsets := themeColorOffsetTable()
	want := countThemeColors(reflect.TypeFor[Theme]())
	if len(offsets) != want {
		t.Fatalf("offset table has %d colors, Theme holds %d",
			len(offsets), want)
	}
	if want < 100 {
		t.Fatalf("Theme holds only %d colors; the walk is broken", want)
	}
}

func TestThemeFadeBlendMidpoint(t *testing.T) {
	from, to := ThemeDark, ThemeLight
	var dst = to
	blendThemeColors(&dst, &from, &to, 0.5)

	check := func(name string, got, a, b Color) {
		t.Helper()
		want := RGBA(
			lerpU8(a.R, b.R, 0.5), lerpU8(a.G, b.G, 0.5),
			lerpU8(a.B, b.B, 0.5), lerpU8(a.A, b.A, 0.5),
		)
		if got != want {
			t.Fatalf("%s = %+v, want %+v", name, got, want)
		}
	}
	// A top-level color, one inside ThemeCfg, and one nested in a
	// TextStyle inside a private widget style.
	check("ColorBackground", dst.ColorBackground,
		from.ColorBackground, to.ColorBackground)
	check("Cfg.ColorBackground", dst.Cfg.ColorBackground,
		from.Cfg.ColorBackground, to.Cfg.ColorBackground)
	check("TextStyleDef.Color", dst.TextStyleDef.Color,
		from.TextStyleDef.Color, to.TextStyleDef.Color)
	if from.ColorBackground == to.ColorBackground {
		t.Fatal("dark and light share a background; test proves nothing")
	}
	// Non-color fields are the caller's: blend leaves them alone.
	if dst.Name != to.Name {
		t.Fatalf("Name = %q, want the target's %q", dst.Name, to.Name)
	}
}

func TestThemeFadeBlendEndsAndUnset(t *testing.T) {
	from, to := ThemeDark, ThemeLight
	var dst Theme

	dst = to
	blendThemeColors(&dst, &from, &to, 0)
	if dst.ColorBackground != from.ColorBackground {
		t.Fatalf("t=0: %+v, want from", dst.ColorBackground)
	}
	dst = to
	blendThemeColors(&dst, &from, &to, 1)
	if dst.ColorBackground != to.ColorBackground {
		t.Fatalf("t=1: %+v, want to", dst.ColorBackground)
	}

	// An unset color on either side has no value to blend from, so
	// the target's value (set or unset) is taken as it is.
	from.ColorBackground = Color{}
	dst = to
	blendThemeColors(&dst, &from, &to, 0.5)
	if dst.ColorBackground != to.ColorBackground {
		t.Fatalf("unset from: %+v, want to", dst.ColorBackground)
	}
	from = ThemeDark
	to.ColorBackground = Color{}
	dst = to
	blendThemeColors(&dst, &from, &to, 0.5)
	if dst.ColorBackground.IsSet() {
		t.Fatalf("unset to: %+v, want unset", dst.ColorBackground)
	}
}

// fadeTestWindow returns a window that already showed one frame of
// ThemeDark, so a later SetTheme is a switch and not the first pin.
func fadeTestWindow(t *testing.T, d time.Duration) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 200, Height: 100})
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill})
	}
	w.SetTheme(ThemeDark)
	w.SetThemeTransition(d)
	w.refreshLayout.Store(true)
	w.FrameFn()
	return w
}

func TestThemeFadeSetThemeFades(t *testing.T) {
	w := fadeTestWindow(t, 200*time.Millisecond)
	w.SetTheme(ThemeLight)

	// The window's theme is the target at once; only the installed
	// colors travel.
	if got := w.Theme(); got.id != ThemeLight.id {
		t.Fatalf("w.Theme() id = %d, want ThemeLight", got.id)
	}
	if !w.HasAnimation(themeFadeAnimationID) {
		t.Fatal("no fade animation registered")
	}
	if got := CurrentTheme().ColorBackground; got != ThemeDark.ColorBackground {
		t.Fatalf("t=0 background = %+v, want dark", got)
	}

	w.themeFadeStep(w.themeFade.gen, 0.5)
	w.FrameFn()
	mid := CurrentTheme().ColorBackground
	if mid == ThemeDark.ColorBackground || mid == ThemeLight.ColorBackground {
		t.Fatalf("t=0.5 background = %+v, want a blend", mid)
	}
	if got := w.FrameBackground(); got != mid {
		t.Fatalf("FrameBackground = %+v, want the blend %+v", got, mid)
	}

	w.themeFadeStep(w.themeFade.gen, 1)
	w.themeFadeDone(w.themeFade.gen)
	w.FrameFn()
	if got := installedThemeID.Load(); got != ThemeLight.id {
		t.Fatalf("after done installed id = %d, want ThemeLight %d",
			got, ThemeLight.id)
	}
}

func TestThemeFadeZeroTransitionSnaps(t *testing.T) {
	w := fadeTestWindow(t, 0)
	w.SetTheme(ThemeLight)
	if w.HasAnimation(themeFadeAnimationID) {
		t.Fatal("fade registered with zero transition")
	}
	if got := installedThemeID.Load(); got != ThemeLight.id {
		t.Fatalf("installed id = %d, want ThemeLight", got)
	}
}

func TestThemeFadeNegativeTransitionSnaps(t *testing.T) {
	// A negative transition clamps to zero and snaps (max(d, 0)).
	w := fadeTestWindow(t, -100*time.Millisecond)
	w.SetTheme(ThemeLight)
	if w.HasAnimation(themeFadeAnimationID) {
		t.Fatal("fade registered with negative transition")
	}
	if got := installedThemeID.Load(); got != ThemeLight.id {
		t.Fatalf("installed id = %d, want ThemeLight", got)
	}
}

func TestThemeFadeFirstPinSnaps(t *testing.T) {
	// Before the first frame there is nothing on screen to fade from.
	w := NewWindow(WindowCfg{State: new(int), Width: 200, Height: 100})
	w.SetThemeTransition(200 * time.Millisecond)
	w.SetTheme(ThemeLight)
	if w.HasAnimation(themeFadeAnimationID) {
		t.Fatal("fade registered before the first frame")
	}
}

func TestThemeFadeReducedMotionSnaps(t *testing.T) {
	w := fadeTestWindow(t, 200*time.Millisecond)
	// Embeds the no-op platform, not a nil interface: SetTheme reaches
	// SetSystemAppearanceCallback through it.
	w.nativePlatform = stubReducedMotionPlatform{
		NativePlatform: noopNativePlatform{}, pref: true,
	}
	w.SetTheme(ThemeLight)
	if w.HasAnimation(themeFadeAnimationID) {
		t.Fatal("fade registered under reduced motion")
	}
	if got := installedThemeID.Load(); got != ThemeLight.id {
		t.Fatalf("installed id = %d, want ThemeLight", got)
	}
}

func TestThemeFadeRestartStartsFromBlend(t *testing.T) {
	w := fadeTestWindow(t, 200*time.Millisecond)
	w.SetTheme(ThemeLight)
	oldGen := w.themeFade.gen
	w.themeFadeStep(oldGen, 0.5)
	w.FrameFn()
	mid := CurrentTheme().ColorBackground

	// Back to dark mid-fade: the new fade starts where the old one
	// stood, not from light, so nothing jumps.
	w.SetTheme(ThemeDark)
	w.FrameFn()
	if got := CurrentTheme().ColorBackground; got != mid {
		t.Fatalf("restart background = %+v, want %+v", got, mid)
	}
	// A value the old tween queued before the restart is ignored.
	w.themeFadeStep(oldGen, 1)
	w.FrameFn()
	if got := CurrentTheme().ColorBackground; got != mid {
		t.Fatalf("stale step moved background to %+v", got)
	}
}

func TestThemeFadeStaleDoneIgnored(t *testing.T) {
	w := fadeTestWindow(t, 200*time.Millisecond)
	w.SetTheme(ThemeLight)
	oldGen := w.themeFade.gen
	w.themeFadeStep(oldGen, 0.5)
	w.FrameFn()

	// Restart mid-fade, then deliver the old fade's Done: it must
	// not end the new fade nor install the old target.
	w.SetTheme(ThemeDark)
	newGen := w.themeFade.gen
	w.FrameFn()
	w.themeFadeDone(oldGen)
	if w.themeFade == nil || !w.themeFade.active {
		t.Fatal("stale done ended the new fade")
	}
	if !w.HasAnimation(themeFadeAnimationID) {
		t.Fatal("stale done removed the new fade animation")
	}
	if got := installedThemeID.Load(); got == ThemeLight.id {
		t.Fatal("stale done installed the old target")
	}

	// The new fade still runs to its own target.
	w.themeFadeStep(newGen, 1)
	w.themeFadeDone(newGen)
	w.FrameFn()
	if got := installedThemeID.Load(); got != ThemeDark.id {
		t.Fatalf("installed id = %d, want ThemeDark", got)
	}
}

func TestThemeFadeSnapCancelsFade(t *testing.T) {
	w := fadeTestWindow(t, 200*time.Millisecond)
	w.SetTheme(ThemeLight)
	w.SetThemeTransition(0)
	w.SetTheme(ThemeDark)
	if w.HasAnimation(themeFadeAnimationID) {
		t.Fatal("snap left the fade animation running")
	}
	w.FrameFn()
	if got := installedThemeID.Load(); got != ThemeDark.id {
		t.Fatalf("installed id = %d, want ThemeDark", got)
	}
}

func TestThemeFadeSameThemeSnaps(t *testing.T) {
	// Re-pinning the theme already shown (an OS appearance event that
	// did not change light/dark) changes no color. A fade here would
	// mint a fresh theme id each frame and throw away every cache
	// keyed on it for the whole transition.
	w := fadeTestWindow(t, 200*time.Millisecond)
	w.SetTheme(ThemeDark)
	if w.HasAnimation(themeFadeAnimationID) {
		t.Fatal("fade registered for a same-theme re-pin")
	}
	w.FrameFn()
	if got := installedThemeID.Load(); got != ThemeDark.id {
		t.Fatalf("installed id = %d, want ThemeDark", got)
	}
}

func TestThemeFadeSameTargetKeepsFade(t *testing.T) {
	// Re-pinning the target of a running fade lets that fade finish:
	// a snap would jump the colors, a restart would stall them.
	w := fadeTestWindow(t, 200*time.Millisecond)
	w.SetTheme(ThemeLight)
	gen := w.themeFade.gen
	w.themeFadeStep(gen, 0.5)
	w.FrameFn()
	mid := CurrentTheme().ColorBackground

	w.SetTheme(ThemeLight)
	if w.themeFade.gen != gen || !w.themeFade.active {
		t.Fatal("same-target re-pin restarted or ended the fade")
	}
	w.FrameFn()
	if got := CurrentTheme().ColorBackground; got != mid {
		t.Fatalf("background = %+v, want %+v", got, mid)
	}
	w.themeFadeStep(gen, 1)
	w.themeFadeDone(gen)
	w.FrameFn()
	if got := installedThemeID.Load(); got != ThemeLight.id {
		t.Fatalf("installed id = %d, want ThemeLight", got)
	}
}

func TestThemeFadeFollowSystemFades(t *testing.T) {
	w := fadeTestWindow(t, 200*time.Millisecond)
	w.appearanceMu.Lock()
	light, dark := ThemeLight, ThemeDark
	w.appearanceLight, w.appearanceDark = &light, &dark
	w.appearanceFollowing = true
	w.appearanceMu.Unlock()

	w.applySystemAppearance(AppearanceLight)
	if !w.HasAnimation(themeFadeAnimationID) {
		t.Fatal("OS appearance change did not fade")
	}
}

func TestThemeFadeInstallNoAlloc(t *testing.T) {
	w := fadeTestWindow(t, 200*time.Millisecond)
	w.SetTheme(ThemeLight)
	gen := w.themeFade.gen
	v := float32(0)
	// AllocsPerRun counts mallocs from every goroutine in the process,
	// not only this one. In the full package run under -race -cover,
	// goroutines left behind by earlier tests sometimes land enough
	// mallocs in the window to read as 1 (about 1 run in 4; never when
	// this test runs alone). Keep the lowest of a few measurements: a
	// real per-frame allocation shows in every one, noise does not.
	allocs := math.Inf(1)
	for range 5 {
		allocs = min(allocs, testing.AllocsPerRun(100, func() {
			v += 0.001
			w.themeFadeStep(gen, v)
			w.installTheme()
		}))
		if allocs == 0 {
			break
		}
	}
	if allocs != 0 {
		t.Fatalf("fade frame allocates %v times", allocs)
	}
}

func BenchmarkThemeFadeInstall(b *testing.B) {
	w := NewWindow(WindowCfg{State: new(int), Width: 200, Height: 100})
	w.SetTheme(ThemeDark)
	w.SetThemeTransition(200 * time.Millisecond)
	w.frameCount = 1
	w.SetTheme(ThemeLight)
	gen := w.themeFade.gen
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		w.themeFadeStep(gen, float32(i%100)/100)
		w.installTheme()
	}
}

// TestGoldenThemeFade records the midpoint of a fade in both
// directions, through the real frame pipeline.
func TestGoldenThemeFade(t *testing.T) {
	dirs := []struct {
		name     string
		from, to Theme
	}{
		{"dark_to_light", ThemeDark, ThemeLight},
		{"light_to_dark", ThemeLight, ThemeDark},
	}
	for _, d := range dirs {
		t.Run(d.name, func(t *testing.T) {
			w := NewWindow(WindowCfg{
				State: new(int), Width: goldenWidth, Height: goldenHeight,
			})
			w.viewGenerator = func(*Window) View {
				return Column(ContainerCfg{
					Sizing: FillFill,
					Content: []View{
						Button(ButtonCfg{ID: "b", Content: []View{
							Text(TextCfg{Text: "Button"}),
						}}),
						Input(InputCfg{ID: "in", Text: "Input"}),
					},
				})
			}
			w.SetTheme(d.from)
			w.SetThemeTransition(200 * time.Millisecond)
			w.refreshLayout.Store(true)
			w.FrameFn()

			w.SetTheme(d.to)
			w.themeFadeStep(w.themeFade.gen, 0.5)
			w.refreshLayout.Store(true)
			w.FrameFn()
			checkGolden(t, "theme_fade_mid."+d.name, serializeCmds(w.renderers))
		})
	}
}
