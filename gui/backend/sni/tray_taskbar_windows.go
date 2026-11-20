//go:build windows

package sni

import (
	"cmp"
	"errors"
	"slices"
	"syscall"
	"unsafe"
)

// taskbarCreatedName names the registered message the shell broadcasts
// to every top-level window when it creates the taskbar, which it does
// again after Explorer restarts. Tests set a private name, so a test
// broadcast reaches no other application's tray.
var taskbarCreatedName = "TaskbarCreated"

// iconData describes icon id with its callback message, tooltip and,
// when hIcon is set, its icon: everything NIM_ADD sends.
//
// The icon is named by hWnd + uID in every call, never by GUID. The
// shell matches an icon added with NIF_GUID by its GUID only, so
// NIM_MODIFY and NIM_DELETE by uID would miss it (#619). A GUID would
// also tie the icon to the path of the executable: NIM_ADD fails after
// the executable moves, and a second instance of the same executable
// cannot add its icon.
func (t *Tray) iconData(id int, hIcon uintptr, tooltip string) notifyIconDataW {
	// Build tooltip as UTF-16.
	tip, _ := syscall.UTF16FromString(tooltip)

	nid := notifyIconDataW{
		cbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:             t.hwnd,
		uID:              uint32(id),
		uFlags:           nifMessage | nifTip | nifShowTip,
		uCallbackMessage: wmAppTray,
		hIcon:            hIcon,
	}
	copy(nid.szTip[:], tip)

	if hIcon != 0 {
		nid.uFlags |= nifIcon
	}
	return nid
}

// addIcon adds icon id to the notification area and sets
// NOTIFYICON_VERSION_4 on it. Create and reAddIcons both add through
// it, so every icon the tray adds uses the layout wndProc decodes.
func (t *Tray) addIcon(id int, hIcon uintptr, tooltip string) error {
	nid := t.iconData(id, hIcon, tooltip)
	if !shellNotifyIcon(nimAdd, &nid) {
		return errors.New("sni: Shell_NotifyIcon(NIM_ADD) failed")
	}

	// Set NOTIFYICON_VERSION_4. It applies to an icon that exists, so it
	// must come after NIM_ADD; sent before, it had no icon to act on and
	// its failure went unseen (#617). wndProc decodes only the version 4
	// layout, so if the shell refuses, the icon's clicks would be decoded
	// wrong: remove the icon and fail instead.
	if !setIconVersion(nid) {
		nidDelete := nid
		nidDelete.uFlags = 0
		shellNotifyIcon(nimDelete, &nidDelete)
		return errors.New("sni: Shell_NotifyIcon(NIM_SETVERSION) failed")
	}
	return nil
}

// setIconVersion sets NOTIFYICON_VERSION_4 on the icon that nid names.
func setIconVersion(nid notifyIconDataW) bool {
	nid.uFlags = 0
	nid.uVersion = notifyIconV4
	return shellNotifyIcon(nimSetVersion, &nid)
}

// reAddIcons puts every live entry back in the notification area, with
// the icon and tooltip it has now. wndProc runs it when the shell
// broadcasts TaskbarCreated: a new taskbar, as after Explorer restarts,
// has none of the icons added before.
func (t *Tray) reAddIcons() {
	// With shellMu held, no entry can be added, changed or removed, so
	// the entries and handles read here stay valid until the shell
	// calls are done.
	t.shellMu.Lock()
	defer t.shellMu.Unlock()

	t.mu.Lock()
	live := make([]*entry, 0, len(t.entries))
	for _, e := range t.entries {
		live = append(live, e)
	}
	t.mu.Unlock()
	// Add them in the order they were created.
	slices.SortFunc(live, func(a, b *entry) int { return cmp.Compare(a.id, b.id) })

	for _, e := range live {
		t.reAddIcon(e)
	}
}

// reAddIcon puts the icon of entry e back. The caller holds shellMu.
//
// The shell can also send TaskbarCreated while the icons are still
// there (a DPI change does), and then NIM_ADD fails because the icon
// exists. NIM_MODIFY then sets the entry's icon and tooltip on that
// icon. If the shell refuses both, the entry stays as it is:
// TaskbarCreated means the taskbar is ready for icons, so there is
// nothing to wait for, and the next TaskbarCreated tries again.
func (t *Tray) reAddIcon(e *entry) {
	t.mu.Lock()
	hIcon, tooltip, iconPNG := e.hIcon, e.tooltip, e.iconPNG
	t.mu.Unlock()

	// The icon is made again from the entry's PNG. An Update that came
	// while the icon was gone was refused by the shell, so the entry
	// kept its old handle, but its PNG is the one the app asked for.
	var fresh uintptr
	if len(iconPNG) > 0 {
		if h, err := iconFromPNG(iconPNG, maxTrayIconDim); err == nil {
			fresh = h
			hIcon = h
		}
	}

	accepted := t.addIcon(e.id, hIcon, tooltip) == nil
	if !accepted {
		nid := t.iconData(e.id, hIcon, tooltip)
		accepted = shellNotifyIcon(nimModify, &nid)
		if accepted {
			// An icon that exists is version 4 from its add, except
			// one left by an add whose version step and the delete
			// after it were both refused. Setting the version again
			// repairs that one; if this is refused too, the next
			// TaskbarCreated sets it.
			setIconVersion(nid)
		}
	}

	// As in Update, the entry swaps to the new handle, and frees the
	// old one, only once the shell has accepted it.
	if fresh == 0 {
		return
	}
	freeIcon := fresh
	if accepted {
		t.mu.Lock()
		freeIcon, e.hIcon = e.hIcon, fresh
		t.mu.Unlock()
	}
	if freeIcon != 0 {
		destroyIcon(freeIcon)
	}
}
