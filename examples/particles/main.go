// Particle system toy — interactive particle simulation with
// configurable physics, emitter types, and visual presets.
// Click/drag on the canvas to move the emitter.
package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// Screen identifies the active view.
type Screen uint8

const (
	ScreenLanding Screen = iota
	ScreenPlaying
)

// EmitterType selects how particles are spawned.
type EmitterType uint8

const (
	EmitterPoint EmitterType = iota
	EmitterRing
	EmitterLine
)

// Particle is a single live particle.
type Particle struct {
	X, Y    float32
	VX, VY  float32
	Life    float32 // 1.0 → 0.0
	Decay   float32 // life loss per second
	Size    float32 // initial radius
	R, G, B uint8
}

// App holds all mutable application state.
type App struct {
	Particles    []Particle
	LandingFrame int
	Version      uint64

	BurstCount int

	EmitterX float32
	EmitterY float32

	SpawnRate   float32 // particles per frame
	SpawnAccum  float32
	SpreadAngle float32 // half-angle in radians

	GravityY float32
	WindX    float32
	Friction float32

	SizeMin  float32
	SizeMax  float32
	Lifetime float32 // seconds

	BaseColor gui.Color
	Screen    Screen

	// Config
	EmitterType EmitterType
}

const (
	maxParticles = 5000
	windowW      = 1100
	windowH      = 700
	sidebarW     = 320
	canvasW      = windowW - sidebarW
	tickAnim     = "particle-tick"
	tickDelay    = 16 * time.Millisecond
	dt           = float32(tickDelay) / float32(time.Second)
)

// --- Color palette ---

var (
	colorBG        = gui.RGB(6, 10, 20)
	colorNeonCyan  = gui.RGB(75, 240, 242)
	colorNeonPink  = gui.RGB(255, 16, 240)
	colorNeonGreen = gui.RGB(57, 255, 20)
	colorDimText   = gui.RGB(178, 191, 222)
	colorPanel     = gui.RGB(18, 22, 32)
)

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	app := &App{}
	presetFountain(app)
	app.EmitterX = canvasW / 2
	app.EmitterY = float32(windowH) * 0.7
	app.Particles = make([]Particle, 0, maxParticles)

	w := gui.NewWindow(gui.WindowCfg{
		State:     app,
		Title:     "Particles",
		Width:     windowW,
		Height:    windowH,
		FixedSize: true,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			w.AnimationAdd(&gui.Animate{
				AnimID: tickAnim,
				Delay:  tickDelay,
				Repeat: true,
				Callback: func(_ *gui.Animate, w *gui.Window) {
					a := gui.State[App](w)
					a.LandingFrame++
					if a.Screen == ScreenPlaying {
						spawnParticles(&a.Particles, a, a.EmitterX, a.EmitterY)
						updateParticles(&a.Particles, a)
					}
					a.Version++
				},
			})
		},
		OnEvent: handleEvent,
	})

	backend.Run(w)
}

// --- Event handling ---

func handleEvent(e *gui.Event, w *gui.Window) {
	if e.Type != gui.EventKeyDown {
		return
	}
	app := gui.State[App](w)
	switch e.KeyCode {
	case gui.KeyEscape:
		if app.Screen == ScreenPlaying {
			app.Screen = ScreenLanding
			app.Particles = app.Particles[:0]
			e.IsHandled = true
		}
	case gui.KeyR:
		if app.Screen == ScreenPlaying {
			app.Particles = app.Particles[:0]
			e.IsHandled = true
		}
	case gui.Key1:
		presetFountain(app)
		e.IsHandled = true
	case gui.Key2:
		presetFire(app)
		e.IsHandled = true
	case gui.Key3:
		presetSnow(app)
		e.IsHandled = true
	case gui.Key4:
		presetExplosion(app)
		e.IsHandled = true
	case gui.Key5:
		presetSparkler(app)
		e.IsHandled = true
	}
}

// --- View routing ---

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	ww, wh := w.WindowSize()
	if app.Screen == ScreenPlaying {
		return playView(w, float32(ww), float32(wh))
	}
	return landingView(w, float32(ww), float32(wh))
}

// --- Landing page ---

func landingView(w *gui.Window, ww, wh float32) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	blink := app.LandingFrame%60 < 30

	startColor := colorNeonGreen
	if !blink {
		startColor = colorNeonGreen.WithOpacity(0.2)
	}

	return gui.Column(gui.ContainerCfg{
		Width:      ww,
		Height:     wh,
		Sizing:     gui.FixedFixed,
		Color:      colorBG,
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Spacing:    gui.SomeF(16),
		SizeBorder: gui.NoBorder,
		Padding:    gui.SomeP(24, 24, 24, 24),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "PARTICLES",
				TextStyle: ts(theme.B1, 52, colorNeonCyan),
			}),
			gui.Text(gui.TextCfg{
				Text:      "INTERACTIVE TOY",
				TextStyle: ts(theme.B2, 24, colorNeonPink),
			}),
			gui.Text(gui.TextCfg{
				Text:      "ARCADE SECTOR 2026",
				TextStyle: ts(theme.M3, 14, colorNeonCyan.WithOpacity(0.6)),
			}),
			gui.Rectangle(gui.RectangleCfg{
				Width:  300,
				Height: 2,
				Sizing: gui.FixedFixed,
				Color:  colorNeonCyan.WithOpacity(0.3),
			}),
			gui.Button(gui.ButtonCfg{
				ID:          "particles_start",
				MinWidth:    180,
				Color:       colorNeonGreen.WithOpacity(0.12),
				ColorHover:  colorNeonGreen.WithOpacity(0.3),
				ColorClick:  colorNeonGreen.WithOpacity(0.5),
				ColorBorder: colorNeonGreen,
				SizeBorder:  gui.SomeF(2),
				Padding:     gui.SomeP(14, 28, 14, 28),
				OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
					a := gui.State[App](w)
					a.Screen = ScreenPlaying
				},
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "START",
						TextStyle: ts(theme.B3, 20, colorNeonGreen),
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      "CLICK TO START",
				TextStyle: ts(theme.B3, 16, startColor),
			}),
			gui.Rectangle(gui.RectangleCfg{
				Width:  300,
				Height: 1,
				Sizing: gui.FixedFixed,
				Color:  colorNeonCyan.WithOpacity(0.15),
			}),
			gui.Text(gui.TextCfg{
				Text:      "DRAG TO AIM  •  1-5 PRESETS  •  ESC MENU",
				TextStyle: ts(theme.M3, 12, colorDimText.WithOpacity(0.6)),
			}),
		},
	})
}

// --- Play view ---

func playView(w *gui.Window, ww, wh float32) gui.View {
	app := gui.State[App](w)
	cw := ww - sidebarW
	return gui.Row(gui.ContainerCfg{
		Width:      ww,
		Height:     wh,
		Sizing:     gui.FixedFixed,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			sidebarView(w, wh),
			gui.DrawCanvas(gui.DrawCanvasCfg{
				ID:      "particles",
				Version: app.Version,
				Width:   cw,
				Height:  wh,
				Sizing:  gui.FixedFixed,
				Color:   colorBG,
				OnDraw: func(dc *gui.DrawContext) {
					drawParticleSlice(dc, app.Particles)
					ex, ey := app.EmitterX, app.EmitterY
					xh := colorNeonCyan.WithOpacity(0.4)
					dc.Line(ex-8, ey, ex+8, ey, xh, 1)
					dc.Line(ex, ey-8, ex, ey+8, xh, 1)
				},
				OnClick: func(l *gui.Layout, e *gui.Event, w *gui.Window) {
					a := gui.State[App](w)
					ox := l.Shape.X
					oy := l.Shape.Y
					a.EmitterX = e.MouseX
					a.EmitterY = e.MouseY
					w.MouseLock(gui.MouseLockCfg{
						MouseMove: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							a := gui.State[App](w)
							a.EmitterX = e.MouseX - ox
							a.EmitterY = e.MouseY - oy
						},
						MouseUp: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
							w.MouseUnlock()
						},
					})
				},
			}),
		},
	})
}

// --- Sidebar ---

func sidebarView(w *gui.Window, wh float32) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	sectionSpacing := gui.SomeF(8)
	sectionPadding := gui.SomeP(10, 12, 10, 12)

	return gui.Column(gui.ContainerCfg{
		Width:      sidebarW,
		Sizing:     gui.FixedFill,
		Color:      colorPanel,
		Spacing:    gui.SomeF(15),
		Padding:    gui.SomeP(10, 10, 10, 10),
		SizeBorder: gui.NoBorder,
		ID:         "particles-scroll",
		Scrollable: true,
		Content: []gui.View{
			// --- Emitter section ---
			gui.Column(gui.ContainerCfg{
				Title:       "Emitter",
				TitleBG:     colorPanel,
				Spacing:     sectionSpacing,
				Padding:     sectionPadding,
				SizeBorder:  gui.SomeF(1),
				ColorBorder: colorNeonCyan.WithOpacity(0.5),
				Content: []gui.View{
					gui.Select(gui.SelectCfg{
						ID:          "emitter-type",
						Selected:    []string{emitterName(app.EmitterType)},
						Options:     []string{"Point", "Ring", "Line"},
						FloatZIndex: 10,
						OnSelect: func(sel []string, _ *gui.Event, w *gui.Window) {
							a := gui.State[App](w)
							switch sel[0] {
							case "Ring":
								a.EmitterType = EmitterRing
							case "Line":
								a.EmitterType = EmitterLine
							default:
								a.EmitterType = EmitterPoint
							}
						},
					}),
					sliderRow("Rate", "spawn-rate", app.SpawnRate, 1, 30, 1,
						func(v float32, _ *gui.Event, w *gui.Window) {
							gui.State[App](w).SpawnRate = v
						}),
					sliderRow("Spread", "spread", app.SpreadAngle, 0, math.Pi, 0.1,
						func(v float32, _ *gui.Event, w *gui.Window) {
							gui.State[App](w).SpreadAngle = v
						}),
				},
			}),
			// --- Physics section ---
			gui.Column(gui.ContainerCfg{
				Title:       "Physics",
				TitleBG:     colorPanel,
				Spacing:     sectionSpacing,
				Padding:     sectionPadding,
				SizeBorder:  gui.SomeF(1),
				ColorBorder: colorNeonCyan.WithOpacity(0.4),
				Content: []gui.View{
					sliderRow("Gravity", "gravity", app.GravityY, -300, 300, 5,
						func(v float32, _ *gui.Event, w *gui.Window) {
							gui.State[App](w).GravityY = v
						}),
					sliderRow("Wind", "wind", app.WindX, -200, 200, 5,
						func(v float32, _ *gui.Event, w *gui.Window) {
							gui.State[App](w).WindX = v
						}),
					sliderRow("Friction", "friction", app.Friction, 0.80, 1.00, 0.005,
						func(v float32, _ *gui.Event, w *gui.Window) {
							gui.State[App](w).Friction = v
						}),
				},
			}),
			// --- Appearance section ---
			gui.Column(gui.ContainerCfg{
				Title:       "Appearance",
				TitleBG:     colorPanel,
				Spacing:     sectionSpacing,
				Padding:     sectionPadding,
				SizeBorder:  gui.SomeF(1),
				ColorBorder: colorNeonCyan.WithOpacity(0.5),
				Content: []gui.View{
					gui.Rectangle(gui.RectangleCfg{Height: 0}),
					gui.ColorPicker(gui.ColorPickerCfg{
						ID:    "base-color",
						Color: app.BaseColor,
						OnColorChange: func(c gui.Color, _ *gui.Event, w *gui.Window) {
							gui.State[App](w).BaseColor = c
						},
					}),
					sliderRow("Min Size", "size-min", app.SizeMin, 1, 10, 0.5,
						func(v float32, _ *gui.Event, w *gui.Window) {
							gui.State[App](w).SizeMin = v
						}),
					sliderRow("Max Size", "size-max", app.SizeMax, 1, 15, 0.5,
						func(v float32, _ *gui.Event, w *gui.Window) {
							gui.State[App](w).SizeMax = v
						}),
					sliderRow("Lifetime", "lifetime", app.Lifetime, 0.3, 8, 0.1,
						func(v float32, _ *gui.Event, w *gui.Window) {
							gui.State[App](w).Lifetime = v
						}),
				},
			}),
			// --- Presets section ---
			gui.Column(gui.ContainerCfg{
				Title:       "Presets",
				TitleBG:     colorPanel,
				Spacing:     gui.SomeF(0),
				Padding:     gui.NoPadding,
				SizeBorder:  gui.SomeF(1),
				ColorBorder: colorNeonCyan.WithOpacity(0.5),
				Content: []gui.View{
					gui.Row(gui.ContainerCfg{
						Padding:    gui.SomeP(12, 10, 8, 10),
						SizeBorder: gui.NoBorder,
						Content: []gui.View{
							presetBtn(w, "Fountain", colorNeonCyan, presetFountain),
							presetBtn(w, "Fire", gui.RGB(255, 80, 20), presetFire),
							presetBtn(w, "Snow", gui.RGB(220, 230, 255), presetSnow),
						},
					}),
					gui.Row(gui.ContainerCfg{
						Padding:    gui.SomeP(0, 10, 10, 10),
						SizeBorder: gui.NoBorder,
						Content: []gui.View{
							presetBtn(w, "Explode", gui.RGB(255, 200, 0), presetExplosion),
							presetBtn(w, "Sparkler", gui.RGB(255, 215, 100), presetSparkler),
						},
					}),
				},
			}),
			// --- Stats ---
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("Particles: %d / %d",
					len(app.Particles), maxParticles),
				TextStyle: ts(theme.M3, 12, colorDimText.WithOpacity(0.5)),
			}),
		},
	})
}

// --- Drawing ---

func drawParticleSlice(dc *gui.DrawContext, particles []Particle) {
	for i := range particles {
		p := &particles[i]
		alpha := p.Life
		if alpha > 1 {
			alpha = 1
		}
		radius := p.Size * alpha
		if radius < 0.5 {
			continue
		}
		dc.FilledCircle(p.X, p.Y, radius,
			gui.RGBA(p.R, p.G, p.B, uint8(alpha*255)))
	}
}

// --- Physics ---

func spawnParticles(particles *[]Particle, app *App, ex, ey float32) {
	// Burst spawn
	if app.BurstCount > 0 {
		n := app.BurstCount
		if len(*particles)+n > maxParticles {
			n = maxParticles - len(*particles)
		}
		for range n {
			*particles = append(*particles, newParticle(app, ex, ey))
		}
		app.BurstCount = 0
	}

	// Continuous spawn
	app.SpawnAccum += app.SpawnRate
	if app.SpawnAccum > app.SpawnRate*2 {
		app.SpawnAccum = app.SpawnRate * 2
	}
	for app.SpawnAccum >= 1 && len(*particles) < maxParticles {
		*particles = append(*particles, newParticle(app, ex, ey))
		app.SpawnAccum--
	}
}

func newParticle(app *App, ex, ey float32) Particle {
	speed := float32(80 + rand.IntN(120))
	angle := float32(0.0)

	switch app.EmitterType {
	case EmitterPoint:
		// Upward cone: center at -pi/2 (up), spread by SpreadAngle
		angle = -math.Pi/2 + (rand.Float32()*2-1)*app.SpreadAngle
	case EmitterRing:
		// Offset on circle, velocity outward
		a := rand.Float32() * 2 * math.Pi
		r := float32(25 + rand.IntN(15))
		ex += r * cos32(a)
		ey += r * sin32(a)
		angle = a
	case EmitterLine:
		// Horizontal line, particles emit upward
		ex += (rand.Float32()*2 - 1) * 100
		angle = -math.Pi/2 + (rand.Float32()*2-1)*app.SpreadAngle*0.3
	}

	size := app.SizeMin + rand.Float32()*(app.SizeMax-app.SizeMin)

	// Color variation: ±20 per channel
	r := clampU8(int(app.BaseColor.R) + rand.IntN(41) - 20)
	g := clampU8(int(app.BaseColor.G) + rand.IntN(41) - 20)
	b := clampU8(int(app.BaseColor.B) + rand.IntN(41) - 20)

	return Particle{
		X:     ex,
		Y:     ey,
		VX:    speed * cos32(angle),
		VY:    speed * sin32(angle),
		Life:  1.0,
		Decay: 1.0 / app.Lifetime,
		Size:  size,
		R:     r,
		G:     g,
		B:     b,
	}
}

func updateParticles(particles *[]Particle, app *App) {
	n := 0
	ps := *particles
	for i := range ps {
		p := &ps[i]
		p.VY += app.GravityY * dt
		p.VX += app.WindX * dt
		p.VX *= app.Friction
		p.VY *= app.Friction
		p.X += p.VX * dt
		p.Y += p.VY * dt
		p.Life -= p.Decay * dt
		if p.Life > 0 {
			ps[n] = ps[i]
			n++
		}
	}
	*particles = ps[:n]
}

// --- Presets ---

func presetFountain(app *App) {
	app.EmitterType = EmitterPoint
	app.SpawnRate = 10
	app.SpreadAngle = 0.3
	app.GravityY = 200
	app.WindX = 0
	app.Friction = 0.99
	app.BaseColor = colorNeonCyan
	app.SizeMin = 2
	app.SizeMax = 5
	app.Lifetime = 2.5
	app.BurstCount = 0
}

func presetFire(app *App) {
	app.EmitterType = EmitterLine
	app.SpawnRate = 15
	app.SpreadAngle = 0.4
	app.GravityY = -60
	app.WindX = 0
	app.Friction = 0.96
	app.BaseColor = gui.RGB(255, 80, 20)
	app.SizeMin = 3
	app.SizeMax = 8
	app.Lifetime = 1.5
	app.BurstCount = 0
}

func presetSnow(app *App) {
	app.EmitterType = EmitterLine
	app.SpawnRate = 4
	app.SpreadAngle = 1.5
	app.GravityY = 30
	app.WindX = 20
	app.Friction = 0.995
	app.BaseColor = gui.RGB(220, 230, 255)
	app.SizeMin = 2
	app.SizeMax = 4
	app.Lifetime = 5
	app.BurstCount = 0
}

func presetExplosion(app *App) {
	app.EmitterType = EmitterPoint
	app.SpawnRate = 0
	app.SpreadAngle = math.Pi
	app.GravityY = 0
	app.WindX = 0
	app.Friction = 0.95
	app.BaseColor = gui.RGB(255, 200, 0)
	app.SizeMin = 3
	app.SizeMax = 7
	app.Lifetime = 1.0
	app.BurstCount = 200
}

func presetSparkler(app *App) {
	app.EmitterType = EmitterPoint
	app.SpawnRate = 20
	app.SpreadAngle = math.Pi
	app.GravityY = 40
	app.WindX = 0
	app.Friction = 0.92
	app.BaseColor = gui.RGB(255, 215, 100)
	app.SizeMin = 1
	app.SizeMax = 3
	app.Lifetime = 0.8
	app.BurstCount = 0
}

// --- Helpers ---

func sliderRow(label, id string, val, min, max, step float32,
	onChange func(float32, *gui.Event, *gui.Window)) gui.View {
	theme := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Padding:    gui.NoPadding,
		Spacing:    gui.SomeF(6),
		SizeBorder: gui.NoBorder,
		VAlign:     gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: ts(theme.M3, 12, colorDimText),
				MinWidth:  58,
			}),
			gui.Slider(gui.SliderCfg{
				ID:       id,
				Value:    val,
				Min:      min,
				Max:      max,
				Step:     step,
				OnChange: onChange,
				Sizing:   gui.FillFixed,
				Height:   16,
				Width:    100,
			}),
		},
	})
}

func presetBtn(_ *gui.Window, label string, color gui.Color,
	apply func(*App)) gui.View {
	theme := gui.CurrentTheme()
	return gui.Button(gui.ButtonCfg{
		Color:       color.WithOpacity(0.1),
		ColorHover:  color.WithOpacity(0.25),
		ColorClick:  color.WithOpacity(0.4),
		ColorBorder: color.WithOpacity(0.6),
		SizeBorder:  gui.SomeF(1),
		Padding:     gui.SomeP(6, 10, 6, 10),
		Radius:      gui.SomeF(4),
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			a := gui.State[App](w)
			apply(a)
		},
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: ts(theme.M3, 11, color),
			}),
		},
	})
}

func emitterName(et EmitterType) string {
	switch et {
	case EmitterRing:
		return "Ring"
	case EmitterLine:
		return "Line"
	default:
		return "Point"
	}
}

func ts(base gui.TextStyle, size float32, color gui.Color) gui.TextStyle {
	base.Size = size
	base.Color = color
	return base
}

func cos32(a float32) float32 { return float32(math.Cos(float64(a))) }
func sin32(a float32) float32 { return float32(math.Sin(float64(a))) }

func clampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
