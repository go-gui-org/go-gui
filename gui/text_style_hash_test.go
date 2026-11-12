package gui

import (
	"reflect"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-glyph"
)

// textStyleShapingHash keys an anonymous text's block by its shaping
// style. Two styles textLayoutStyleEqual calls different must hash
// differently, or two Texts would share a block and reset it to each
// other's style every frame; two it calls equal must hash alike, or a
// recolour would miss the cache. Each field of TextStyle is set in turn,
// so a field added later that shapes fails here until it is hashed.
func TestTextStyleShapingHashCoversEquality(t *testing.T) {
	var base TextStyle
	typ := reflect.TypeOf(base)
	for i := range typ.NumField() {
		mod := base
		f := reflect.ValueOf(&mod).Elem().Field(i)
		// Unexported fields are set through their address.
		f = reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
		switch f.Kind() {
		case reflect.String:
			f.SetString("x")
		case reflect.Float32, reflect.Float64:
			f.SetFloat(1.5)
		case reflect.Bool:
			f.SetBool(true)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			f.SetInt(1)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			f.SetUint(1)
		case reflect.Pointer:
			f.Set(reflect.New(f.Type().Elem()))
		case reflect.Struct:
			if f.Type() != reflect.TypeOf(Color{}) {
				t.Fatalf("field %s: no test value for %s", typ.Field(i).Name, f.Type())
			}
			f.Set(reflect.ValueOf(RGB(1, 2, 3)))
		default:
			t.Fatalf("field %s: no test value for kind %s", typ.Field(i).Name, f.Kind())
		}
		equal := textLayoutStyleEqual(base, mod)
		sameHash := textStyleShapingHash(base) == textStyleShapingHash(mod)
		if equal != sameHash {
			t.Errorf("field %s: styles equal %v, hashes equal %v",
				typ.Field(i).Name, equal, sameHash)
		}
	}
}

// Features shape by value, as textLayoutStyleEqual compares them: two
// sets with the same content hash alike, a different value does not.
func TestTextStyleShapingHashFeaturesByValue(t *testing.T) {
	feat := func(v int) *glyph.FontFeatures {
		return &glyph.FontFeatures{OpenTypeFeatures: []glyph.FontFeature{{Tag: "liga", Value: v}}}
	}
	a := TextStyle{Features: feat(0)}
	b := TextStyle{Features: feat(0)}
	c := TextStyle{Features: feat(1)}
	if textStyleShapingHash(a) != textStyleShapingHash(b) {
		t.Error("equal features at two addresses hash differently")
	}
	if textStyleShapingHash(a) == textStyleShapingHash(c) {
		t.Error("different feature values hash alike")
	}
}
