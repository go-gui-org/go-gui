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

// docPageExempt names the catalog entries that carry no widget doc.
// These pages build their own markdown source rather than reading it
// from widgetDocFiles. Everything else must have a doc, which is what
// keeps a new demo from shipping with an empty Docs panel.
var docPageExempt = map[string]bool{
	"welcome":  true,
	"commands": true,
	"sound":    true,
}

func TestComponentDocsExist(t *testing.T) {
	for _, entry := range demoEntries {
		if docPageExempt[entry.ID] {
			continue
		}
		if doc := componentDoc(entry.ID); doc == "" {
			t.Errorf("no docs for %q; add docs/widget_%s.md and "+
				"register it in widgetDocFiles", entry.ID, entry.ID)
		}
	}
}

func TestDocPagesExist(t *testing.T) {
	// Every registered doc-only page must resolve: the nav entry, the
	// detail branch and the embedded file have to agree, and a missing
	// mirror is otherwise only visible at runtime.
	for id := range docPageFiles {
		if doc := docPageSource(id); doc == "" {
			t.Errorf("expected doc page for %s", id)
		}
	}
	for _, id := range []string{"welcome", "commands", "sound"} {
		if doc := docPageSource(id); doc == "" {
			t.Errorf("expected doc page for %s", id)
		}
	}
}

// soundWiringSpy stands in for the real cue player, which needs an
// audio device.
type soundWiringSpy struct{ cues []gui.SoundCue }

func (s *soundWiringSpy) PlaySound(c gui.SoundCue, _ float32) {
	s.cues = append(s.cues, c)
}

func (s *soundWiringSpy) SoundAvailable() bool { return true }

// Exercises the showcase's own opt-in path end to end: the theme
// rebuild inside installWidgetSounds, then a click on a real showcase
// button. Without this the wiring is only observable by ear.
func TestShowcaseWidgetSoundWiring(t *testing.T) {
	app := newShowcaseApp()
	app.SelectedGroup = groupFeedback
	app.SelectedComponent = "audio"
	w := gui.NewTestWindow(gui.WindowCfg{State: app})
	w.SetView(mainView)

	installWidgetSounds(w, soundPlayerSynth)
	spy := &soundWiringSpy{}
	w.SetSoundPlayer(spy)
	w.SetSoundVolume(0.5)
	w.TestRender(nil)

	// The demo panel sits under the "detail" scope, so the effective ID
	// is what the public Test* API takes.
	if err := w.TestClick("detail:widget-sound-click"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if len(spy.cues) != 1 || spy.cues[0] != gui.SoundClick {
		t.Fatalf("cues = %v, want [SoundClick]", spy.cues)
	}

	// SoundDisabled on one instance must stay silent.
	spy.cues = nil
	if err := w.TestClick("detail:widget-sound-muted"); err != nil {
		t.Fatalf("TestClick muted: %v", err)
	}
	if len(spy.cues) != 0 {
		t.Fatalf("SoundDisabled button emitted %v", spy.cues)
	}
}

// The player Select maps labels onto player kinds and back; a slipped
// mapping installs a different player than the one the user picked.
func TestSoundPlayerKindRoundTrip(t *testing.T) {
	for _, kind := range []soundPlayerKind{
		soundPlayerSynth, soundPlayerSystem, soundPlayerBeep,
	} {
		label := soundPlayerValue(kind)
		if got := soundPlayerKindFor(label); got != kind {
			t.Errorf("round trip %v → %q → %v", kind, label, got)
		}
	}
	// An unrecognised label falls back to the synthesized player.
	if got := soundPlayerKindFor("nonsense"); got != soundPlayerSynth {
		t.Errorf("unknown label → %v, want soundPlayerSynth", got)
	}
}

func TestTreeTitleBarShowsDocToggle(t *testing.T) {
	layout := gui.GenerateViewLayout(viewTitleBar(DemoEntry{
		ID:    "tree",
		Label: "Tree View",
	}, false), &gui.Window{})

	if _, ok := layout.FindByID("btn-doc-toggle"); !ok {
		t.Fatal("expected tree title bar to include btn-doc-toggle")
	}
}

func TestTreeTitleBarSpacerHasNoBorder(t *testing.T) {
	layout := gui.GenerateViewLayout(viewTitleBar(DemoEntry{
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

	layout := gui.GenerateViewLayout(demoTree(w), w)
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

	layout := gui.GenerateViewLayout(detailPanel(w), w)
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

	layout := gui.GenerateViewLayout(detailPanel(w), w)
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
	// line() is a gui.Separator: a transparent inset wrapper around the
	// rule node.
	if len(line.Children) == 0 {
		t.Fatal("len(layout.Children[0].Children[1].Children) = 0, want the rule")
	}
	rule := line.Children[0]
	if got, want := rule.Shape.SizeBorder, float32(0); got != want {
		t.Fatalf("rule.Shape.SizeBorder = %v, want %v", got, want)
	}
	if got, want := rule.Shape.Height, float32(1); got != want {
		t.Fatalf("rule.Shape.Height = %v, want %v", got, want)
	}
}

func TestDemoWelcomePanelHasNoBorder(t *testing.T) {
	w := &gui.Window{}
	layout := gui.GenerateViewLayout(demoWelcome(w), w)
	if got, want := layout.Shape.SizeBorder, float32(0); got != want {
		t.Fatalf("layout.Shape.SizeBorder = %v, want %v", got, want)
	}
}

// The data-source grid fits its contents: columns plus gutter and
// borders size the grid instead of stretching to the panel (issue
// #642 keeps the Fill alternative pinned in gui/datagrid for narrow
// panels; this demo opts into Fit at its catalog width).
func TestDemoDataSourceGridFitsContent(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{State: newShowcaseApp()})
	layout := gui.GenerateViewLayout(demoDataSource(w), w)
	grid, ok := layout.FindByID("catalog-data-source")
	if !ok {
		t.Fatal("catalog-data-source not found")
	}
	if grid.Shape.Sizing.Width != gui.FitFit.Width {
		t.Fatalf("data-source grid width sizing = %v, want Fit",
			grid.Shape.Sizing.Width)
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

// TestThemeGenFieldsPresent guards the five density knobs on the theme
// maker page. They are built from one shared helper, so a wiring
// mistake would drop all of them at once rather than fail visibly.
func TestThemeGenFieldsPresent(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{State: newShowcaseApp()})
	layout := gui.GenerateViewLayout(demoThemeGen(w), w)

	for _, id := range []string{
		"theme-gen-radius",
		"theme-gen-border",
		"theme-gen-pad",
		"theme-gen-scrollbar",
		"theme-gen-scroll-gap",
	} {
		if _, ok := layout.FindByID(id); !ok {
			t.Errorf("theme maker page is missing %s", id)
		}
	}
}

// TestGenerateThemeCfgDerivesPaddingLadder pins the "one Pad value
// drives the ladder" rule, including that a zero scrollbar offset
// survives as an explicit choice.
func TestGenerateThemeCfgDerivesPaddingLadder(t *testing.T) {
	cfg := generateThemeCfg(
		gui.RGB(0, 128, 255),
		"mono",
		true,
		0,
		gui.White,
		themeGenSizes{Radius: 6, Border: 1, Pad: 10, Scrollbar: 12, ScrollGap: 0},
	)

	if got, want := cfg.Padding.Top, float32(10); got != want {
		t.Errorf("cfg.Padding.Top = %v, want %v", got, want)
	}
	if got, want := cfg.PaddingSmall.Top, float32(5); got != want {
		t.Errorf("cfg.PaddingSmall.Top = %v, want %v", got, want)
	}
	if got, want := cfg.PaddingLarge.Top, float32(15); got != want {
		t.Errorf("cfg.PaddingLarge.Top = %v, want %v", got, want)
	}
	if got, want := cfg.SizeScrollbar, float32(12); got != want {
		t.Errorf("cfg.SizeScrollbar = %v, want %v", got, want)
	}
	if gap, ok := cfg.SizeScrollbarGap.Value(); !ok || gap != 0 {
		t.Errorf("cfg.SizeScrollbarGap = %v set=%v, want 0 set=true", gap, ok)
	}
}

func TestThemeCfgSaveLoadRoundTrip(t *testing.T) {
	cfg := generateThemeCfg(
		gui.RGB(255, 85, 0),
		"analogous",
		true,
		35,
		gui.White,
		themeGenSizes{
			Radius:    7,
			Border:    2,
			Pad:       9,
			Scrollbar: 11,
			ScrollGap: 0,
		},
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
	if got.Padding != cfg.Padding {
		t.Fatalf("expected padding %v, got %v", cfg.Padding, got.Padding)
	}
	if got.PaddingLarge != cfg.PaddingLarge {
		t.Fatalf("expected large padding %v, got %v", cfg.PaddingLarge, got.PaddingLarge)
	}
	if got.SizeScrollbar != cfg.SizeScrollbar {
		t.Fatalf("expected scrollbar %.1f, got %.1f", cfg.SizeScrollbar, got.SizeScrollbar)
	}
	// ScrollGap 0 is the interesting case: it must come back as an
	// explicitly-set zero, not as unset.
	gap, ok := got.SizeScrollbarGap.Value()
	if !ok || gap != 0 {
		t.Fatalf("expected scrollbar gap set to 0, got %v set=%v", gap, ok)
	}
}

func TestDemoTextLayout(t *testing.T) {
	w := &gui.Window{}
	layout := gui.GenerateViewLayout(demoText(w), w)

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
		if !l.Shape.Focusable {
			t.Fatal("text-selectable-block should be focusable")
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

func TestDemoTypographyLayout(t *testing.T) {
	w := &gui.Window{}
	layout := gui.GenerateViewLayout(demoTypography(w), w)

	t.Run("intro text wraps", func(t *testing.T) {
		l, ok := layout.FindByID("typo-intro")
		if !ok {
			t.Fatal("typo-intro not found")
		}
		if l.Shape.TC == nil || l.Shape.TC.TextMode != gui.TextModeWrap {
			t.Fatal("typo-intro should use TextModeWrap")
		}
	})

	t.Run("scale card holds specimens plus sized rows", func(t *testing.T) {
		card, ok := layout.FindByID("typo-scale")
		if !ok {
			t.Fatal("typo-scale not found")
		}
		// A card title, five plain specimens from
		// typographyScaleSpecimens, then six labeled rows from
		// typographyScaleExtraRows.
		if len(card.Children) != 12 {
			t.Fatalf("len(typo-scale.Children) = %d, want 12",
				len(card.Children))
		}
		first := card.Children[1].Shape.TC
		if first == nil || first.Text != "Display" {
			t.Fatalf("first specimen = %v, want Display", first)
		}
		for i := range 6 {
			id := gui.ScopeIDN("typo-scale", "row", i)
			if _, ok := layout.FindByID(id); !ok {
				t.Fatalf("%s not found", id)
			}
		}
	})

	t.Run("roles and modifiers cards exist", func(t *testing.T) {
		for _, id := range []string{"typo-roles", "typo-modifiers"} {
			if _, ok := layout.FindByID(id); !ok {
				t.Fatalf("%s not found", id)
			}
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
	_ = gui.GenerateViewLayout(detailPanel(w), w)
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
	_ = gui.GenerateViewLayout(detailPanel(w), w)
	if app.TreeLazyLoadAbort != abortBefore+1 {
		t.Fatalf("TreeLazyLoadAbort = %d, want %d",
			app.TreeLazyLoadAbort, abortBefore+1)
	}
	if len(app.TreeLazyNodes) != 0 {
		t.Fatalf("TreeLazyNodes not cleared on nav-away, got %d entries",
			len(app.TreeLazyNodes))
	}
}

// Every demo page must be free of identity defects: no two shapes
// sharing an ID, and no ID-less shape claiming focus, scroll, or
// OnMouseLeave. These collapse two widgets onto one tab stop and one
// state slot without producing any error at runtime, so the check has
// to be an assertion rather than something noticed on stderr.
//
// The sweep sees the window as rendered, which is why it walks every
// entry: a widget behind an unselected catalog item is not in the tree.
func TestDemoPagesHaveNoIDDefects(t *testing.T) {
	w := gui.NewTestWindow(gui.WindowCfg{State: newShowcaseApp()})
	w.SetView(mainView)
	app := appState(w)

	for _, entry := range demoEntries {
		app.SelectedGroup = entry.Group
		app.SelectedComponent = entry.ID
		app.ShowDocs = false
		if found := w.TestDuplicateIDs(); len(found) > 0 {
			t.Errorf("demo %q: %v", entry.ID, found)
		}

		// The docs view of a page is a separate tree.
		app.ShowDocs = true
		if found := w.TestDuplicateIDs(); len(found) > 0 {
			t.Errorf("demo %q docs: %v", entry.ID, found)
		}
		app.ShowDocs = false
	}
}

// The color picker demo shows both the packed widget and the
// components it is built from. Generating it is what proves the
// composable controls survive a real app's state wiring.
func TestDemoColorPickerBuildsComponents(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{State: newShowcaseApp()})
	layout := gui.GenerateViewLayout(demoColorPicker(w), w)

	for _, id := range []string{
		"color-picker",
		"color-parts-plane",
		"color-parts-wheel",
		"color-parts-swatch",
		"color-parts-hue",
		"color-parts-alpha",
	} {
		if _, ok := layout.FindByID(id); !ok {
			t.Errorf("color picker demo is missing %q", id)
		}
	}
}

// TestThemeCfgLoadOmittedGapIsUnset covers the nil arm of
// optFloatFromJSON: a theme file written before the offset existed has
// no size_scrollbar_gap key, and must load as unset — not as an
// explicit zero, which would flatten the bar against the edge.
func TestThemeCfgLoadOmittedGapIsUnset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(path, []byte(`{"name":"legacy","radius":6}`), 0o600); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	got, err := themeCfgLoad(path)
	if err != nil {
		t.Fatalf("themeCfgLoad failed: %v", err)
	}
	if got.SizeScrollbarGap.IsSet() {
		t.Errorf("SizeScrollbarGap is set, want unset")
	}
	// Unset must reach ThemeMaker's built-in inset, not zero.
	if gap := gui.ThemeMaker(got).ScrollbarStyle.GapEdge; gap != themeGenDefaultScrollGap {
		t.Errorf("GapEdge = %v, want %v", gap, themeGenDefaultScrollGap)
	}
}

// TestSyncThemeGenFromCfgMirrorsDensity pins the read-back path: after
// Load Theme or a preset reset, the three density knobs must show the
// values the cfg actually carries, text buffers included.
func TestSyncThemeGenFromCfgMirrorsDensity(t *testing.T) {
	app := newShowcaseApp()
	cfg := gui.ThemeDark.Cfg
	cfg.Padding = gui.PadAll(9)
	cfg.SizeScrollbar = 11
	cfg.SizeScrollbarGap = gui.SomeF(0)

	syncThemeGenFromCfg(app, cfg)

	if got, want := app.ThemeGenPad, float32(9); got != want {
		t.Errorf("ThemeGenPad = %v, want %v", got, want)
	}
	if got, want := app.ThemeGenPadText, floatString(9); got != want {
		t.Errorf("ThemeGenPadText = %q, want %q", got, want)
	}
	if got, want := app.ThemeGenScrollbar, float32(11); got != want {
		t.Errorf("ThemeGenScrollbar = %v, want %v", got, want)
	}
	// An explicit zero gap must show as 0, not as the fallback 3.
	if got := app.ThemeGenScrollGap; got != 0 {
		t.Errorf("ThemeGenScrollGap = %v, want 0", got)
	}
}

// TestSyncThemeGenResetDarkKeepsSeed pins the reset-dark seed: the
// dark and light preset cfgs leave ColorSelect unset (it resolves to
// the accent in ThemeMaker), so syncing from them must fall back to
// the accent rather than an unset color, which the picker renders as
// red (255, 0, 0).
func TestSyncThemeGenResetDarkKeepsSeed(t *testing.T) {
	for _, cfg := range []gui.ThemeCfg{gui.ThemeDark.Cfg, gui.ThemeLight.Cfg} {
		app := newShowcaseApp()
		syncThemeGenFromCfg(app, cfg)
		if !app.ThemeGenSeed.IsSet() {
			t.Fatalf("ThemeGenSeed unset after sync from %q", cfg.Name)
		}
		if got, want := app.ThemeGenSeed, cfg.ColorAccent; got != want {
			t.Errorf("ThemeGenSeed from %q = %v, want accent %v", cfg.Name, got, want)
		}
	}
	if got := newShowcaseApp().ThemeGenSeed; !got.IsSet() {
		t.Errorf("newShowcaseApp ThemeGenSeed unset, want %v", gui.ThemeDark.ColorSelect)
	}
}

// TestGenerateThemeCfgZeroTintRestoresPreset pins the reset round-trip:
// syncing from a preset then regenerating at tint 0 must reproduce the
// preset surfaces verbatim. The generator used to rebuild every surface
// from its own value ladder even at 0 (dark bg 48,48,48 vs preset
// 23,25,28), so sliding back to 0 never restored the preset.
func TestGenerateThemeCfgZeroTintRestoresPreset(t *testing.T) {
	for _, preset := range []gui.Theme{gui.ThemeDark, gui.ThemeLight} {
		app := newShowcaseApp()
		syncThemeGenFromCfg(app, preset.Cfg)
		cfg := generateThemeCfg(
			app.ThemeGenSeed, app.ThemeGenStrategy,
			preset.TitlebarDark, app.ThemeGenTint, app.ThemeGenText,
			themeGenSizesOf(app),
		)
		got := gui.ThemeMaker(cfg)
		surfaces := []struct {
			name string
			got  gui.Color
			want gui.Color
		}{
			{"background", got.ColorBackground, preset.ColorBackground},
			{"panel", got.ColorPanel, preset.ColorPanel},
			{"interior", got.ColorInterior, preset.ColorInterior},
			{"hover", got.ColorHover, preset.ColorHover},
			{"focus", got.ColorFocus, preset.ColorFocus},
			{"active", got.ColorActive, preset.ColorActive},
			{"border", got.ColorBorder, preset.ColorBorder},
		}
		for _, s := range surfaces {
			if s.got != s.want {
				t.Errorf("%s tint-0 %s = %v, want preset %v",
					preset.Cfg.Name, s.name, s.got, s.want)
			}
		}
	}
}

// TestGenerateThemeCfgTintOneIsSubtle pins the slider meaning: tint is
// a blend amount, so 1% must sit within a few levels of the preset. The
// old ladder rebuild jumped ~25 levels at any tint above 0.
func TestGenerateThemeCfgTintOneIsSubtle(t *testing.T) {
	for _, preset := range []gui.Theme{gui.ThemeDark, gui.ThemeLight} {
		app := newShowcaseApp()
		syncThemeGenFromCfg(app, preset.Cfg)
		cfg := generateThemeCfg(
			app.ThemeGenSeed, app.ThemeGenStrategy,
			preset.TitlebarDark, 1, app.ThemeGenText,
			themeGenSizesOf(app),
		)
		got := gui.ThemeMaker(cfg).ColorBackground
		for _, ch := range []struct {
			name string
			got  uint8
			want uint8
		}{
			{"R", got.R, preset.ColorBackground.R},
			{"G", got.G, preset.ColorBackground.G},
			{"B", got.B, preset.ColorBackground.B},
		} {
			d := int(ch.got) - int(ch.want)
			if d < -4 || d > 4 {
				t.Errorf("%s tint-1 bg.%s = %v, want within 4 of %v (got %v)",
					preset.Cfg.Name, ch.name, ch.got, ch.want, got)
			}
		}
	}
}

func TestDemoTextAnimLayout(t *testing.T) {
	// demoTextAnim reads app state for the replay counter, so this
	// needs a window with the state slot filled, not a bare one.
	w := gui.NewTestWindow(gui.WindowCfg{State: newShowcaseApp()})
	layout := gui.GenerateViewLayout(demoTextAnim(w), w)

	t.Run("entrance labels carry the replay counter", func(t *testing.T) {
		// The counter is part of the ID, not a separate knob: that is
		// what makes a click replay a one-shot animation.
		for _, part := range []string{"fade", "slide-up", "slide-left", "pop"} {
			id := gui.ScopeIDN("text-anim", part, 0)
			if _, ok := layout.FindByID(id); !ok {
				t.Fatalf("%s not found", id)
			}
		}
	})

	t.Run("loop and custom labels exist", func(t *testing.T) {
		for _, id := range []string{
			"text-anim-pulse", "text-anim-shimmer", "text-anim-shake",
			"text-anim-typewriter", "text-anim-custom",
		} {
			if _, ok := layout.FindByID(id); !ok {
				t.Fatalf("%s not found", id)
			}
		}
	})

	t.Run("replay button exists", func(t *testing.T) {
		if _, ok := layout.FindByID("text-anim-replay"); !ok {
			t.Fatal("text-anim-replay not found")
		}
	})
}

// The fixed-size demo boxes center their label: theme container
// padding used to fill the whole box, pinning the text to the
// padding edge so it touched the box bottom.
func TestDemoBoxSizedCentersLabel(t *testing.T) {
	w := gui.NewTestWindow(gui.WindowCfg{Width: 400, Height: 200})
	defer w.Close()
	root := w.TestRender(func(*gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Content: []gui.View{demoBoxSized("H1", gui.Gray, 80, 30)},
		})
	})
	box := &root.Children[0].Children[0]
	text := &box.Children[0]
	// Metric centre plus the optical correction: the label sits below
	// metric centre by the cap-band offset (headless fallback:
	// 0.0886 * size for "H1", which has no descenders).
	metric := (box.Shape.Height - text.Shape.Height) / 2
	off := gui.CurrentTheme().TextStyleBodyLarge.Size * 0.0886
	if got := text.Shape.Y - box.Shape.Y; got-metric-off > 0.01 || metric+off-got > 0.01 {
		t.Fatalf("label offset = %v, want %v (box h=%v text h=%v)",
			got, metric+off, box.Shape.Height, text.Shape.Height)
	}
}
