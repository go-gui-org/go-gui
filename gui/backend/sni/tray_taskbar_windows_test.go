//go:build windows

package sni

import (
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	procSendMessageW    = user32.NewProc("SendMessageW")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
)

// hwndBroadcast is HWND_BROADCAST: a message posted to it goes to every
// top-level window.
const hwndBroadcast = 0xFFFF

// shellIcon is one icon as the fake notification area holds it.
type shellIcon struct {
	callback uint32
	hIcon    uintptr
	tip      string
	showTip  bool
	version  uint32
}

// notificationArea models the shell's icon table on top of the fake
// shell of newFakeTray. Like the real shell, it refuses NIM_ADD for an
// icon that exists, and NIM_MODIFY, NIM_SETVERSION and NIM_DELETE for
// one that does not. restart empties it, as an Explorer restart does.
type notificationArea struct {
	fs    *fakeShell
	mu    sync.Mutex
	icons map[uint32]shellIcon
	made  []uintptr // every icon handle iconFromPNG made
	// usedFreed lists handles sent to the shell after they were freed.
	usedFreed []uintptr
}

// newNotificationArea gives a fake tray whose shell keeps icon state.
// Fake icon handles carry the first byte of their PNG in bits 16-23,
// so a test can tell which PNG the shell's icon was made from. Not
// parallel: it swaps package-level seams.
func newNotificationArea(t *testing.T) (*Tray, *fakeShell, *notificationArea) {
	t.Helper()
	tr, fs := newFakeTray(t)
	na := &notificationArea{fs: fs, icons: map[uint32]shellIcon{}}
	record := shellNotifyIcon
	shellNotifyIcon = func(message uint32, nid *notifyIconDataW) bool {
		if !record(message, nid) {
			return false
		}
		return na.apply(message, nid)
	}
	iconFromPNG = func(png []byte, _ int) (uintptr, error) {
		fs.nextIcon++
		h := fs.nextIcon | uintptr(png[0])<<16
		na.made = append(na.made, h)
		return h, nil
	}
	return tr, fs, na
}

func (na *notificationArea) apply(message uint32, nid *notifyIconDataW) bool {
	na.mu.Lock()
	defer na.mu.Unlock()
	if nid.uFlags&nifIcon != 0 && slices.Contains(na.fs.destroyed, nid.hIcon) {
		na.usedFreed = append(na.usedFreed, nid.hIcon)
	}
	ic, exists := na.icons[nid.uID]
	switch message {
	case nimAdd:
		if exists {
			return false
		}
		ic = shellIcon{}
	case nimModify, nimSetVersion, nimDelete:
		if !exists {
			return false
		}
	}
	switch message {
	case nimDelete:
		delete(na.icons, nid.uID)
		return true
	case nimSetVersion:
		ic.version = nid.uVersion
	default:
		if nid.uFlags&nifMessage != 0 {
			ic.callback = nid.uCallbackMessage
		}
		if nid.uFlags&nifIcon != 0 {
			ic.hIcon = nid.hIcon
		}
		if nid.uFlags&nifTip != 0 {
			ic.tip = syscall.UTF16ToString(nid.szTip[:])
		}
		ic.showTip = nid.uFlags&nifShowTip != 0
	}
	na.icons[nid.uID] = ic
	return true
}

// restart drops every icon, as a new taskbar has none.
func (na *notificationArea) restart() {
	na.mu.Lock()
	defer na.mu.Unlock()
	clear(na.icons)
}

func (na *notificationArea) snapshot() map[uint32]shellIcon {
	na.mu.Lock()
	defer na.mu.Unlock()
	return maps.Clone(na.icons)
}

// checkHandles fails the test if a freed handle reached the shell, or
// a handle was freed twice, or a handle neither an entry nor the shell
// uses was never freed.
func (na *notificationArea) checkHandles(t *testing.T, tr *Tray) {
	t.Helper()
	if len(na.usedFreed) != 0 {
		t.Errorf("freed handles sent to the shell: %#x", na.usedFreed)
	}
	freed := map[uintptr]int{}
	for _, h := range na.fs.destroyed {
		freed[h]++
		if freed[h] > 1 {
			t.Errorf("handle %#x freed twice", h)
		}
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	held := map[uintptr]bool{}
	for _, e := range tr.entries {
		held[e.hIcon] = true
	}
	for _, h := range na.made {
		if held[h] == (freed[h] > 0) {
			t.Errorf("handle %#x: held by an entry %v, freed %d times", h, held[h], freed[h])
		}
	}
}

// pngA and pngB are icon data the fake tells apart by the first byte.
var (
	pngA = []byte{0xA}
	pngB = []byte{0xB}
)

// iconPNGByte reads back which PNG a fake handle was made from.
func iconPNGByte(h uintptr) byte { return byte(h >> 16) }

// After Explorer restarts, the new taskbar has none of the icons added
// before. On TaskbarCreated every live entry is added again with its
// current icon, tooltip and callback, and set to version 4, because
// wndProc decodes only that layout. A removed entry stays removed.
func TestTrayReAddsEveryIconAfterTaskbarCreated(t *testing.T) {
	tr, _, na := newNotificationArea(t)
	a, err := tr.Create(gui.SystemTrayCfg{Tooltip: "a", IconPNG: pngA}, nil)
	if err != nil {
		t.Fatalf("Create a: %v", err)
	}
	b, err := tr.Create(gui.SystemTrayCfg{Tooltip: "b"}, nil)
	if err != nil {
		t.Fatalf("Create b: %v", err)
	}
	c, err := tr.Create(gui.SystemTrayCfg{Tooltip: "c", IconPNG: pngA}, nil)
	if err != nil {
		t.Fatalf("Create c: %v", err)
	}
	tr.Remove(c)
	tr.Update(a, gui.SystemTrayCfg{Tooltip: "a2", IconPNG: pngB})

	na.restart()
	tr.reAddIcons()

	want := map[uint32]shellIcon{
		uint32(a): {callback: wmAppTray, hIcon: entryIcon(tr, a), tip: "a2", showTip: true, version: notifyIconV4},
		uint32(b): {callback: wmAppTray, tip: "b", showTip: true, version: notifyIconV4},
	}
	if got := na.snapshot(); !maps.Equal(got, want) {
		t.Errorf("shell icons after re-add:\n got %+v\nwant %+v", got, want)
	}
	if got := iconPNGByte(entryIcon(tr, a)); got != pngB[0] {
		t.Errorf("icon a made from PNG %#x, want the updated %#x", got, pngB[0])
	}
	na.checkHandles(t, tr)
}

// TaskbarCreated can come while the icons still exist (a DPI change
// sends it). The icon must stay and be brought up to date in place:
// not removed and not added twice. Here it is behind the entry, because
// a busy shell refused the last Update. If the shell refuses both the
// add and the modify, the entry stays live, and the next TaskbarCreated
// adds it.
func TestTrayReAddWhenIconStillExists(t *testing.T) {
	tr, fs, na := newNotificationArea(t)
	a, err := tr.Create(gui.SystemTrayCfg{Tooltip: "a", IconPNG: pngA}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	fs.refuse[nimModify] = true
	tr.Update(a, gui.SystemTrayCfg{Tooltip: "b", IconPNG: pngB})
	fs.refuse[nimModify] = false

	fs.calls = nil
	tr.reAddIcons()
	want := map[uint32]shellIcon{
		uint32(a): {callback: wmAppTray, hIcon: entryIcon(tr, a), tip: "b", showTip: true, version: notifyIconV4},
	}
	if got := iconPNGByte(entryIcon(tr, a)); got != pngB[0] {
		t.Errorf("icon made from PNG %#x, want the updated %#x", got, pngB[0])
	}
	if got := na.snapshot(); !maps.Equal(got, want) {
		t.Errorf("icon still there:\n got %+v\nwant %+v", got, want)
	}
	for _, call := range fs.calls {
		if call.message == nimDelete {
			t.Errorf("re-add deleted the existing icon: %+v", fs.calls)
		}
	}

	// A shell that refuses everything leaves the entry as it is.
	na.restart()
	fs.refuse[nimAdd], fs.refuse[nimModify] = true, true
	tr.reAddIcons()
	if got := na.snapshot(); len(got) != 0 {
		t.Errorf("refused: shell icons %+v, want none", got)
	}
	fs.refuse[nimAdd], fs.refuse[nimModify] = false, false
	tr.reAddIcons()
	want[uint32(a)] = shellIcon{callback: wmAppTray, hIcon: entryIcon(tr, a), tip: "b", showTip: true, version: notifyIconV4}
	if got := na.snapshot(); !maps.Equal(got, want) {
		t.Errorf("next re-add:\n got %+v\nwant %+v", got, want)
	}
	na.checkHandles(t, tr)
}

// An Update between the Explorer restart and TaskbarCreated finds no
// icon, so the shell refuses it and the entry keeps its old handle. The
// re-add must still show the icon that Update asked for, not the old
// one. A later Update that names no icon changes the tooltip and keeps
// that icon: it must not bring the old one back.
func TestTrayReAddShowsIconUpdatedWhileGone(t *testing.T) {
	tr, _, na := newNotificationArea(t)
	a, err := tr.Create(gui.SystemTrayCfg{Tooltip: "a", IconPNG: pngA}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	na.restart()
	tr.Update(a, gui.SystemTrayCfg{Tooltip: "b", IconPNG: pngB})
	tr.Update(a, gui.SystemTrayCfg{Tooltip: "c"})
	tr.reAddIcons()

	got := na.snapshot()[uint32(a)]
	if got.tip != "c" || iconPNGByte(got.hIcon) != pngB[0] {
		t.Errorf("shell shows tooltip %q, icon from PNG %#x; want %q from %#x",
			got.tip, iconPNGByte(got.hIcon), "c", pngB[0])
	}
	if got.hIcon != entryIcon(tr, a) {
		t.Errorf("shell icon %#x, entry holds %#x", got.hIcon, entryIcon(tr, a))
	}
	na.checkHandles(t, tr)
}

// An add whose version step is refused deletes the icon again. If the
// shell refuses that delete too, a version 0 icon stays, and wndProc
// cannot decode its clicks. The next re-add finds the icon there, so it
// modifies it; it must set the version as well.
func TestTrayReAddSetsVersionOnIconThatExists(t *testing.T) {
	tr, fs, na := newNotificationArea(t)
	a, err := tr.Create(gui.SystemTrayCfg{Tooltip: "a", IconPNG: pngA}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	na.restart()
	fs.refuse[nimSetVersion], fs.refuse[nimDelete] = true, true
	tr.reAddIcons()
	left, exists := na.snapshot()[uint32(a)]
	if !exists || left.version != 0 {
		t.Fatalf("setup: shell icon %+v (exists: %v), want the version 0 icon the refusals leave",
			left, exists)
	}

	fs.refuse[nimSetVersion], fs.refuse[nimDelete] = false, false
	fs.calls = nil
	tr.reAddIcons()
	if got := na.snapshot()[uint32(a)].version; got != notifyIconV4 {
		t.Errorf("icon version %d after the next re-add, want %d", got, notifyIconV4)
	}
	modified := slices.ContainsFunc(fs.calls, func(c notifyCall) bool { return c.message == nimModify })
	if !modified {
		t.Errorf("the next re-add did not modify the icon that exists: %+v", fs.calls)
	}
	na.checkHandles(t, tr)
}

// A refused version step is not a refused add. addIcon has deleted the
// icon again, so there is nothing to modify, and the re-add must not
// try. The entry stays live, and the next TaskbarCreated adds it.
func TestTrayReAddDoesNotModifyAfterRefusedVersion(t *testing.T) {
	tr, fs, na := newNotificationArea(t)
	a, err := tr.Create(gui.SystemTrayCfg{Tooltip: "a", IconPNG: pngA}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	na.restart()
	fs.refuse[nimSetVersion] = true
	fs.calls = nil
	tr.reAddIcons()
	for _, call := range fs.calls {
		if call.message == nimModify {
			t.Errorf("re-add sent NIM_MODIFY after a refused version step: %+v", fs.calls)
		}
	}
	if got := na.snapshot(); len(got) != 0 {
		t.Errorf("shell icons after the refused version step: %+v, want none", got)
	}

	fs.refuse[nimSetVersion] = false
	tr.reAddIcons()
	if got := na.snapshot()[uint32(a)]; got.version != notifyIconV4 || got.hIcon != entryIcon(tr, a) {
		t.Errorf("next re-add: shell icon %+v, want version %d with the entry's icon %#x",
			got, notifyIconV4, entryIcon(tr, a))
	}
	na.checkHandles(t, tr)
}

// A re-add pass lists the entries and then takes shellMu for one icon
// at a time, so a Remove can come between the listing and an entry's
// turn. That entry must be skipped: adding it would bring back an icon
// nobody owns.
func TestTrayReAddSkipsEntryRemovedDuringPass(t *testing.T) {
	tr, fs, na := newNotificationArea(t)
	a, err := tr.Create(gui.SystemTrayCfg{Tooltip: "a", IconPNG: pngA}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	tr.mu.Lock()
	listed := tr.entries[a]
	tr.mu.Unlock()

	na.restart()
	tr.Remove(a)
	fs.calls = nil
	tr.reAddIcon(listed)

	if len(fs.calls) != 0 {
		t.Errorf("re-add of a removed entry reached the shell: %+v", fs.calls)
	}
	if got := na.snapshot(); len(got) != 0 {
		t.Errorf("shell icons: %+v, want none", got)
	}
	na.checkHandles(t, tr)
}

// The handle swap after a shell call needs the entry to be the live one
// still. shellMu keeps Remove out during the call, but a path that takes
// the entry without it has freed the icon the entry held. Swapping then
// frees that icon a second time and leaks the new handle.
func TestTrayHandleSwapNeedsLiveEntry(t *testing.T) {
	cases := []struct {
		name    string
		message uint32 // the shell call during which the entry is taken
		act     func(tr *Tray, id int)
	}{
		{"Update", nimModify, func(tr *Tray, id int) {
			tr.Update(id, gui.SystemTrayCfg{IconPNG: pngB})
		}},
		{"re-add", nimAdd, func(tr *Tray, _ int) { tr.reAddIcons() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr, _, na := newNotificationArea(t)
			a, err := tr.Create(gui.SystemTrayCfg{IconPNG: pngA}, nil)
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if tc.message == nimAdd {
				na.restart() // so the re-add's NIM_ADD is accepted
			}
			model := shellNotifyIcon
			shellNotifyIcon = func(message uint32, nid *notifyIconDataW) bool {
				accepted := model(message, nid)
				if message == tc.message {
					// Take the entry as Remove does, but without shellMu.
					tr.mu.Lock()
					taken := tr.entries[a]
					delete(tr.entries, a)
					tr.mu.Unlock()
					if taken != nil {
						destroyIcon(taken.hIcon)
					}
				}
				return accepted
			}

			tc.act(tr, a)
			na.checkHandles(t, tr)
		})
	}
}

// Explorer restarts and DPI changes can send several TaskbarCreated
// messages close together. Those that come during a pass must ask for
// one more pass together, not for one each.
func TestTrayCoalescesTaskbarCreated(t *testing.T) {
	tr, _, na := newNotificationArea(t)
	if _, err := tr.Create(gui.SystemTrayCfg{Tooltip: "a", IconPNG: pngA}, nil); err != nil {
		t.Fatalf("Create: %v", err)
	}
	na.restart()

	inAdd := make(chan struct{})
	release := make(chan struct{})
	// A failed test must not leave the pass waiting in the shell call.
	releaseShell := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseShell)
	var adds atomic.Int32
	model := shellNotifyIcon
	shellNotifyIcon = func(message uint32, nid *notifyIconDataW) bool {
		if message == nimAdd && adds.Add(1) == 1 {
			close(inAdd)
			<-release
		}
		return model(message, nid)
	}

	tr.requestReAdd()
	waitClosed(t, inAdd, "the pass to reach NIM_ADD")
	for range 5 {
		tr.requestReAdd()
	}
	releaseShell()
	waitReAddIdle(t, tr)

	// One NIM_ADD per pass: the pass that was running, and one more.
	if got := adds.Load(); got != 2 {
		t.Errorf("%d re-add passes for 5 messages during one pass, want 2", got)
	}
	na.checkHandles(t, tr)
}

// waitClosed fails the test if done is not closed in time, so a worker
// that never gets there does not block the test until its timeout.
func waitClosed(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// waitReAddIdle waits until the passes that requestReAdd started are
// done.
func waitReAddIdle(t *testing.T, tr *Tray) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for tr.reAddState.Load() != reAddIdle {
		if time.Now().After(deadline) {
			t.Fatal("the re-add passes did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

// A Remove while a re-add is in the shell must wait for it. If it went
// ahead, its NIM_DELETE would find no icon yet, the re-add's NIM_ADD
// would then bring back an icon nobody owns, with a handle Remove had
// freed.
func TestTrayRemoveDuringReAdd(t *testing.T) {
	tr, _, na := newNotificationArea(t)
	a, err := tr.Create(gui.SystemTrayCfg{Tooltip: "a", IconPNG: pngA}, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	na.restart()

	inAdd := make(chan struct{})
	release := make(chan struct{})
	// A failed test must not leave the re-add waiting in the shell call.
	releaseShell := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseShell)
	deleted := make(chan struct{}, 1)
	model := shellNotifyIcon
	shellNotifyIcon = func(message uint32, nid *notifyIconDataW) bool {
		switch message {
		case nimAdd:
			close(inAdd)
			<-release
		case nimDelete:
			deleted <- struct{}{}
		}
		return model(message, nid)
	}

	reAdded := make(chan struct{})
	go func() {
		tr.reAddIcons()
		close(reAdded)
	}()
	waitClosed(t, inAdd, "the re-add to reach NIM_ADD")
	removed := make(chan struct{})
	go func() {
		tr.Remove(a)
		close(removed)
	}()
	// Remove has had time to reach the shell if nothing stops it.
	select {
	case <-deleted:
		t.Error("Remove reached the shell while the re-add was in it")
	case <-time.After(100 * time.Millisecond):
	}
	releaseShell()
	waitClosed(t, reAdded, "the re-add to finish")
	waitClosed(t, removed, "Remove to finish")

	if got := na.snapshot(); len(got) != 0 {
		t.Errorf("shell icons after Remove: %+v, want none", got)
	}
	na.checkHandles(t, tr)
}

// The root cause: a message-only window gets no broadcast messages, so
// the tray never heard TaskbarCreated. The tray window must get a
// registered message broadcast to every top-level window, and stay
// hidden. The broadcast uses a private message name, so no other
// application's tray re-adds its icons.
func TestTrayWindowHearsTaskbarCreatedBroadcast(t *testing.T) {
	origName, origNotify := taskbarCreatedName, shellNotifyIcon
	t.Cleanup(func() { taskbarCreatedName, shellNotifyIcon = origName, origNotify })
	// One fixed name: a registered message stays registered until the
	// user signs out, so a new name for each run would use up the
	// session's message IDs.
	taskbarCreatedName = "go-gui-sni-test-TaskbarCreated"
	added := make(chan uint32, 1)
	shellNotifyIcon = func(message uint32, nid *notifyIconDataW) bool {
		if message == nimAdd {
			added <- nid.uID
		}
		return true
	}

	// The window outlives the test, and with one fixed name it hears the
	// broadcast of every later run (-count). newRealTray takes its
	// entries away when the test ends, so it then has nothing to add.
	tr := newRealTray(t)
	if visible, _, _ := procIsWindowVisible.Call(tr.hwnd); visible != 0 {
		t.Error("the tray window is visible")
	}
	tr.mu.Lock()
	tr.entries = map[int]*entry{1: {id: 1}}
	tr.mu.Unlock()

	if r, _, err := procPostMessageW.Call(hwndBroadcast, uintptr(tr.taskbarCreated), 0, 0); r == 0 {
		t.Fatalf("PostMessageW(HWND_BROADCAST): %v", err)
	}
	select {
	case id := <-added:
		if id != 1 {
			t.Errorf("re-added uID %d, want 1", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the tray window did not get the broadcast")
	}
	// Wait for the pass before Cleanup puts the real shell back.
	waitReAddIdle(t, tr)
}

// A top-level window can be sent WM_CLOSE by other processes; taskkill
// without /F does that to every top-level window of a process. The tray
// window must not close, or the app keeps running with dead icons.
func TestTrayWindowIgnoresWMClose(t *testing.T) {
	tr := newRealTray(t)
	fired := addTestEntry(tr)

	// SendMessageW returns once the window has handled WM_CLOSE, so the
	// click below is queued after anything WM_CLOSE posted.
	procSendMessageW.Call(tr.hwnd, wmClose, 0, 0)
	if r, _, err := procPostMessageW.Call(tr.hwnd, wmAppTray, 0, trayLParam(ninSelect, 1)); r == 0 {
		t.Fatalf("PostMessageW: %v", err)
	}
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("the tray stopped handling clicks after WM_CLOSE")
	}
}
