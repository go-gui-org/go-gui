package gui

import "time"

// SidebarRuntimeState tracks animation state for a sidebar.
type SidebarRuntimeState struct {
	AnimFrac    float32
	PrevOpen    bool
	Initialized bool
}

// SidebarCfg configures a sidebar view.
type SidebarCfg struct {
	Shadow      *BoxShadow
	TweenEasing EasingFn
	ID          string

	// Accessibility
	A11YLabel       string
	A11YDescription string
	Content         []View
	TweenDuration   time.Duration
	Padding         Opt[Padding]
	// TweenDuration > 0 uses tween; 0 uses spring.
	Spring    SpringCfg
	Width     float32
	Radius    float32
	Color     Color
	Sizing    Sizing
	Open      bool
	Clip      bool
	Disabled  bool
	Invisible bool
}

// Sidebar creates an animated panel that slides in/out.
func (w *Window) Sidebar(cfg SidebarCfg) View {
	if cfg.Width == 0 {
		cfg.Width = 250
	}
	if cfg.Sizing == (Sizing{}) {
		cfg.Sizing = FixedFill
	}
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.ColorPanel
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(guiTheme.ContainerStyle.Padding)
	}
	if cfg.Spring == (SpringCfg{}) {
		cfg.Spring = SpringStiff
	}
	if cfg.TweenDuration == 0 && cfg.TweenEasing == nil {
		cfg.TweenDuration = 300 * time.Millisecond
		cfg.TweenEasing = EaseInOutCubic
	}

	if cfg.Invisible {
		return invisibleContainerView()
	}

	animW := sidebarAnimatedWidth(w, cfg)
	p := cfg.Padding.Get(Padding{})
	padW := p.Left + p.Right
	pad := cfg.Padding
	if animW <= padW {
		pad = Some(Padding{})
	}

	return Column(ContainerCfg{
		ID:              cfg.ID,
		Sizing:          cfg.Sizing,
		Width:           animW,
		Padding:         pad,
		Color:           cfg.Color,
		Shadow:          cfg.Shadow,
		Radius:          Some(cfg.Radius),
		Clip:            cfg.Clip,
		Disabled:        cfg.Disabled,
		A11YRole:        AccessRoleGroup,
		A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
		A11YDescription: cfg.A11YDescription,
		Content:         cfg.Content,
	})
}

func sidebarAnimatedWidth(w *Window, cfg SidebarCfg) float32 {
	sm := StateMap[string, SidebarRuntimeState](
		w, nsSidebar, capFew)

	rt, ok := sm.Get(cfg.ID)
	if !ok {
		rt = SidebarRuntimeState{}
	}

	target := float32(0)
	if cfg.Open {
		target = 1
	}

	if !rt.Initialized {
		rt.AnimFrac = target
		rt.PrevOpen = cfg.Open
		rt.Initialized = true
		sm.Set(cfg.ID, rt)
		return cfg.Width * target
	}

	if cfg.Open != rt.PrevOpen {
		rt.PrevOpen = cfg.Open
		sm.Set(cfg.ID, rt)
		sidebarStartAnimation(cfg.ID, rt.AnimFrac, target,
			cfg.Spring, cfg.TweenDuration, cfg.TweenEasing, w)
	}

	return cfg.Width * f32Max(0, rt.AnimFrac)
}

func sidebarStartAnimation(
	sidebarID string, from, to float32,
	springCfg SpringCfg,
	tweenDur time.Duration, tweenEasing EasingFn,
	w *Window,
) {
	animID := "sidebar:" + sidebarID
	onValue := func(v float32, w *Window) {
		sm := StateMap[string, SidebarRuntimeState](
			w, nsSidebar, capFew)
		rt, _ := sm.Get(sidebarID)
		rt.AnimFrac = v
		sm.Set(sidebarID, rt)
	}
	if tweenDur > 0 {
		w.animationAdd(&TweenAnimation{
			AnimID:   animID,
			From:     from,
			To:       to,
			Duration: tweenDur,
			Easing:   tweenEasing,
			OnValue:  onValue,
		})
	} else {
		sp := &SpringAnimation{
			AnimID:  animID,
			Config:  springCfg,
			OnValue: onValue,
		}
		sp.SpringTo(from, to)
		w.animationAdd(sp)
	}
}
