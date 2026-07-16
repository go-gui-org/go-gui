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

type S struct{}

func (S) Widget(_ WidgetCfg) {}

func Widget(_ WidgetCfg) {}
func helper(_ WidgetCfg) {}
func useN(_ NoReqCfg)    {}
func Focus(_ FocusCfg)   {}
func NoID(_ NoIDCfg)     {}
func Flip(_ FlipCfg)     {}

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
