package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRunPointerAmendGates covers runPointerAmend end-to-end: an
// AmendLayout hook that reads the pointer without pointerAmend fails the
// audit; a paired flag, a marked read, or no read at all passes.
func TestRunPointerAmendGates(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{
			name: "paired flag passes",
			src: `package gui

func tipAmend(ctx EventCtx) {
	mx := ctx.Window.viewState.mousePosX
	_ = mx
}

func build() {
	_ = ContainerCfg{
		AmendLayout:  tipAmend,
		pointerAmend: true,
	}
}
`,
		},
		{
			name: "unpaired hook fails",
			src: `package gui

func tipAmend(ctx EventCtx) {
	mx := ctx.Window.viewState.mousePosX
	_ = mx
}

func build() {
	_ = ContainerCfg{AmendLayout: tipAmend}
}
`,
			wantErr: true,
		},
		{
			name: "doc-marked hook passes",
			src: `package gui

// tipAmend clears state on leave.
//
// ergonomics-audit:pointeramend — safe: tooltip state forces a
// rebuild on each move, so the hook never misses one.
func tipAmend(ctx EventCtx) {
	mx := ctx.Window.viewState.mousePosX
	_ = mx
}

func build() {
	_ = ContainerCfg{AmendLayout: tipAmend}
}
`,
		},
		{
			name: "same-line marked read passes",
			src: `package gui

func tipAmend(ctx EventCtx) {
	mx := ctx.Window.viewState.mousePosX // ergonomics-audit:pointeramend
	_ = mx
}

func build() {
	_ = ContainerCfg{AmendLayout: tipAmend}
}
`,
		},
		{
			name: "wrapper around a reader fails",
			src: `package gui

func inner(ctx EventCtx) {
	mx := ctx.Window.viewState.pointerY
	_ = mx
}

func outer(ctx EventCtx) {
	inner(ctx)
}

func build() {
	_ = ContainerCfg{AmendLayout: outer}
}
`,
			wantErr: true,
		},
		{
			name: "wrapper around a marked reader passes",
			src: `package gui

// inner clears state on leave.
//
// ergonomics-audit:pointeramend — safe: tooltip state forces a
// rebuild on each move, so the hook never misses one.
func inner(ctx EventCtx) {
	mx := ctx.Window.viewState.pointerY
	_ = mx
}

func outer(ctx EventCtx) {
	inner(ctx)
}

func build() {
	_ = ContainerCfg{AmendLayout: outer}
}
`,
		},
		{
			name: "factory returning a reader fails",
			src: `package gui

func makeAmend() func(EventCtx) {
	return func(ctx EventCtx) {
		mx := ctx.Window.viewState.mousePosX
		_ = mx
	}
}

func build() {
	_ = ContainerCfg{AmendLayout: makeAmend()}
}
`,
			wantErr: true,
		},
		{
			name: "factory returning a reader passes with the flag",
			src: `package gui

func makeAmend() func(EventCtx) {
	return func(ctx EventCtx) {
		mx := ctx.Window.viewState.mousePosX
		_ = mx
	}
}

func build() {
	_ = ContainerCfg{
		AmendLayout:  makeAmend(),
		pointerAmend: true,
	}
}
`,
		},
		{
			name: "amendAll member reading fails",
			src: `package gui

func ring(ctx EventCtx) {
}

func slide(ctx EventCtx) {
	my := ctx.Window.viewState.mousePosY
	_ = my
}

func build() {
	_ = ContainerCfg{AmendLayout: amendAll(ring, slide)}
}
`,
			wantErr: true,
		},
		{
			name: "assignment install reading fails",
			src: `package gui

func ring(ctx EventCtx) {
	mx := ctx.Window.viewState.mousePosX
	_ = mx
}

func build(s *shape) {
	s.events.AmendLayout = ring
}
`,
			wantErr: true,
		},
		{
			name: "unreached reader passes",
			src: `package gui

// animationTooltip checks the position from a timer, not from AmendLayout.
func animationTooltip() {
	mx := w.viewState.mousePosX
	_ = mx
}

func build() {
	_ = ContainerCfg{AmendLayout: ring}
}

func ring(ctx EventCtx) {
}
`,
		},
		{
			name: "non-hook helper reads stay out",
			src: `package gui

func scrollHelper(w *Window) {
	mx := w.viewState.mousePosX
	_ = mx
}

func hook(ctx EventCtx) {
	scrollHelper(ctx.Window)
}

func build() {
	_ = ContainerCfg{AmendLayout: hook}
}
`,
		},
		{
			name: "qualified EventCtx counts as hook-shaped",
			src: `package gui

func tipAmend(ctx gui.EventCtx) {
	mx := ctx.Window.viewState.mousePosX
	_ = mx
}

func build() {
	_ = ContainerCfg{AmendLayout: tipAmend}
}
`,
			wantErr: true,
		},
		{
			name: "amendAll nested factory reading fails",
			src: `package gui

func makeRing() func(EventCtx) {
	return func(ctx EventCtx) {
		mx := ctx.Window.viewState.mousePosX
		_ = mx
	}
}

func ring(ctx EventCtx) {
}

func build() {
	_ = ContainerCfg{AmendLayout: amendAll(ring, makeRing())}
}
`,
			wantErr: true,
		},
		{
			name: "explicit pointerAmend false still fails",
			src: `package gui

func tipAmend(ctx EventCtx) {
	mx := ctx.Window.viewState.mousePosX
	_ = mx
}

func build() {
	_ = ContainerCfg{
		AmendLayout:  tipAmend,
		pointerAmend: false,
	}
}
`,
			wantErr: true,
		},
		{
			name: "hook without reads passes",
			src: `package gui

func ring(ctx EventCtx) {
	ctx.Layout.Shape.Color = red
}

func build() {
	_ = ContainerCfg{AmendLayout: ring}
}
`,
		},
		{
			name: "write to the position is not a read",
			src: `package gui

func record(w *Window, x float32) {
	w.viewState.mousePosX = x
}

func build() {
	_ = ContainerCfg{AmendLayout: ring}
}

func ring(ctx EventCtx) {
}
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			guiDir := filepath.Join(repo, "gui")
			if err := os.MkdirAll(guiDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(
				filepath.Join(guiDir, "hook.go"), []byte(tc.src), 0o644,
			); err != nil {
				t.Fatal(err)
			}
			err := runPointerAmend([]string{repo})
			if tc.wantErr && err == nil {
				t.Fatal("runPointerAmend passed with an unflagged pointer read")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("runPointerAmend failed on clean code: %v", err)
			}
		})
	}
}
