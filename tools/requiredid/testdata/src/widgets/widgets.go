package widgets

type WidgetCfg struct {
	ID   string `gui:"required"`
	Name string
}

type NoReqCfg struct {
	ID string
}

// FocusCfg has an untagged ID: the Focusable rule must fire on its own,
// independent of gui:"required".
type FocusCfg struct {
	ID        string
	Focusable bool
}

// NoIDCfg is focusable but has no ID field to set.
type NoIDCfg struct {
	Focusable bool
}

// FlipCfg mirrors the focusable-by-default Cfgs (Input, Select, …):
// no Focusable field, FocusDisabled opt-out. The analyzer keys on the
// literal `Focusable` field, so these literals must never diagnose.
type FlipCfg struct {
	ID            string
	FocusDisabled bool
}

// ScopedCfg mirrors the nine focusable-by-default Cfgs after phase 1:
// ID is required, but only for a control that joins focus traversal.
type ScopedCfg struct {
	ID            string `gui:"required,focus"`
	FocusDisabled bool
}

// UnscopedCfg carries the plain tag alongside a FocusDisabled field,
// pinning that the opt-out applies only to the `focus` option.
type UnscopedCfg struct {
	ID            string `gui:"required"`
	FocusDisabled bool
}

type S struct{}

func (S) Widget(_ WidgetCfg) {}

func Widget(_ WidgetCfg)     {}
func helper(_ WidgetCfg)     {}
func useN(_ NoReqCfg)        {}
func Focus(_ FocusCfg)       {}
func NoID(_ NoIDCfg)         {}
func Flip(_ FlipCfg)         {}
func Scoped(_ ScopedCfg)     {}
func Unscoped(_ UnscopedCfg) {}

func good() {
	Widget(WidgetCfg{ID: "ok", Name: "x"})
}

func missingID() {
	Widget(WidgetCfg{Name: "x"}) // want `WidgetCfg.ID is required`
}

func emptyID() {
	Widget(WidgetCfg{ID: "", Name: "x"}) // want `WidgetCfg.ID is required`
}

func methodCall() {
	var s S
	s.Widget(WidgetCfg{Name: "x"}) // want `WidgetCfg.ID is required`
}

func noTagIgnored() {
	useN(NoReqCfg{})
}

func ignoredByDirective() {
	Widget(WidgetCfg{Name: "x"}) //requiredid:ignore
}

func helperArgSkipped() {
	helper(WidgetCfg{Name: "x"})
}

func returnedSkipped() WidgetCfg {
	return WidgetCfg{Name: "x"}
}

func varAssignSkipped() {
	_ = WidgetCfg{Name: "x"}
}

func focusableNoID() {
	Focus(FocusCfg{Focusable: true}) // want `FocusCfg sets Focusable: true without an ID`
}

func focusableEmptyID() {
	Focus(FocusCfg{ID: "", Focusable: true}) // want `FocusCfg sets Focusable: true without an ID`
}

func focusableWithID() {
	Focus(FocusCfg{ID: "ok", Focusable: true})
}

// A non-literal ID cannot be proven empty; stay quiet.
func focusableComputedID() {
	id := "x"
	Focus(FocusCfg{ID: id, Focusable: true})
}

// Focusable is not statically true; stay quiet.
func focusableComputedFlag() {
	on := true
	Focus(FocusCfg{Focusable: on})
}

func notFocusableNoID() {
	Focus(FocusCfg{})
}

// --- gui:"required,focus" ---

func scopedMissingID() {
	Scoped(ScopedCfg{}) // want `ScopedCfg.ID is required`
}

func scopedEmptyID() {
	Scoped(ScopedCfg{ID: ""}) // want `ScopedCfg.ID is required`
}

func scopedWithID() {
	Scoped(ScopedCfg{ID: "ok"})
}

// FocusDisabled satisfies the focus-scoped requirement: the control
// never joins the tab order, so it has no identity to name.
func scopedOptedOut() {
	Scoped(ScopedCfg{FocusDisabled: true})
}

// The opt-out is scoped to the `focus` option. A plain gui:"required"
// field stays required even when the literal sets FocusDisabled: its
// state is keyed by ID whether or not the control takes focus.
func unscopedNotExemptedByFocusDisabled() {
	Unscoped(UnscopedCfg{FocusDisabled: true}) // want `UnscopedCfg.ID is required`
}

// No ID field exists to set, so the diagnostic would be unfixable.
func focusableCfgWithoutIDField() {
	NoID(NoIDCfg{Focusable: true})
}

func focusableIgnoredByDirective() {
	Focus(FocusCfg{Focusable: true}) //requiredid:ignore
}

// Focusable-by-default Cfgs never set Focusable, so the rule stays
// silent — with or without an ID (no-ID is a valid inert choice), and
// regardless of FocusDisabled.
func flippedCfgSilent() {
	Flip(FlipCfg{})
	Flip(FlipCfg{ID: "ok"})
	Flip(FlipCfg{FocusDisabled: true})
}
