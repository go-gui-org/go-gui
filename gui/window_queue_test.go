package gui

import "testing"

type queueTestState struct {
	count int
}

func TestFlushCommandsRunsAllAndClears(t *testing.T) {
	state := &queueTestState{}
	w := &Window{}
	w.state = state
	w.commandsMu.Lock()
	w.commands = append(w.commands, queuedCommand{
		kind: queuedCommandWindowFn,
		windowFn: func(win *Window) {
			State[queueTestState](win).count++
		},
	})
	w.commands = append(w.commands, queuedCommand{
		kind: queuedCommandWindowFn,
		windowFn: func(win *Window) {
			State[queueTestState](win).count++
		},
	})
	w.commandsMu.Unlock()

	w.flushCommands()

	if state.count != 2 {
		t.Errorf("count: got %d, want 2", state.count)
	}
	if len(w.commands) != 0 {
		t.Errorf("commands not cleared: len=%d", len(w.commands))
	}
}

func TestFlushCommandsEmptyQueueIsNoop(t *testing.T) {
	w := &Window{}
	w.flushCommands()
	if len(w.commands) != 0 {
		t.Errorf("commands: got %d, want 0", len(w.commands))
	}
}

func TestQueueValueCommandExecutes(t *testing.T) {
	w := &Window{}
	var gotVal float32
	w.queueValueCommand(func(v float32, _ *Window) {
		gotVal = v
	}, 3.14)
	w.flushCommands()
	if gotVal != 3.14 {
		t.Errorf("value = %f, want 3.14", gotVal)
	}
}

func TestQueueAnimateCommandExecutes(t *testing.T) {
	w := &Window{}
	a := &Animate{AnimID: "test"}
	var gotAnimate *Animate
	w.queueAnimateCommand(func(anim *Animate, _ *Window) {
		gotAnimate = anim
	}, a)
	w.flushCommands()
	if gotAnimate != a {
		t.Error("animate pointer mismatch")
	}
}

func TestQueueCommandNilNoOp(t *testing.T) {
	w := &Window{}
	w.QueueCommand(nil)
	if len(w.commands) != 0 {
		t.Errorf("commands: got %d, want 0 for nil cb", len(w.commands))
	}
}

func TestQueueValueCommandNilNoOp(t *testing.T) {
	w := &Window{}
	w.queueValueCommand(nil, 1.0)
	if len(w.commands) != 0 {
		t.Errorf("commands: got %d, want 0 for nil cb", len(w.commands))
	}
}

func TestQueueAnimateCommandNilNoOp(t *testing.T) {
	w := &Window{}
	w.queueAnimateCommand(nil, nil)
	if len(w.commands) != 0 {
		t.Errorf("commands: got %d, want 0 for nil cb", len(w.commands))
	}
}

func TestQueueCommandsBatchMultiple(t *testing.T) {
	w := &Window{}
	count := 0
	cmds := []queuedCommand{
		{kind: queuedCommandWindowFn, windowFn: func(_ *Window) { count++ }},
		{kind: queuedCommandWindowFn, windowFn: func(_ *Window) { count++ }},
		{kind: queuedCommandWindowFn, windowFn: func(_ *Window) { count++ }},
	}
	w.queueCommandsBatch(cmds)
	w.flushCommands()
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestQueueCommandsBatchEmpty(t *testing.T) {
	w := &Window{}
	w.queueCommandsBatch(nil)
	if len(w.commands) != 0 {
		t.Errorf("commands: got %d, want 0", len(w.commands))
	}
}

func TestReclaimCommandScratch(t *testing.T) {
	w := &Window{}
	w.commandScratch = make([]queuedCommand, 0, 8)
	w.commands = nil
	w.commandsMu.Lock()
	w.reclaimCommandScratch()
	w.commandsMu.Unlock()
	if w.commands == nil {
		t.Error("commands should be reclaimed from scratch")
	}
	if cap(w.commands) != 8 {
		t.Errorf("cap = %d, want 8", cap(w.commands))
	}
	if w.commandScratch != nil {
		t.Error("commandScratch should be nil after reclaim")
	}
}

func TestReclaimCommandScratchSkipsWhenNotNil(t *testing.T) {
	w := &Window{}
	existing := make([]queuedCommand, 0, 4)
	w.commands = existing
	w.commandScratch = make([]queuedCommand, 0, 8)
	w.commandsMu.Lock()
	w.reclaimCommandScratch()
	w.commandsMu.Unlock()
	if cap(w.commands) != 4 {
		t.Errorf("commands cap = %d, want 4 (unchanged)", cap(w.commands))
	}
}

func TestComposeLayout(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	layers := []Layout{
		{Shape: &Shape{Width: 800, Height: 600}},
		{Shape: &Shape{Width: 100, Height: 50}},
	}
	root := composeLayout(layers, w)
	if root.Shape.Width != 800 || root.Shape.Height != 600 {
		t.Errorf("root size = %fx%f, want 800x600",
			root.Shape.Width, root.Shape.Height)
	}
	if len(root.Children) != 2 {
		t.Errorf("children = %d, want 2", len(root.Children))
	}
}
