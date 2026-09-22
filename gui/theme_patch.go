package gui

import (
	"fmt"
	"reflect"
)

// Per-widget theme overrides (issue #754).
//
// An app cannot say "all buttons use radius 2" with ThemeCfg tokens:
// tokens affect every widget, and a Cfg value on each call site breaks
// the rule that a call site never states a visual value. The patch is
// the middle case: it lives in the theme, like every other override.
//
// A patch holds geometry only (padding, border, radius). Colors stay
// with WithColors. Each field is unset by default: Opt for the two
// floats (zero is a valid choice there), plain Padding elsewhere
// (it flags itself). The patch applies once, at build time, to the
// private styles ThemeMaker owns. Frames read the styles, never the
// patch, so the steady state costs nothing.
//
// Patches ride the ext slot, so every rebuild path carries them with
// no extra field: WithPadding, WithBorders and AdjustFontSize assign
// ext across the rebuild and then re-apply the stored patches. A value
// stored here is immutable like any ext value: it holds no pointer or
// alias to caller memory.

// ButtonPatch overrides the geometry of every button variant.
// The four variant styles share one geometry, so one patch moves all
// of them together.
//
// exportaudit:keep — per-widget override surface (issue #754).
type ButtonPatch struct {
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
}

// InputPatch overrides the geometry of text inputs.
//
// exportaudit:keep — per-widget override surface (issue #754).
type InputPatch struct {
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
}

// SelectPatch overrides the geometry of select dropdowns.
//
// exportaudit:keep — per-widget override surface (issue #754).
type SelectPatch struct {
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
}

// DialogPatch overrides the geometry of dialogs.
//
// exportaudit:keep — per-widget override surface (issue #754).
type DialogPatch struct {
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
}

// ContainerPatch overrides the geometry of containers.
//
// exportaudit:keep — per-widget override surface (issue #754).
type ContainerPatch struct {
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
}

// With returns t carrying patch for one widget class. The result has
// a fresh theme id. A patch of the same type replaces the stored one.
// A patch of another type coexists with it. Any other type panics:
// an ignored patch would look applied while it changed nothing.
//
// A NaN, infinite or negative length in the patch becomes 0 before it
// is stored, so a bad computed value cannot reach layout or render.
//
// exportaudit:keep — per-widget override surface (issue #754).
func (t Theme) With(patch any) Theme {
	// Each case sanitizes, stores (WithExt stamps the fresh id and
	// clones the map, so the parent keeps its own patches), then
	// applies. The stored value is the sanitized one, so rebuild
	// paths that re-apply from the slot see the same clean values.
	switch p := patch.(type) {
	case ButtonPatch:
		p = ButtonPatch(patchGeometry(p).sanitized())
		t = WithExt(t, p)
		applyButtonPatch(&t, p)
	case InputPatch:
		p = InputPatch(patchGeometry(p).sanitized())
		t = WithExt(t, p)
		applyInputPatch(&t, p)
	case SelectPatch:
		p = SelectPatch(patchGeometry(p).sanitized())
		t = WithExt(t, p)
		applySelectPatch(&t, p)
	case DialogPatch:
		p = DialogPatch(patchGeometry(p).sanitized())
		t = WithExt(t, p)
		applyDialogPatch(&t, p)
	case ContainerPatch:
		p = ContainerPatch(patchGeometry(p).sanitized())
		t = WithExt(t, p)
		applyContainerPatch(&t, p)
	default:
		// %T names the wrong type, so the panic points at the misspelled
		// override instead of only saying that one exists.
		panic(fmt.Sprintf("gui: Theme.With: unknown patch type %T", patch))
	}
	return t
}

// patchFor reads the stored patch of type T without copying the
// theme. Ext would copy ~12 KB per call; this reads the map only.
func patchFor[T any](t *Theme) (T, bool) {
	var zero T
	if len(t.ext) == 0 {
		return zero, false
	}
	stored, found := t.ext[reflect.TypeFor[T]()]
	if !found {
		return zero, false
	}
	typed, matched := stored.(T)
	if !matched {
		return zero, false
	}
	return typed, true
}

// applyWidgetPatches re-applies every stored patch to a rebuilt
// theme. Rebuild paths (WithPadding, WithBorders, AdjustFontSize)
// construct fresh styles from the Cfg and then carry ext across, so
// the map arrives but the styles do not reflect it until this runs.
func applyWidgetPatches(t *Theme) {
	if len(t.ext) == 0 {
		return
	}
	if p, ok := patchFor[ButtonPatch](t); ok {
		applyButtonPatch(t, p)
	}
	if p, ok := patchFor[InputPatch](t); ok {
		applyInputPatch(t, p)
	}
	if p, ok := patchFor[SelectPatch](t); ok {
		applySelectPatch(t, p)
	}
	if p, ok := patchFor[DialogPatch](t); ok {
		applyDialogPatch(t, p)
	}
	if p, ok := patchFor[ContainerPatch](t); ok {
		applyContainerPatch(t, p)
	}
}

// patchGeometry is the field set every patch type shares. The five
// exported types have identical fields, so each converts to this one
// and a single sanitize and apply serve all of them. A field added to
// one patch type and not the others breaks the conversion at compile
// time, which is the wanted failure.
type patchGeometry struct {
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
}

// sanitized returns g with every set length made finite and
// non-negative. Unset fields stay unset.
func (g patchGeometry) sanitized() patchGeometry {
	if g.Padding.IsSet() {
		g.Padding = NewPadding(patchLength(g.Padding.Top),
			patchLength(g.Padding.Right), patchLength(g.Padding.Bottom),
			patchLength(g.Padding.Left))
	}
	if v, ok := g.SizeBorder.Value(); ok {
		g.SizeBorder = SomeF(patchLength(v))
	}
	if v, ok := g.Radius.Value(); ok {
		g.Radius = SomeF(patchLength(v))
	}
	return g
}

// patchLength maps a NaN, infinite or negative length to 0.
func patchLength(v float32) float32 {
	if !f32IsFinite(v) || v < 0 {
		return 0
	}
	return v
}

// apply writes each set field of g into the style fields it points at.
func (g patchGeometry) apply(padding *Padding, sizeBorder, radius *float32) {
	if g.Padding.IsSet() {
		*padding = g.Padding
	}
	if v, ok := g.SizeBorder.Value(); ok {
		*sizeBorder = v
	}
	if v, ok := g.Radius.Value(); ok {
		*radius = v
	}
}

// applyButtonPatch moves all four button variants together: they share
// one geometry.
func applyButtonPatch(t *Theme, p ButtonPatch) {
	g := patchGeometry(p)
	g.apply(&t.buttonStyle.Padding, &t.buttonStyle.SizeBorder, &t.buttonStyle.Radius)
	g.apply(&t.buttonStylePrimary.Padding, &t.buttonStylePrimary.SizeBorder,
		&t.buttonStylePrimary.Radius)
	g.apply(&t.buttonStyleGhost.Padding, &t.buttonStyleGhost.SizeBorder,
		&t.buttonStyleGhost.Radius)
	g.apply(&t.buttonStyleDanger.Padding, &t.buttonStyleDanger.SizeBorder,
		&t.buttonStyleDanger.Radius)
}

func applyInputPatch(t *Theme, p InputPatch) {
	patchGeometry(p).apply(&t.inputStyle.Padding, &t.inputStyle.SizeBorder,
		&t.inputStyle.Radius)
}

func applySelectPatch(t *Theme, p SelectPatch) {
	patchGeometry(p).apply(&t.selectStyle.Padding, &t.selectStyle.SizeBorder,
		&t.selectStyle.Radius)
}

func applyDialogPatch(t *Theme, p DialogPatch) {
	patchGeometry(p).apply(&t.dialogStyle.Padding, &t.dialogStyle.SizeBorder,
		&t.dialogStyle.Radius)
}

func applyContainerPatch(t *Theme, p ContainerPatch) {
	patchGeometry(p).apply(&t.containerStyle.Padding, &t.containerStyle.SizeBorder,
		&t.containerStyle.Radius)
}
