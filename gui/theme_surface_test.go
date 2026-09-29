package gui

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestThemeSurface pins the exported fields of Theme, ThemeCfg and
// TextStyle to testdata/theme_surface.golden (issue #846). Apps read
// the text roles from Theme, custom themes build through ThemeCfg, and
// both read TextStyle fields, so a rename on any of the three breaks
// callers outside this repo.
//
// The golden is the contract in list form. The test only keeps it in
// step with the code; scripts/theme-surface-check.sh is the gate. It
// fails a branch whose golden diff removes a line unless CHANGELOG.md
// carries a **BREAKING: entry under Unreleased. So re-recording with
// -update does not hide a rename: the removed line is still in the
// diff the script reads.
//
// Lines are sorted, not in declaration order, so moving a field inside
// a struct is not seen as a removal. Adding a field only adds lines,
// which the gate lets through: adding a role is free.
//
//	go test ./gui/ -run TestThemeSurface -update
func TestThemeSurface(t *testing.T) {
	var lines []string
	for _, typ := range []reflect.Type{
		reflect.TypeFor[Theme](),
		reflect.TypeFor[ThemeCfg](),
		reflect.TypeFor[TextStyle](),
	} {
		for f := range typ.Fields() {
			if !f.IsExported() {
				continue
			}
			lines = append(lines, typ.Name()+"."+f.Name+" "+f.Type.String())
		}
	}
	sort.Strings(lines)
	checkGolden(t, "theme_surface", strings.Join(lines, "\n")+"\n")
}
