package tempdir

import (
	"os"
	"testing"
)

func TestAdoptSetsTMPDIRWhenGoSeesNone(t *testing.T) {
	t.Setenv("TMPDIR", "")
	if !Adopt("/data/user/0/app/cache") {
		t.Fatal("Adopt did not set TMPDIR")
	}
	if got := os.Getenv("TMPDIR"); got != "/data/user/0/app/cache" {
		t.Fatalf("TMPDIR = %q", got)
	}
}

func TestAdoptKeepsExistingTMPDIR(t *testing.T) {
	t.Setenv("TMPDIR", "/host/chose/this")
	if Adopt("/data/user/0/app/cache") {
		t.Fatal("Adopt replaced a TMPDIR Go already saw")
	}
	if got := os.Getenv("TMPDIR"); got != "/host/chose/this" {
		t.Fatalf("TMPDIR = %q", got)
	}
}

func TestAdoptIgnoresEmptyValue(t *testing.T) {
	t.Setenv("TMPDIR", "")
	if Adopt("") || os.Getenv("TMPDIR") != "" {
		t.Fatal("Adopt set TMPDIR from an empty value")
	}
}
