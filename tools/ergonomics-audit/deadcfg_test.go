package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"
)

// deadCfgSrc runs the deadcfg rules over a set of in-memory files. The
// first file supplies the declarations (it stands in for gui/), every
// file supplies reads — the same split the real two-pass walk makes.
// Findings come back as "Cfg.Field" strings.
func deadCfgSrc(t *testing.T, decl string, readers ...string) (gating, deferred []string) {
	t.Helper()
	a := newDeadCfgAnalysis()

	parse := func(name, src string) (*token.FileSet, *ast.File) {
		t.Helper()
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name, src, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		return fset, f
	}

	fset, f := parse("gui/decl.go", decl)
	a.collectFields(fset, f, "gui/decl.go")
	// Declarations are collected before reads, because a selector only
	// matters when its name matches a declared field.
	a.collectReads(f)
	for _, src := range readers {
		_, rf := parse("gui/read.go", src)
		a.collectReads(rf)
	}

	found, def := a.findings()
	return deadCfgNames(found), deadCfgNames(def)
}

func deadCfgNames(fs []deadCfgField) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.cfg + "." + f.field
	}
	sort.Strings(out)
	return out
}

func joinNames(names []string) string { return strings.Join(names, " ") }

// The #503 shape: the field is read, but only to be written straight
// back into a field of the same name, so the value never arrives
// anywhere that acts on it. A plain reference count calls this used.
func TestDeadCfgForwardedFieldIsDead(t *testing.T) {
	t.Parallel()
	const decl = `package gui

type NumericStepCfg struct {
	Step        float64
	MouseWheel  bool
	ShowButtons bool
}

func normalize(cfg NumericStepCfg) NumericStepCfg {
	return NumericStepCfg{
		Step:        cfg.Step,
		MouseWheel:  cfg.MouseWheel,
		ShowButtons: cfg.ShowButtons,
	}
}

func render(cfg NumericStepCfg) float64 {
	if cfg.ShowButtons {
		return cfg.Step * 2
	}
	return 0
}
`
	gating, _ := deadCfgSrc(t, decl)
	if got := joinNames(gating); got != "NumericStepCfg.MouseWheel" {
		t.Errorf("findings = %q, want only NumericStepCfg.MouseWheel", got)
	}
}

// The same forward through assignment rather than a literal: the value
// is copied into a field of the same name and never acted on, so it is
// still dead. This guards the assignRole path the literal test above
// does not reach.
func TestDeadCfgAssignForwardIsDead(t *testing.T) {
	t.Parallel()
	const decl = `package gui

type NumericStepCfg struct {
	Step       float64
	MouseWheel bool
}

func normalize(cfg NumericStepCfg) NumericStepCfg {
	out := NumericStepCfg{Step: cfg.Step}
	out.MouseWheel = cfg.MouseWheel
	return out
}

func render(cfg NumericStepCfg) float64 { return cfg.Step }
`
	gating, _ := deadCfgSrc(t, decl)
	if got := joinNames(gating); got != "NumericStepCfg.MouseWheel" {
		t.Errorf("findings = %q, want only NumericStepCfg.MouseWheel", got)
	}
}

// A field nothing mentions at all is the easy case, and must still be
// caught once the forwarding logic is in play.
func TestDeadCfgUnreferencedFieldIsDead(t *testing.T) {
	t.Parallel()
	const decl = `package gui

type ThemeCfg struct {
	FillBorder bool
	Radius     float32
}

func use(cfg ThemeCfg) float32 { return cfg.Radius }
`
	gating, _ := deadCfgSrc(t, decl)
	if got := joinNames(gating); got != "ThemeCfg.FillBorder" {
		t.Errorf("findings = %q, want only ThemeCfg.FillBorder", got)
	}
}

// A copy into a DIFFERENTLY named field is a real use, not a forward:
// the value lands in something that goes on to be read. Treating these
// as forwards reported six live go-gui fields as dead, so this case is
// the regression guard on the same-name restriction.
func TestDeadCfgCopyToOtherNameIsConsumption(t *testing.T) {
	t.Parallel()
	const decl = `package gui

type ThemeCfg struct {
	MonoFontFamily string
	InitialValue   string
	PreTextChange  func()
}

type spec struct {
	Family        string
	initialValue  string
	preTextChange func()
}

func build(cfg ThemeCfg) spec {
	var s spec
	s.Family = cfg.MonoFontFamily
	s.initialValue = cfg.InitialValue
	return spec{
		preTextChange: cfg.PreTextChange,
		Family:        s.Family,
		initialValue:  s.initialValue,
	}
}
`
	gating, _ := deadCfgSrc(t, decl)
	if len(gating) != 0 {
		t.Errorf("findings = %q, want none: a copy into another name is a use",
			joinNames(gating))
	}
}

// Writing a field is not reading it. A field the whole world assigns
// and no one consumes is exactly the bug, so assignment must not clear
// it.
func TestDeadCfgWriteIsNotARead(t *testing.T) {
	t.Parallel()
	const decl = `package gui

type ThemeCfg struct {
	FillBorder bool
}

func preset(cfg *ThemeCfg) {
	cfg.FillBorder = true
}
`
	gating, _ := deadCfgSrc(t, decl)
	if got := joinNames(gating); got != "ThemeCfg.FillBorder" {
		t.Errorf("findings = %q, want ThemeCfg.FillBorder", got)
	}
}

// A read in another file clears the field: the analysis is
// module-wide, not per-file.
func TestDeadCfgReadInAnotherFileCounts(t *testing.T) {
	t.Parallel()
	const decl = `package gui

type ThemeCfg struct {
	FillBorder bool
}
`
	const reader = `package gui

func paint(cfg ThemeCfg) bool {
	return cfg.FillBorder
}
`
	gating, _ := deadCfgSrc(t, decl, reader)
	if len(gating) != 0 {
		t.Errorf("findings = %q, want none: the field is read elsewhere",
			joinNames(gating))
	}
}

// A marked field is reported as deferred rather than gating, and the
// reason after the marker is carried into the report.
func TestDeadCfgMarkerDefers(t *testing.T) {
	t.Parallel()
	const decl = `package gui

type ThemeCfg struct {
	// ergonomics-audit:deadcfg-keep — read by go-charts
	FillBorder bool
	Radius     float32
}
`
	gating, deferred := deadCfgSrc(t, decl)
	if got := joinNames(gating); got != "ThemeCfg.Radius" {
		t.Errorf("gating = %q, want only ThemeCfg.Radius", got)
	}
	if got := joinNames(deferred); got != "ThemeCfg.FillBorder" {
		t.Errorf("deferred = %q, want ThemeCfg.FillBorder", got)
	}
}

// Unexported fields are not caller-facing, so they are not the mode's
// business, and an embedded field is audited where it is declared.
func TestDeadCfgSkipsUnexportedAndEmbedded(t *testing.T) {
	t.Parallel()
	const decl = `package gui

type A11YCfg struct {
	A11YLabel string
}

type ButtonCfg struct {
	A11YCfg
	hidden bool
	Label  string
}

func use(cfg ButtonCfg) string { return cfg.Label + cfg.A11YLabel }
`
	gating, _ := deadCfgSrc(t, decl)
	if len(gating) != 0 {
		t.Errorf("findings = %q, want none", joinNames(gating))
	}
}

// The reason text is what makes a marker reviewable, so it must survive
// the punctuation the repo's comment style puts in front of it.
func TestDeadCfgReasonParsed(t *testing.T) {
	t.Parallel()
	const decl = `package gui

type ThemeCfg struct {
	// ergonomics-audit:deadcfg-keep — read by go-charts
	FillBorder bool
}
`
	a := newDeadCfgAnalysis()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "gui/decl.go", decl, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	a.collectFields(fset, f, "gui/decl.go")
	_, deferred := a.findings()
	if len(deferred) != 1 {
		t.Fatalf("deferred = %d field(s), want 1", len(deferred))
	}
	if got := deferred[0].reason; got != "read by go-charts" {
		t.Errorf("reason = %q, want %q", got, "read by go-charts")
	}
}
