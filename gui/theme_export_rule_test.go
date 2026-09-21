package gui

import (
	"reflect"
	"strings"
	"testing"
)

// TestThemeStyleFieldsPrivate pins the export rule from issue #735:
// per-widget styles are private and derived only. A style is
// customized through ThemeCfg tokens, never by field assignment.
// ScrollbarStyle stays public while external layout code reads the
// scrollbar gutter size; TextStyle fields belong to the text roles
// and ladder owned by issue #734.
func TestThemeStyleFieldsPrivate(t *testing.T) {
	public := map[string]bool{
		"ScrollbarStyle": true,
	}
	typ := reflect.TypeFor[Theme]()
	for f := range typ.Fields() {
		typeName := f.Type.Name()
		if typeName == "TextStyle" {
			continue // #734 owns the text grid and roles.
		}
		if !strings.HasSuffix(strings.ToLower(typeName), "style") {
			continue
		}
		if f.PkgPath == "" && !public[f.Name] {
			t.Errorf("Theme.%s is an exported per-widget style; "+
				"keep widget styles private (#735)", f.Name)
		}
	}
}
