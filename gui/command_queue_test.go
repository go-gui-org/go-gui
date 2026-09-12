package gui

import "testing"

func TestAnimationCommands_AppendOnDone(t *testing.T) {
	var cmds []queuedCommand
	ac := newAnimationCommands(&cmds)
	called := false
	ac.appendOnDone(func(_ *Window) { called = true })

	if len(cmds) != 1 {
		t.Fatalf("len = %d, want 1", len(cmds))
	}
	if cmds[0].kind != queuedCommandWindowFn {
		t.Errorf("kind = %d, want %d", cmds[0].kind, queuedCommandWindowFn)
	}
	cmds[0].windowFn(nil)
	if !called {
		t.Error("callback not invoked")
	}
}

func TestAnimationCommands_AppendOnDoneNilSkips(t *testing.T) {
	var cmds []queuedCommand
	ac := newAnimationCommands(&cmds)
	ac.appendOnDone(nil)

	if len(cmds) != 0 {
		t.Errorf("len = %d, want 0 for nil fn", len(cmds))
	}
}

func TestAnimationCommands_AppendOnValue(t *testing.T) {
	var cmds []queuedCommand
	ac := newAnimationCommands(&cmds)
	var gotVal float32
	ac.appendOnValue(func(v float32, _ *Window) { gotVal = v }, 42.5)

	if len(cmds) != 1 {
		t.Fatalf("len = %d, want 1", len(cmds))
	}
	if cmds[0].kind != queuedCommandValueFn {
		t.Errorf("kind = %d, want %d", cmds[0].kind, queuedCommandValueFn)
	}
	if cmds[0].value != 42.5 {
		t.Errorf("value = %f, want 42.5", cmds[0].value)
	}
	cmds[0].valueFn(cmds[0].value, nil)
	if gotVal != 42.5 {
		t.Errorf("callback got %f, want 42.5", gotVal)
	}
}

// Nil receiver / nil inner must be safe — an Animation constructed
// without a wrapping AnimationCommands (e.g. unit test calling Update
// directly) should not panic when enqueuing.
func TestAnimationCommands_NilSafe(t *testing.T) {
	var nilAC *AnimationCommands
	nilAC.appendOnDone(func(*Window) {})
	nilAC.appendOnValue(func(float32, *Window) {}, 1)

	emptyAC := AnimationCommands{}
	emptyAC.appendOnDone(func(*Window) {})
	emptyAC.appendOnValue(func(float32, *Window) {}, 1)
}

func TestCommandMarkLayoutRefreshSetsFlag(t *testing.T) {
	w := &Window{}
	commandMarkLayoutRefresh(w)
	if !w.refreshLayout {
		t.Error("refreshLayout should be true")
	}
}

func TestCommandMarkRenderOnlyRefreshSetsFlag(t *testing.T) {
	w := &Window{}
	commandMarkRenderOnlyRefresh(w)
	if !w.refreshRenderOnly {
		t.Error("refreshRenderOnly should be true")
	}
}

func TestAnimationCommands_ExportedAppend(t *testing.T) {
	var cmds []queuedCommand
	ac := newAnimationCommands(&cmds)
	ac.AppendOnDone(func(_ *Window) {})
	ac.AppendOnValue(func(float32, *Window) {}, 7)
	if len(cmds) != 2 {
		t.Fatalf("len = %d, want 2", len(cmds))
	}
	if cmds[0].kind != queuedCommandWindowFn {
		t.Errorf("kind = %d, want windowFn", cmds[0].kind)
	}
	if cmds[1].kind != queuedCommandValueFn || cmds[1].value != 7 {
		t.Errorf("value cmd mismatch: %+v", cmds[1])
	}
}

func TestAnimationCommands_ExportedNilSafe(t *testing.T) {
	var nilAC *AnimationCommands
	nilAC.AppendOnDone(func(*Window) {})
	nilAC.AppendOnValue(func(float32, *Window) {}, 1)
	emptyAC := AnimationCommands{}
	emptyAC.AppendOnDone(func(*Window) {})
	emptyAC.AppendOnValue(func(float32, *Window) {}, 1)
	var cmds []queuedCommand
	exportedAC := newAnimationCommands(&cmds)
	exportedAC.AppendOnDone(nil)
	exportedAC.AppendOnValue(nil, 1)
	if len(cmds) != 0 {
		t.Errorf("len = %d, want 0 for nil fns", len(cmds))
	}
}
