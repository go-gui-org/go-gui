package gui

// interactive.go — Interactive, the first behavior helper (issue #650,
// docs/specs/interactive-behavior-helper.md).
//
// A custom look that reads hover or press state has two problems when it
// is written by hand. It must be a named view type, because a factory
// body runs before the scope of the widget exists. It must also resolve
// the effective ID itself and give it to each query. Interactive does
// both inside its own GenerateLayout, and gives the builder the result.

// InteractionState is the interaction state of one widget, read from the
// last arranged frame. Interactive gives it to its builder.
type InteractionState struct {
	// Hovered is true when the pointer is over the widget or one of its
	// ID-bearing descendants. See [Window.IsHovered].
	Hovered bool
	// Pressed is true while a left press that started on the widget is
	// held, also when the pointer moved off it. See [Window.IsPressed].
	Pressed bool
	// Armed is Pressed and Hovered: the press is held and the pointer
	// is still over the widget. A push button shows its pressed look
	// only while it is armed, so a drag off the button releases it.
	Armed bool
	// Focused is true when the widget itself has keyboard focus. A
	// focused descendant does not count. See [Window.IsFocus].
	// exportaudit:keep — one member of the state set; the set ships whole
	Focused bool
}

// Interactive builds a view whose look depends on its interaction state.
// It resolves id against the enclosing ID scope, reads the hover, press
// and focus state for that effective ID, and calls build with it:
//
//	gui.Interactive("ok", func(s gui.InteractionState) gui.View {
//		face := normal
//		if s.Armed {
//			face = pressed
//		}
//		return gui.Row(gui.ContainerCfg{ID: "ok", Color: face, ...})
//	})
//
// The root of the view that build returns must have the ID id. Interactive
// does not set it, because it does not change the caller's view. With a
// different root ID, or an empty id, the state never turns true;
// gui.Debug reports that under [DebugMissingIDs].
//
// Disabled shapes are never hovered or pressed, so the state is false
// for a disabled widget and build needs no separate check.
//
// build runs during layout generation, under the frame lock: it must
// not call a window-mutating API (see [Window.QueueCommand]).
func Interactive(id string, build func(InteractionState) View) View {
	return interactiveView{id: id, build: build}
}

// interactiveView defers build to layout generation. At that time the
// scope of the parent is open, so EffID joins the correct scope.
type interactiveView struct {
	id    string
	build func(InteractionState) View
}

func (v interactiveView) GenerateLayout(w *Window) Layout {
	if v.build == nil {
		return Layout{}
	}
	eid := w.EffID(v.id)
	hovered := w.IsHovered(eid)
	pressed := w.IsPressed(eid)
	child := v.build(InteractionState{
		Hovered: hovered,
		Pressed: pressed,
		Armed:   hovered && pressed,
		Focused: w.IsFocus(eid),
	})
	if child == nil {
		return Layout{}
	}
	layout := generateViewLayout(child, w)
	// generateViewLayout always sets a Shape, so no nil check is needed.
	// An empty id matches an ID-less root, but no shape without an ID
	// is ever hovered, pressed or focused, so it is reported too.
	if v.id == "" || layout.Shape.ID != v.id {
		w.debugWarn(debugCheckInteractiveIDMismatch, v.id,
			"Interactive(%q) got a view whose root ID is %q, so its "+
				"hover, press and focus state is never true. Set the root "+
				"ID to %q.", v.id, layout.Shape.ID, v.id)
	}
	return layout
}
