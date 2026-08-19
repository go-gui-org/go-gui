package gui

import "time"

// SidebarRuntimeState tracks animation state for a sidebar.
type sidebarRuntimeState struct {
	animFrac    float32
	prevOpen    bool
	Initialized bool
}

// SidebarCfg configures a sidebar view.
type SidebarCfg struct {
	Shadow *BoxShadow
	// TweenEasing is the easing for tween animation. Ignored when
	// TweenDuration is zero (spring mode).
	// exportaudit:keep — caller-facing config (issue #372)
	TweenEasing EasingFn
	ID          string

	// Accessibility
	A11YCfg
	Content []View
	// TweenDuration > 0 uses tween animation; 0 uses spring.
	// exportaudit:keep — caller-facing config (issue #372)
	TweenDuration time.Duration
	Padding       Padding
	// Spring tunes the spring animation (used when TweenDuration
	// is zero). Zero takes the stiff default.
	// exportaudit:keep — caller-facing config (issue #372)
	Spring    SpringCfg
	Width     float32
	Radius    float32 // ergonomics-audit:opt-plain — sidebar radius 0 (sharp) is a real choice; no theme default distinct from it
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
	cfg.Sizing = cfg.Sizing.Or(FixedFill)
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.ColorPanel
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = guiTheme.ContainerStyle.Padding
	}
	if cfg.Spring == (SpringCfg{}) {
		cfg.Spring = SpringStiff
	}
	if cfg.TweenDuration == 0 && cfg.TweenEasing == nil {
		cfg.TweenDuration = 300 * time.Millisecond
		cfg.TweenEasing = easeInOutCubic
	}

	if cfg.Invisible {
		return invisibleContainerView()
	}

	// One resolved identity for every key below; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)

	animW := sidebarAnimatedWidth(w, cfg)
	p := cfg.Padding.Or(PaddingNone)
	padW := p.Left + p.Right
	pad := cfg.Padding
	if animW <= padW {
		pad = PaddingNone
	}

	// A Fixed-width box with Width 0 degrades to content sizing in
	// layoutWidths (issue #94), so a fully closed sidebar that kept
	// its children would snap back to content width the moment the
	// close animation lands on exactly 0. Drop the content instead:
	// childless Fixed-0 boxes still resolve to width 0.
	content := cfg.Content
	if animW <= 0 {
		content = nil
	}

	return Column(ContainerCfg{
		ID:       cfg.ID,
		Sizing:   cfg.Sizing,
		Width:    animW,
		Padding:  pad,
		Color:    cfg.Color,
		Shadow:   cfg.Shadow,
		Radius:   Some(cfg.Radius),
		Clip:     cfg.Clip,
		Disabled: cfg.Disabled,
		A11YRole: AccessRoleGroup,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
			A11YDescription: cfg.A11YDescription,
		},
		Content: content,
	})
}

func sidebarAnimatedWidth(w *Window, cfg SidebarCfg) float32 {
	sm := StateMap[string, sidebarRuntimeState](
		w, nsSidebar, capFew)

	rt, ok := sm.Get(cfg.ID)
	if !ok {
		rt = sidebarRuntimeState{}
	}

	target := float32(0)
	if cfg.Open {
		target = 1
	}

	if !rt.Initialized {
		rt.animFrac = target
		rt.prevOpen = cfg.Open
		rt.Initialized = true
		sm.Set(cfg.ID, rt)
		return cfg.Width * target
	}

	if cfg.Open != rt.prevOpen {
		rt.prevOpen = cfg.Open
		sm.Set(cfg.ID, rt)
		sidebarStartAnimation(cfg.ID, rt.animFrac, target,
			cfg.Spring, cfg.TweenDuration, cfg.TweenEasing, w)
	}

	return cfg.Width * f32Max(0, rt.animFrac)
}

func sidebarStartAnimation(
	sidebarID string, from, to float32,
	spring SpringCfg,
	tweenDur time.Duration, tweenEasing EasingFn,
	w *Window,
) {
	animID := ScopeID("sidebar", sidebarID)
	onValue := func(v float32, w *Window) {
		sm := StateMap[string, sidebarRuntimeState](
			w, nsSidebar, capFew)
		// Default zero: valid initial state for a new sidebar.
		rt := sm.GetOr(sidebarID, sidebarRuntimeState{})
		rt.animFrac = v
		sm.Set(sidebarID, rt)
	}
	if tweenDur > 0 {
		w.AnimationAdd(&TweenAnimation{
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
			Config:  spring,
			OnValue: onValue,
		}
		sp.SpringTo(from, to)
		w.AnimationAdd(sp)
	}
}
