// The scroll demo shows scroll containers, programmatic scroll
// control, and theme toggling.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	Light bool
	Pct   float32
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "scroll_demo",
		Width:  400,
		Height: 600,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			// Give the scroll panel focus so its scrollbar is visible on startup.
			w.SetFocus("scroll-panel")
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		Content: []gui.View{
			topRow(app),
			gui.Rectangle(gui.RectangleCfg{Height: 0.5, Sizing: gui.FillFixed}),
			pctRow(app),
			gui.Rectangle(gui.RectangleCfg{Height: 0.5, Sizing: gui.FillFixed}),
			scrollColumn("scroll-panel", "scroll-panel", scrollText, w),
		},
	})
}

func scrollColumn(scrollID string, focusID string, text string, w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	overflow := gui.ScrollbarHidden
	if w.IsFocus(focusID) {
		overflow = gui.ScrollbarVisible
	}

	var colorBorder gui.Color
	if w.IsFocus(focusID) {
		colorBorder = theme.ButtonStyle.ColorBorderFocus
	} else {
		colorBorder = theme.ContainerStyle.Color
	}

	pad := gui.PaddingSmall
	pad.Right = theme.ScrollbarStyle.Size + 4

	return gui.Column(gui.ContainerCfg{
		ID:         scrollID,
		Scrollable: true,
		ScrollbarCfgY: &gui.ScrollbarCfg{
			Overflow: overflow,
		},
		ColorBorder: colorBorder,
		Padding:     gui.Some(pad),
		Sizing:      gui.FillFill,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				ID:        focusID,
				Focusable: true,
				Text:      text,
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}

func pctRow(app *App) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.Some[float32](4),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("%.0f%%", app.Pct*100),
				TextStyle: gui.CurrentTheme().B3,
			}),
			pctButton("scroll-panel", 0),
			pctButton("scroll-panel", 25),
			pctButton("scroll-panel", 50),
			pctButton("scroll-panel", 75),
			pctButton("scroll-panel", 100),
		},
	})
}

func pctButton(idScroll string, pct int) gui.View {
	pctF := float32(pct) / 100
	return gui.Button(gui.ButtonCfg{
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: fmt.Sprintf("%d%%", pct)}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			w.ScrollVerticalToPct(idScroll, pctF)
			app := gui.State[App](w)
			app.Pct = w.ScrollVerticalPct(idScroll)
			e.IsHandled = true
		},
	})
}

func topRow(app *App) gui.View {
	theme := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Scroll Demo",
				TextStyle: theme.B1,
			}),
			gui.Rectangle(gui.RectangleCfg{
				Sizing: gui.FillFit,
				Color:  gui.ColorTransparent,
			}),
			themeButton(app),
		},
	})
}

func themeButton(app *App) gui.View {
	textSel := gui.IconMoon
	textUnsel := gui.IconSunnyO
	return gui.Toggle(gui.ToggleCfg{
		Focusable:    true,
		TextSelect:   textSel,
		TextUnselect: textUnsel,
		TextStyle:    gui.CurrentTheme().Icon3,
		Padding:      gui.Some(gui.PaddingSmall),
		Selected:     app.Light,
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			app := gui.State[App](w)
			app.Light = !app.Light
			if app.Light {
				w.SetTheme(gui.ThemeLight.WithBorders(true))
			} else {
				w.SetTheme(gui.ThemeDark.WithBorders(true))
			}
			e.IsHandled = true
		},
	})
}

const scrollText = `Fish and. Called it, earth great image, i set gathering blessed to of two every you'll and Their. Saw sea good. Called every and creeping have also. God cattle earth. Air thing subdue multiply of made land living bearing great. Our so yielding greater together whose third fly don't given bring creature. Seasons midst fowl Have man. Saw living signs. Air them signs. Very created. Of. She'd.

Firmament and in. Man. Green stars very. It dry created, said air day every fowl our their form which seed may land. He sea first yielding over abundantly set divided every let that doesn't hath place Let heaven subdue it to behold moving lesser a heaven. Likeness. So yielding moving bring lesser.

Behold waters our gathering moveth may kind saying itself shall fowl light i fruitful deep moved the seas don't you're be sea every face whales were the third seas replenish. Together moving. Deep evening a said winged Make heaven, replenish let green male likeness in multiply our, fruitful likeness and, fish replenish in evening Darkness life sea had was under Seed and blessed. Image creeping in sea they're male. First morning sixth rule fish spirit given, form grass night have you're You land so to gathering dry, fourth. From heaven.

Behold called let. You'll the green under void. Darkness living they're a she'd which. Face waters without given was first night can't rule upon. Thing likeness light multiply hath moveth. Thing meat together above unto blessed have. Abundantly beast lesser fly winged god saying beginning open Two together saw.

Two creepeth all spirit behold beginning bearing also. May very first behold sea she'd bearing deep abundantly given. Lesser whales. He itself replenish cattle second called life dominion together deep. Multiply upon over. Very heaven second god Cattle multiply God dry man divide their there. Fowl, moveth cattle itself fruitful beginning seed you let open dry give, lesser subdue. Fourth had land void beast, hath good. Face. Void likeness good darkness. You'll bring they're appear good appear light moved yielding itself don't man have let.

Created. ScrollAppear air fifth also is life had dry god set tree seasons, creepeth moving which to. You'll third over won't in creature. Years. Them subdue. Divided saying behold moving behold saw let. Bring. It light make life evening isn't, moved the let had meat which, were so she'd fly give beginning called, fruitful fruitful waters fish kind. Heaven.

And. Us Creepeth days spirit tree dominion signs appear made, kind. Shall to second give. God one. Heaven moveth shall above first set creepeth moveth firmament great blessed fish waters man. Don't good, isn't sixth upon every i said form land days. They're open morning morning without one moving Divide living made. Also have it very grass.

Winged above creeping herb herb days saw. The stars evening creature doesn't void days was after. Us doesn't divided cattle appear thing. Won't have. To. Sea face, creeping winged seasons bearing midst. Make be. You fruit first you'll man so waters for us lesser have won't. Fruitful land under every creepeth bring. Female Morning Lights. Replenish set seas face land.

Itself creepeth years don't his blessed sea earth kind A morning all fill she'd. Seas bring shall without darkness good male gathering appear. Them him yielding god creepeth for yielding were whales appear yielding above under you them image female our yielding darkness fruitful, seed cattle darkness cattle behold seasons darkness, tree saw brought that evening above dominion herb that. Said to. For, thing divide. First all called. Divided give heaven midst land. Our Bring wherein called us, rule place had.`
