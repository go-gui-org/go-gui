//go:build windows

package sni

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// The tray sets NOTIFYICON_VERSION_4, and with that version the shell
// hides the standard tooltip unless NIF_SHOWTIP is set next to NIF_TIP.
// Without it SystemTrayCfg.Tooltip never showed on Windows (issue #1274).
func TestTrayAsksForTheStandardTooltip(t *testing.T) {
	tr, _ := newFakeTray(t)
	flags := map[uint32]uint32{}
	record := shellNotifyIcon
	shellNotifyIcon = func(message uint32, nid *notifyIconDataW) bool {
		flags[message] = nid.uFlags
		return record(message, nid)
	}

	id, err := tr.Create(gui.SystemTrayCfg{Tooltip: "a"}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	tr.Update(id, gui.SystemTrayCfg{Tooltip: "b"})

	const want = nifTip | nifShowTip
	for _, message := range []uint32{nimAdd, nimModify} {
		got, sent := flags[message]
		if !sent {
			t.Errorf("message %d was not sent", message)
			continue
		}
		if got&want != want {
			t.Errorf("message %d: flags %#x lack NIF_TIP|NIF_SHOWTIP (%#x)",
				message, got, want)
		}
	}
}
