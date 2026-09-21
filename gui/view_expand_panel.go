package gui

// ExpandPanelCfg configures an expand panel. It consists of a
// header (always visible) and content (visible when expanded).
type ExpandPanelCfg struct {
	Head     View
	Content  View
	OnToggle func(EventCtx)
	ID       string

	// Accessibility
	A11YCfg
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
	MinWidth   float32
	MaxWidth   float32
	MinHeight  float32
	MaxHeight  float32

	Color Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it.
	Colors ColorSet
	Sizing Sizing
	Open   bool
	// FocusDisabled opts the header row out of the tab order and
	// its focus ring. Use it for decorative or demo panels where
	// keyboard toggling is not wanted.
	FocusDisabled bool

	// Sound overrides the theme's toggle cue for this instance.
	// SoundNone (the zero value) takes the theme's cue for that role,
	// which is itself silent unless the app opted in (issue #446).
	// exportaudit:keep — caller-facing config (issue #467)
	Sound SoundCue

	// SoundDisabled suppresses the header's sound regardless of the theme
	// and of Sound above.
	// exportaudit:keep — caller-facing config (issue #467)
	SoundDisabled bool
}

// ExpandPanel creates an expandable panel view.
func ExpandPanel(cfg ExpandPanelCfg) View {
	applyExpandPanelDefaults(&cfg)
	sizeBorder := cfg.SizeBorder.Get(guiTheme.expandPanelStyle.SizeBorder)
	radius := cfg.Radius.Get(guiTheme.expandPanelStyle.Radius)

	// A header that cannot take focus never draws a focus ring, so
	// skip the amend closure rather than install a dead one.
	headFocusable := !cfg.FocusDisabled
	colors := cfg.Colors
	// The header is a transparent row over the panel body: at rest
	// the panel shows through, and hover/press tint only the header.
	// Its set leaves Base and Border unset rather than taking the
	// panel's, which is exactly what the header row carries out of
	// generation, so pick's resting answer paints nothing. Used
	// directly — no resolve(), which would back Hover and Click from
	// Base and wipe them.
	headerColors := ColorSet{
		Hover:       colors.Hover,
		Click:       colors.Click,
		BorderFocus: colors.BorderFocus,
	}
	// Captured at generation; see focusRingAmend.
	ring := guiTheme.focusRing
	var headAmend func(EventCtx)
	if headFocusable {
		headAmend = func(ctx EventCtx) {
			// Same guards focusRingAmend carries: a hand-built
			// Layout in a test reaches the amend pass with no
			// Window and no Shape.
			if ctx.Layout == nil || ctx.Window == nil {
				return
			}
			shape := ctx.Layout.Shape
			if shape == nil {
				return
			}
			// Amend sees focus and a held Space but no pointer, so
			// hovered stays false. Space toggles the panel
			// (ClickOnSpace), so a held Space is the press this pass
			// can see, the way it is for Button; the hover pass
			// re-picks with the pointer state.
			// ANDed with !Disabled, like Input's amend: a disabled
			// header that still holds focus must not draw the ring.
			// focusRingAmend, which this replaced, returned early on
			// a disabled shape for the same reason.
			key := shape.idKey()
			focused := !shape.Disabled && ctx.Window.IsFocus(key)
			shape.Color, shape.ColorBorder = headerColors.pick(stateFlags{
				disabled: shape.Disabled,
				pressed:  ctx.Window.isKeyPressed(key),
				focused:  focused,
			})
			if focused {
				applyFocusRingShadow(shape, ctx.Window, ring)
			}
		}
	}

	onToggle := cfg.OnToggle

	a11yState := AccessState(0)
	if cfg.Open {
		a11yState = AccessStateExpanded
	}

	// The cue names what the click will do: an open panel is about to
	// close (issue #467).
	themeCue := guiTheme.Sounds.ToggleOn
	if cfg.Open {
		themeCue = guiTheme.Sounds.ToggleOff
	}
	soundCue := resolveSoundCue(themeCue, cfg.Sound, cfg.SoundDisabled)

	// The header row joins the tab order: Space/Enter toggle the
	// panel (issue #345). The body's own focusables sit after it in
	// tab order because the header row precedes them in the Column.
	headID := ScopeID(cfg.ID, "head")

	return Column(ContainerCfg{
		ID:          cfg.ID,
		A11YRole:    AccessRoleDisclosure,
		A11YState:   a11yState,
		A11YCfg:     cfg.A11YCfg,
		Color:       colors.Base,
		ColorBorder: colors.Border,
		SizeBorder:  Some(sizeBorder),
		Padding:     cfg.Padding,
		Radius:      Some(radius),
		Sizing:      cfg.Sizing,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		MinHeight:   cfg.MinHeight,
		MaxHeight:   cfg.MaxHeight,
		Spacing:     SomeF(0),
		Content: []View{
			Row(ContainerCfg{
				ID:           headID,
				Padding:      NoPadding,
				Sizing:       FillFit,
				VAlign:       VAlignMiddle,
				Focusable:    headFocusable,
				ClickOnSpace: true,
				ClickOnEnter: true,
				Sound:        soundCue,
				AmendLayout:  headAmend,
				Content: []View{
					cfg.Head,
					// Flexible gap: it absorbs the surplus width, so
					// the disclosure arrow sits at the header's
					// trailing edge instead of hugging the head view.
					Row(ContainerCfg{
						Sizing:     FillFit,
						Padding:    NoPadding,
						SizeBorder: NoBorder, // structural wrapper
					}),
					Row(ContainerCfg{
						Padding: NewPadding(0, PadMedium, 0, 0),
						Content: []View{
							disclosureArrow(cfg.Open, guiTheme.TextStyleBody),
						},
					}),
				},
				OnClick: func(ctx EventCtx) {
					if onToggle != nil {
						onToggle(ctx)
						ctx.Consume()
					}
				},
				OnHover: func(ctx EventCtx) {
					ctx.Window.SetMouseCursorPointingHand()
					// This pass sees everything the amend pass saw plus
					// the pointer, so it re-picks both channels outright.
					// Either press counts: a held mouse button here, a
					// held Space from the amend pass's half of the state.
					ctx.Layout.Shape.Color, ctx.Layout.Shape.ColorBorder =
						headerColors.pick(stateFlags{
							disabled: ctx.Layout.Shape.Disabled,
							pressed: ctx.Event.MouseButton == MouseLeft ||
								ctx.Window.isKeyPressed(
									ctx.Layout.Shape.idKey()),
							focused: ctx.Window.IsFocus(
								ctx.Layout.Shape.idKey()),
							hovered: true,
						})
					ctx.Consume()
				},
			}),
			Column(ContainerCfg{
				Invisible: !cfg.Open,
				Padding:   NoPadding,
				Sizing:    FillFit,
				Spacing:   SomeF(0),
				Content: []View{
					cfg.Content,
				},
			}),
		},
	})
}

// applyExpandPanelDefaults fills zero-value color fields from the theme.
func applyExpandPanelDefaults(cfg *ExpandPanelCfg) {
	d := &defaultExpandPanelStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, d.Colors)
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
	// A disclosure panel is a full-width block whose height follows
	// its body. Left at the zero Sizing (FitFit) it widens to its
	// longest unwrapped line — and since a container does not clip,
	// it paints through its own border and off the window.
	cfg.Sizing = cfg.Sizing.Or(FillFit)
}
