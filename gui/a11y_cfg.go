package gui

// A11YCfg carries the accessible name and description for a widget.
// Embedded in every Cfg whose widget reaches the accessibility tree.
//
// The two fields are a perfect co-occurrence group across the widget
// Cfgs — every Cfg that carries one carries the other — so they are
// declared once here rather than repeated per widget. Promotion keeps
// every read site (cfg.A11YLabel) spelled exactly as before; only
// composite literals that *set* a field name the embed:
//
//	gui.ButtonCfg{ID: "save", A11YCfg: gui.A11YCfg{A11YLabel: "Save"}}
type A11YCfg struct {
	// A11YLabel is the accessible name announced by screen readers.
	// Widgets that can derive a sensible name from their own content
	// fall back to it via a11yLabel when this is empty.
	A11YLabel string

	// A11YDescription is supplementary detail announced after the
	// label. Empty means no description node is emitted.
	A11YDescription string
}
