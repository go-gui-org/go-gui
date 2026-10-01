package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// release_test.go covers the #852 flags (-dmg, -build-pkg, -arch,
// -ldflags, -entitlements, -notarize, -notary-profile). It carries no
// build tag: every external tool goes through the execOutput seam, so
// no test needs macOS, a certificate or Apple credentials.

// recordedCall is one execOutput invocation.
type recordedCall struct {
	env  []string
	name string
	args []string
}

// stubTools replaces the tool seam with a recorder that succeeds, and
// restores both vars when the test ends. Override stub.impl for
// per-test error injection; every invocation still lands in
// stub.calls first.
type toolStub struct {
	calls []recordedCall
	impl  func(env []string, name string, args ...string) ([]byte, error)
}

func stubTools(t *testing.T) *toolStub {
	t.Helper()
	stub := &toolStub{}
	stub.impl = func(env []string, name string, args ...string) ([]byte, error) {
		stub.calls = append(stub.calls, recordedCall{env, name, slices.Clone(args)})
		return []byte("ok"), nil
	}
	oldOut, oldLook := execOutput, execLookPath
	execOutput = func(env []string, name string, args ...string) ([]byte, error) {
		return stub.impl(env, name, args...)
	}
	execLookPath = func(string) error { return nil }
	t.Cleanup(func() { execOutput, execLookPath = oldOut, oldLook })
	return stub
}

// darwinStub cross-compiles a trivial main package for darwin, so the
// macOS packager tests run on any host. The stub is pure Go, which
// needs no macOS SDK.
func darwinStub(t *testing.T) string {
	t.Helper()
	return crossStub(t, "darwin", runtime.GOARCH, "stub")
}

func envVal(env []string, key string) string {
	for _, kv := range env {
		if v, ok := strings.CutPrefix(kv, key+"="); ok {
			return v
		}
	}
	return ""
}

// callsFor returns the recorded invocations of name.
func callsFor(calls []recordedCall, name string) [][]string {
	var out [][]string
	for _, c := range calls {
		if c.name == name {
			out = append(out, c.args)
		}
	}
	return out
}

func TestResolveBinaryVariants(t *testing.T) {
	if _, err := resolveBinary("", nil); err == nil {
		t.Error("no binary and no -build-pkg must be an error")
	}
	if _, err := resolveBinary("", []string{"a", "b"}); err == nil {
		t.Error("two binaries must be an error")
	}
	got, err := resolveBinary("", []string{"a"})
	if err != nil || got != "a" {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := resolveBinary("./myapp", []string{"a"}); err == nil {
		t.Error("-build-pkg with a binary must be an error")
	}
	if got, err := resolveBinary("./myapp", nil); err != nil || got != "" {
		t.Errorf("-build-pkg alone must succeed with no binary yet, got %q, %v", got, err)
	}
}

func TestMacOSOnlyFlagsRejected(t *testing.T) {
	for _, o := range []bundleOpts{
		{Platform: "windows", Dmg: true},
		{Platform: "linux", Entitlements: "e.plist"},
		{Platform: "windows", Notarize: true},
		{Platform: "linux", NotaryProfile: "p"},
	} {
		o.Binary = "whatever"
		if err := build(o); err == nil ||
			!strings.Contains(err.Error(), "macOS only") {
			t.Errorf("platform %+v: want a macOS-only error, got %v", o, err)
		}
	}
}

func TestNotarizeNeedsEntitlements(t *testing.T) {
	o := bundleOpts{
		Platform: "darwin", Binary: "nope",
		Notarize: true, NotaryProfile: "p",
	}
	err := build(o)
	if err == nil || !strings.Contains(err.Error(), "-entitlements") {
		t.Fatalf("want an error naming -entitlements, got %v", err)
	}
}

func TestNotaryProfileNeedsNotarize(t *testing.T) {
	o := bundleOpts{
		Platform: "darwin", Binary: "nope", NotaryProfile: "p",
	}
	err := build(o)
	if err == nil || !strings.Contains(err.Error(), "-notarize") {
		t.Fatalf("want an error naming -notarize, got %v", err)
	}
}

func TestNotarizeNeedsProfile(t *testing.T) {
	o := bundleOpts{
		Platform: "darwin", Binary: "nope",
		Entitlements: "e.plist", Notarize: true,
	}
	err := build(o)
	if err == nil || !strings.Contains(err.Error(), "-notary-profile") {
		t.Fatalf("want an error naming -notary-profile, got %v", err)
	}
}

func TestEntitlementsMissingFile(t *testing.T) {
	o := bundleOpts{
		Platform: "darwin", Binary: "nope",
		Entitlements: filepath.Join(t.TempDir(), "missing.plist"),
	}
	err := build(o)
	if err == nil || !strings.Contains(err.Error(), "entitlements") {
		t.Fatalf("want an error naming the entitlements file, got %v", err)
	}
}

func TestCompileWindowsAddsGuiAndDisablesCgo(t *testing.T) {
	stub := stubTools(t)
	out := filepath.Join(t.TempDir(), "myapp.exe")
	o := bundleOpts{
		Platform: "windows", Arch: "amd64",
		BuildPkg: "./myapp", Ldflags: "-X main.v=1",
	}
	if err := compilePkg(o, out); err != nil {
		t.Fatal(err)
	}
	if len(stub.calls) != 1 {
		t.Fatalf("want 1 go build call, got %d", len(stub.calls))
	}
	c := (stub.calls)[0]
	if c.name != "go" {
		t.Fatalf("ran %q, want go", c.name)
	}
	if got := envVal(c.env, "GOOS"); got != "windows" {
		t.Errorf("GOOS = %q, want windows", got)
	}
	if got := envVal(c.env, "CGO_ENABLED"); got != "0" {
		t.Errorf("CGO_ENABLED = %q, want 0", got)
	}
	if got := envVal(c.env, "GOARCH"); got != "amd64" {
		t.Errorf("GOARCH = %q, want amd64", got)
	}
	joined := strings.Join(c.args, " ")
	if !strings.Contains(joined, "-H windowsgui") {
		t.Errorf("windows build must add -H windowsgui, got %q", joined)
	}
	if !strings.Contains(joined, "-X main.v=1") {
		t.Errorf("windows build must keep caller ldflags, got %q", joined)
	}
}

func TestCompileWindowsGuiNotDuplicated(t *testing.T) {
	stub := stubTools(t)
	o := bundleOpts{
		Platform: "windows", BuildPkg: "./myapp",
		Ldflags: "-H windowsgui",
	}
	if err := compilePkg(o, filepath.Join(t.TempDir(), "a.exe")); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.Join((stub.calls)[0].args, " "), "windowsgui"); n != 1 {
		t.Errorf("windowsgui appears %d times, want 1", n)
	}
}

func TestCompileLinuxCgoFreeNoGuiFlag(t *testing.T) {
	stub := stubTools(t)
	o := bundleOpts{Platform: "linux", Arch: "arm64", BuildPkg: "./myapp"}
	if err := compilePkg(o, filepath.Join(t.TempDir(), "myapp")); err != nil {
		t.Fatal(err)
	}
	c := (stub.calls)[0]
	if got := envVal(c.env, "CGO_ENABLED"); got != "0" {
		t.Errorf("CGO_ENABLED = %q, want 0", got)
	}
	if strings.Contains(strings.Join(c.args, " "), "windowsgui") {
		t.Errorf("linux build must not add windowsgui: %q", c.args)
	}
}

func TestCompileDarwinKeepsCgo(t *testing.T) {
	stub := stubTools(t)
	o := bundleOpts{Platform: "darwin", BuildPkg: "./myapp"}
	if err := compilePkg(o, filepath.Join(t.TempDir(), "myapp")); err != nil {
		t.Fatal(err)
	}
	if got := envVal((stub.calls)[0].env, "CGO_ENABLED"); got != "1" {
		t.Errorf("CGO_ENABLED = %q, want 1", got)
	}
}

func TestCompileErrorCarriesToolOutput(t *testing.T) {
	stub := stubTools(t)
	stub.impl = func(env []string, name string, args ...string) ([]byte, error) {
		return []byte("no such package"), errors.New("exit status 1")
	}
	o := bundleOpts{Platform: "linux", BuildPkg: "./nope"}
	err := compilePkg(o, filepath.Join(t.TempDir(), "x"))
	if err == nil || !strings.Contains(err.Error(), "no such package") {
		t.Fatalf("want the go build output in the error, got %v", err)
	}
}

func TestWithEnvReplacesDuplicates(t *testing.T) {
	got := withEnv([]string{"GOOS=linux", "KEEP=1"},
		[]string{"GOOS=windows", "CGO_ENABLED=0"})
	if envVal(got, "GOOS") != "windows" {
		t.Errorf("duplicate GOOS not replaced: %q", got)
	}
	if len(got) != 3 {
		t.Errorf("want 3 entries, got %q", got)
	}
}

func TestSignHardenedArgs(t *testing.T) {
	stub := stubTools(t)
	bin := darwinStub(t)
	ent := filepath.Join(t.TempDir(), "app.entitlements")
	if err := os.WriteFile(ent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := bundleOpts{
		Platform: "darwin", Binary: bin, OutDir: t.TempDir(),
		Name: "Hard", Version: "1.0", SignID: "Dev ID", Entitlements: ent,
	}
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	signs := callsFor(stub.calls, "codesign")
	if len(signs) != 1 {
		t.Fatalf("want 1 codesign call, got %d", len(signs))
	}
	args := strings.Join(signs[0], " ")
	for _, want := range []string{
		"--options", "runtime", "--entitlements", ent, "--timestamp",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("hardened sign lacks %q: %s", want, args)
		}
	}
	if strings.Contains(args, "--deep") {
		t.Errorf("hardened sign must drop --deep: %s", args)
	}
}

func TestSignAdhocKeepsDeep(t *testing.T) {
	stub := stubTools(t)
	bin := darwinStub(t)
	o := bundleOpts{
		Platform: "darwin", Binary: bin, OutDir: t.TempDir(),
		Name: "Plain", Version: "1.0",
	}
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	signs := callsFor(stub.calls, "codesign")
	if len(signs) != 1 {
		t.Fatalf("want 1 codesign call, got %d", len(signs))
	}
	if !strings.Contains(strings.Join(signs[0], " "), "--deep") {
		t.Errorf("ad-hoc sign must keep --deep: %q", signs[0])
	}
}

func TestDmgArgs(t *testing.T) {
	stub := stubTools(t)
	bin := darwinStub(t)
	out := t.TempDir()
	o := bundleOpts{
		Platform: "darwin", Binary: bin, OutDir: out,
		Name: "My App", Version: "2.0", Dmg: true,
	}
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	dmgs := callsFor(stub.calls, "hdiutil")
	if len(dmgs) != 1 {
		t.Fatalf("want 1 hdiutil call, got %d", len(dmgs))
	}
	args := dmgs[0]
	want := []string{"create", "-srcfolder", "-volname", "My App 2.0",
		"-format", "UDZO", filepath.Join(out, "my-app-2.0.dmg")}
	for _, w := range want {
		if !slices.Contains(args, w) {
			t.Errorf("hdiutil args lack %q: %q", w, args)
		}
	}
}

func TestDmgFailureErrors(t *testing.T) {
	stub := stubTools(t)
	stub.impl = func(env []string, name string, args ...string) ([]byte, error) {
		if name == "hdiutil" {
			return []byte("no space"), errors.New("exit status 1")
		}
		return []byte("ok"), nil
	}
	bin := darwinStub(t)
	o := bundleOpts{
		Platform: "darwin", Binary: bin, OutDir: t.TempDir(),
		Name: "My App", Version: "2.0", Dmg: true,
	}
	err := build(o)
	if err == nil || !strings.Contains(err.Error(), "no space") {
		t.Fatalf("want the hdiutil output in the error, got %v", err)
	}
}

func TestNotarizeSubmitsDmg(t *testing.T) {
	stub := stubTools(t)
	bin := darwinStub(t)
	out := t.TempDir()
	ent := filepath.Join(t.TempDir(), "app.entitlements")
	if err := os.WriteFile(ent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := bundleOpts{
		Platform: "darwin", Binary: bin, OutDir: out,
		Name: "My App", Version: "2.0", Dmg: true,
		Entitlements: ent, Notarize: true, NotaryProfile: "ACME",
	}
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	dmg := filepath.Join(out, "my-app-2.0.dmg")
	xcruns := callsFor(stub.calls, "xcrun")
	if len(xcruns) != 2 {
		t.Fatalf("want submit + staple, got %d: %q", len(xcruns), xcruns)
	}
	submit := xcruns[0]
	for _, want := range []string{
		"notarytool", "submit", dmg,
		"--keychain-profile", "ACME", "--wait",
	} {
		if !slices.Contains(submit, want) {
			t.Errorf("submit lacks %q: %q", want, submit)
		}
	}
	staple := strings.Join(xcruns[1], " ")
	if !strings.Contains(staple, "stapler staple "+dmg) {
		t.Errorf("staple must target the dmg, got %q", staple)
	}
}

func TestNotarizeWithoutDmgSubmitsApp(t *testing.T) {
	stub := stubTools(t)
	bin := darwinStub(t)
	out := t.TempDir()
	ent := filepath.Join(t.TempDir(), "app.entitlements")
	if err := os.WriteFile(ent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := bundleOpts{
		Platform: "darwin", Binary: bin, OutDir: out,
		Name: "Solo", Version: "1.0",
		Entitlements: ent, Notarize: true, NotaryProfile: "ACME",
	}
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	if len(callsFor(stub.calls, "hdiutil")) != 0 {
		t.Error("no dmg requested, hdiutil must not run")
	}
	xcruns := callsFor(stub.calls, "xcrun")
	if len(xcruns) != 2 {
		t.Fatalf("want submit + staple, got %q", xcruns)
	}
	if !slices.Contains(xcruns[0], filepath.Join(out, "Solo.app")) {
		t.Errorf("submit must target the .app: %q", xcruns[0])
	}
}

func TestCompileTempDirNaming(t *testing.T) {
	stub := stubTools(t)
	for _, tc := range []struct{ pkg, platform, want string }{
		{"./examples/showcase/", "linux", "showcase"},
		{".", "linux", "app"},
		{"..", "linux", "app"},
		{".", "windows", "app.exe"},
		{"./myapp", "windows", "myapp.exe"},
	} {
		stub.calls = (stub.calls)[:0]
		got, err := compileInTempDir(bundleOpts{
			Platform: tc.platform, BuildPkg: tc.pkg,
		})
		if err != nil {
			t.Fatalf("%s: %v", tc.pkg, err)
		}
		if filepath.Base(got) != tc.want {
			t.Errorf("pkg %q on %s: staged %q, want %q",
				tc.pkg, tc.platform, filepath.Base(got), tc.want)
		}
		_ = os.RemoveAll(filepath.Dir(got))
	}
}

// A stale image at the destination must not stop the new one: the
// .app overwrite needs no prompt, and neither does the .dmg.
func TestDmgOverwritesStale(t *testing.T) {
	stub := stubTools(t)
	bin := darwinStub(t)
	out := t.TempDir()
	stale := filepath.Join(out, "my-app-2.0.dmg")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := bundleOpts{
		Platform: "darwin", Binary: bin, OutDir: out,
		Name: "My App", Version: "2.0", Dmg: true,
	}
	if err := build(o); err != nil {
		t.Fatal(err)
	}
	if len(callsFor(stub.calls, "hdiutil")) != 1 {
		t.Error("hdiutil must still run over a stale dmg")
	}
}

func TestMissingToolsError(t *testing.T) {
	stubTools(t)
	oldLook := execLookPath
	// Only the tool under test is missing; codesign still resolves so
	// the dmg case reaches hdiutil and the notarize case reaches xcrun.
	execLookPath = func(name string) error {
		if name == "hdiutil" || name == "xcrun" {
			return errors.New(name + " not found here")
		}
		return nil
	}
	t.Cleanup(func() { execLookPath = oldLook })
	bin := darwinStub(t)
	out := t.TempDir()
	dmgOpt := bundleOpts{
		Platform: "darwin", Binary: bin, OutDir: out,
		Name: "My App", Version: "2.0", Dmg: true,
	}
	if err := build(dmgOpt); err == nil ||
		!strings.Contains(err.Error(), "hdiutil not found") {
		t.Errorf("want an hdiutil-missing error, got %v", err)
	}
	ent := filepath.Join(t.TempDir(), "app.entitlements")
	if err := os.WriteFile(ent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	notOpt := bundleOpts{
		Platform: "darwin", Binary: bin, OutDir: out,
		Name: "My App", Version: "2.0",
		Entitlements: ent, Notarize: true, NotaryProfile: "ACME",
	}
	if err := build(notOpt); err == nil ||
		!strings.Contains(err.Error(), "xcrun not found") {
		t.Errorf("want an xcrun-missing error, got %v", err)
	}
}
