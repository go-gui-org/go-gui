package main

import (
	"math"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// demoTextAnim shows TextCfg.Anim: the canned text animations and the
// Custom escape hatch. Nothing here is wired to a per-effect control.
// An animation declared on TextCfg.Anim registers itself the first
// time the text is generated and retires on its own once the text
// leaves the view tree.
func demoTextAnim(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.Some(t.SpacingSmall),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				ID: "text-anim-intro",
				Text: "TextCfg.Anim animates a Text view. Kind names a " +
					"canned effect; Custom takes a function of progress. " +
					"An animated text needs an ID: the animation and its " +
					"progress are keyed by identity.",
				TextStyle: t.N5,
				Mode:      gui.TextModeWrap,
			}),
			textAnimEntranceCard(t, app),
			textAnimLoopCard(t),
			textAnimCustomCard(t),
		},
	})
}

// textAnimEntranceCard holds the one-shot kinds. An entrance plays once
// per identity and then sits at its resting appearance, so replaying it
// means giving the labels a new identity: the counter goes into the ID,
// which retires the finished animation and registers a fresh one. That
// is the supported way to retrigger today; TextAnimCfg has no Retrigger
// field.
func textAnimEntranceCard(t gui.Theme, app *ShowcaseApp) gui.View {
	label := func(part, text string, kind gui.TextAnimKind) gui.View {
		return gui.Text(gui.TextCfg{
			ID:        gui.ScopeIDN("text-anim", part, app.TextAnimReplay),
			Text:      text,
			TextStyle: t.B4,
			Anim: gui.TextAnimCfg{
				Kind:     kind,
				Duration: 600 * time.Millisecond,
			},
		})
	}
	return textDemoCard("", "Entrances (play once per identity)", 0, []gui.View{
		gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.Some(t.SpacingMedium),
			Padding: gui.NoPadding,
			VAlign:  gui.VAlignMiddle,
			Content: []gui.View{
				label("fade", "Fade In", gui.TextAnimFadeIn),
				label("slide-up", "Slide Up", gui.TextAnimSlideUp),
				label("slide-left", "Slide Left", gui.TextAnimSlideLeft),
				label("pop", "Pop", gui.TextAnimPop),
			},
		}),
		gui.Button(gui.ButtonCfg{
			ID:      "text-anim-replay",
			Padding: gui.NewPadding(6, 16, 6, 16),
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: "Replay", TextStyle: t.N3}),
			},
			OnClick: func(ctx gui.EventCtx) {
				appState(ctx.Window).TextAnimReplay++
				ctx.Consume()
			},
		}),
	})
}

// textAnimLoopCard holds the repeating kinds. Repeat keeps the driver
// alive, so these run for as long as the page is on screen.
func textAnimLoopCard(t gui.Theme) gui.View {
	label := func(id, text string, kind gui.TextAnimKind) gui.View {
		return gui.Text(gui.TextCfg{
			ID:        id,
			Text:      text,
			TextStyle: t.B4,
			Anim:      gui.TextAnimCfg{Kind: kind, Repeat: true},
		})
	}
	return textDemoCard("", "Loops", 0, []gui.View{
		gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFit,
			Spacing: gui.Some(t.SpacingMedium),
			Padding: gui.NoPadding,
			VAlign:  gui.VAlignMiddle,
			Content: []gui.View{
				label("text-anim-pulse", "Pulse", gui.TextAnimPulse),
				label("text-anim-shimmer", "Shimmer", gui.TextAnimShimmer),
				label("text-anim-shake", "Shake", gui.TextAnimShake),
				// The typewriter measures the whole string and paints a
				// prefix, so the row keeps its width from the first
				// frame and nothing beside it reflows as the text types
				// itself out.
				label("text-anim-typewriter", "Typing this out",
					gui.TextAnimTypewriter),
			},
		}),
	})
}

// textAnimCustomCard is the escape hatch: a rise and fall that no
// canned kind covers, written as a function of eased progress.
func textAnimCustomCard(t gui.Theme) gui.View {
	return textDemoCard("", "Custom", 0, []gui.View{
		gui.Text(gui.TextCfg{
			ID:        "text-anim-custom",
			Text:      "Custom: sine offset and sway",
			TextStyle: t.B4,
			Anim: gui.TextAnimCfg{
				Duration: 2 * time.Second,
				Repeat:   true,
				Custom: func(p float32) gui.TextAnimFrame {
					phase := float64(p) * 2 * math.Pi
					return gui.TextAnimFrame{
						OffsetY:  6 * float32(math.Sin(phase)),
						Rotation: 0.08 * float32(math.Sin(2*phase)),
					}
				},
			},
		}),
	})
}
