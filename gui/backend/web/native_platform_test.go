//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
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

// --- webKeyboardAttrs (issue #770) ---

func TestWebKeyboardAttrs(t *testing.T) {
	tests := []struct {
		kind     gui.KeyboardKind
		secure   bool
		wantMode string
		wantType string
	}{
		{gui.KeyboardText, false, "text", "text"},
		{gui.KeyboardNumber, false, "numeric", "text"},
		{gui.KeyboardDecimal, false, "decimal", "text"},
		{gui.KeyboardPhone, false, "tel", "text"},
		{gui.KeyboardEmail, false, "email", "text"},
		{gui.KeyboardURL, false, "url", "text"},
		{gui.KeyboardNone, false, "none", "text"},
		{gui.KeyboardText, true, "text", "password"},
		{gui.KeyboardNumber, true, "numeric", "password"},
		{gui.KeyboardKind(200), false, "text", "text"},
	}
	for _, tt := range tests {
		mode, typ := webKeyboardAttrs(tt.kind, tt.secure)
		if mode != tt.wantMode || typ != tt.wantType {
			t.Errorf("webKeyboardAttrs(%v, %v) = (%q, %q), want (%q, %q)",
				tt.kind, tt.secure, mode, typ, tt.wantMode, tt.wantType)
		}
	}
}

// --- settings (localStorage) ---

// fakeStorage builds a JS object with the Storage getItem/setItem
// methods over a plain JS map, since Node has no localStorage.
func fakeStorage(t *testing.T) js.Value {
	t.Helper()
	newFn := js.Global().Get("Function")
	obj := js.Global().Get("Object").New()
	obj.Set("m", js.Global().Get("Object").New())
	obj.Set("getItem", newFn.New("k", "return (k in this.m) ? this.m[k] : null;"))
	obj.Set("setItem", newFn.New("k", "v", "this.m[k] = String(v);"))
	return obj
}

func TestStorageRoundTrip(t *testing.T) {
	s := fakeStorage(t)
	get := func() js.Value { return s }
	data, err := storageLoad(get, "org.example.app")
	if err != nil || data != nil {
		t.Fatalf("empty load = (%q, %v), want (nil, nil)", data, err)
	}
	if err = storageSave(get, "org.example.app", []byte(`{"A":1}`)); err != nil {
		t.Fatal(err)
	}
	if got := s.Get("m").Get(settingsKeyPrefix + "org.example.app").String(); got != `{"A":1}` {
		t.Fatalf("stored %q under the prefixed key", got)
	}
	data, err = storageLoad(get, "org.example.app")
	if err != nil || string(data) != `{"A":1}` {
		t.Fatalf("load = (%q, %v)", data, err)
	}
}

func TestStorageThrowIsError(t *testing.T) {
	s := fakeStorage(t)
	thrower := js.Global().Get("Function").New("throw new Error('QuotaExceededError');")
	s.Set("setItem", thrower)
	s.Set("getItem", thrower)
	get := func() js.Value { return s }
	if err := storageSave(get, "a", []byte("x")); err == nil {
		t.Fatal("save: want an error when setItem throws")
	}
	if _, err := storageLoad(get, "a"); err == nil {
		t.Fatal("load: want an error when getItem throws")
	}
}

func TestStorageUnavailableIsError(t *testing.T) {
	get := js.Undefined
	if _, err := storageLoad(get, "a"); err == nil {
		t.Fatal("load: want an error with no localStorage")
	}
	if err := storageSave(get, "a", nil); err == nil {
		t.Fatal("save: want an error with no localStorage")
	}
}

// The settings hook in gui is unexported and found by a type assertion,
// so a changed method set here would silently drop web back to the file
// store, which fails on js. This assertion makes that a compile error.
var _ interface {
	SettingsLoad(appID string) ([]byte, error)
	SettingsSave(appID string, data []byte) error
} = (*nativePlatform)(nil)
