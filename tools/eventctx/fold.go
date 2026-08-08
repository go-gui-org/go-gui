package eventctx

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
)

// guiAlias returns the name this file uses for the go-gui gui package,
// or "" when the file is itself in package gui.
func guiAlias(f *ast.File) string {
	for _, im := range f.Imports {
		path := strings.Trim(im.Path.Value, `"`)
		if path != "github.com/go-gui-org/go-gui/gui" {
			continue
		}
		if im.Name != nil {
			return im.Name.Name
		}
		return "gui"
	}
	return ""
}

// FoldSpec identifies one call site whose argument list must collapse
// into an EventCtx. Calls made through a callback-typed variable or
// struct field cannot be found syntactically — the callee's type lives
// elsewhere — so the compiler locates them and this drives the edit.
//
// Have is the parameter list the compiler reported as passed, e.g.
// "*Layout, *Event, *Window" or "GridRow, *Event, *Window". It is the
// only thing needed to decide which arguments fold and which stay.
type FoldSpec struct {
	Line, Col int
	Have      string

	// WrapWindow selects the other diagnostic shape. When a callback
	// gains no parameters overall — func(T, *Window) becoming
	// func(T, EventCtx) — the argument count still matches, so the
	// compiler reports a type mismatch on the trailing *Window rather
	// than a surplus argument, and Have is unavailable. Only that one
	// argument needs wrapping; every other argument stays put.
	WrapWindow bool
}

// FoldCalls applies the given fold specs to one file.
func FoldCalls(filename string, src []byte, specs []FoldSpec) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	tf := fset.File(f.Pos())
	r := &rewriter{
		fset: fset, src: src, base: tf.Base(), guiAlias: guiAlias(f),
	}

	// The compiler reports the position of the first surplus argument,
	// so index calls by every argument position (and by the callee, for
	// the shapes where it points there instead).
	byPos := map[int]*ast.CallExpr{}
	ast.Inspect(f, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		byPos[tf.Offset(c.Fun.Pos())] = c
		for _, a := range c.Args {
			byPos[tf.Offset(a.Pos())] = c
		}
		return true
	})

	for _, sp := range specs {
		off := tf.Offset(tf.LineStart(sp.Line)) + sp.Col - 1
		call, ok := byPos[off]
		if !ok {
			return nil, fmt.Errorf("%s:%d:%d: no call found", filename, sp.Line, sp.Col)
		}
		if sp.WrapWindow {
			r.wrapWindowArg(call, off)
			continue
		}
		text, err := foldArgs(r, call, sp.Have)
		if err != nil {
			return nil, fmt.Errorf("%s:%d:%d: %w", filename, sp.Line, sp.Col, err)
		}
		r.replace(call.Lparen+1, call.Rparen, text)
	}
	if len(r.edits) == 0 {
		return src, nil
	}
	return r.apply(), nil
}

// wrapWindowArg replaces the single *Window argument at off with an
// EventCtx. Inside an already-converted callback the window is spelled
// ctx.Window, and the whole context is both available and strictly more
// informative, so pass ctx through rather than discarding the layout
// and event it carries.
func (r *rewriter) wrapWindowArg(call *ast.CallExpr, off int) {
	for _, a := range call.Args {
		if r.offset(a.Pos()) != off {
			continue
		}
		text := r.srcOf(a)
		if text == "ctx.Window" {
			r.replace(a.Pos(), a.End(), "ctx")
			return
		}
		ctxName := "EventCtx"
		if r.guiAlias != "" {
			r.replace(a.Pos(), a.End(),
				r.guiAlias+".EventCtx{Window: "+text+"}")
			return
		}
		r.replace(a.Pos(), a.End(), ctxName+"{nil, nil, "+text+"}")
		return
	}
}

// foldArgs builds the replacement argument list. The shape of Have says
// where the Layout, Event and payload arguments are; anything absent
// becomes a nil field of the EventCtx.
func foldArgs(r *rewriter, call *ast.CallExpr, have string) (string, error) {
	parts := splitParams(have)
	args := call.Args
	if len(parts) != len(args) {
		return "", fmt.Errorf("have %q does not match %d args", have, len(args))
	}
	// The compiler prints the package's real name (gui), but the file
	// may import it under an alias (gg), so take the qualifier from the
	// import list rather than from the diagnostic.
	ctxName := "EventCtx"
	if strings.Contains(have, ".Window") && r.guiAlias != "" {
		ctxName = r.guiAlias + ".EventCtx"
	}
	src := func(i int) string { return r.srcOf(args[i]) }

	// The window is always the last argument. Everything else is
	// matched by type name, then any remaining untyped nil is filled
	// right to left: event first, then layout. An untyped nil carries
	// no type in the diagnostic, so position is the only signal.
	// layout stays empty while no argument maps to it, which
	// distinguishes "the old signature had no *Layout at all" from "the
	// caller deliberately passed nil for one". Only the former may be
	// widened to the enclosing ctx below.
	layout, event := "", "nil"
	var payloads []string // built back to front, reversed below
	window := src(len(parts) - 1)
	for i := len(parts) - 2; i >= 0; i-- {
		p := parts[i]
		switch {
		case strings.HasSuffix(p, "*Layout") || strings.HasSuffix(p, ".Layout"):
			layout = src(i)
		case strings.HasSuffix(p, "*Event") || strings.HasSuffix(p, ".Event"):
			event = src(i)
		case p == "nil":
			if event == "nil" {
				event = src(i)
			} else {
				layout = src(i)
			}
		default:
			// Any number of payload arguments is legitimate:
			// OnTextCommit carries a string and an
			// InputCommitReason. They keep their relative order.
			payloads = append(payloads, src(i))
		}
	}
	slices.Reverse(payloads)
	payload := strings.Join(payloads, ", ")

	// Inside an already-converted callback the whole context is in
	// scope. Where the old signature carried no *Layout, synthesising
	// EventCtx{nil, ctx.Event, ctx.Window} would manufacture an absence
	// — there is a layout, the callback is dispatched from it — so pass
	// ctx through and let the callback see it. A layout the caller
	// explicitly passed as nil is left alone; that was a decision.
	layoutAbsent := layout == ""
	if layoutAbsent {
		layout = "nil"
	}
	if layoutAbsent && event == "ctx.Event" && window == "ctx.Window" {
		return join(payload, "ctx"), nil
	}

	out := ctxName + "{" + layout + ", " + event + ", " + window + "}"
	if ctxName != "EventCtx" {
		// go vet's composites check rejects unkeyed literals of a type
		// from another package.
		out = ctxName + "{Layout: " + layout + ", Event: " + event +
			", Window: " + window + "}"
	}
	if layout == "ctx.Layout" && window == "ctx.Window" &&
		(event == "ctx.Event" || event == "nil") {
		out = "ctx"
	}
	if payload != "" {
		out = payload + ", " + out
	}
	return out, nil
}

// join prefixes the payload arguments, if any, to the context argument.
func join(payload, ctx string) string {
	if payload == "" {
		return ctx
	}
	return payload + ", " + ctx
}

// splitParams splits a compiler-reported parameter list on commas at
// depth zero, so "map[string]int, *Event, *Window" survives.
func splitParams(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '[', '(', '{':
			depth++
		case ']', ')', '}':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if strings.TrimSpace(s[start:]) != "" {
		out = append(out, strings.TrimSpace(s[start:]))
	}
	return out
}

// ParseBuildErrors turns `go build -gcflags=-e` output into fold specs,
// keyed by file. It recognises the "too many arguments in call to X /
// have (...) / want (...)" triple.
func ParseBuildErrors(out string) map[string][]FoldSpec {
	specs := map[string][]FoldSpec{}
	lines := strings.Split(out, "\n")
	for i, ln := range lines {
		surplus := strings.Contains(ln, "too many arguments in call to")
		// The second shape: same argument count, wrong type on the
		// trailing *Window. The column points at that argument.
		mismatch := strings.Contains(ln, "EventCtx value in argument to")
		if !surplus && !mismatch {
			continue
		}
		head := strings.SplitN(ln, ":", 4)
		if len(head) < 4 {
			continue
		}
		file := head[0]
		var line, col int
		if _, err := fmt.Sscanf(head[1]+" "+head[2], "%d %d", &line, &col); err != nil {
			continue
		}
		if mismatch {
			specs[file] = append(specs[file],
				FoldSpec{Line: line, Col: col, WrapWindow: true})
			continue
		}
		have := ""
		for j := i + 1; j < len(lines) && j <= i+2; j++ {
			t := strings.TrimSpace(lines[j])
			if rest, ok := strings.CutPrefix(t, "have ("); ok {
				have = strings.TrimSuffix(rest, ")")
				break
			}
		}
		if have == "" {
			continue
		}
		specs[file] = append(specs[file], FoldSpec{Line: line, Col: col, Have: have})
	}
	return specs
}
