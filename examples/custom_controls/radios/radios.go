// Package radios demonstrates custom radio button looks built from hover and
// press state read while the view is built (advanced: build-time interaction
// state).
//
// It ports go-shirei's custom-radios demo. Two looks:
//
//   - Material. An outlined circle; the accent ring and a solid center dot
//     when selected.
//   - Windows XP (Luna). A sunken round well: blue ring, gray-to-white face, a
//     gold rim on hover and a small green dot when selected.
//
// Each option is a gui.Interactive around a row that holds the circle and its
// label. The app owns the selected value. A group adds what a single option
// cannot do: the arrow keys move the selection, and only the selected option
// is in the Tab order, so Tab moves between groups, not between options.
//
// A key goes only to the focused shape; it does not travel on to the group.
// So every option carries the group's arrow key handler.
package radios

import (
	"github.com/go-gui-org/go-gui/examples/custom_controls/internal/look"
	"github.com/go-gui-org/go-gui/gui"
)

// option is one choice in a group. id is its leaf ID inside the group.
type option struct {
	id, label, value string
	disabled         bool
}

// group describes one radio group: its leaf ID, its options, where its value
// lives, and, for the groups a switch can lock, where that switch lives.
type group struct {
	id      string
	options []option
	value   func(*App) *string
	locked  func(*App) bool
}

// The groups are package values so the app can build every handler once.
var (
	materialSize = group{id: "size", options: []option{
		{id: "s", label: "Small", value: "s"},
		{id: "m", label: "Medium", value: "m"},
		{id: "l", label: "Large", value: "l"},
		{id: "disabled", label: "Disabled sample", value: "x", disabled: true},
	}, value: func(a *App) *string { return &a.size }}

	materialTheme = group{id: "theme", options: []option{
		{id: "light", label: "Light", value: "light"},
		{id: "dark", label: "Dark", value: "dark"},
	}, value: func(a *App) *string { return &a.theme },
		locked: func(a *App) bool { return a.materialLocked }}

	xpDrive = group{id: "drive", options: []option{
		{id: "c", label: "Local Disk (C:)", value: "c"},
		{id: "d", label: "DVD Drive (D:)", value: "d"},
		{id: "z", label: "Network Drive (Z:)", value: "z"},
		{id: "disabled", label: "Disabled sample", value: "x", disabled: true},
	}, value: func(a *App) *string { return &a.drive }}

	// Shares its value with xpDrive, as in go-shirei: picking Floppy clears
	// the selection above.
	xpDriveLocked = group{id: "drive2", options: []option{
		{id: "a", label: "Floppy (A:)", value: "a"},
		{id: "e", label: "CD-ROM (E:)", value: "e"},
	}, value: func(a *App) *string { return &a.drive },
		locked: func(a *App) bool { return a.xpLocked }}

	allGroups = []group{materialSize, materialTheme, xpDrive, xpDriveLocked}
)

// App holds each group's value, the two lock switches and the last action.
type App struct {
	mood, size, theme, drive string
	materialLocked, xpLocked bool
	log                      string

	// Handlers are built once, so a frame does not allocate a new handler
	// per option. pick is keyed by group ID and option ID. The look
	// builder passed to gui.Interactive is still a new closure each frame.
	pick map[string]func(gui.EventCtx)
	keys map[string]func(gui.EventCtx)
	lock map[string]func(gui.EventCtx)
	// onMood is the default group's select handler.
	onMood func(string, gui.EventCtx)
}

// New returns the page state with its starting values.
func New() *App {
	app := &App{
		mood: "ok", size: "m", theme: "light", drive: "c",
		log:  "Pick an option.",
		pick: map[string]func(gui.EventCtx){},
		keys: map[string]func(gui.EventCtx){},
	}
	for _, g := range allGroups {
		for _, opt := range g.options {
			app.pick[gui.ScopeID(g.id, opt.id)] = func(ctx gui.EventCtx) {
				app.selectOption(g, opt)
				ctx.Consume()
			}
		}
		app.keys[g.id] = app.arrowKeys(g)
	}
	app.lock = map[string]func(gui.EventCtx){
		"material": func(ctx gui.EventCtx) { app.materialLocked = !app.materialLocked; ctx.Consume() },
		"xp":       func(ctx gui.EventCtx) { app.xpLocked = !app.xpLocked; ctx.Consume() },
	}
	app.onMood = func(v string, ctx gui.EventCtx) {
		app.mood = v
		app.log = "Default → " + v
		ctx.Consume()
	}
	return app
}

func (app *App) selectOption(g group, opt option) {
	*g.value(app) = opt.value
	app.log = opt.label + " selected"
}

// optionDisabled reports whether an option takes no input: disabled itself,
// or in a group its switch locked.
func (app *App) optionDisabled(g group, opt option) bool {
	return opt.disabled || (g.locked != nil && g.locked(app))
}

// arrowKeys moves the selection to the next or previous enabled option and
// moves the focus with it. Down and Right go forward, Up and Left back; both
// wrap. A key handler runs with no frame lock held, so SetFocus is allowed
// here.
func (app *App) arrowKeys(g group) func(gui.EventCtx) {
	return func(ctx gui.EventCtx) {
		step := 0
		switch ctx.Event.KeyCode {
		case gui.KeyDown, gui.KeyRight:
			step = 1
		case gui.KeyUp, gui.KeyLeft:
			step = -1
		default:
			return
		}
		if ctx.Event.Modifiers != gui.ModNone {
			return
		}
		n := len(g.options)
		// Step from the selected option. A group can have none: the second
		// XP group shares its value with the first. Then step from the
		// focused option, which is the shape that got the key, so Down does
		// not start from index 0 and land back on the option that has focus.
		cur, focused := -1, 0
		groupEff := ctx.EffID(g.id)
		for i, opt := range g.options {
			if opt.value == *g.value(app) {
				cur = i
			}
			if ctx.Window.IsFocus(gui.ScopeID(groupEff, opt.id)) {
				focused = i
			}
		}
		if cur < 0 {
			cur = focused
		}
		for i := 1; i < n; i++ {
			next := g.options[((cur+step*i)%n+n)%n]
			if app.optionDisabled(g, next) {
				continue
			}
			app.selectOption(g, next)
			// ctx.Layout is the focused option. EffID walks up from it to
			// the group's shape and returns the group's effective ID; the
			// option's ID is scoped under it.
			ctx.Window.SetFocus(gui.ScopeID(ctx.EffID(g.id), next.id))
			break
		}
		ctx.Consume()
	}
}

// View builds the page. The caller owns app, so the page can sit in a
// window whose state is a different type.
func View(app *App) gui.View {
	return gui.Column(gui.ContainerCfg{
		ID:         "page",
		Scrollable: true,
		Sizing:     gui.FillFill,
		Color:      pageBG,
		Padding:    look.PagePadding,
		Spacing:    look.PageSpacing,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Custom radios with gui.Interactive", TextStyle: look.Light.Title}),

			sectionTitle("Default gui.RadioButtonGroupColumn, for comparison"),
			gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
				ID:         "mood",
				Value:      app.mood,
				SizeBorder: gui.NoBorder,
				Padding:    gui.PaddingNone,
				Options: []gui.RadioOption{
					gui.NewRadioOption("Great", "great"),
					gui.NewRadioOption("Okay", "ok"),
					gui.NewRadioOption("Rough", "rough"),
				},
				OnSelect: app.onMood,
			}),

			sectionTitle("Material Design inspired"),
			radioGroup(app, materialSize, gui.ColorTransparent, 10, materialLook(materialPrimary)),
			lockRow("material-lock", "Disable the next Material radios", app.materialLocked,
				app.lock["material"],
				radioGroup(app, materialTheme, gui.ColorTransparent, 8, materialLook(materialTeal))),

			sectionTitle("Windows XP (Luna / XP.css inspired)"),
			radioGroup(app, xpDrive, xpSurface, 8, xpLook),
			lockRow("xp-lock", "Disable the next XP radios", app.xpLocked, app.lock["xp"],
				radioGroup(app, xpDriveLocked, gui.ColorTransparent, 6, xpLook)),

			gui.Text(gui.TextCfg{ID: "log", Text: app.log, TextStyle: look.Light.Body}),
		},
	})
}

func sectionTitle(s string) gui.View {
	return gui.Text(gui.TextCfg{Text: s, TextStyle: look.Light.Label})
}

// optionState is what the group knows about one option when it builds it.
type optionState struct {
	option
	selected, disabled bool
	// tabStop is true for the one option of the group in the Tab order.
	tabStop            bool
	onClick, onKeyDown func(gui.EventCtx)
}

// lookFunc builds one option's view from its state and its interaction state.
type lookFunc func(o optionState, s gui.InteractionState) gui.View

// radioGroup is one ID-bearing column of options. The ID scopes the options,
// so "disabled" in the size group is "page:size:disabled" and in the drive
// group "page:drive:disabled".
func radioGroup(app *App, g group, bg gui.Color, spacing float32, lk lookFunc) gui.View {
	value := *g.value(app)
	// The Tab stop is the selected option. A group with no selection (the
	// second XP group shares its value with the first) uses its first
	// enabled option, so Tab can still reach it.
	tabStop := ""
	for _, opt := range g.options {
		if opt.value == value {
			tabStop = opt.id
		}
	}
	if tabStop == "" {
		for _, opt := range g.options {
			if !app.optionDisabled(g, opt) {
				tabStop = opt.id
				break
			}
		}
	}
	content := make([]gui.View, 0, len(g.options))
	for i, opt := range g.options {
		// The builder captures a few words (app, the group, the index, the
		// look and the Tab stop) and makes the option's state when it runs,
		// so the closure does not hold a copy of the whole optionState.
		content = append(content, gui.Interactive(opt.id, func(s gui.InteractionState) gui.View {
			return lk(app.optionState(g, i, tabStop), s)
		}))
	}
	pad, border := gui.PaddingNone, gui.NoBorder
	if bg != gui.ColorTransparent {
		// The XP group is a dialog surface with a thin frame.
		pad, border = gui.PadAll(14), gui.SomeF(1)
	}
	return gui.Column(gui.ContainerCfg{
		ID:          g.id,
		A11YRole:    gui.AccessRoleRadioGroup,
		Color:       bg,
		ColorBorder: xpSurfaceBorder,
		SizeBorder:  border,
		Radius:      gui.SomeF(0),
		Padding:     pad,
		Spacing:     gui.SomeF(spacing),
		Content:     content,
	})
}

// optionState makes the state of option i of g. tabStop is the ID of the
// group's one option in the Tab order.
func (app *App) optionState(g group, i int, tabStop string) optionState {
	opt := g.options[i]
	return optionState{
		option:    opt,
		tabStop:   opt.id == tabStop,
		selected:  opt.value == *g.value(app),
		disabled:  app.optionDisabled(g, opt),
		onClick:   app.pick[gui.ScopeID(g.id, opt.id)],
		onKeyDown: app.keys[g.id],
	}
}

// lockRow pairs a default gui.Toggle that locks a group with that group.
func lockRow(id, label string, locked bool, onClick func(gui.EventCtx), target gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		ID:         id,
		Padding:    gui.PaddingNone,
		Spacing:    gui.SomeF(16),
		SizeBorder: gui.NoBorder,
		VAlign:     gui.VAlignMiddle,
		Content: []gui.View{
			gui.Toggle(gui.ToggleCfg{ID: "lock", Label: label, Selected: locked, OnClick: onClick}),
			target,
		},
	})
}

// radioShell is the part every custom radio shares: the ID that makes it a
// hover and press target, click and keyboard activation, and the radio role
// with its selected state. Only the group's Tab stop is in the Tab order; the
// arrow keys reach the others. The look goes in cfg.
func radioShell(cfg gui.ContainerCfg, o optionState) gui.ContainerCfg {
	cfg.ID = o.id
	cfg.OnClick = o.onClick
	cfg.OnKeyDown = o.onKeyDown
	cfg.ClickOnSpace = true
	cfg.ClickOnEnter = true
	cfg.Focusable = true
	cfg.FocusSkip = !o.tabStop
	cfg.Disabled = o.disabled
	cfg.A11YRole = gui.AccessRoleRadioButton
	if o.selected {
		cfg.A11YState = gui.AccessStateSelected
	}
	cfg.A11YCfg = gui.A11YCfg{A11YLabel: o.label}
	cfg.Padding = gui.PaddingNone
	cfg.SizeBorder = gui.NoBorder
	cfg.VAlign = gui.VAlignMiddle
	return cfg
}

var (
	pageBG    = gui.Hex(0xf4f4f6)
	white     = gui.Hex(0xffffff)
	textDark  = gui.Hex(0x262626)
	textMuted = gui.Hex(0x8c8c8c)
)

// --- Material --------------------------------------------------------------

var (
	materialPrimary = gui.Hex(0x2277d3)
	materialTeal    = gui.Hex(0x29a397)
	materialBorder  = gui.Hex(0x8c8c8c)
	materialMuted   = gui.Hex(0xbfbfbf)
	materialOffFill = gui.Hex(0xf5f5f5)
)

// materialLook is a Material radio in one accent color.
func materialLook(accent gui.Color) lookFunc {
	return func(o optionState, s gui.InteractionState) gui.View {
		const size, ring = 20, 2
		const dot = 9
		border, bg, dotColor, text := materialBorder, white, accent, textDark
		switch {
		case o.disabled:
			border, bg, dotColor, text = materialMuted, materialOffFill, materialMuted, textMuted
		case o.selected:
			border = accent
			switch {
			case s.Armed:
				border = look.Darken(accent, 0.88)
			case s.Hovered:
				border = look.Mix(accent, white, 0.12)
			}
		case s.Armed:
			border, bg = accent, look.Mix(white, accent, 0.16)
		case s.Hovered:
			border, bg = accent, look.Mix(white, accent, 0.06)
		}
		var center []gui.View
		if o.selected {
			center = []gui.View{circle(dot, dotColor, nil)}
		}
		return gui.Row(radioShell(gui.ContainerCfg{
			Spacing: gui.SomeF(10),
			Content: []gui.View{
				gui.Row(gui.ContainerCfg{
					Width:       size,
					Height:      size,
					Sizing:      gui.FixedFixed,
					Radius:      gui.SomeF(size / 2),
					Color:       bg,
					ColorBorder: border,
					SizeBorder:  gui.SomeF(ring),
					Padding:     gui.PaddingNone,
					HAlign:      gui.HAlignCenter,
					VAlign:      gui.VAlignMiddle,
					Content:     center,
				}),
				gui.Text(gui.TextCfg{Text: o.label, TextStyle: look.Light.Text(text, 14)}),
			},
		}, o))
	}
}

// circle is a borderless round container with optional centered content.
func circle(size float32, c gui.Color, content []gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Width:      size,
		Height:     size,
		Sizing:     gui.FixedFixed,
		Radius:     gui.SomeF(size / 2),
		Color:      c,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PaddingNone,
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Content:    content,
	})
}

// --- Windows XP ------------------------------------------------------------

// Luna colors, from XP.css (https://botoxparty.github.io/XP.css/).
var (
	xpSurface       = gui.Hex(0xece9d8)
	xpSurfaceBorder = gui.Hex(0xbab8ab)
	xpBorder        = gui.Hex(0x1d5281)
	xpBorderMuted   = gui.Hex(0xcac8bb)
	xpGoldLo        = gui.Hex(0xf8b636)
	xpGoldHi        = gui.Hex(0xfedf9c)
	xpDotGreen      = gui.Hex(0x22a122)
	xpText          = gui.Hex(0x222222)

	// Built once: a GradientDef is a pointer, so sharing it costs nothing
	// per frame.
	xpFaceIdle = &gui.GradientDef{Direction: gui.GradientToBottomRight, Stops: []gui.GradientStop{
		{Color: gui.Hex(0xdcdcd7), Pos: 0},
		{Color: gui.Hex(0xffffff), Pos: 1},
	}}
	xpFacePressed = &gui.GradientDef{Direction: gui.GradientToBottomRight, Stops: []gui.GradientStop{
		{Color: gui.Hex(0xb0b0a7), Pos: 0},
		{Color: gui.Hex(0xe3e1d2), Pos: 1},
	}}
)

// xpLook is a Luna radio: a round well that stays sunken in every state. The
// well is four nested circles — blue ring, gold rim (bottom-right), gold rim
// (top-left), face — so hover changes colors only, never the size.
func xpLook(o optionState, s gui.InteractionState) gui.View {
	const face = 13
	border, text, dotColor := xpBorder, xpText, xpDotGreen
	grad, faceColor := xpFaceIdle, gui.Color{}
	switch {
	case o.disabled:
		border, text, dotColor = xpBorderMuted, textMuted, xpBorderMuted
		grad, faceColor = nil, white
	case s.Armed:
		grad = xpFacePressed
	}
	rimLo, rimHi := gui.ColorTransparent, gui.ColorTransparent
	if !o.disabled && s.Hovered && !s.Armed {
		rimLo, rimHi = xpGoldLo, xpGoldHi
	}
	var center []gui.View
	if o.selected {
		center = []gui.View{circle(5, dotColor, nil)}
	}
	const inner = face - 2
	well := gui.Row(gui.ContainerCfg{
		Color:      border,
		Radius:     gui.SomeF(face/2 + 1),
		Padding:    gui.PadAll(1),
		SizeBorder: gui.NoBorder,
		Content: []gui.View{gui.Row(gui.ContainerCfg{
			// The face gradient sits under the rims, so a see-through rim
			// shows the face.
			Color:      faceColor,
			Gradient:   grad,
			Radius:     gui.SomeF(face / 2),
			Padding:    gui.PaddingNone,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				ring(rimLo, gui.NewPadding(0, 1, 1, 0), face/2,
					ring(rimHi, gui.NewPadding(1, 0, 0, 1), face/2-1,
						gui.Row(gui.ContainerCfg{
							Width:      inner,
							Height:     inner,
							Sizing:     gui.FixedFixed,
							Radius:     gui.SomeF(inner / 2),
							Padding:    gui.PaddingNone,
							SizeBorder: gui.NoBorder,
							HAlign:     gui.HAlignCenter,
							VAlign:     gui.VAlignMiddle,
							Content:    center,
						}))),
			},
		})},
	})
	return gui.Row(radioShell(gui.ContainerCfg{
		Spacing: gui.SomeF(6),
		Content: []gui.View{
			well,
			gui.Text(gui.TextCfg{Text: o.label, TextStyle: look.Light.Text(text, 12)}),
		},
	}, o))
}

// ring pads content with one color on the sides pad names, rounded.
func ring(c gui.Color, pad gui.Padding, radius float32, content gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Color:      c,
		Radius:     gui.SomeF(radius),
		Padding:    pad,
		SizeBorder: gui.NoBorder,
		Content:    []gui.View{content},
	})
}
