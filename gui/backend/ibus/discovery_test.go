//go:build linux

package ibus

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDisplaySuffix(t *testing.T) {
	tests := []struct {
		name        string
		ibusDisplay string
		display     string
		want        string
	}{
		{"no display", "", "", "unix-0"},
		{"local display", "", ":0", "unix-0"},
		{"display with screen", "", ":0.0", "unix-0"},
		{"remote host", "", "host:1", "host-1"},
		{"remote with screen", "", "host:2.1", "host-2"},
		{"colon only", "", ":", "unix-0"},
		{"ibus overrides", "host:3", ":0", "host-3"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("IBUS_DISPLAY", tc.ibusDisplay)
			t.Setenv("DISPLAY", tc.display)
			if got := displaySuffix(); got != tc.want {
				t.Errorf("displaySuffix() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadSocketFile(t *testing.T) {
	dir := t.TempDir()

	t.Run("live daemon", func(t *testing.T) {
		p := writeSocketFile(t, dir, "unix:path=/run/ibus", os.Getpid())
		if got := readSocketFile(p); got != "unix:path=/run/ibus" {
			t.Errorf("readSocketFile = %q, want the recorded address", got)
		}
	})

	t.Run("dead daemon", func(t *testing.T) {
		// kill(pid, 0) on a pid no process can have.
		p := writeSocketFile(t, dir, "unix:path=/run/ibus", 1<<30)
		if got := readSocketFile(p); got != "" {
			t.Errorf("readSocketFile = %q, want empty for a dead pid", got)
		}
	})

	t.Run("no pid line", func(t *testing.T) {
		p := filepath.Join(dir, "nopid")
		writeFile(t, p, "IBUS_ADDRESS=unix:path=/run/ibus\n")
		if got := readSocketFile(p); got != "" {
			t.Errorf("readSocketFile = %q, want empty without a pid", got)
		}
	})

	t.Run("bogus pid", func(t *testing.T) {
		p := filepath.Join(dir, "bogus")
		writeFile(t, p, "IBUS_DAEMON_PID=abc\n")
		if got := readSocketFile(p); got != "" {
			t.Errorf("readSocketFile = %q, want empty for a bogus pid", got)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		if got := readSocketFile(filepath.Join(dir, "nope")); got != "" {
			t.Errorf("readSocketFile = %q, want empty", got)
		}
	})
}

func writeSocketFile(t *testing.T, dir, addr string, pid int) string {
	t.Helper()
	p := filepath.Join(dir, "bus")
	writeFile(t, p, "IBUS_ADDRESS="+addr+"\nIBUS_DAEMON_PID="+strconv.Itoa(pid)+"\n")
	return p
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
