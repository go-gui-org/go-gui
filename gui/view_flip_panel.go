package gui

import "time"

// view_flip_panel.go — FlipPanel, a panel that turns over to show its
// back face (#1020).
//
// The two faces sit on top of each other. The front turns to the
// angle, the back to the angle plus 180°, and both hide their back
// faces, so only the face toward the viewer draws. A spring moves the
// angle in render-only frames: the faces read it through TransformFn,
// so a turn needs no rebuild. A toggle during a turn replaces the
// spring, and AnimationAdd carries the old spring's position and
// velocity into the new one, so the panel turns back smoothly.
//
// The back face leaves the layout flow (Shape.outOfFlow) and fills the
// panel, so it lands on the front face; the panel takes the size of
// the front face, or the size its own Sizing gives it. The face that
// is turned away is inert: it takes no pointer, has no tab stops and is
// not in the accessibility tree.

// FlipAxis is the axis a FlipPanel turns about.
type FlipAxis uint8

const (
	// FlipAxisY turns the panel about its vertical axis: it turns over
	// from side to side. This is the default.
	FlipAxisY FlipAxis = iota
	// FlipAxisX turns the panel about its horizontal axis: it turns
	// over from top to bottom.
	FlipAxisX
)

// FlipPanelCfg configures a FlipPanel.
type FlipPanelCfg struct {
	// Front is the face that shows when Flipped is false.
	Front View
	// Back is the face that shows when Flipped is true.
	Back View
	// OnFlip is called when the user clicks the panel, or presses
	// Enter or Space while it has focus. The app owns Flipped: invert
	// it here. A click that a control on the face consumes does not
	// reach the panel.
	OnFlip func(EventCtx)
	ID     string `gui:"auto"`

	// Accessibility
	A11YCfg

	// Spring sets how the panel turns. The zero value takes a spring
	// that settles in about 600 ms with a light bounce. A Stiffness or
	// Damping that is not above zero takes that spring's value, since
	// without it the panel would never come to rest.
	Spring SpringCfg
	// Sizing, Width and Height size the panel. The zero Sizing (FitFit)
	// takes the size of the front face. The back face always fills the
	// panel.
	Sizing Sizing
	Width  float32
	Height float32
	// Perspective is the distance from the viewer to the panel, in
	// logical pixels. A smaller distance gives a stronger 3D effect.
	// Zero takes 800.
	Perspective float32
	// Axis is the axis the panel turns about. The zero value is
	// FlipAxisY.
	Axis FlipAxis
	// Flipped shows the back face. The panel turns when it changes.
	Flipped bool
	// FocusDisabled takes the panel out of the tab order. It can then
	// only be turned by a click.
	FocusDisabled bool
}

const (
	flipPanelPerspective float32 = 800
	// flipPanelFadeDuration is the cross-fade that replaces the turn
	// when the system asks for reduced motion.
	flipPanelFadeDuration = 200 * time.Millisecond
)

// flipPanelSpring settles in about 600 ms with a light bounce. The
// values come from a 600 ms period and a damping ratio of 0.85 (a
// bounce of 0.15) with mass 1: stiffness = (2π/0.6)² ≈ 110, damping =
// 4π·0.85/0.6 ≈ 17.8. The spring moves an angle in degrees, so the
// threshold is a hundredth of a degree.
var flipPanelSpring = SpringCfg{Stiffness: 110, Damping: 17.8, Mass: 1, Threshold: 0.01}

// flipPanelState is the per-panel motion, kept in StateMap under the
// panel's effective ID. It is a pointer so that the animation callbacks
// and the TransformFn closures share it.
type flipPanelState struct {
	// angle is the turn in degrees: 0 shows the front, 180 the back.
	angle float32
	// fade is the reduced-motion cross-fade: 0 shows the front, 1 the
	// back.
	fade float32
	// flipped is the Flipped value of the last generated frame. A
	// change starts the motion.
	flipped bool
}

// FlipPanel builds a panel that turns over in 3D to show its back face.
// The app owns which face shows: set Flipped, and invert it in OnFlip.
//
//	gui.FlipPanel(gui.FlipPanelCfg{
//		ID:      "card",
//		Front:   front,
//		Back:    back,
//		Flipped: st.flipped,
//		OnFlip: func(ctx gui.EventCtx) {
//			gui.State[App](ctx.Window).flipped = !st.flipped
//		},
//	})
//
// When the system asks for reduced motion, the faces cross-fade and do
// not turn.
func FlipPanel(cfg FlipPanelCfg) View {
	return flipPanelView{cfg: cfg}
}

type flipPanelView struct {
	cfg FlipPanelCfg
}

func (v flipPanelView) GenerateLayout(w *Window) Layout {
	c := v.cfg
	// No ID: take a generated leaf here, at generation time, where the
	// enclosing scope is known (#881).
	if c.ID == "" {
		c.ID = w.autoLeaf("flippanel")
	}
	st := flipPanelMotion(w, &c)
	reduced := w.prefersReducedMotion()

	persp := c.Perspective
	if !(persp > 0) || !f32IsFinite(persp) {
		persp = flipPanelPerspective
	}
	axis := c.Axis
	// One closure per face. Under reduced motion there is no turn, so
	// neither face gets one: the cross-fade below does the work.
	var frontXf, backXf func() Transform
	if !reduced {
		frontXf = func() Transform { return flipPanelTurn(axis, persp, st.angle) }
		backXf = func() Transform { return flipPanelTurn(axis, persp, st.angle+180) }
	}

	onFlip := c.OnFlip
	panel := Column(ContainerCfg{
		ID:           c.ID,
		A11YRole:     AccessRoleGroup,
		A11YCfg:      c.A11YCfg,
		Sizing:       c.Sizing,
		Width:        c.Width,
		Height:       c.Height,
		Padding:      NoPadding,
		Spacing:      NoSpacing,
		SizeBorder:   NoBorder, // structural wrapper
		Focusable:    !c.FocusDisabled,
		ClickOnSpace: true,
		ClickOnEnter: true,
		AmendLayout:  focusRingAmend(Color{}, Color{}),
		OnClick: func(ctx EventCtx) {
			if onFlip != nil {
				onFlip(ctx)
				ctx.Consume()
			}
		},
		Content: []View{
			// The back face comes first. It is taken out of the flow
			// below, so it is placed where the next in-flow child (the
			// front face) starts, and fills the panel.
			flipPanelFace(c.Back, backXf, !c.Flipped),
			flipPanelFace(c.Front, frontXf, c.Flipped),
		},
	})
	layout := generateViewLayout(panel, w)
	if len(layout.Children) != 2 {
		return layout
	}
	back, front := &layout.Children[0], &layout.Children[1]
	back.Shape.outOfFlow = true
	back.Shape.fillOutOfFlow = true
	hidden := back
	if c.Flipped {
		hidden = front
	}
	flipPanelDropHiddenFocus(w, hidden, layout.Shape.idKey(), !c.FocusDisabled)
	if reduced {
		flipPanelFadeLayout(back, st.fade, 0)
		flipPanelFadeLayout(front, 1-st.fade, 0)
	}
	return layout
}

// flipPanelDropHiddenFocus moves focus off the face that is turned
// away. Inert, like the HTML attribute, also takes focus away: a
// control there that kept focus would still take Enter and Space, and
// it is no longer in the accessibility tree, so a screen reader would
// lose the focused node. Focus moves to the panel, or is cleared when
// the panel cannot take it. This runs under the frame lock, where
// SetFocus panics, so the change is queued; the queued command checks
// that focus has not moved in the meantime. It runs every frame, so
// focus set into the hidden face by code moves off it too.
func flipPanelDropHiddenFocus(w *Window, hidden *Layout, panelID string, panelFocusable bool) {
	fid := w.FocusID()
	if fid == "" {
		return
	}
	if _, ok := findLayoutByFocusID(hidden, fid); !ok {
		return
	}
	w.QueueCommand(func(qw *Window) {
		if qw.FocusID() != fid {
			return
		}
		if panelFocusable {
			qw.SetFocus(panelID)
		} else {
			qw.ClearFocus()
		}
	})
}

// flipPanelFace wraps one face in a container that fills the panel and
// carries the face's turn. inert marks the face that is turned away.
// The wrapper always has a transform record (inert or TransformFn sets
// one), so every hit test of the face goes through xfCoordsInverse.
func flipPanelFace(face View, xf func() Transform, inert bool) View {
	var content []View
	if face != nil {
		content = []View{face}
	}
	return Column(ContainerCfg{
		Sizing:      FillFill,
		Padding:     NoPadding,
		Spacing:     NoSpacing,
		SizeBorder:  NoBorder, // structural wrapper
		TransformFn: xf,
		inert:       inert,
		Content:     content,
	})
}

// flipPanelTurn is the transform of a face turned to deg about axis.
func flipPanelTurn(axis FlipAxis, persp, deg float32) Transform {
	t := Perspective(persp)
	if axis == FlipAxisX {
		t = t.RotateX(deg)
	} else {
		t = t.RotateY(deg)
	}
	return t.BackfaceHidden()
}

// flipPanelMotion returns the panel's motion state and starts a turn,
// or a cross-fade under reduced motion, when Flipped has changed since
// the last frame. A panel seen for the first time starts at rest on
// the face that Flipped names, with no motion.
func flipPanelMotion(w *Window, c *FlipPanelCfg) *flipPanelState {
	eid := w.EffID(c.ID)
	var target, fadeTarget float32
	if c.Flipped {
		target, fadeTarget = 180, 1
	}
	states := StateMap[string, *flipPanelState](w, nsFlipPanel, capMany)
	st, ok := states.Get(eid)
	if !ok || st == nil {
		st = &flipPanelState{angle: target, fade: fadeTarget, flipped: c.Flipped}
		states.Set(eid, st)
		return st
	}
	if st.flipped == c.Flipped {
		return st
	}
	st.flipped = c.Flipped
	animID := nsFlipPanel + ":" + eid // ergonomics-audit:not-an-id (animation key)
	if w.prefersReducedMotion() {
		// No turn: the angle rests on the target, so the faces are
		// right if reduced motion is later switched off.
		st.angle = target
		tw := NewTweenAnimation(animID, st.fade, fadeTarget, func(v float32, _ *Window) {
			st.fade = v
		})
		tw.Duration = flipPanelFadeDuration
		tw.Easing = EaseInOutCubic
		// Opacity is set at generation, so the fade needs layout
		// frames (the tween's default refresh).
		w.AnimationAdd(tw)
		return st
	}
	st.fade = fadeTarget
	spring := c.Spring
	if spring == (SpringCfg{}) {
		spring = flipPanelSpring
	}
	// A spring with no stiffness has no force toward the target, and
	// one with no damping never loses energy: neither comes to rest, so
	// the panel would animate forever. Take the panel's own values.
	if !(spring.Stiffness > 0) || !f32IsFinite(spring.Stiffness) {
		spring.Stiffness = flipPanelSpring.Stiffness
	}
	if !(spring.Damping > 0) || !f32IsFinite(spring.Damping) {
		spring.Damping = flipPanelSpring.Damping
	}
	// A new spring object: when it replaces a spring that is still
	// moving, AnimationAdd carries that spring's position and velocity
	// into it, so a second toggle turns back with no jump.
	sp := NewSpringAnimation(animID, func(v float32, _ *Window) {
		st.angle = v
	})
	sp.Config = spring
	sp.Refresh = AnimationRefreshRenderOnly
	sp.SpringTo(st.angle, target)
	w.AnimationAdd(sp)
	return st
}

// flipPanelFadeLayout multiplies the opacity of a face and everything
// in it by alpha. Opacity is per shape, not per subtree, so each shape
// takes its share.
func flipPanelFadeLayout(layout *Layout, alpha float32, depth int) {
	if alpha >= 1 || overMaxDepth(depth) || layout.Shape == nil {
		return
	}
	layout.Shape.Opacity *= max(alpha, 0)
	for i := range layout.Children {
		flipPanelFadeLayout(&layout.Children[i], alpha, depth+1)
	}
}
