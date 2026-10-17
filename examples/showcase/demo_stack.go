package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
)

// stackDemoCard is one layer of the cards demo. The fills are
// translucent, so where two cards overlap the lower one shows through.
type stackDemoCard struct {
	id, name string
	color    gui.Color
}

var stackDemoCards = [...]stackDemoCard{
	{"red", "Red", gui.RGBA(220, 60, 60, 220)},
	{"green", "Green", gui.RGBA(40, 160, 90, 220)},
	{"blue", "Blue", gui.RGBA(50, 100, 220, 220)},
}

// demoStack shows Stack two ways: overlapping cards whose layer order a
// click changes, and two common layouts that need one view over another.
func demoStack(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)
	top := stackDemoCards[app.StackOrder[len(app.StackOrder)-1]].name
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: "A Stack lays its children one over the other. The first child" +
					" is the bottom layer and the last is on top.",
				TextStyle: t.TextStyleBody,
				Mode:      gui.TextModeWrap,
			}),
			gui.Text(gui.TextCfg{
				Text: "Click a card to bring it to the front. Where cards overlap," +
					" the top one gets the click. On top: " + top,
				TextStyle: t.TextStyleTitleSmall,
				Mode:      gui.TextModeWrap,
			}),
			demoStackCards(app, t),
			gui.Text(gui.TextCfg{Text: "Common uses:", TextStyle: t.TextStyleTitleSmall}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingLarge,
				Padding: gui.NoPadding,
				VAlign:  gui.VAlignBottom,
				Content: []gui.View{
					demoStackAvatar(t),
					demoStackCaption(t),
				},
			}),
		},
	})
}

// demoStackCards lays the cards in a Stack, bottom first. Each card's
// wrapper pads it down and right by a fixed step per card, so a card
// keeps its place when it changes layer: only which card covers which
// changes.
func demoStackCards(app *ShowcaseApp, t gui.Theme) gui.View {
	// The card colors are the same in every theme, so the text on them
	// is too.
	name := t.TextStyleTitle
	name.Color = gui.White
	note := t.TextStyleBodySmall
	note.Color = gui.White
	layers := make([]gui.View, 0, len(app.StackOrder))
	for layer, c := range app.StackOrder {
		step := float32(c)
		layers = append(layers, gui.Column(gui.ContainerCfg{
			// The wrapper covers the whole Stack. It has no click
			// handler, so a press outside its card goes to the layers
			// under it.
			Sizing:     gui.FillFill,
			Padding:    gui.NewPadding(step*50, 0, 0, step*80),
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Column(gui.ContainerCfg{
					ID:          stackDemoCards[c].id,
					Width:       170,
					Height:      110,
					Sizing:      gui.FixedFixed,
					Color:       stackDemoCards[c].color,
					ColorBorder: gui.White,
					Radius:      gui.RadiusMedium,
					Padding:     gui.PaddingMedium,
					Spacing:     gui.SpacingSmall,
					OnClick: func(ctx gui.EventCtx) {
						stackDemoRaise(appState(ctx.Window), c)
						ctx.Consume()
					},
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: stackDemoCards[c].name, TextStyle: name}),
						gui.Text(gui.TextCfg{Text: fmt.Sprintf("layer %d", layer+1), TextStyle: note}),
					},
				}),
			},
		}))
	}
	return gui.Stack(gui.ContainerCfg{
		ID:      "stack-cards",
		Width:   330,
		Height:  210,
		Sizing:  gui.FixedFixed,
		Padding: gui.NoPadding,
		Content: layers,
	})
}

// stackDemoRaise moves card c to the top of the order. The cards above
// it each drop one layer.
func stackDemoRaise(app *ShowcaseApp, c int) {
	k := 0
	for k < len(app.StackOrder) && app.StackOrder[k] != c {
		k++
	}
	if k == len(app.StackOrder) {
		return // not a card in the order; a raise would duplicate one
	}
	for ; k < len(app.StackOrder)-1; k++ {
		app.StackOrder[k] = app.StackOrder[k+1]
	}
	app.StackOrder[len(app.StackOrder)-1] = c
}

// stackDemoAvatarSize is the avatar's width and height. Its radius is
// half of it, so a change to the size keeps the avatar a circle.
const stackDemoAvatarSize = 64

// demoStackAvatar puts a count badge over the top-right corner of an
// initials avatar. The avatar's wrapper pads it away from that corner,
// so the badge hangs over the avatar's edge. It takes the theme from
// demoStack, like demoStackCards, so all three demos read one theme.
func demoStackAvatar(t gui.Theme) gui.View {
	initials := t.TextStyleTitle
	initials.Color = gui.White // the avatar fill is the same in every theme
	return gui.Stack(gui.ContainerCfg{
		ID:         "stack-avatar",
		Sizing:     gui.FitFit,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				Padding:    gui.NewPadding(8, 8, 0, 0), // ergonomics-audit:spacing — room for the badge to overhang the avatar's corner, not an inset
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Width:      stackDemoAvatarSize,
						Height:     stackDemoAvatarSize,
						Sizing:     gui.FixedFixed,
						Color:      gui.Hex(0x7a4bc2),
						Radius:     gui.RadiusPx(stackDemoAvatarSize / 2), // ergonomics-audit:spacing — half the avatar: a circle, not a control corner
						SizeBorder: gui.NoBorder,
						HAlign:     gui.HAlignCenter,
						VAlign:     gui.VAlignMiddle,
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "GG", TextStyle: initials}),
						},
					}),
				},
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFill,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				HAlign:     gui.HAlignEnd,
				VAlign:     gui.VAlignTop,
				Content: []gui.View{
					gui.Badge(gui.BadgeCfg{Label: "3", Variant: gui.BadgeError}),
				},
			}),
		},
	})
}

// demoStackCaption puts a translucent caption bar along the bottom of a
// photo. The bar's wrapper fills the Stack and aligns the bar to its
// bottom; the bar fills the width.
func demoStackCaption(t gui.Theme) gui.View {
	caption := t.TextStyleBody
	caption.Color = gui.White // the bar is dark in every theme
	return gui.Stack(gui.ContainerCfg{
		ID:         "stack-caption",
		Sizing:     gui.FitFit,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Image(gui.ImageCfg{
				Src:    embeddedAssetPath("assets/image_clip_face.jpg"),
				Width:  180,
				Height: 180,
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFill,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignBottom,
				Content: []gui.View{
					gui.Column(gui.ContainerCfg{
						Sizing:     gui.FillFit,
						Color:      gui.RGBA(0, 0, 0, 150),
						Radius:     gui.NoRadius,
						SizeBorder: gui.NoBorder,
						Padding:    gui.PadVH(gui.PadSmall, gui.PadMedium),
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Caption over a photo", TextStyle: caption}),
						},
					}),
				},
			}),
		},
	})
}
