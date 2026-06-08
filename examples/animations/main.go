// The animations example showcases tween, spring, keyframe, layout,
// and hero transitions in one window.
package main

import (
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// Animation System Demo
//
// Demonstrates the animation types available in go-gui:
//
// 1. TWEEN   - interpolate between values over a fixed duration
// 2. SPRING  - physics-based motion with configurable bounciness
// 3. BOUNCE  - chained tween with bounce easing
// 4. ELASTIC - tween with elastic overshoot easing
// 5. KEYFRAME - multi-waypoint animation (shake effect)
// 6. LAYOUT  - automatic position/size interpolation between frames
// 7. HERO    - morph elements between different views

type State struct {
	SidebarWidth float32
	BoxX         float32
	SpringValue  float32
	ShowDetail   bool
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State: &State{
			SidebarWidth: 200,
			BoxX:         50,
			SpringValue:  100,
		},
		Title:  "Animations",
		Width:  800,
		Height: 600,
		OnInit: func(w *gui.Window) {
			// Register the root view once; animation callbacks mutate state.
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	s := gui.State[State](w)
	ww, wh := w.WindowSize()

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Spacing: gui.Some[float32](20),
		Padding: gui.SomeP(20, 20, 20, 20),
		Content: []gui.View{
			// Control buttons
			gui.Row(gui.ContainerCfg{
				Spacing: gui.Some[float32](10),
				Content: []gui.View{
					animButton("Tween", tweenBox),
					animButton("Spring", springSidebar),
					animButton("Bounce", bounceAnim),
					animButton("Elastic", elasticAnim),
					animButton("Keyframe", keyframeAnim),
					animButton("Layout", layoutAnim),
					animButton("Hero", heroAnim),
				},
			}),
			// Demo area
			gui.Row(gui.ContainerCfg{
				Sizing: gui.FillFill,
				Content: []gui.View{
					// Animated sidebar
					gui.Column(gui.ContainerCfg{
						ID:      "sidebar",
						Width:   s.SidebarWidth,
						Sizing:  gui.FixedFill,
						Color:   gui.Purple,
						Radius:  gui.Some[float32](8),
						Padding: gui.SomeP(10, 10, 10, 10),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Sidebar"}),
						},
					}),
					// Canvas for absolute positioning
					gui.Canvas(gui.ContainerCfg{
						Sizing: gui.FillFill,
						Content: []gui.View{
							// Hero card
							gui.Column(gui.ContainerCfg{
								ID:      "hero-card",
								Hero:    true,
								X:       50,
								Y:       220,
								Width:   120,
								Height:  80,
								Sizing:  gui.FixedFixed,
								Color:   gui.Orange,
								Radius:  gui.Some[float32](12),
								Padding: gui.SomeP(10, 10, 10, 10),
								VAlign:  gui.VAlignMiddle,
								HAlign:  gui.HAlignCenter,
								Content: []gui.View{
									gui.Text(gui.TextCfg{
										Text: "Click Hero",
										TextStyle: gui.TextStyle{
											Size:  gui.CurrentTheme().N4.Size,
											Color: gui.Black,
										},
									}),
								},
							}),
							// Tween/elastic box
							gui.Column(gui.ContainerCfg{
								ID:     "box",
								X:      s.BoxX,
								Y:      50,
								Width:  80,
								Height: 80,
								Sizing: gui.FixedFixed,
								Color:  gui.Blue,
								Radius: gui.Some[float32](8),
							}),
							// Spring/bounce circle
							gui.Column(gui.ContainerCfg{
								ID:     "spring",
								X:      s.SpringValue,
								Y:      150,
								Width:  40,
								Height: 40,
								Sizing: gui.FixedFixed,
								Color:  gui.Green,
								Radius: gui.Some[float32](20),
							}),
						},
					}),
				},
			}),
		},
	})
}

func detailView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Spacing: gui.Some[float32](20),
		Padding: gui.SomeP(20, 20, 20, 20),
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Content: []gui.View{
					animButton("Back", heroBack),
				},
			}),
			// Hero card — expanded
			gui.Column(gui.ContainerCfg{
				ID:      "hero-card",
				Hero:    true,
				Sizing:  gui.FillFill,
				Color:   gui.Orange,
				Radius:  gui.Some[float32](16),
				Padding: gui.SomeP(20, 20, 20, 20),
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						ID:   "detail-title",
						Hero: true,
						Text: "Detail View",
						TextStyle: gui.TextStyle{
							Size:  theme.B1.Size,
							Color: gui.Black,
						},
					}),
					gui.Text(gui.TextCfg{
						ID:   "detail-text1",
						Hero: true,
						Text: "The card morphed from small to large.",
						TextStyle: gui.TextStyle{
							Size:  theme.N4.Size,
							Color: gui.Black,
						},
					}),
					gui.Text(gui.TextCfg{
						ID:   "detail-text2",
						Hero: true,
						Text: "Click Back to morph it back.",
						TextStyle: gui.TextStyle{
							Size:  theme.N4.Size,
							Color: gui.Black,
						},
					}),
				},
			}),
		},
	})
}

func animButton(label string, action func(w *gui.Window)) gui.View {
	return gui.Button(gui.ButtonCfg{
		Content: []gui.View{gui.Text(gui.TextCfg{Text: label})},
		OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
			action(w)
		},
	})
}

// Tween: interpolate box_x over fixed duration with easing.
func tweenBox(w *gui.Window) {
	s := gui.State[State](w)
	target := float32(400)
	if s.BoxX >= 300 {
		target = 50
	}
	a := gui.NewTweenAnimation("box_move", s.BoxX, target,
		func(v float32, w *gui.Window) {
			gui.State[State](w).BoxX = v
		})
	a.Duration = 500 * time.Millisecond
	a.Easing = gui.EaseOutCubic
	w.AnimationAdd(a)
}

// Spring: physics-based sidebar resize.
func springSidebar(w *gui.Window) {
	s := gui.State[State](w)
	target := float32(60)
	if s.SidebarWidth <= 100 {
		target = 200
	}
	a := gui.NewSpringAnimation("sidebar",
		func(v float32, w *gui.Window) {
			gui.State[State](w).SidebarWidth = v
		})
	a.Config = gui.SpringBouncy
	a.SpringTo(s.SidebarWidth, target)
	w.AnimationAdd(a)
}

// Bounce: chained tween with bounce easing, then return.
func bounceAnim(w *gui.Window) {
	s := gui.State[State](w)
	a := gui.NewTweenAnimation("bounce", s.SpringValue, 300,
		func(v float32, w *gui.Window) {
			gui.State[State](w).SpringValue = v
		})
	a.Duration = 800 * time.Millisecond
	a.Easing = gui.EaseOutBounce
	a.OnDone = func(w *gui.Window) {
		ret := gui.NewTweenAnimation("bounce_return", 300, 100,
			func(v float32, w *gui.Window) {
				gui.State[State](w).SpringValue = v
			})
		ret.Duration = 300 * time.Millisecond
		ret.Easing = gui.EaseOutQuad
		w.AnimationAdd(ret)
	}
	w.AnimationAdd(a)
}

// Elastic: tween with elastic overshoot easing.
func elasticAnim(w *gui.Window) {
	s := gui.State[State](w)
	target := float32(500)
	if s.BoxX >= 300 {
		target = 50
	}
	a := gui.NewTweenAnimation("elastic", s.BoxX, target,
		func(v float32, w *gui.Window) {
			gui.State[State](w).BoxX = v
		})
	a.Duration = 1000 * time.Millisecond
	a.Easing = gui.EaseOutElastic
	w.AnimationAdd(a)
}

// Layout: capture positions then modify state; framework animates.
func layoutAnim(w *gui.Window) {
	w.AnimateLayout(gui.LayoutTransitionCfg{
		Duration: 300 * time.Millisecond,
	})
	s := gui.State[State](w)
	if s.SidebarWidth > 100 {
		s.SidebarWidth = 60
	} else {
		s.SidebarWidth = 200
	}
}

// Hero: morph hero-card between main and detail views.
func heroAnim(w *gui.Window) {
	w.AnimationAdd(gui.NewHeroTransition(gui.HeroTransitionCfg{
		Duration: 600 * time.Millisecond,
	}))
	w.UpdateView(detailView)
}

func heroBack(w *gui.Window) {
	w.AnimationAdd(gui.NewHeroTransition(gui.HeroTransitionCfg{
		Duration: 600 * time.Millisecond,
	}))
	w.UpdateView(mainView)
}

// Keyframe: multi-waypoint shake effect.
func keyframeAnim(w *gui.Window) {
	s := gui.State[State](w)
	center := s.BoxX
	a := gui.NewKeyframeAnimation("shake", []gui.Keyframe{
		{At: 0.0, Value: center},
		{At: 0.2, Value: center - 30, Easing: gui.EaseOutQuad},
		{At: 0.4, Value: center + 25, Easing: gui.EaseOutQuad},
		{At: 0.6, Value: center - 15, Easing: gui.EaseOutQuad},
		{At: 0.8, Value: center + 8, Easing: gui.EaseOutQuad},
		{At: 1.0, Value: center, Easing: gui.EaseOutQuad},
	}, func(v float32, w *gui.Window) {
		gui.State[State](w).BoxX = v
	})
	a.Duration = 500 * time.Millisecond
	w.AnimationAdd(a)
}
