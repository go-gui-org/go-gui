//go:build js && wasm

package web

import (
	"fmt"
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// fakeDOM builds elements whose DOM methods append to log, so a test
// can count what a sync wrote. Listeners are kept per element so a
// test can dispatch a click.
type fakeDOM struct {
	log       []string
	listeners map[string][]js.Value // element id -> click listeners
	funcs     []js.Func
}

func (d *fakeDOM) method(obj js.Value, name string, fn func(this js.Value, args []js.Value)) {
	f := js.FuncOf(func(this js.Value, args []js.Value) any {
		fn(this, args)
		return nil
	})
	d.funcs = append(d.funcs, f)
	obj.Set(name, f)
}

func (d *fakeDOM) element() js.Value {
	el := js.Global().Get("Object").New()
	el.Set("style", js.Global().Get("Object").New())
	idOf := func(this js.Value) string { return this.Get("id").String() }
	d.method(el, "setAttribute", func(this js.Value, a []js.Value) {
		d.log = append(d.log, fmt.Sprintf("%s set %s=%s", idOf(this), a[0].String(), a[1].String()))
	})
	d.method(el, "removeAttribute", func(this js.Value, a []js.Value) {
		d.log = append(d.log, fmt.Sprintf("%s rm %s", idOf(this), a[0].String()))
	})
	d.method(el, "appendChild", func(this js.Value, a []js.Value) {
		d.log = append(d.log, fmt.Sprintf("%s append %s", idOf(this), idOf(a[0])))
	})
	d.method(el, "remove", func(this js.Value, _ []js.Value) {
		d.log = append(d.log, idOf(this)+" remove")
	})
	d.method(el, "addEventListener", func(this js.Value, a []js.Value) {
		d.listeners[idOf(this)] = append(d.listeners[idOf(this)], a[1])
		d.log = append(d.log, idOf(this)+" listen")
	})
	d.method(el, "removeEventListener", func(this js.Value, a []js.Value) {
		ls := d.listeners[idOf(this)]
		for i, l := range ls {
			if l.Equal(a[1]) {
				d.listeners[idOf(this)] = append(ls[:i], ls[i+1:]...)
				break
			}
		}
		d.log = append(d.log, idOf(this)+" unlisten")
	})
	return el
}

func newFakeA11y(t *testing.T, onAction func(action, idx int)) (*a11yState, *fakeDOM) {
	t.Helper()
	d := &fakeDOM{listeners: map[string][]js.Value{}}
	doc := js.Global().Get("Object").New()
	d.method(doc, "createElement", func(js.Value, []js.Value) {})
	// createElement must return a value, which method() cannot; set it
	// directly.
	create := js.FuncOf(func(js.Value, []js.Value) any { return d.element() })
	d.funcs = append(d.funcs, create)
	doc.Set("createElement", create)
	a := &a11yState{doc: doc, actionCb: onAction, focused: -1}
	a.root = d.element()
	a.root.Set("id", "root")
	t.Cleanup(func() {
		a.destroy()
		for _, f := range d.funcs {
			f.Release()
		}
	})
	return a, d
}

func a11yTestNodes() []gui.A11yNode {
	return []gui.A11yNode{
		{Role: gui.AccessRoleGroup, Label: "grid", ParentIdx: -1},
		{Role: gui.AccessRoleImage, Label: "tiger", ParentIdx: 0},
		{Role: gui.AccessRoleButton, Label: "ok", ParentIdx: 0},
	}
}

func TestA11ySyncUnchangedTreeWritesNothing(t *testing.T) {
	a, d := newFakeA11y(t, func(int, int) {})
	nodes := a11yTestNodes()
	a.sync(nodes, len(nodes), 2)
	if len(d.log) == 0 {
		t.Fatal("first sync wrote nothing")
	}
	d.log = d.log[:0]
	a.sync(nodes, len(nodes), 2)
	if len(d.log) != 0 {
		t.Fatalf("unchanged sync wrote %d DOM calls: %v", len(d.log), d.log)
	}
}

func TestA11ySyncWritesOnlyChanges(t *testing.T) {
	a, d := newFakeA11y(t, func(int, int) {})
	nodes := a11yTestNodes()
	a.sync(nodes, len(nodes), -1)

	d.log = d.log[:0]
	nodes[1].Label = "lion"
	a.sync(nodes, len(nodes), -1)
	if want := []string{"a11y-1 set aria-label=lion"}; fmt.Sprint(d.log) != fmt.Sprint(want) {
		t.Errorf("label change wrote %v, want %v", d.log, want)
	}

	d.log = d.log[:0]
	nodes[2].State |= gui.AccessStateDisabled
	a.sync(nodes, len(nodes), -1)
	if want := []string{"a11y-2 set aria-disabled=true"}; fmt.Sprint(d.log) != fmt.Sprint(want) {
		t.Errorf("state change wrote %v, want %v", d.log, want)
	}

	d.log = d.log[:0]
	nodes[2].ParentIdx = 1
	a.sync(nodes, len(nodes), -1)
	want := []string{"root append a11y-0", "a11y-0 append a11y-1", "a11y-1 append a11y-2"}
	if fmt.Sprint(d.log) != fmt.Sprint(want) {
		t.Errorf("parent change wrote %v, want relink %v", d.log, want)
	}

	d.log = d.log[:0]
	a.sync(nodes[:2], 2, -1)
	if want := []string{"a11y-2 remove"}; fmt.Sprint(d.log) != fmt.Sprint(want) {
		t.Errorf("shrink wrote %v, want %v", d.log, want)
	}
}

// The full-rewrite sync released its click callbacks every time but
// left them attached; a click after the second sync hit a released
// function. One listener must stay attached and live across syncs.
func TestA11yClickSurvivesResync(t *testing.T) {
	var got []int
	a, d := newFakeA11y(t, func(action, idx int) {
		if action == gui.A11yActionPress {
			got = append(got, idx)
		}
	})
	nodes := a11yTestNodes()
	for range 3 {
		a.sync(nodes, len(nodes), -1)
	}
	ls := d.listeners["a11y-2"]
	if len(ls) != 1 {
		t.Fatalf("button has %d click listeners, want 1", len(ls))
	}
	ls[0].Invoke()
	if fmt.Sprint(got) != "[2]" {
		t.Errorf("click pressed %v, want [2]", got)
	}

	nodes[2].Role = gui.AccessRoleStaticText
	a.sync(nodes, len(nodes), -1)
	if n := len(d.listeners["a11y-2"]); n != 0 {
		t.Errorf("non-actionable node keeps %d listeners", n)
	}
}
