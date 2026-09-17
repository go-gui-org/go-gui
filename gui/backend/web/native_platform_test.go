//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"
)

// --- dotExtensions ---

func TestDotExtensions(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"png"}, ".png"},
		{[]string{"png", "jpg", "gif"}, ".png,.jpg,.gif"},
		{[]string{"tar+gz"}, ".tar+gz"},
	}
	for _, tt := range tests {
		got := dotExtensions(tt.in)
		if got != tt.want {
			t.Errorf("dotExtensions(%v) = %q, want %q",
				tt.in, got, tt.want)
		}
	}
}

// --- validateOpenURI ---

func TestValidateOpenURI(t *testing.T) {
	valid := []string{
		"http://example.com",
		"https://example.com/path?q=1",
		"HTTPS://example.com",
		"mailto:user@example.com",
		"MAILTO:user@example.com",
	}
	for _, raw := range valid {
		if err := validateOpenURI(raw); err != nil {
			t.Errorf("validateOpenURI(%q) = %v, want nil", raw, err)
		}
	}
	invalid := []string{
		"",
		"ftp://example.com",
		"javascript:alert(1)",
		"file:///etc/passwd",
		"data:text/html,x",
		"http://example.com\nrm -rf /",
		"http://example.com\x00",
	}
	for _, raw := range invalid {
		if err := validateOpenURI(raw); err == nil {
			t.Errorf("validateOpenURI(%q) = nil, want error", raw)
		}
	}
	long := "https://example.com/" + string(make([]byte, maxOpenURILen))
	if err := validateOpenURI(long); err == nil {
		t.Error("validateOpenURI(long) = nil, want length error")
	}
}

// --- jsObject ---

func TestJsObject(t *testing.T) {
	obj := jsObject()
	if obj.Type() != js.TypeObject {
		t.Fatalf("jsObject() type = %v, want Object", obj.Type())
	}
}
