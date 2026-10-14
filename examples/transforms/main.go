// The transforms example shows ContainerCfg.Transform: a card that lifts
// on hover, a tilted badge, and a card that flips over in 3D with its
// back face hidden (#1019).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const (
	cardW = 160
	cardH = 100
)

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Transforms",
		Width:  640,
		Height: 300,
		State:  &app{},
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

type app struct {
	flipped bool
	// angle is the flip card's turn around its vertical axis, in
	// degrees. The flip animation moves it from 0 to 180 and back.
	angle float32
	// lift is the hover card's lift amount, from 0 (rest) to 1
	// (raised). HoverLift moves it toward 0 or 1.
	lift float32
	// badgeHit tells where in the window the last click on the badge
	// was.
	badgeHit string
}

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingLarge,
		Padding: gui.PaddingLarge,
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Transforms",
				TextStyle: theme.TextStyleTitle,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				Spacing:    gui.SpacingLarge,
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignTop,
				Content: []gui.View{
					liftCard(w, theme),
					badgeCard(w, theme),
					flipCard(w, theme),
				},
			}),
		},
	})
}

// liftCard moves up and grows a little while the pointer is over it.
// The ID and the hover test are on an outer row that does not move, so
// the lifted card cannot slide out from under a still pointer.
//
// HoverLift runs the lift tween and hands the card a TransformFn, so
// the animation moves the card in render-only frames, with no rebuild,
// like the flip card below.
func liftCard(w *gui.Window, theme gui.Theme) gui.View {
	st := gui.State[app](w)
	return gui.HoverLift("lift", gui.Translate(0, -6).Scale(1.05), &st.lift,
		func(xf func() gui.Transform) gui.View {
			return gui.Row(gui.ContainerCfg{
				ID:         "lift",
				Sizing:     gui.FitFit,
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					cardFn(theme, "card", "Hover me", theme.ColorPanel, xf),
				},
			})
		})
}

// doc:snippet-begin transforms-badge
// The badge is turned 12 degrees. Layout ignores the transform, so the
// row it is in keeps its size and nothing around it moves.

func badge(theme gui.Theme) gui.View {
	return gui.Row(gui.ContainerCfg{
		ID:         "badge",
		Padding:    gui.PaddingSmall,
		Radius:     gui.RadiusSmall,
		Color:      theme.ColorSelect,
		SizeBorder: gui.NoBorder,
		Transform:  gui.Rotate(-12),
		OnClick:    badgeClick,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "NEW",
				TextStyle: gui.TextStyle{Color: theme.ColorTextOnAccent},
			}),
		},
	})
}

// doc:snippet-end transforms-badge

// badgeCard puts the tilted badge on a card, with a line that tells
// where the last click on the badge was.
func badgeCard(w *gui.Window, theme gui.Theme) gui.View {
	hit := gui.State[app](w).badgeHit
	if hit == "" {
		hit = "Click the badge"
	}
	return gui.Column(gui.ContainerCfg{
		Width:   cardW,
		Height:  cardH,
		Sizing:  gui.FixedFixed,
		Padding: gui.PaddingMedium,
		Spacing: gui.SpacingSmall,
		Radius:  gui.RadiusMedium,
		Color:   theme.ColorPanel,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Inbox"}),
			badge(theme),
			gui.Text(gui.TextCfg{
				Text:      hit,
				TextStyle: theme.TextStyleCaption,
			}),
		},
	})
}

// badgeClick reports where the click was in the window. e.MouseX and
// e.MouseY are in the badge's own box. LocalToWindow maps a point in a
// widget's box out through every transform above it and its own, so the
// window point is where the pointer was, not where the unturned box
// would put it.
func badgeClick(ctx gui.EventCtx) {
	e := ctx.Event
	wx, wy, ok := ctx.Window.LocalToWindow(ctx.EffID("badge"), e.MouseX, e.MouseY)
	if ok {
		gui.State[app](ctx.Window).badgeHit = fmt.Sprintf("Hit at %.0f, %.0f", wx, wy)
	}
	ctx.Consume()
}

// flipCard turns over in 3D on a click. Both faces sit on the same spot:
// the back face is moved left by one card width onto the front. Each
// face hides while its back is toward the viewer, so only one shows at a
// time. The back face starts turned 180 degrees, so it reads the right
// way round once the card has flipped.
//
// The angle is read through TransformFn when the card is drawn, so the
// animation moves the card in render-only frames, with no rebuild.
func flipCard(w *gui.Window, theme gui.Theme) gui.View {
	angle := func() float32 { return gui.State[app](w).angle }
	front := func() gui.Transform {
		return gui.Perspective(600).RotateY(angle()).BackfaceHidden()
	}
	back := func() gui.Transform {
		return gui.Translate(-cardW, 0).Perspective(600).
			RotateY(angle() + 180).BackfaceHidden()
	}
	return gui.Row(gui.ContainerCfg{
		ID:         "flip",
		Width:      cardW,
		Height:     cardH,
		Sizing:     gui.FixedFixed,
		Padding:    gui.PaddingNone,
		Spacing:    gui.NoSpacing,
		SizeBorder: gui.NoBorder,
		OnClick:    flip,
		Content: []gui.View{
			cardFn(theme, "front", "Click to flip", theme.ColorPanel, front),
			cardFn(theme, "back", "Back side", theme.ColorActive, back),
		},
	})
}

// flip starts the flip animation toward the other face.
func flip(ctx gui.EventCtx) {
	s := gui.State[app](ctx.Window)
	s.flipped = !s.flipped
	var to float32
	if s.flipped {
		to = 180
	}
	tw := gui.NewTweenAnimation("flip", s.angle, to, func(v float32, w *gui.Window) {
		gui.State[app](w).angle = v
	})
	tw.Duration = 500 * time.Millisecond
	tw.Easing = gui.EaseInOutCubic
	tw.Refresh = gui.AnimationRefreshRenderOnly
	ctx.Window.AnimationAdd(tw)
	ctx.Consume()
}

func cardFn(theme gui.Theme, id, label string, color gui.Color, fn func() gui.Transform) gui.View {
	return gui.Column(cardCfg(theme, id, label, color, gui.Transform{}, fn))
}

func cardCfg(theme gui.Theme, id, label string, color gui.Color,
	xf gui.Transform, fn func() gui.Transform) gui.ContainerCfg {
	return gui.ContainerCfg{
		ID:          id,
		Width:       cardW,
		Height:      cardH,
		Sizing:      gui.FixedFixed,
		Padding:     gui.PaddingMedium,
		Radius:      gui.RadiusMedium,
		Color:       color,
		HAlign:      gui.HAlignCenter,
		VAlign:      gui.VAlignMiddle,
		Transform:   xf,
		TransformFn: fn,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: theme.TextStyleTitleSmall}),
		},
	}
}
