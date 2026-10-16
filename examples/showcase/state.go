package main

import (
	"cmp"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	datagrid "github.com/go-gui-org/go-gui/gui/datagrid"
)

// Catalog groups. An entry goes in the group of the job it does for
// the user (#1025): start here, act, choose, enter a value, show text,
// show data, arrange, navigate, give a message, show progress, move,
// draw, or control the app and window. A new entry takes the group of
// its job, not the group of the widget it looks like.
const (
	groupStart    = "start"
	groupAll      = "all"
	groupActions  = "actions"
	groupChoices  = "choices"
	groupEntry    = "entry"
	groupText     = "text"
	groupData     = "data"
	groupLayout   = "layout"
	groupNav      = "navigation"
	groupMessages = "messages"
	groupProgress = "progress"
	groupMotion   = "motion"
	groupDrawing  = "drawing"
	groupApp      = "app"
)

var showcaseBreadcrumbPath = []gui.BreadcrumbItemCfg{
	gui.NewBreadcrumbItem("home", "Home", nil),
	gui.NewBreadcrumbItem("docs", "Docs", nil),
	gui.NewBreadcrumbItem("guide", "Guide", nil),
	gui.NewBreadcrumbItem("page", "Getting Started", nil),
}

// ShowcaseApp holds all state for the showcase example.
type ShowcaseApp struct {
	DataGridSelection   datagrid.GridSelection
	DataSourceSelection datagrid.GridSelection
	ButtonCopyUntil     time.Time

	InputDate  time.Time
	RollerDate time.Time

	ShaderStartTime time.Time
	DataSource      datagrid.DataGridDataSource

	TableSelected     map[int]bool
	TreeLazyNodes     map[string][]gui.TreeNodeCfg
	TreeLazyLoadAbort uint64

	DockRoot *gui.DockNode

	Form FormModel

	DataGridQuery     datagrid.GridQueryState
	DataSourceQuery   datagrid.GridQueryState
	DataGridHidden    map[string]bool
	NavQuery          string
	SelectedGroup     string
	SelectedComponent string

	InputText      string
	InputPassword  string
	InputPhone     string
	InputExpiry    string
	InputMultiline string
	RadioValue     string
	SegmentValue   string
	SegmentView    string

	ComboboxValue      string
	SvgSpinnerCategory string

	NumericENText       string
	NumericDEText       string
	NumericCurrencyText string
	NumericPercentText  string
	NumericPlainText    string

	InputGroupUser   string
	InputGroupDomain string
	InputGroupAmount string
	InputGroupSearch string

	BCSelected  string
	TabSelected string

	ThemePickerResult string

	DialogResult string
	NotifyResult string

	PrintingLastPath string
	PrintingStatus   string

	AudioStatus string

	PaletteAction     string
	ContextMenuResult string

	MenuSearchText string

	TableBorderStyle string

	TreeSelected       string
	DragTabSel         string
	ThemeGenStrategy   string
	ThemeGenRadiusText string
	ThemeGenBorderText string
	// Text buffers for the three density knobs. NumericInput keeps
	// the raw string so a half-typed value survives the frame.
	ThemeGenPadText       string
	ThemeGenScrollbarText string
	ThemeGenScrollGapText string
	ThemeGenName          string

	GesturePadLabel string

	SelectValue     []string
	ListBoxSelected []string

	// VirtualList demo. VLSeqs holds one sequence number per row; the
	// row text derives from it, so 5,000 rows cost 5,000 ints.
	VLSeqs    []int
	VLNextSeq int
	VLJump    string
	VLJumpErr string
	VLPinned  bool

	DatePickerDates []time.Time
	BCPath          []gui.BreadcrumbItemCfg

	DragListItems []gui.ListBoxOption
	DragTabItems  []gui.TabItemCfg
	DragTreeNodes []gui.TreeNodeCfg

	GesturePadMarkers    []GestureMarker
	NumericENValue       gui.Opt[float64]
	NumericDEValue       gui.Opt[float64]
	NumericCurrencyValue gui.Opt[float64]
	NumericPercentValue  gui.Opt[float64]
	NumericPlainValue    gui.Opt[float64]

	LocaleIndex int

	ButtonClicks int

	// TextAnimReplay gives the entrance labels in demoTextAnim a new
	// identity on every click, which is what replays a one-shot text
	// animation: progress is keyed by effective ID.
	TextAnimReplay int

	CmdButtonCount    int
	TableSortBy       int
	GesturePadVersion uint64

	DrawCanvasVersion uint64

	SplitterMainState   gui.SplitterState
	SplitterDetailState gui.SplitterState
	RangeValue          float32
	AudioVolume         float64
	// WidgetSoundOn tracks the widget-sound opt-in (issue #446). Two
	// things must be true for a widget to sound: a theme naming a cue
	// per role, and an installed SoundPlayer. The switch does both.
	WidgetSoundOn     bool
	WidgetSoundVolume float32
	WidgetSoundDemoOn bool
	WidgetSoundPlayer soundPlayerKind
	AudioMusicLoaded  bool
	AudioMusicPlaying bool
	AudioMusicPaused  bool

	AnimTweenX    float32
	AnimSpringX   float32
	AnimKeyframeX float32

	ThemeGenTint   float32
	ThemeGenRadius float32
	ThemeGenBorder float32
	// ThemeGenPad is the base padding the whole ladder derives from;
	// ThemeGenScrollGap is ScrollbarStyle.GapEdge.
	ThemeGenPad        float32
	ThemeGenScrollbar  float32
	ThemeGenScrollGap  float32
	MultiWindowChildID uint32

	// WindowOpacity mirrors Window.SetWindowOpacity so the slider has a
	// value to render. Seeded to 1 in newShowcaseApp, matching a window
	// that was never faded; the zero value would draw the slider at
	// "invisible" while the window was fully opaque.
	WindowOpacity float32

	GesturePadOffsetX  float32
	GesturePadOffsetY  float32
	GesturePadScale    float32
	GesturePadRotation float32

	DrawCanvasMarkerX float32
	DrawCanvasMarkerY float32

	// Path trim panel on the Draw Canvas page (demo_path_trim.go).
	// PathTrimProgress is how much of each path is drawn, 0 to 1; it
	// starts at 1 so the page opens with the paths whole.
	// PathTrimVersion changes with it, so the canvas repaints on a
	// render-only frame. The mark and ring are built once; the trimmed
	// copy is reused every frame.
	PathTrimProgress float32
	PathTrimVersion  uint64
	PathTrimMark     gui.CanvasPath
	PathTrimRing     gui.CanvasPath
	PathTrimOut      gui.CanvasPath

	// Transform playground sliders (demo_transform.go). TransformScale
	// is seeded to 1 in newShowcaseApp; the zero value would collapse
	// the preview to its origin. TransformMirrorX is the X-axis mirror
	// factor (1 normal, -1 flipped); TransformPersp is the viewing
	// distance for the RotateX/RotateY sliders.
	TransformRotate  float32
	TransformScale   float32
	TransformTX      float32
	TransformTY      float32
	TransformMirrorX float32
	TransformRotX    float32
	TransformRotY    float32
	TransformPersp   float32
	// TransformPlaygroundText backs the playground dialog's text
	// field, so it accepts input.
	TransformPlaygroundText string
	// TransformLift is the hover card's lift amount, 0 at rest to 1
	// raised.
	TransformLift float32

	// Flip panel page (demo_flip_panel.go). Each bool chooses the face
	// one FlipPanel shows; FlipPanelLikes counts presses of the button
	// on the back of the details panel.
	FlipPanelY         bool
	FlipPanelX         bool
	FlipPanelDetails   bool
	FlipPanelBouncy    bool
	FlipPanelClickOnly bool
	FlipPanelLikes     int

	ColorPickerColor gui.Color

	// ColorHSLA drives the composable color components. It is an HSLA
	// rather than a Color so hue survives a drag through gray or black.
	ColorHSLA gui.HSLA

	ThemeGenSeed gui.Color
	ThemeGenText gui.Color

	InputSpellCheck bool

	ToggleA        bool
	CheckboxA      bool
	SwitchA        bool
	ColorPickerHSL bool

	SidebarOpen bool
	ExpandOpen  bool

	AudioReady        bool
	TableMultiSelect  bool
	TableFreezeHeader bool
	ThemeGenPickText  bool
}

// GestureMarker records a tap or long-press position.
type GestureMarker struct {
	X, Y      float32
	LongPress bool
}

// appState is the typed state accessor for the showcase window.
// Every demo repeats appState(w); centralizing the
// type assertion here keeps call sites one word and the type
// mention in one place.
func appState(w *gui.Window) *ShowcaseApp {
	return gui.State[ShowcaseApp](w)
}

func newShowcaseApp() *ShowcaseApp {
	return &ShowcaseApp{
		ShaderStartTime:       time.Now(),
		SelectedGroup:         groupAll,
		WindowOpacity:         1,
		PathTrimProgress:      1,
		SelectedComponent:     "welcome",
		InputMultiline:        "Now is the time for all good men to come to the aid of their country",
		RadioValue:            "go",
		SegmentValue:          "week",
		SegmentView:           "list",
		RangeValue:            50,
		NumericENText:         "1,234.50",
		InputGroupDomain:      "example.com",
		NumericENValue:        gui.Some(1234.5),
		NumericDEText:         "1.234,50",
		NumericDEValue:        gui.Some(1234.5),
		NumericCurrencyText:   "$1,234.50",
		NumericCurrencyValue:  gui.Some(1234.5),
		NumericPercentText:    "12.50%",
		NumericPercentValue:   gui.Some(0.125),
		InputDate:             time.Now(),
		RollerDate:            time.Now(),
		ColorPickerColor:      gui.RGBA(51, 79, 103, 255),
		ColorHSLA:             gui.HSLA{H: 205, S: 0.34, L: 0.3, A: 1},
		BCSelected:            "page",
		BCPath:                append([]gui.BreadcrumbItemCfg(nil), showcaseBreadcrumbPath...),
		TabSelected:           "overview",
		AudioVolume:           0.8,
		WidgetSoundVolume:     0.5,
		SplitterMainState:     gui.SplitterState{Ratio: 0.30},
		SplitterDetailState:   gui.SplitterState{Ratio: 0.55},
		SidebarOpen:           true,
		ThemeGenSeed:          gui.ThemeDark.ColorSelect,
		ThemeGenStrategy:      "mono",
		ThemeGenRadius:        gui.ThemeDark.Cfg.Radius,
		ThemeGenRadiusText:    floatString(gui.ThemeDark.Cfg.Radius),
		ThemeGenBorder:        gui.ThemeDark.Cfg.SizeBorder,
		ThemeGenBorderText:    floatString(gui.ThemeDark.Cfg.SizeBorder),
		ThemeGenPad:           gui.ThemeDark.Cfg.Padding.Top,
		ThemeGenPadText:       floatString(gui.ThemeDark.Cfg.Padding.Top),
		ThemeGenScrollbar:     gui.ThemeDark.Cfg.SizeScrollbar,
		ThemeGenScrollbarText: floatString(gui.ThemeDark.Cfg.SizeScrollbar),
		ThemeGenScrollGap:     themeGenDefaultScrollGap,
		ThemeGenScrollGapText: floatString(themeGenDefaultScrollGap),
		ThemeGenText:          gui.ThemeDark.Cfg.TextStyleDef.Color,
		DataGridSelection: datagrid.GridSelection{
			SelectedRowIDs: map[string]bool{},
		},
		TableBorderStyle: "all",
		DataSourceSelection: datagrid.GridSelection{
			SelectedRowIDs: map[string]bool{},
		},
		GesturePadScale:   1,
		TransformScale:    1,
		TransformMirrorX:  1,
		TransformPersp:    500,
		DrawCanvasMarkerX: 240,
		DrawCanvasMarkerY: 140,
		DrawCanvasVersion: 1,
		TreeLazyNodes:     make(map[string][]gui.TreeNodeCfg),
		VLSeqs:            virtualListDemoSeqs(virtualListDemoRows),
		VLNextSeq:         virtualListDemoRows,
		DragListItems: []gui.ListBoxOption{
			gui.NewListBoxOption("apple", "Apple", ""),
			gui.NewListBoxOption("banana", "Banana", ""),
			gui.NewListBoxOption("cherry", "Cherry", ""),
			gui.NewListBoxOption("date", "Date", ""),
			gui.NewListBoxOption("elderberry", "Elderberry", ""),
			gui.NewListBoxOption("fig", "Fig", ""),
		},
		DragTabItems: []gui.TabItemCfg{
			gui.NewTabItem("alpha", "Alpha", nil),
			gui.NewTabItem("beta", "Beta", nil),
			gui.NewTabItem("gamma", "Gamma", nil),
			gui.NewTabItem("delta", "Delta", nil),
		},
		DragTabSel: "alpha",
		DragTreeNodes: []gui.TreeNodeCfg{
			{ID: "src", Text: "src", Nodes: []gui.TreeNodeCfg{
				{ID: "main.go", Text: "main.go"},
				{ID: "util.go", Text: "util.go"},
				{ID: "app.go", Text: "app.go"},
			}},
			{ID: "docs", Text: "docs", Nodes: []gui.TreeNodeCfg{
				{ID: "README.md", Text: "README.md"},
				{ID: "GUIDE.md", Text: "GUIDE.md"},
			}},
			{ID: "tests", Text: "tests"},
			{ID: "build", Text: "build"},
		},
	}
}

// demoGroups is the one list of catalog groups. The sidebar pins
// Start first and renders the rest in alphabetical order by label,
// so declaration order carries no display meaning.
var demoGroups = []DemoGroup{
	{Key: groupStart, Label: "Start"},
	{Key: groupActions, Label: "Actions"},
	{Key: groupChoices, Label: "Choices"},
	{Key: groupEntry, Label: "Entry"},
	{Key: groupText, Label: "Text"},
	{Key: groupData, Label: "Data"},
	{Key: groupLayout, Label: "Layout"},
	{Key: groupNav, Label: "Navigation"},
	{Key: groupMessages, Label: "Messages"},
	{Key: groupProgress, Label: "Progress"},
	{Key: groupMotion, Label: "Motion"},
	{Key: groupDrawing, Label: "Drawing"},
	{Key: groupApp, Label: "App & Window"},
}

// DemoEntry describes a single showcase destination.
type DemoEntry struct {
	ID      string
	Label   string
	Group   string
	Summary string

	idLower    string // populated by init()
	labelLower string // populated by init()
	Tags       []string
}

// DemoGroup describes a catalog group in the left pane.
type DemoGroup struct {
	Key   string
	Label string

	// The catalog list's row keys for this group's header, populated
	// by init(). The first header in the list has no gap above it, so
	// its height differs and it needs its own key.
	headerKey      string
	firstHeaderKey string
}

var demoEntries = []DemoEntry{
	{ID: "welcome", Label: "Welcome", Group: groupStart, Summary: "Start here for a quick introduction to Go-Gui and this showcase.", Tags: []string{"start", "intro", "overview"}},
	{ID: "commands", Label: "Commands & Hotkeys", Group: groupStart, Summary: "Centralized command registry with keyboard shortcuts, menu integration, and command palette.", Tags: []string{"command", "hotkey", "shortcut", "keyboard", "palette"}},
	{ID: "sound", Label: "Sound Feedback", Group: groupApp, Summary: "Opt-in interaction sounds: semantic cues, an injected player, per-widget override.", Tags: []string{"sound", "audio", "cue", "feedback", "click", "beep", "volume"}},
	{ID: "dx_cheat_sheet", Label: "DX Cheat Sheet", Group: groupStart, Summary: "The few places where the obvious reading of the API is wrong, and how to find them.", Tags: []string{"tips", "gotchas", "focus", "id", "hover", "pitfalls", "help"}},

	{ID: "color_picker", Label: "Color Picker", Group: groupEntry, Summary: "Pick RGBA and optional HSV values.", Tags: []string{"color", "hsv", "rgba"}},
	{ID: "date_picker", Label: "Date Picker", Group: groupEntry, Summary: "Select one or many dates from a calendar.", Tags: []string{"calendar", "dates", "selection"}},
	{ID: "date_picker_roller", Label: "Date Picker Roller", Group: groupEntry, Summary: "Roll wheel-style month/day/year controls.", Tags: []string{"date", "roller", "time"}},
	{ID: "input", Label: "Input", Group: groupEntry, Summary: "Single-line, password, and multiline text input.", Tags: []string{"text", "textarea", "password"}},
	{ID: "input_date", Label: "Input Date", Group: groupEntry, Summary: "Text input with date picker dropdown.", Tags: []string{"date", "input", "calendar"}},
	{ID: "input_group", Label: "Input Group", Group: groupEntry, Summary: "Join inputs, selects, buttons and text addons into one control.", Tags: []string{"addon", "prefix", "suffix", "group", "form"}},
	{ID: "numeric_input", Label: "Numeric Input", Group: groupEntry, Summary: "Locale-aware number input with step controls.", Tags: []string{"number", "decimal", "locale", "spinner"}},
	{ID: "forms", Label: "Forms", Group: groupEntry, Summary: "Form runtime with sync and async validation states recreated in example code.", Tags: []string{"form", "validation", "async", "touched", "dirty"}},

	{ID: "listbox", Label: "List Box", Group: groupChoices, Summary: "Single and multi-select list options.", Tags: []string{"list", "multi", "select"}},
	{ID: "radio", Label: "Radio", Group: groupChoices, Summary: "Single radio control.", Tags: []string{"option", "boolean", "choice"}},
	{ID: "radio_group", Label: "Radio Button Group", Group: groupChoices, Summary: "Mutually exclusive options in row or column.", Tags: []string{"group", "options", "select"}},
	{ID: "segmented_control", Label: "Segmented Control", Group: groupChoices, Summary: "Compact single choice in one shared track.", Tags: []string{"segment", "options", "select", "toggle"}},
	{ID: "slider", Label: "Slider", Group: groupEntry, Summary: "Drag horizontal or vertical value controls.", Tags: []string{"slider", "value"}},
	{ID: "gesture", Label: "Gestures", Group: groupMotion, Summary: "Touch gesture recognition: tap, pan, pinch, rotate, swipe.", Tags: []string{"touch", "gesture", "pinch", "swipe", "pan", "rotate", "tap", "mobile"}},
	{ID: "drag_reorder", Label: "Drag Reorder", Group: groupMotion, Summary: "Drag-to-reorder items in lists, tabs, and trees.", Tags: []string{"drag", "reorder", "list", "tabs", "tree", "keyboard"}},
	{ID: "combobox", Label: "Combobox", Group: groupChoices, Summary: "Single-select with typeahead filtering.", Tags: []string{"dropdown", "filter", "typeahead", "autocomplete"}},
	{ID: "select", Label: "Select", Group: groupChoices, Summary: "Dropdown with optional multi-select.", Tags: []string{"dropdown", "pick", "options"}},
	{ID: "switch", Label: "Switch", Group: groupChoices, Summary: "On/off switch control.", Tags: []string{"toggle", "boolean", "control"}},
	{ID: "toggle", Label: "Toggle", Group: groupChoices, Summary: "Checkbox-style and icon toggles.", Tags: []string{"checkbox", "boolean", "control"}},

	{ID: "image", Label: "Image", Group: groupDrawing, Summary: "Render local image assets.", Tags: []string{"photo", "asset", "media"}},
	{ID: "rectangle", Label: "Rectangle", Group: groupDrawing, Summary: "Draw colored shapes with border and radius.", Tags: []string{"shape", "primitive", "box"}},
	{ID: "svg", Label: "SVG", Group: groupDrawing, Summary: "Render vector graphics from SVG strings.", Tags: []string{"vector", "icon", "path"}},
	{ID: "printing", Label: "Printing", Group: groupApp, Summary: "Export current view to PDF and open native print dialog.", Tags: []string{"print", "pdf", "export"}},
	{ID: "animations", Label: "Animations", Group: groupMotion, Summary: "Tween, spring, and layout transition samples.", Tags: []string{"motion", "tween", "spring"}},
	{ID: "gradient", Label: "Gradients", Group: groupDrawing, Summary: "Linear and radial gradient fills.", Tags: []string{"gradient", "linear", "radial", "fill"}},
	{ID: "blur", Label: "Blur", Group: groupDrawing, Summary: "Gaussian blur effect on shapes and containers.", Tags: []string{"blur", "gaussian", "glow", "soft"}},
	{ID: "box_shadows", Label: "Box Shadows", Group: groupDrawing, Summary: "Shadow presets with spread behavior notes.", Tags: []string{"shadow", "depth", "blur"}},
	{ID: "shader", Label: "Custom Shaders", Group: groupDrawing, Summary: "Custom fragment shaders for dynamic fills.", Tags: []string{"shader", "glsl", "metal"}},
	{ID: "color_filter", Label: "Color Filters", Group: groupDrawing, Summary: "Color matrix transforms: grayscale, sepia, saturate, contrast, brightness.", Tags: []string{"filter", "color", "grayscale", "sepia", "saturate", "matrix"}},
	{ID: "draw_canvas", Label: "Draw Canvas", Group: groupDrawing, Summary: "Procedural 2D drawing with cached tessellation.", Tags: []string{"canvas", "draw", "chart", "line", "polygon", "circle", "path", "trim"}},
	{ID: "transform", Label: "Transforms", Group: groupDrawing, Summary: "Move, turn, scale and skew containers with visual transforms.", Tags: []string{"transform", "rotate", "scale", "translate", "skew", "perspective", "flip", "hover", "3d"}},
	{ID: "theme_gen", Label: "Theme", Group: groupApp, Summary: "Generate a theme from a seed color, tint level, and palette strategy.", Tags: []string{"theme", "color", "palette", "generator"}},

	{ID: "markdown", Label: "Markdown", Group: groupText, Summary: "Render markdown into styled rich content.", Tags: []string{"docs", "text", "rich"}},
	{ID: "rtf", Label: "Rich Text Format", Group: groupText, Summary: "Mixed styles, links, and inline rich runs.", Tags: []string{"rich text", "link", "style"}},
	{ID: "text", Label: "Text", Group: groupText, Summary: "Typography, gradients, outlines, and curved text.", Tags: []string{"font", "type", "styles", "gradient", "outline", "stroke", "curve"}},
	{ID: "typography", Label: "Typography", Group: groupText, Summary: "Type scale, roles, and modifiers.", Tags: []string{"font", "type", "scale", "roles", "style"}},
	{ID: "text_anim", Label: "Text Animation", Group: groupMotion, Summary: "Canned text animations from TextCfg.Anim, plus the Custom hook.", Tags: []string{"animation", "fade", "slide", "pulse", "shimmer", "typewriter", "shake", "motion"}},
	{ID: "icons", Label: "Icons", Group: groupText, Summary: "Icon font catalog and glyph references.", Tags: []string{"icon", "font", "glyph"}},

	{ID: "table", Label: "Table", Group: groupData, Summary: "Declarative and sortable table data.", Tags: []string{"rows", "columns", "csv"}},
	{ID: "data_grid", Label: "Data Grid", Group: groupData, Summary: "Controlled virtualized grid for interactive tabular data.", Tags: []string{"grid", "virtualized", "data"}},
	{ID: "data_source", Label: "Data Source", Group: groupData, Summary: "Async data-source backed grid with CRUD.", Tags: []string{"async", "pagination", "crud", "source"}},
	{ID: "tree", Label: "Tree View", Group: groupData, Summary: "Hierarchical expandable node display.", Tags: []string{"nodes", "hierarchy", "outline"}},
	{ID: "virtual_list", Label: "Virtual List", Group: groupData, Summary: "Virtualized list of app-built rows whose heights differ.", Tags: []string{"virtualized", "feed", "chat", "log", "scroll", "variable height"}},

	{ID: "breadcrumb", Label: "Breadcrumb", Group: groupNav, Summary: "Trail navigation with optional content panels.", Tags: []string{"breadcrumb", "navigation", "trail", "path"}},
	{ID: "menus", Label: "Menus + Menubar", Group: groupActions, Summary: "Nested menus, separators, and custom menu items.", Tags: []string{"menu", "menubar", "submenu"}},
	{ID: "tab_control", Label: "Tab Control", Group: groupNav, Summary: "Switch content panels with keyboard-friendly tabs.", Tags: []string{"tabs", "navigation", "panes"}},

	{ID: "button", Label: "Button", Group: groupActions, Summary: "Trigger actions with click and keyboard focus.", Tags: []string{"action", "press", "click"}},
	{ID: "command_button", Label: "Command Button", Group: groupActions, Summary: "Button wired to a registered command with auto-label, shortcut hint, and CanExecute.", Tags: []string{"command", "button", "shortcut", "action"}},
	{ID: "theme_picker", Label: "Theme Picker", Group: groupApp, Summary: "Palette icon dropdown for switching registered themes.", Tags: []string{"theme", "picker", "palette", "dark", "light"}},
	{ID: "progress_bar", Label: "Progress Bar", Group: groupProgress, Summary: "Determinate and indeterminate progress indicators.", Tags: []string{"progress", "loader", "status"}},
	{ID: "pulsar", Label: "Pulsar", Group: groupProgress, Summary: "Animated pulse indicator with optional icons.", Tags: []string{"pulse", "loading", "indicator"}},
	{ID: "toast", Label: "Toast", Group: groupMessages, Summary: "Non-blocking notifications with auto-dismiss and actions.", Tags: []string{"notification", "alert", "severity", "stack"}},
	{ID: "badge", Label: "Badge", Group: groupMessages, Summary: "Numeric and colored pill labels for counts and status.", Tags: []string{"badge", "count", "status", "pill", "label"}},
	{ID: "skeleton", Label: "Skeleton", Group: groupProgress, Summary: "Shimmer placeholder for loading states.", Tags: []string{"skeleton", "shimmer", "loading", "placeholder"}},
	{ID: "spinner", Label: "Math Spinner", Group: groupProgress, Summary: "Animated mathematical curves as loading indicators.", Tags: []string{"math spinner", "loading", "curve", "animation", "rose", "lissajous", "butterfly", "cardioid", "spiral"}},
	{ID: "svg_spinner", Label: "SVG Spinner", Group: groupProgress, Summary: "One hundred six built-in animated SVG spinners grouped by visual family.", Tags: []string{"svg", "spinner", "loading", "animation", "dots", "bars", "ring", "loader", "blocks"}},
	{ID: "thinking_orb", Label: "Thinking Orb", Group: groupProgress, Summary: "Nine semantic dotted-3D spinners for AI and agent states.", Tags: []string{"thinking", "orb", "spinner", "loading", "ai", "agent", "searching", "composing"}},
	{ID: "native_notification", Label: "Native Notification", Group: groupMessages, Summary: "OS-level notifications on macOS, Windows, and Linux.", Tags: []string{"notification", "native", "os", "alert", "push"}},
	{ID: "audio", Label: "Audio", Group: groupApp, Summary: "Sound effects and music playback via beep.", Tags: []string{"audio", "sound", "music", "sfx", "beep", "mixer"}},

	{ID: "dialog", Label: "Dialog", Group: groupMessages, Summary: "Message, confirm, prompt, and custom dialogs.", Tags: []string{"modal", "confirm", "prompt"}},
	{ID: "flip_panel", Label: "Flip Panel", Group: groupLayout, Summary: "Card that turns over in 3D to show its back face.", Tags: []string{"flip", "card", "panel", "3d", "rotate", "turn"}},
	{ID: "expand_panel", Label: "Expand Panel", Group: groupLayout, Summary: "Collapsible region with custom header and content.", Tags: []string{"accordion", "collapse", "panel"}},
	{ID: "command_palette", Label: "Command Palette", Group: groupActions, Summary: "Keyboard-first searchable action list.", Tags: []string{"command", "search", "palette", "keyboard"}},
	{ID: "context_menu", Label: "Context Menu", Group: groupActions, Summary: "Right-click floating menu with submenus and keyboard navigation.", Tags: []string{"context", "menu", "right-click", "popup", "floating"}},
	{ID: "tooltip", Label: "Tooltip", Group: groupMessages, Summary: "Hover hints with custom placement and content.", Tags: []string{"hover", "hint", "floating"}},
	{ID: "inspector", Label: "Inspector", Group: groupApp, Summary: "Dev-mode layout tree and property inspector.", Tags: []string{"inspector", "debug", "devtools", "layout", "tree"}},

	{ID: "rotated_box", Label: "Rotated Box", Group: groupLayout, Summary: "Rotate child content by quarter turns.", Tags: []string{"rotate", "transform", "layout"}},
	{ID: "row", Label: "Row", Group: groupLayout, Summary: "Horizontal container arranging children left-to-right.", Tags: []string{"row", "horizontal", "container", "layout"}},
	{ID: "column_demo", Label: "Column", Group: groupLayout, Summary: "Vertical container arranging children top-to-bottom.", Tags: []string{"column", "vertical", "container", "layout"}},
	{ID: "wrap_panel", Label: "Wrap Panel", Group: groupLayout, Summary: "Flow layout that wraps children to the next line.", Tags: []string{"wrap", "flow", "reflow", "layout"}},
	{ID: "overflow_panel", Label: "Overflow Panel", Group: groupLayout, Summary: "Row that hides non-fitting children in a dropdown.", Tags: []string{"overflow", "toolbar", "responsive", "layout"}},
	{ID: "sidebar", Label: "Sidebar", Group: groupLayout, Summary: "Animated panel that slides in and out.", Tags: []string{"sidebar", "panel", "slide", "layout"}},
	{ID: "scrollbar", Label: "Scrollable Containers", Group: groupLayout, Summary: "Bind scrollable layouts to shared scroll ids.", Tags: []string{"scrollbar", "scroll", "container", "layout"}},
	{ID: "splitter", Label: "Splitter", Group: groupLayout, Summary: "Resizable panes with drag, keyboard, and collapse.", Tags: []string{"split", "pane", "resize", "layout"}},
	{ID: "dock_layout", Label: "Dock Layout", Group: groupLayout, Summary: "IDE-style docking panels with splits, tabs, and drag-and-drop.", Tags: []string{"dock", "split", "tabs", "drag", "ide", "layout"}},
	{ID: "window_opacity", Label: "Window Opacity", Group: groupApp, Summary: "Fade the whole window, content included, with a runtime setter.", Tags: []string{"window", "opacity", "fade", "alpha", "transparent", "compositor"}},
	{ID: "multi_window", Label: "Multi-Window", Group: groupApp, Summary: "Open and manage additional windows with cross-window communication.", Tags: []string{"window", "multi", "app", "broadcast", "layout"}},
}

func init() {
	for i := range demoEntries {
		demoEntries[i].idLower = strings.ToLower(demoEntries[i].ID)
		demoEntries[i].labelLower = strings.ToLower(demoEntries[i].Label)
	}
	// A ':' cannot appear in an entry ID, so these keys never collide
	// with an entry's key.
	for i := range demoGroups {
		demoGroups[i].headerKey = gui.ScopeID("header", demoGroups[i].Key)
		demoGroups[i].firstHeaderKey = gui.ScopeID("header-first", demoGroups[i].Key)
	}
}

func entryMatchesQuery(entry DemoEntry, query string) bool {
	if query == "" {
		return true
	}
	q := strings.ToLower(query)
	if strings.Contains(entry.idLower, q) ||
		strings.Contains(entry.labelLower, q) ||
		strings.Contains(entry.Group, q) {
		return true
	}
	for _, tag := range entry.Tags {
		if strings.Contains(tag, q) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(entry.Summary), q)
}

func filteredEntries(app *ShowcaseApp) []DemoEntry {
	out := make([]DemoEntry, 0, len(demoEntries))
	for _, entry := range demoEntries {
		if app.SelectedGroup != groupAll && entry.Group != app.SelectedGroup {
			continue
		}
		if !entryMatchesQuery(entry, app.NavQuery) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func hasEntry(entries []DemoEntry, selected string) bool {
	for _, entry := range entries {
		if entry.ID == selected {
			return true
		}
	}
	return false
}

func selectedEntry(entries []DemoEntry, selected string) DemoEntry {
	for _, entry := range entries {
		if entry.ID == selected {
			return entry
		}
	}
	if len(entries) == 0 {
		return DemoEntry{}
	}
	return entries[0]
}

func preferredComponentForGroup(entries []DemoEntry) string {
	if len(entries) == 0 {
		return ""
	}
	best := entries[0]
	for _, entry := range entries[1:] {
		if entryCompare(entry, best) < 0 {
			best = entry
		}
	}
	return best.ID
}

func entryCompare(a, b DemoEntry) int {
	if c := cmp.Compare(entryPin(a.ID), entryPin(b.ID)); c != 0 {
		return c
	}
	if c := cmp.Compare(a.labelLower, b.labelLower); c != 0 {
		return c
	}
	return cmp.Compare(a.idLower, b.idLower)
}

func entryPin(id string) int {
	switch id {
	case "welcome":
		return 0
	default:
		return 2
	}
}
