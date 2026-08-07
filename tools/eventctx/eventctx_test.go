package eventctx

import (
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGolden rewrites each testdata/*.input and compares against the
// matching .golden. Run with -update to regenerate.
func TestGolden(t *testing.T) {
	inputs, err := filepath.Glob("testdata/*.input")
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) == 0 {
		t.Fatal("no testdata inputs")
	}
	for _, in := range inputs {
		t.Run(filepath.Base(in), func(t *testing.T) {
			src, err := os.ReadFile(in)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Rewrite(in, src, nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := format.Source(res.Src)
			if err != nil {
				t.Fatalf("rewritten source does not parse: %v\n%s", err, res.Src)
			}
			golden := strings.TrimSuffix(in, ".input") + ".golden"
			if os.Getenv("UPDATE_GOLDEN") != "" {
				if err := os.WriteFile(golden, out, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != string(want) {
				t.Errorf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s",
					out, want)
			}
		})
	}
}

// TestRule4Report checks that a consume-class callback with an early
// return is reported rather than silently rewritten.
func TestRule4Report(t *testing.T) {
	src := []byte(`package p

func f(c *Cfg) {
	c.OnClick = func(l *Layout, e *Event, w *Window) {
		if !hit(l, e) {
			return
		}
		act(w)
	}
	c.OnMouseMove = func(l *Layout, e *Event, w *Window) {
		if !hit(l, e) {
			return
		}
		act(w)
	}
}
`)
	res, err := Rewrite("p.go", src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("want 1 finding, got %d: %v", len(res.Findings), res.Findings)
	}
	if res.Findings[0].Func != "OnClick" {
		t.Errorf("finding owner = %q, want OnClick", res.Findings[0].Func)
	}
	if res.Findings[0].Kind != "early-return" {
		t.Errorf("finding kind = %q, want early-return", res.Findings[0].Kind)
	}
}

// TestBubble checks that an explicit unhandled assignment becomes
// ctx.Bubble() regardless of class.
func TestBubble(t *testing.T) {
	src := []byte(`package p

var f = func(l *Layout, e *Event, w *Window) {
	e.IsHandled = false
}
`)
	res, err := Rewrite("p.go", src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Src), "ctx.Bubble()") {
		t.Errorf("want ctx.Bubble(), got:\n%s", res.Src)
	}
}

// TestQualified checks that files outside package gui get the
// gui.EventCtx spelling.
func TestQualified(t *testing.T) {
	src := []byte(`package main

var f = func(l *gui.Layout, e *gui.Event, w *gui.Window) {
	e.IsHandled = true
}
`)
	res, err := Rewrite("m.go", src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.Src), "ctx gui.EventCtx") {
		t.Errorf("want gui.EventCtx, got:\n%s", res.Src)
	}
}
