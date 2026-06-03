//go:build darwin && !ios

package metal

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/filedialog"
	"github.com/go-gui-org/go-gui/gui/backend/nativemenu"
	"github.com/go-gui-org/go-gui/gui/backend/printdialog"
	"github.com/go-gui-org/go-gui/gui/backend/spellcheck"
	"github.com/veandco/go-sdl2/sdl"
)

// nativePlatform implements gui.NativePlatform for the Metal
// backend.
type nativePlatform struct {
	window *sdl.Window
}

func (n *nativePlatform) OpenURI(uri string) error {
	if err := validateOpenURI(uri); err != nil {
		return err
	}
	if err := exec.Command("open", uri).Run(); err != nil {
		return fmt.Errorf("OpenURI: open: %w", err)
	}
	return nil
}

func validateOpenURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https", "mailto":
		return nil
	default:
		return fmt.Errorf("unsupported URI scheme: %q",
			u.Scheme)
	}
}

func (n *nativePlatform) ShowOpenDialog(title, startDir string, extensions []string, allowMultiple bool) gui.PlatformDialogResult {
	return filedialog.ShowOpenDialog(title, startDir, extensions, allowMultiple)
}

func (n *nativePlatform) ShowSaveDialog(title, startDir, defaultName, defaultExt string, extensions []string, confirmOverwrite bool) gui.PlatformDialogResult {
	return filedialog.ShowSaveDialog(title, startDir, defaultName, defaultExt, extensions, confirmOverwrite)
}

func (n *nativePlatform) ShowFolderDialog(title, startDir string) gui.PlatformDialogResult {
	return filedialog.ShowFolderDialog(title, startDir)
}

func (n *nativePlatform) ShowMessageDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return filedialog.ShowMessageDialog(title, body, level)
}

func (n *nativePlatform) ShowConfirmDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return filedialog.ShowConfirmDialog(title, body, level)
}

func (n *nativePlatform) ShowSaveDiscardDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return filedialog.ShowSaveDiscardDialog(title, body, level)
}

func (n *nativePlatform) SendNotification(title, body string) gui.NativeNotificationResult {
	cmd := exec.Command("osascript",
		"-e", "on run argv",
		"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
		"-e", "end run",
		"--", title, body)
	if err := cmd.Run(); err != nil {
		return gui.NativeNotificationResult{
			Status:       gui.NotificationError,
			ErrorCode:    "exec_failed",
			ErrorMessage: err.Error(),
		}
	}
	return gui.NativeNotificationResult{
		Status: gui.NotificationOK,
	}
}

func (n *nativePlatform) ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	return printdialog.ShowPrintDialog(cfg)
}

func (n *nativePlatform) BookmarkLoadAll(_ string) []gui.BookmarkEntry { return nil }
func (n *nativePlatform) BookmarkPersist(_, _ string, _ []byte)        {}
func (n *nativePlatform) BookmarkStopAccess(_ []byte)                  {}

func (n *nativePlatform) A11yInit(cb func(action, index int)) {
	a11yActionCallback = cb
	a11yInitBridge(n.window)
}

func (n *nativePlatform) A11ySync(nodes []gui.A11yNode, count, focusedIdx int) {
	_, h := n.window.GetSize()
	a11ySyncBridge(nodes, count, focusedIdx, float32(h))
}

func (n *nativePlatform) A11yDestroy() {
	a11yDestroyBridge()
}

func (n *nativePlatform) A11yAnnounce(text string) {
	a11yAnnounceBridge(text)
}

func (n *nativePlatform) IMEStart() { sdl.StartTextInput() }
func (n *nativePlatform) IMEStop()  { sdl.StopTextInput() }
func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	sdl.SetTextInputRect(&sdl.Rect{X: x, Y: y, W: w, H: h})
}
func (n *nativePlatform) TitlebarDark(_ bool) {}

func (n *nativePlatform) SpellCheck(text string) []gui.SpellRange {
	return spellcheck.Check(text)
}

func (n *nativePlatform) SpellSuggest(text string, startByte, lenBytes int) []string {
	return spellcheck.Suggest(text, startByte, lenBytes)
}

func (n *nativePlatform) SpellLearn(word string) {
	spellcheck.Learn(word)
}

func (n *nativePlatform) SetNativeMenubar(
	cfg gui.NativeMenubarCfg, actionCb func(string),
) {
	nativemenu.SetMenubar(cfg, actionCb)
}

func (n *nativePlatform) ClearNativeMenubar() {
	nativemenu.ClearMenubar()
}

func (n *nativePlatform) CreateSystemTray(
	cfg gui.SystemTrayCfg, actionCb func(string),
) (int, error) {
	return nativemenu.CreateSystemTray(cfg, actionCb)
}

func (n *nativePlatform) UpdateSystemTray(
	id int, cfg gui.SystemTrayCfg,
) {
	nativemenu.UpdateSystemTray(id, cfg)
}

func (n *nativePlatform) RemoveSystemTray(id int) {
	nativemenu.RemoveSystemTray(id)
}
