package gui

import "time"

// hover_lift.go — HoverLift, a behavior helper that animates a view
// transform on hover (#1019).
//
// A hover lift written by hand needs three pieces: a float in app state
// for the lift amount, a tween that moves it toward 0 or 1 when hover
// changes, and a TransformFn that lerps the transform from that float.
// HoverLift owns the middle piece: it watches hover for its ID, starts
// and replaces the tween on transitions, and hands the builder a
// TransformFn that reads the caller's float. Like Interactive, it
// resolves the ID against the enclosing scope inside its own
// GenerateLayout, so the caller names only the leaf.

const hoverLiftDuration = 200 * time.Millisecond

// HoverLift builds a view that eases between the identity transform and
// lifted while the pointer is over it.
//
// value holds the lift amount, from 0 at rest to 1 raised, and stays
// owned by the caller: the tween writes it through the pointer, and
// the xf it hands the builder lerps from it, so it doubles as the
// TransformFn:
//
//	gui.HoverLift("lift", gui.Translate(0, -6).Scale(1.05), &st.lift,
//		func(xf func() gui.Transform) gui.View {
//			return gui.Row(gui.ContainerCfg{ID: "lift", TransformFn: xf, ...})
//		})
//
// The tween runs in render-only frames with no rebuild. It replaces
// itself on the same ID when hover flips mid-flight, so a quick
// enter-leave turns smoothly instead of jumping. Duration and easing
// are fixed; a variant with parameters can come later.
//
// The root of the view that build returns must have the ID id, as with
// Interactive: the hover test is read for that ID, and a different root
// ID is reported under [DebugMissingIDs].
//
// value must be non-nil.
func HoverLift(id string, lifted Transform, value *float32, build func(xf func() Transform) View) View {
	return hoverLiftView{id: id, lifted: lifted, value: value, build: build}
}

type hoverLiftView struct {
	id     string
	lifted Transform
	value  *float32
	build  func(xf func() Transform) View
}

func (v hoverLiftView) GenerateLayout(w *Window) Layout {
	if v.build == nil {
		return Layout{}
	}
	if v.value == nil {
		panic("gui: HoverLift requires a non-nil value")
	}
	eid := w.EffID(v.id)
	hovered := w.IsHovered(eid)
	targets := StateMap[string, bool](w, nsHoverLift, capFew)
	target := targets.GetOr(eid, false)
	if hovered != target {
		targets.Set(eid, hovered)
		var to float32
		if hovered {
			to = 1
		}
		value := v.value
		// A non-finite lift (caller wrote NaN/Inf into value) would
		// wedge the tween: updateTween spins without stopping on a
		// non-finite From. Snap to the target instead; Lerp already
		// guards the read path the same way.
		from := *value
		if !f32IsFinite(from) {
			from = to
			*value = to
		}
		tw := NewTweenAnimation(eid, from, to, func(val float32, _ *Window) {
			*value = val
		})
		tw.Duration = hoverLiftDuration
		tw.Easing = EaseOutCubic
		tw.Refresh = AnimationRefreshRenderOnly
		w.AnimationAdd(tw)
	}
	lifted := v.lifted
	value := v.value
	xf := func() Transform {
		return Transform{}.Lerp(lifted, *value)
	}
	child := v.build(xf)
	if child == nil {
		return Layout{}
	}
	layout := generateViewLayout(child, w)
	// generateViewLayout always sets a Shape, so no nil check is needed.
	if v.id == "" || layout.Shape.ID != v.id {
		w.debugWarn(debugCheckInteractiveIDMismatch, v.id,
			"HoverLift(%q) got a view whose root ID is %q, so its "+
				"hover state is never true. Set the root "+
				"ID to %q.", v.id, layout.Shape.ID, v.id)
	}
	return layout
}
