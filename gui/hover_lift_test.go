package gui

import (
	"math"
	"strings"
	"testing"
)

func hoverLiftFixture(lift *float32, xfOut *func() Transform) func(*Window) View {
	return func(*Window) View {
		return HoverLift("hl", Translate(0, -6).Scale(1.05), lift,
			func(xf func() Transform) View {
				if xfOut != nil {
					*xfOut = xf
				}
				return Row(ContainerCfg{
					ID:          "hl",
					Width:       100,
					Height:      50,
					Sizing:      FixedFixed,
					TransformFn: xf,
				})
			})
	}
}

func TestHoverLiftStartsTweenOnHover(t *testing.T) {
	var lift float32
	var xf func() Transform
	w := NewTestWindow(t, WindowCfg{Width: 800, Height: 800})
	w.TestRender(hoverLiftFixture(&lift, &xf))
	if w.HasAnimation("hl") {
		t.Fatal("tween started without hover")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseX: 10, MouseY: 10})
	w.TestRender(nil)
	if !w.IsHovered("hl") {
		t.Fatal("fixture not hovered")
	}
	tw, ok := w.animations["hl"].(*TweenAnimation)
	if !ok || !w.HasAnimation("hl") {
		t.Fatal("hover did not start the lift tween")
	}
	if tw.From != 0 || tw.To != 1 {
		t.Errorf("tween = %v->%v, want 0->1", tw.From, tw.To)
	}
	if tw.RefreshKind() != AnimationRefreshRenderOnly {
		t.Errorf("refresh = %v, want render-only", tw.RefreshKind())
	}
	// A quiet frame restarts nothing.
	w.TestRender(nil)
	if again, ok := w.animations["hl"].(*TweenAnimation); !ok || again != tw {
		t.Error("quiet frame restarted the tween")
	}
	// The handed TransformFn lerps the caller's float.
	lift = 0
	if got := xf(); got != (Transform{}) {
		t.Errorf("rest xf = %v, want identity", got)
	}
	lift = 1
	if got, want := xf(), Translate(0, -6).Scale(1.05); got != want {
		t.Errorf("raised xf = %v, want %v", got, want)
	}
	// The tween writes the caller's float.
	tw.OnValue(0.5, w)
	if lift != 0.5 {
		t.Errorf("lift = %v after OnValue, want 0.5", lift)
	}
}

func TestHoverLiftReversesOnLeave(t *testing.T) {
	var lift float32
	w := NewTestWindow(t, WindowCfg{Width: 800, Height: 800})
	w.TestRender(hoverLiftFixture(&lift, nil))
	w.EventFn(&Event{Type: EventMouseMove, MouseX: 10, MouseY: 10})
	w.TestRender(nil)
	lift = 0.5 // mid-flight
	w.EventFn(&Event{Type: EventMouseMove, MouseX: 700, MouseY: 700})
	w.TestRender(nil)
	if w.IsHovered("hl") {
		t.Fatal("still hovered after leave")
	}
	tw, ok := w.animations["hl"].(*TweenAnimation)
	if !ok {
		t.Fatal("leave did not start a tween")
	}
	if tw.From != 0.5 || tw.To != 0 {
		t.Errorf("leave tween = %v->%v, want 0.5->0", tw.From, tw.To)
	}
}

func TestHoverLiftNilValuePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("HoverLift with nil value should panic")
		}
	}()
	w := newTestWindow()
	generateViewLayout(HoverLift("hl", Translate(0, -6), nil,
		func(func() Transform) View { return nil }), w)
}

func TestHoverLiftNilBuild(t *testing.T) {
	w := newTestWindow()
	var lift float32
	layout := generateViewLayout(HoverLift("hl", Translate(0, -6), &lift, nil), w)
	if layout.Shape == nil {
		t.Fatal("nil builder must give an empty normalized layout")
	}
}

// A poisoned lift snaps to the hover target instead of wedging the
// tween on a non-finite From.
func TestHoverLiftNonFiniteValueSnapsToTarget(t *testing.T) {
	var lift float32
	w := NewTestWindow(t, WindowCfg{Width: 800, Height: 800})
	w.TestRender(hoverLiftFixture(&lift, nil))
	lift = float32(math.NaN())
	w.EventFn(&Event{Type: EventMouseMove, MouseX: 10, MouseY: 10})
	w.TestRender(nil)
	if lift != 1 {
		t.Fatalf("lift = %v after hover with NaN, want 1", lift)
	}
	tw, ok := w.animations["hl"].(*TweenAnimation)
	if !ok {
		t.Fatal("hover did not start the lift tween")
	}
	if !f32IsFinite(tw.From) {
		t.Errorf("tween From = %v, want finite", tw.From)
	}
}

func TestHoverLiftIDMismatchWarns(t *testing.T) {
	buf := captureDebugMask(t, DebugMissingIDs)
	w := newTestWindow()
	var lift float32
	mk := func(root string) View {
		return HoverLift("hl", Translate(0, -6), &lift,
			func(xf func() Transform) View {
				return Row(ContainerCfg{ID: root, TransformFn: xf})
			})
	}

	generateViewLayout(mk("hl"), w)
	if s := buf.String(); s != "" {
		t.Fatalf("matching root ID must stay silent, got %q", s)
	}

	generateViewLayout(mk("other"), w)
	s := buf.String()
	if !strings.Contains(s, `"hl"`) || !strings.Contains(s, `"other"`) {
		t.Fatalf("mismatch must name both IDs, got %q", s)
	}
}
