//go:build windows

package nativehost

import (
	"encoding/xml"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestDevelopmentNotificationIdentity(t *testing.T) {
	executable := `C:\Temp\go-build123\b001\exe\showcase.exe`
	otherRun := `D:\CustomTemp\go-build456\b099\exe\showcase.exe`
	first := developmentNotificationIdentity(executable, `C:\Projects\gui`, "example.org/gui/examples/showcase")
	second := developmentNotificationIdentity(otherRun, `c:\projects\GUI`, "example.org/gui/examples/showcase")
	if first.appID != second.appID || first.protocol != second.protocol || first.activator != second.activator {
		t.Fatal("temporary build paths split the development identity")
	}
	if second.executable != otherRun {
		t.Fatal("activation did not follow the current executable")
	}
	for _, other := range []notificationIdentity{
		developmentNotificationIdentity(executable, `C:\OtherCheckout\gui`, "example.org/gui/examples/showcase"),
		developmentNotificationIdentity(executable, `C:\Projects\gui`, "example.org/gui/examples/other"),
		developmentNotificationIdentity(`C:\Temp\go-build123\b001\exe\other.exe`, `C:\Projects\gui`, "command-line-arguments"),
		identityForExecutable(executable),
	} {
		if first.appID == other.appID {
			t.Fatal("separate project/program identities collided")
		}
	}
	for _, tc := range []struct {
		path string
		want bool
	}{
		{executable, true}, {otherRun, true},
		{`C:\Apps\showcase.exe`, false},
		{`C:\Temp\go-build123\b001\showcase.exe`, false},
		{`C:\Temp\go-build\b001\exe\showcase.exe`, false},
		{`C:\Temp\go-buildabc\b001\exe\showcase.exe`, false},
		{`C:\Temp\go-build123\bin\exe\showcase.exe`, false},
	} {
		if got := goRunExecutable(tc.path, ""); got != tc.want {
			t.Errorf("goRunExecutable(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
	hash := strings.Repeat("ab", 32)
	for _, tc := range []struct {
		path string
		want bool
	}{
		{`C:\Cache\ab\` + hash + `-d\showcase.exe`, true},
		{`C:\OtherCache\ab\` + hash + `-d\showcase.exe`, false},
		{`C:\Cache\cd\` + hash + `-d\showcase.exe`, false},
		{`C:\Cache\ab\` + strings.Repeat("xy", 32) + `-d\showcase.exe`, false},
	} {
		if got := goRunExecutable(tc.path, `c:\cache`); got != tc.want {
			t.Errorf("cached goRunExecutable(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestNotificationRegistrationRetriesAndCachesSuccess(t *testing.T) {
	want := identityForExecutable(`C:\Example.exe`)
	transient := errors.New("temporary registration failure")
	var calls atomic.Int32
	register := cacheNotificationRegistration(func() (notificationIdentity, error) {
		if calls.Add(1) == 1 {
			return notificationIdentity{}, transient
		}
		return want, nil
	})
	if _, err := register(); !errors.Is(err, transient) {
		t.Fatalf("first error = %v", err)
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			got, err := register()
			if err != nil || got != want {
				t.Errorf("retry = %+v, %v", got, err)
			}
		})
	}
	wg.Wait()
	if got := calls.Load(); got != 2 {
		t.Errorf("registration calls = %d, want one failure and one success", got)
	}
}

func TestNotificationIdentityBindsExecutable(t *testing.T) {
	exe := `C:\Program Files\PLN\Example.exe`
	identity := identityForExecutable(exe)
	same := identityForExecutable(`c:\program files\pln\EXAMPLE.EXE`)
	if identity.appID != same.appID || identity.activator != same.activator || identity.protocol != same.protocol {
		t.Fatal("case changes split application identity")
	}
	other := identityForExecutable(`D:\Other\Example.exe`)
	if identity.appID == other.appID || identity.protocol == other.protocol || identity.activator == other.activator {
		t.Fatal("unrelated executables share identity")
	}
	if identity.name != "Example" || identity.executable != exe {
		t.Fatalf("identity = %+v", identity)
	}
	if len(identity.appID) > 128 || strings.ContainsAny(identity.appID, `\ /`) {
		t.Fatalf("invalid AUMID: %q", identity.appID)
	}
	values := map[string]string{}
	registration := notificationRegistration{
		write: func(key, name, value string) error { values[key+"|"+name] = value; return nil },
	}
	if err := installNotificationIdentity(identity, registration); err != nil {
		t.Fatal(err)
	}
	root := `Software\Classes\`
	if command := values[root+identity.protocol+`\shell\open\command|`]; command != `"`+exe+`"` || strings.Contains(command, "%1") {
		t.Errorf("activation must quote the source executable and take no URL arguments: %q", command)
	}
	if display := values[root+`AppUserModelId\`+identity.appID+"|DisplayName"]; display != "Example" {
		t.Errorf("display = %q", display)
	}
	if activator := values[root+`AppUserModelId\`+identity.appID+"|CustomActivator"]; activator != identity.activator.String() {
		t.Errorf("activator = %q", activator)
	}
	if _, ok := values[root+identity.protocol+"|URL Protocol"]; !ok {
		t.Fatal("protocol marker missing")
	}
}

func TestNotificationIdentityRegistrationFailure(t *testing.T) {
	want := errors.New("registration denied")
	for failAt := range 5 {
		calls := 0
		step := func() error {
			current := calls
			calls++
			if current == failAt {
				return want
			}
			return nil
		}
		err := installNotificationIdentity(identityForExecutable(`C:\app.exe`), notificationRegistration{
			write: func(string, string, string) error { return step() },
		})
		if !errors.Is(err, want) || calls != failAt+1 {
			t.Errorf("failure %d: err=%v calls=%d", failAt, err, calls)
		}
	}
}

func TestNotificationToastXMLIsData(t *testing.T) {
	title, body := `PLN's <title> & "test"`, "Hello ä¸–ç•Œ ðŸ˜€\n</text><actions/>"
	identity := identityForExecutable(`C:\Example.exe`)
	source := notificationToastXML(title, body, identity.protocol)
	var parsed struct {
		Activation string `xml:"activationType,attr"`
		Launch     string `xml:"launch,attr"`
		Binding    struct {
			Template string   `xml:"template,attr"`
			Text     []string `xml:"text"`
		} `xml:"visual>binding"`
	}
	if err := xml.Unmarshal([]byte(source), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Activation != "protocol" || parsed.Launch != identity.protocol+":" {
		t.Fatalf("activation = %+v", parsed)
	}
	if parsed.Binding.Template != "ToastGeneric" || len(parsed.Binding.Text) != 2 || parsed.Binding.Text[0] != title || parsed.Binding.Text[1] != body {
		t.Fatalf("notification text changed structure: %+v", parsed.Binding)
	}
	if strings.Contains(source, "<actions/>") {
		t.Fatal("notification text injected XML")
	}
}

func TestNotificationHRESULT(t *testing.T) {
	for _, hr := range []uintptr{0, 1} {
		if err := notificationHRESULT(hr); err != nil {
			t.Fatal(err)
		}
	}
	err := notificationHRESULT(0x80070490)
	if !errors.Is(err, notificationCOMError(0x80070490)) {
		t.Fatalf("HRESULT = %v", err)
	}
	if result := toastError(err); result.Status != gui.NotificationError || result.ErrorCode != "notification_failed" {
		t.Fatalf("result = %+v", result)
	}
	// Reject invalid text before registration or Windows API calls.
	for _, text := range [][2]string{{"x\x00", "body"}, {"title", "x\x00"}} {
		if result := sendNotification(text[0], text[1]); result.ErrorCode != "invalid_cfg" {
			t.Fatalf("result = %+v", result)
		}
	}
}
