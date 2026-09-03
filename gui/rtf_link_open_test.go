package gui

// rtf_link_open_test.go covers RTF link activation (issue #488): the
// single rtfOpenLink path shared by left-click and the "Open Link"
// context-menu action, the anchor branch on both, and the links that
// cannot be opened at all — reported through the Debug gate instead of
// being handed to the platform opener, which rejects them silently.

import (
	"errors"
	"strings"
	"testing"
)

// rtfOpenErrPlatform fails every OpenURI call, standing in for a
// missing xdg-open or a non-zero exit from the platform handler.
type rtfOpenErrPlatform struct {
	noopNativePlatform
	calls int
}

func (p *rtfOpenErrPlatform) OpenURI(_ string) error {
	p.calls++
	return errors.New("no handler")
}

// rtfLinkDebugWindow returns a window with the Debug gate collecting
// findings into the returned slice instead of writing to stderr.
func rtfLinkDebugWindow(t *testing.T) (*Window, *[]string) {
	t.Helper()
	prevOn := DebugEnabled()
	Debug(true)
	t.Cleanup(func() { Debug(prevOn) })

	w := newTestWindow()
	found := new([]string)
	w.debug.collect = found
	return w, found
}

// TestRtfOpenLinkAbsoluteReachesPlatform asserts an allowlisted scheme
// still goes to the platform opener and reports nothing.
func TestRtfOpenLinkAbsoluteReachesPlatform(t *testing.T) {
	w, found := rtfLinkDebugWindow(t)
	p := &rtfURLCapturePlatform{}
	w.nativePlatform = p

	rtfOpenLink(w, "https://example.com/a", "")

	if p.opened != "https://example.com/a" {
		t.Fatalf("OpenURI = %q, want the link", p.opened)
	}
	if len(*found) != 0 {
		t.Fatalf("unexpected findings: %v", *found)
	}
}

// TestRtfOpenLinkRelativeIsReportedNotOpened asserts a relative link —
// which markdown.IsSafeURL admits, so it renders as a link — is never
// handed to OpenURI, whose allowlist would reject it silently.
func TestRtfOpenLinkRelativeIsReportedNotOpened(t *testing.T) {
	for _, link := range []string{
		"/docs/x", "./y", "../z", "?q=1", "docs/x.md",
	} {
		w, found := rtfLinkDebugWindow(t)
		p := &rtfURLCapturePlatform{}
		w.nativePlatform = p

		rtfOpenLink(w, link, "")

		if p.opened != "" {
			t.Errorf("link %q: OpenURI called with %q, want no call",
				link, p.opened)
		}
		if len(*found) != 1 ||
			!strings.Contains((*found)[0], link) {
			t.Errorf("link %q: findings = %v, want one naming it",
				link, *found)
		}
	}
}

// TestRtfOpenLinkPlatformErrorIsReported asserts a failing opener is
// reported rather than discarded.
func TestRtfOpenLinkPlatformErrorIsReported(t *testing.T) {
	w, found := rtfLinkDebugWindow(t)
	p := &rtfOpenErrPlatform{}
	w.nativePlatform = p

	rtfOpenLink(w, "https://example.com", "")

	if p.calls != 1 {
		t.Fatalf("OpenURI calls = %d, want 1", p.calls)
	}
	if len(*found) != 1 ||
		!strings.Contains((*found)[0], "no handler") {
		t.Fatalf("findings = %v, want one naming the error", *found)
	}
}

// TestRtfOpenLinkUnknownAnchorIsReported asserts an anchor naming no
// shape reports instead of falling through to the platform.
func TestRtfOpenLinkUnknownAnchorIsReported(t *testing.T) {
	w, found := rtfLinkDebugWindow(t)
	p := &rtfURLCapturePlatform{}
	w.nativePlatform = p

	rtfOpenLink(w, "#nowhere", "md")

	if p.opened != "" {
		t.Fatalf("OpenURI called with %q, want no call", p.opened)
	}
	if len(*found) != 1 ||
		!strings.Contains((*found)[0], "#nowhere") {
		t.Fatalf("findings = %v, want one naming the anchor", *found)
	}
}

// TestShowLinkContextMenuCarriesMarkdownID asserts the document scope
// travels into the menu state: the Action callback runs with no RTF
// layout in reach, so it cannot read TC.markdownID back.
func TestShowLinkContextMenuCarriesMarkdownID(t *testing.T) {
	w := newTestWindow()
	showLinkContextMenu(w, "#heading", 10, 20, 7, "panel:md")

	st := StateReadOr(
		w, nsRtfLinkMenu, nsRtfLinkMenu, rtfLinkMenuState{})
	if st.MarkdownID != "panel:md" {
		t.Fatalf("MarkdownID = %q, want %q", st.MarkdownID, "panel:md")
	}
}

// TestRtfLinkMenuOpenAnchorScrollsToView is the issue's repro: right-
// click an in-document anchor, choose "Open Link", and the view must
// scroll the same way a left-click does. Before the fix the menu
// action handed "#heading" to OpenURI, which rejected it.
func TestRtfLinkMenuOpenAnchorScrollsToView(t *testing.T) {
	w := mdAnchorWindow(t, mdAnchorSource, false, false, false)

	block, ok := mdAnchorFindBlock(&w.layout, "view:md", "jump")
	if !ok {
		t.Fatal("no link block in the window")
	}
	x, y := block.Shape.X+15, block.Shape.Y+10
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseRight,
		MouseX: x, MouseY: y})
	w.settle()

	st := StateReadOr(
		w, nsRtfLinkMenu, nsRtfLinkMenu, rtfLinkMenuState{})
	if !st.Open || st.Link != "#heading" {
		t.Fatalf("menu state = %+v, want open on #heading", st)
	}

	if _, before, err := w.TestScrollOffset("view"); err != nil {
		t.Fatalf("scroll offset before: %v", err)
	} else if before != 0 {
		t.Fatalf("scroll offset before = %g, want 0", before)
	}

	if err := w.TestClick(
		ScopeID(rtfLinkMenuFocusID, "open_link")); err != nil {
		t.Fatalf("click Open Link: %v", err)
	}
	w.settle()

	_, after, err := w.TestScrollOffset("view")
	if err != nil {
		t.Fatalf("scroll offset after: %v", err)
	}
	// scrollToView pins the target to the container top; the sign of
	// the resulting offset is the scroll system's, so assert movement.
	if after == 0 {
		t.Fatalf("scroll offset after = 0, want the view moved")
	}
}

// TestRtfOpenLinkTrimsWhitespace asserts a link the render gate saw as
// an anchor (markdown.IsSafeURL trims before classifying) still takes
// the anchor branch here, rather than reading the leading space and
// reporting the link as unopenable.
func TestRtfOpenLinkTrimsWhitespace(t *testing.T) {
	w := mdAnchorWindow(t, mdAnchorSource, false, false, false)
	prevOn := DebugEnabled()
	Debug(true)
	defer Debug(prevOn)
	var found []string
	w.debug.collect = &found

	rtfOpenLink(w, "  #heading\n", "view:md")

	if len(found) != 0 {
		t.Fatalf("findings = %v, want none", found)
	}
	if _, sy, err := w.TestScrollOffset("view"); err != nil {
		t.Fatalf("scroll offset: %v", err)
	} else if sy == 0 {
		t.Fatal("padded anchor did not scroll the container")
	}
}

// TestRtfOpenLinkBlankLinkIsInert asserts an empty or whitespace-only
// link reports nothing and calls nothing: there is no user mistake to
// name and no target to reach.
func TestRtfOpenLinkBlankLinkIsInert(t *testing.T) {
	for _, link := range []string{"", "   "} {
		w, found := rtfLinkDebugWindow(t)
		p := &rtfURLCapturePlatform{}
		w.nativePlatform = p

		rtfOpenLink(w, link, "")

		if p.opened != "" || len(*found) != 0 {
			t.Errorf("link %q: opened %q, findings %v; want neither",
				link, p.opened, *found)
		}
	}
}

// TestRtfOpenLinkNilPlatformIsInert asserts the headless case — tests
// and any window with no native platform injected — neither panics nor
// reports: the link is fine, the opener is simply absent.
func TestRtfOpenLinkNilPlatformIsInert(t *testing.T) {
	w, found := rtfLinkDebugWindow(t)
	w.nativePlatform = nil

	rtfOpenLink(w, "https://example.com", "")

	if len(*found) != 0 {
		t.Fatalf("findings = %v, want none", *found)
	}
}

// TestRtfOpenLinkReportIsBounded asserts a long link is shortened
// before it becomes a message and a warn-once key: link text is the
// document author's, and the key is retained for the window's life.
func TestRtfOpenLinkReportIsBounded(t *testing.T) {
	w, found := rtfLinkDebugWindow(t)
	long := "/docs/" + strings.Repeat("x", 4096)

	rtfOpenLink(w, long, "")

	if len(*found) != 1 {
		t.Fatalf("findings = %d, want 1", len(*found))
	}
	if len((*found)[0]) > rtfLinkMaxReportLen+200 {
		t.Fatalf("finding is %d bytes, want the link shortened",
			len((*found)[0]))
	}
	if !strings.Contains((*found)[0], "...") {
		t.Fatalf("finding = %q, want the shortened marker", (*found)[0])
	}
}

// TestRtfOpenLinkReportsOncePerLink asserts the warn-once key is the
// link, so a repeatedly clicked bad link does not flood stderr.
func TestRtfOpenLinkReportsOncePerLink(t *testing.T) {
	w, found := rtfLinkDebugWindow(t)

	rtfOpenLink(w, "/docs/x", "")
	rtfOpenLink(w, "/docs/x", "")
	rtfOpenLink(w, "/docs/y", "")

	if len(*found) != 2 {
		t.Fatalf("findings = %v, want one per distinct link", *found)
	}
}

// TestRtfLinkIsOpenable pins the scheme allowlist mirrored from
// nativehost.ValidateOpenURI, including the case-insensitivity a URI
// scheme has and the schemes that must never reach the opener.
func TestRtfLinkIsOpenable(t *testing.T) {
	for link, want := range map[string]bool{
		"http://a":    true,
		"HTTPS://a":   true,
		"MailTo:a@b":  true,
		"http:/a":     false,
		"httpx://a":   false,
		"javascript:": false,
		"file:///a":   false,
		"#a":          false,
		"/a":          false,
		"":            false,
		"htt":         false,
	} {
		if got := rtfLinkIsOpenable(link); got != want {
			t.Errorf("rtfLinkIsOpenable(%q) = %v, want %v",
				link, got, want)
		}
	}
}

// TestRtfSelectLinkClickDoesNotArmDrag is the reported symptom: in a
// selectable RTF the link click armed drag-select as well as
// navigating. The lock survives the release — the release lands on
// whatever the navigation put under the pointer — so moving the mouse
// afterwards kept extending the selection with no button held.
func TestRtfSelectLinkClickDoesNotArmDrag(t *testing.T) {
	h := newRtfSelectHarness(t, rtfTwoRuns())
	h.w.SetNativePlatform(&rtfURLCapturePlatform{})

	x, y := rtfRunePoint(h.shape(t), 8) // inside the link run
	h.press(x, y)

	if h.w.mouseIsLocked() {
		t.Fatal("link click armed drag-select")
	}

	// The pointer moves with no button held. Nothing may extend the
	// selection the click collapsed.
	h.release(x, y)
	x0, y0 := rtfRunePoint(h.shape(t), 0)
	h.move(x0, y0)
	expectRtfSel(t, h, 8, 8)
}

// TestRtfSelectPlainClickStillArmsDrag is the other half: a click on a
// non-link run must keep locking the mouse, or the fix above would
// have taken drag-select away from the widget entirely.
func TestRtfSelectPlainClickStillArmsDrag(t *testing.T) {
	h := newRtfSelectHarness(t, rtfTwoRuns())

	x, y := rtfRunePoint(h.shape(t), 2) // inside the plain run
	h.press(x, y)

	if !h.w.mouseIsLocked() {
		t.Fatal("plain click did not arm drag-select")
	}
	h.release(x, y)
}
