package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	glyph "github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
	datagrid "github.com/go-gui-org/go-gui/gui/datagrid"
)

func TestEntryMatchesQuery(t *testing.T) {
	entry := DemoEntry{
		ID:         "numeric_input",
		Label:      "Numeric Input",
		Group:      groupInput,
		Summary:    "Locale-aware number input with step controls.",
		Tags:       []string{"number", "decimal"},
		idLower:    "numeric_input",
		labelLower: "numeric input",
	}

	tests := []string{"numeric", "locale-aware", "input", "decimal"}
	for _, query := range tests {
		if !entryMatchesQuery(entry, query) {
			t.Fatalf("expected query %q to match entry", query)
		}
	}
}

func TestFilteredEntriesByGroupAndQuery(t *testing.T) {
	app := &ShowcaseApp{
		SelectedGroup: groupLayout,
		NavQuery:      "splitter",
	}
	entries := filteredEntries(app)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != "splitter" {
		t.Fatalf("expected splitter, got %s", entries[0].ID)
	}
}

func TestPreferredComponentForGroupPinsWelcome(t *testing.T) {
	entries := []DemoEntry{
		{ID: "doc_tables", Label: "Tables"},
		{ID: "welcome", Label: "Welcome"},
		{ID: "doc_get_started", Label: "Get Started"},
	}
	if got := preferredComponentForGroup(entries); got != "welcome" {
		t.Fatalf("expected welcome, got %s", got)
	}
}

func TestRelatedExamplesMap(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", ".."))

	for _, entry := range demoEntries {
		got := relatedExamplePaths(entry.ID)
		if len(got) == 0 {
			t.Fatalf("expected related examples for %s", entry.ID)
		}
		for _, path := range got {
			if !strings.HasPrefix(path, "examples/") {
				t.Fatalf("expected examples/* path for %s, got %q", entry.ID, path)
			}
			if strings.HasPrefix(path, "/") {
				t.Fatalf("expected relative example path for %s, got %q", entry.ID, path)
			}
			info, err := os.Stat(filepath.Join(repoRoot, path))
			if err != nil {
				t.Fatalf("expected related example path for %s to exist: %s (%v)", entry.ID, path, err)
			}
			if info.IsDir() {
				t.Fatalf("expected related example path for %s to be a file, got directory %s", entry.ID, path)
			}
		}
	}
}

func TestComponentDocsExist(t *testing.T) {
	for _, id := range []string{"data_source", "tree", "drag_reorder"} {
		if doc := componentDoc(id); doc == "" {
			t.Fatalf("expected docs for %s", id)
		}
	}
}

func TestDocPagesExist(t *testing.T) {
	if doc := docPageSource("welcome"); doc == "" {
		t.Fatal("expected doc page for welcome")
	}
}

func TestTreeTitleBarShowsDocToggle(t *testing.T) {
	layout := gui.GenerateViewLayout(viewTitleBar(DemoEntry{ //nolint:staticcheck
		ID:    "tree",
		Label: "Tree View",
	}, false), &gui.Window{})

	if _, ok := layout.FindByID("btn-doc-toggle"); !ok {
		t.Fatal("expected tree title bar to include btn-doc-toggle")
	}
}

func TestTreeTitleBarSpacerHasNoBorder(t *testing.T) {
	layout := gui.GenerateViewLayout(viewTitleBar(DemoEntry{ //nolint:staticcheck
		ID:    "tree",
		Label: "Tree View",
	}, false), &gui.Window{})
	if len(layout.Children) == 0 {
		t.Fatal("len(layout.Children) = 0, want title row")
	}

	row := layout.Children[0]
	if len(row.Children) < 2 {
		t.Fatalf("len(layout.Children[0].Children) = %d, want >= 2", len(row.Children))
	}

	spacer := row.Children[1]
	if got, want := spacer.Shape.SizeBorder, float32(0); got != want {
		t.Fatalf("layout.Children[0].Children[1].Shape.SizeBorder = %v, want %v", got, want)
	}
}

func TestDemoTreeWrapsIntroText(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{State: newShowcaseApp()})

	layout := gui.GenerateViewLayout(demoTree(w), w) //nolint:staticcheck
	if len(layout.Children) < 2 {
		t.Fatalf("len(layout.Children) = %d, want >= 2", len(layout.Children))
	}

	for idx := range 2 {
		tc := layout.Children[idx].Shape.TC
		if tc == nil {
			t.Fatalf("layout.Children[%d].Shape.TC = nil, want text config", idx)
		}
		if tc.TextMode != gui.TextModeWrap {
			t.Fatalf("layout.Children[%d].Shape.TC.TextMode = %v, want %v", idx, tc.TextMode, gui.TextModeWrap)
		}
	}
}

func TestDetailPanelSummaryWraps(t *testing.T) {
	app := newShowcaseApp()
	app.SelectedGroup = groupData
	app.SelectedComponent = "tree"
	w := gui.NewWindow(gui.WindowCfg{State: app})

	layout := gui.GenerateViewLayout(detailPanel(w), w) //nolint:staticcheck
	if len(layout.Children) < 2 {
		t.Fatalf("len(layout.Children) = %d, want >= 2", len(layout.Children))
	}

	tc := layout.Children[1].Shape.TC
	if tc == nil {
		t.Fatal("layout.Children[1].Shape.TC = nil, want summary text")
	}
	if tc.TextMode != gui.TextModeWrap {
		t.Fatalf("layout.Children[1].Shape.TC.TextMode = %v, want %v", tc.TextMode, gui.TextModeWrap)
	}
}

func TestDetailPanelWelcomeWrappersHaveNoBorder(t *testing.T) {
	app := newShowcaseApp()
	app.SelectedGroup = groupWelcome
	app.SelectedComponent = "welcome"
	w := gui.NewWindow(gui.WindowCfg{State: app})

	layout := gui.GenerateViewLayout(detailPanel(w), w) //nolint:staticcheck
	if got, want := layout.Shape.SizeBorder, float32(0); got != want {
		t.Fatalf("layout.Shape.SizeBorder = %v, want %v", got, want)
	}
	if len(layout.Children) == 0 {
		t.Fatal("len(layout.Children) = 0, want title bar")
	}

	title := layout.Children[0]
	if got, want := title.Shape.SizeBorder, float32(0); got != want {
		t.Fatalf("layout.Children[0].Shape.SizeBorder = %v, want %v", got, want)
	}
	if len(title.Children) < 2 {
		t.Fatalf("len(layout.Children[0].Children) = %d, want >= 2", len(title.Children))
	}

	line := title.Children[1]
	if got, want := line.Shape.SizeBorder, float32(0); got != want {
		t.Fatalf("layout.Children[0].Children[1].Shape.SizeBorder = %v, want %v", got, want)
	}
	// line() returns a single Row node; Height 1 renders the separator.
	if got, want := line.Shape.Height, float32(1); got != want {
		t.Fatalf("layout.Children[0].Children[1].Shape.Height = %v, want %v", got, want)
	}
}

func TestDemoWelcomePanelHasNoBorder(t *testing.T) {
	w := &gui.Window{}
	layout := gui.GenerateViewLayout(demoWelcome(w), w) //nolint:staticcheck
	if got, want := layout.Shape.SizeBorder, float32(0); got != want {
		t.Fatalf("layout.Shape.SizeBorder = %v, want %v", got, want)
	}
}

func TestShowcaseDataGridApplyQuery(t *testing.T) {
	rows := showcaseDataGridRows()
	query := datagrid.GridQueryState{
		QuickFilter: "core",
		Sorts: []datagrid.GridSort{
			{ColID: "name", Dir: datagrid.GridSortDesc},
		},
	}

	filtered := showcaseDataGridApplyQuery(rows, query)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered rows, got %d", len(filtered))
	}
	if filtered[0].Cells["name"] != "Priya" {
		t.Fatalf("expected Priya first after desc sort, got %s", filtered[0].Cells["name"])
	}
}

func TestThemeCfgSaveLoadRoundTrip(t *testing.T) {
	cfg := generateThemeCfg(
		gui.RGB(255, 85, 0),
		"analogous",
		true,
		35,
		gui.White,
		7,
		2,
	)

	path := filepath.Join(t.TempDir(), "theme.json")
	if err := themeCfgSave(path, cfg); err != nil {
		t.Fatalf("themeCfgSave failed: %v", err)
	}

	got, err := themeCfgLoad(path)
	if err != nil {
		t.Fatalf("themeCfgLoad failed: %v", err)
	}

	if got.ColorSelect != cfg.ColorSelect {
		t.Fatalf("expected color select %v, got %v", cfg.ColorSelect, got.ColorSelect)
	}
	if got.Radius != cfg.Radius {
		t.Fatalf("expected radius %.1f, got %.1f", cfg.Radius, got.Radius)
	}
	if got.SizeBorder != cfg.SizeBorder {
		t.Fatalf("expected border %.1f, got %.1f", cfg.SizeBorder, got.SizeBorder)
	}
}

func TestDemoTextLayout(t *testing.T) {
	w := &gui.Window{}
	layout := gui.GenerateViewLayout(demoText(w), w) //nolint:staticcheck

	t.Run("intro text wraps", func(t *testing.T) {
		l, ok := layout.FindByID("text-intro")
		if !ok {
			t.Fatal("text-intro not found")
		}
		if l.Shape.TC == nil || l.Shape.TC.TextMode != gui.TextModeWrap {
			t.Fatal("text-intro should use TextModeWrap")
		}
	})

	t.Run("wrap keep spaces has tab size", func(t *testing.T) {
		l, ok := layout.FindByID("text-wrap-keep-spaces")
		if !ok {
			t.Fatal("text-wrap-keep-spaces not found")
		}
		tc := l.Shape.TC
		if tc == nil {
			t.Fatal("text-wrap-keep-spaces TC is nil")
		}
		if tc.TextMode != gui.TextModeWrapKeepSpaces {
			t.Fatalf("expected TextModeWrapKeepSpaces, got %v", tc.TextMode)
		}
		if tc.TextTabSize != 8 {
			t.Fatalf("expected TabSize 8, got %d", tc.TextTabSize)
		}
	})

	t.Run("selectable block is focusable multiline", func(t *testing.T) {
		l, ok := layout.FindByID("text-selectable-block")
		if !ok {
			t.Fatal("text-selectable-block not found")
		}
		if l.Shape.IDFocus == 0 {
			t.Fatal("text-selectable-block should have IDFocus > 0")
		}
		if l.Shape.TC == nil || l.Shape.TC.TextMode != gui.TextModeMultiline {
			t.Fatal("text-selectable-block should use TextModeMultiline")
		}
	})

	t.Run("transform sections exist", func(t *testing.T) {
		for _, id := range []string{"text-transform-rotation", "text-transform-affine"} {
			if _, ok := layout.FindByID(id); !ok {
				t.Fatalf("%s not found", id)
			}
		}
	})

	t.Run("gradient sections exist", func(t *testing.T) {
		for _, id := range []string{"text-gradient-horizontal", "text-gradient-vertical"} {
			if _, ok := layout.FindByID(id); !ok {
				t.Fatalf("%s not found", id)
			}
		}
	})

	t.Run("vertical gradient config exists", func(t *testing.T) {
		l, ok := layout.FindByID("text-gradient-vertical")
		if !ok {
			t.Fatal("text-gradient-vertical not found")
		}
		if l.Shape.TC == nil || l.Shape.TC.TextStyle == nil {
			t.Fatal("text-gradient-vertical missing text config")
		}
		g := l.Shape.TC.TextStyle.Gradient
		if g == nil {
			t.Fatal("text-gradient-vertical missing gradient config")
		}
		if g.Direction != glyph.GradientVertical {
			t.Fatalf("direction = %v, want GradientVertical",
				g.Direction)
		}
	})
}

func TestFormValidationHelpers(t *testing.T) {
	snap := func(v string) gui.FormFieldSnapshot {
		return gui.FormFieldSnapshot{Value: v}
	}
	fs := gui.FormSnapshot{}

	if issues := validateUsernameFormSync(snap(""), fs); len(issues) == 0 || issues[0].Msg != "username required" {
		t.Fatalf("unexpected username required result: %v", issues)
	}
	if issues := validateUsernameFormSync(snap("ab"), fs); len(issues) == 0 || issues[0].Msg != "username min length is 3" {
		t.Fatalf("unexpected username length result: %v", issues)
	}
	if issues := validateEmailFormSync(snap("userexample.com"), fs); len(issues) == 0 || issues[0].Msg != "email must contain @" {
		t.Fatalf("unexpected email validation result: %v", issues)
	}
	if issues := validateAgeFormSync(snap(""), fs); len(issues) == 0 || issues[0].Msg != "age required" {
		t.Fatalf("unexpected age validation result: %v", issues)
	}
}

func TestValidateUsernameReserved(t *testing.T) {
	snap := func(v string) gui.FormFieldSnapshot {
		return gui.FormFieldSnapshot{Value: v}
	}
	fs := gui.FormSnapshot{}

	if issues := validateUsernameFormAsync(snap("admin"), fs, nil); len(issues) == 0 || issues[0].Msg != "username already taken" {
		t.Fatalf("unexpected reserved username result: %v", issues)
	}
	if issues := validateUsernameFormAsync(snap("available"), fs, nil); len(issues) != 0 {
		t.Fatalf("expected no issue for available, got %v", issues)
	}
}

func TestDetailPanel_BumpsAbortCounterWhenNavigatingAwayFromTree(t *testing.T) {
	app := newShowcaseApp()
	app.SelectedGroup = groupData
	app.SelectedComponent = "tree"
	// Seed lazy nodes so the test can verify they are cleared on
	// navigation-away.
	app.TreeLazyNodes = map[string][]gui.TreeNodeCfg{
		"remote_a": {{ID: "a", Text: "a"}},
	}
	w := gui.NewWindow(gui.WindowCfg{State: app})

	// Staying on "tree" should not bump the abort counter or clear
	// lazy nodes.
	abortBefore := app.TreeLazyLoadAbort
	_ = gui.GenerateViewLayout(detailPanel(w), w) //nolint:staticcheck
	if app.TreeLazyLoadAbort != abortBefore {
		t.Fatal("abort counter bumped while still viewing tree demo")
	}
	if len(app.TreeLazyNodes) == 0 {
		t.Fatal("TreeLazyNodes cleared while still viewing tree demo")
	}

	// Navigate to "button" — abort counter increments and lazy nodes
	// are discarded.
	app.SelectedComponent = "button"
	app.SelectedGroup = groupButtons
	_ = gui.GenerateViewLayout(detailPanel(w), w) //nolint:staticcheck
	if app.TreeLazyLoadAbort != abortBefore+1 {
		t.Fatalf("TreeLazyLoadAbort = %d, want %d",
			app.TreeLazyLoadAbort, abortBefore+1)
	}
	if len(app.TreeLazyNodes) != 0 {
		t.Fatalf("TreeLazyNodes not cleared on nav-away, got %d entries",
			len(app.TreeLazyNodes))
	}
}
