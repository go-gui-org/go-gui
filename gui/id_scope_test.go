package gui

import (
	"math"
	"strings"
	"testing"
)

func TestScopeIDCompose(t *testing.T) {
	tests := []struct {
		name  string
		owner string
		parts []string
		want  string
	}{
		{"no parts", "grid", nil, "grid"},
		{"one part", "grid", []string{"header"}, "grid:header"},
		{"two parts", "grid", []string{"header", "name"}, "grid:header:name"},
		{
			"three parts", "grid",
			[]string{"cell", "row-7", "name"}, "grid:cell:row-7:name",
		},
		{
			"four parts", "grid",
			[]string{"a", "b", "c", "d"}, "grid:a:b:c:d",
		},
		{"empty owner", "", []string{"header"}, "header"},
		{"empty owner two parts", "", []string{"a", "b"}, "a:b"},
		{"empty part", "grid", []string{""}, "grid"},
		{"empty middle part", "grid", []string{"a", "", "b"}, "grid:a:b"},
		{"all empty", "", []string{"", ""}, ""},
		{"owner already composed", "grid:header:name",
			[]string{"resize"}, "grid:header:name:resize"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScopeID(tc.owner, tc.parts...); got != tc.want {
				t.Errorf("ScopeID(%q, %q) = %q, want %q",
					tc.owner, tc.parts, got, tc.want)
			}
		})
	}
}

// Nesting must be associative: composing in one call and composing the
// same segments in stages have to agree, or a widget that hoists a
// prefix out of a loop silently changes its children's identity.
func TestScopeIDNestsAssociatively(t *testing.T) {
	once := ScopeID("grid", "header", "name", "resize")
	staged := ScopeID(ScopeID(ScopeID("grid", "header"), "name"), "resize")
	if once != staged {
		t.Errorf("staged composition = %q, single call = %q", staged, once)
	}
}

func TestScopeIDN(t *testing.T) {
	tests := []struct {
		name  string
		owner string
		part  string
		n     int
		want  string
	}{
		{"owner part index", "cal", "day", 7, "cal:day:7"},
		{"zero", "cal", "day", 0, "cal:day:0"},
		{"no part", "group", "", 3, "group:3"},
		{"no owner", "", "day", 3, "day:3"},
		{"bare index", "", "", 3, "3"},
		{"negative", "cal", "day", -12, "cal:day:-12"},
		{"large", "cal", "day", 1 << 40, "cal:day:1099511627776"},
		{"composed owner", "grid:rows", "row", 42, "grid:rows:row:42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScopeIDN(tc.owner, tc.part, tc.n)
			if got != tc.want {
				t.Errorf("ScopeIDN(%q, %q, %d) = %q, want %q",
					tc.owner, tc.part, tc.n, got, tc.want)
			}
		})
	}
}

// The one-allocation contract is load-bearing: these run in the
// per-frame view phase, once per row and once per cell. A future edit
// that lets parts escape, or that drops the exact Grow, shows up here.
func TestScopeIDAllocs(t *testing.T) {
	cases := []struct {
		name string
		fn   func()
	}{
		{"one part", func() { sinkID = ScopeID("grid", "header") }},
		{"two parts", func() { sinkID = ScopeID("grid", "header", "name") }},
		{"three parts", func() {
			sinkID = ScopeID("grid", "cell", "row-7", "name")
		}},
		{"four parts", func() {
			sinkID = ScopeID("grid", "cell", "row-7", "name", "edit")
		}},
		{"scope idn", func() { sinkID = ScopeIDN("cal", "day", 12345) }},
		{"scope idn no part", func() { sinkID = ScopeIDN("group", "", 7) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := testing.AllocsPerRun(100, tc.fn); got != 1 {
				t.Errorf("got %v allocs, want 1", got)
			}
		})
	}
}

// sinkID defeats dead-store elimination in the alloc benchmarks above.
var sinkID string

// ScopeIDN renders digits into a local [20]byte. math.MinInt64 is
// "-9223372036854775808" — exactly 20 bytes, the tightest case there
// is. If that array is ever narrowed, AppendInt silently grows onto the
// heap and the one-allocation contract breaks without any test failing
// on the returned string. Pin both the value and the alloc count.
func TestScopeIDNIntExtremes(t *testing.T) {
	cases := []struct {
		name string
		n    int
		want string
	}{
		{"min int64", math.MinInt64, "cal:day:-9223372036854775808"},
		{"max int64", math.MaxInt64, "cal:day:9223372036854775807"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ScopeIDN("cal", "day", tc.n); got != tc.want {
				t.Errorf("ScopeIDN(cal, day, %d) = %q, want %q",
					tc.n, got, tc.want)
			}
			allocs := testing.AllocsPerRun(100, func() {
				sinkID = ScopeIDN("cal", "day", tc.n)
			})
			if allocs != 1 {
				t.Errorf("got %v allocs, want 1", allocs)
			}
		})
	}
}

// The general (3+ parts) path sizes its buffer from a segment count
// that starts at zero when the owner is empty. Getting that wrong
// over-allocates and costs a second alloc rather than producing a
// wrong string, so assert the alloc count alongside the value.
func TestScopeIDEmptyOwnerManyParts(t *testing.T) {
	if got := ScopeID("", "a", "b", "c"); got != "a:b:c" {
		t.Errorf("ScopeID(\"\", a, b, c) = %q, want %q", got, "a:b:c")
	}
	allocs := testing.AllocsPerRun(100, func() {
		sinkID = ScopeID("", "a", "b", "c")
	})
	if allocs != 1 {
		t.Errorf("got %v allocs, want 1", allocs)
	}
}

// A composed ID must remain splittable, because the datagrid recovers a
// column ID by trimming a known prefix off a header shape's ID.
func TestScopeIDPrefixIsRecoverable(t *testing.T) {
	prefix := ScopeID("grid", "header") + IDSep
	id := ScopeID("grid", "header", "first-name")
	rest, ok := strings.CutPrefix(id, prefix)
	if !ok || rest != "first-name" {
		t.Errorf("CutPrefix(%q, %q) = %q, %v", id, prefix, rest, ok)
	}
}
