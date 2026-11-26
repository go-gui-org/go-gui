//go:build ios && cgo

package keyring

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// errSecMissingEntitlement is the status a process gets when it has no
// keychain access group, as a bare test binary on a device can.
const errSecMissingEntitlement = -34018

// TestIOSSaveSetsAccessible checks the iOS-only attribute that the cgo
// form set and the purego form must keep (#1342): a new item is
// kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly, so a background task
// can read it and a backup restore does not move it to another device.
//
// Run it on a booted simulator:
//
//	go test -c -o /tmp/keyring.test ./gui/backend/internal/keyring   (GOOS=ios, cgo on)
//	xcrun simctl spawn booted /tmp/keyring.test -test.v
func TestIOSSaveSetsAccessible(t *testing.T) {
	const service = "org.go-gui.keyring-test"
	key := fmt.Sprintf("it-ios-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = Delete(service, key) })
	if err := Save(service, key, []byte("secret")); err != nil {
		if strings.HasSuffix(err.Error(), fmt.Sprintf("(OSStatus %d)", errSecMissingEntitlement)) {
			t.Skipf("no keychain access group: %v", err)
		}
		t.Fatal(err)
	}
	got, err := Load(service, key)
	if err != nil || string(got) != "secret" {
		t.Fatalf("Load = %q, %v; want \"secret\"", got, err)
	}

	// Read the item's attributes back and compare kSecAttrAccessible.
	var getValue func(dict, key uintptr) uintptr
	cf := darwinffi.Open(darwinffi.PathCoreFoundation)
	purego.RegisterLibFunc(&getValue, cf, "CFDictionaryGetValue")
	lib := darwinffi.Open(darwinffi.PathSecurity)
	q, err := query(service, key)
	if err != nil {
		t.Fatal(err)
	}
	defer darwinffi.CFRelease(q)
	darwinffi.CFDictSet(q, darwinffi.Data(lib, "kSecReturnAttributes"), darwinffi.CFTrue())
	darwinffi.CFDictSet(q, sec.matchLimit, sec.matchLimitOne)
	res := new(uintptr)
	if st := sec.copyMatching(q, res); st != errSecSuccess {
		t.Fatalf("SecItemCopyMatching attributes: OSStatus %d", st)
	}
	defer darwinffi.CFRelease(*res)
	// Both values are CFStrings ("cku"): compare their text.
	have := darwinffi.GoString(getValue(*res, sec.attrAccessible))
	want := darwinffi.GoString(sec.accessibleAfterFirstUnlockThisDeviceOnly)
	if have != want {
		t.Errorf("kSecAttrAccessible = %q, want %q (AfterFirstUnlockThisDeviceOnly)", have, want)
	}
}
