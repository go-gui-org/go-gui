package main

func componentDoc(id string) string {
	switch id {
	case "native_notification":
		id = "notification"
	case "column_demo":
		id = "column"
	}
	if doc, ok := widgetDocs[id]; ok {
		return doc
	}
	return ""
}

var widgetDocs = map[string]string{
	// Feedback
	"button": `Clickable button with hover, focus, and click color states.
Content is any set of child views.

## Usage

` + "```go" + `
gui.Button(gui.ButtonCfg{
    ID:      "submit",
    IDFocus: 100,
    Content: []gui.View{gui.Text(gui.TextCfg{Text: "Submit"})},
    OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
        e.IsHandled = true
    },
})
` + "```" + `

## Disabled Button

` + "```go" + `
gui.Button(gui.ButtonCfg{
    ID:       "noop",
    Disabled: true,
    Content:  []gui.View{gui.Text(gui.TextCfg{Text: "Disabled"})},
})
` + "```" + `

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| Content   | []View       | Child views inside the button        |
| IDFocus   | uint32       | Tab-order focus ID (> 0 to enable)   |
| Float     | bool         | Float above siblings                 |
| HAlign    | HorizontalAlign | Horizontal content alignment      |
| VAlign    | VerticalAlign   | Vertical content alignment        |
| Disabled  | bool         | Disable interaction                  |
| Invisible | bool         | Hide without removing from layout    |
| Sizing    | Sizing       | Combined axis sizing mode            |
| Width     | float32      | Fixed width                          |
| Height    | float32      | Fixed height                         |
| MinWidth  | float32      | Minimum width                        |
| MaxWidth  | float32      | Maximum width                        |
| MinHeight | float32      | Minimum height                       |
| MaxHeight | float32      | Maximum height                       |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| Radius           | Opt[float32] | Corner radius                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Background color                 |
| ColorHover       | Color        | Background on hover              |
| ColorFocus       | Color        | Background when focused          |
| ColorClick       | Color        | Background on click              |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| BlurRadius       | float32      | Background blur radius           |
| Shadow           | *BoxShadow   | Drop shadow                      |
| Gradient         | *GradientDef | Background gradient              |

## Events

| Callback | Signature                        | Fired when       |
|----------|----------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)   | Button clicked   |
| OnHover  | func(*Layout, *Event, *Window)   | Mouse hover      |

## Accessibility

| Property        | Type        | Description                      |
|-----------------|-------------|----------------------------------|
| A11YRole        | AccessRole  | Accessible role override         |
| A11YState       | AccessState | Accessible state override        |
| A11YLabel       | string      | Accessible label                 |
| A11YDescription | string      | Accessible description           |
`,

	"command_button": `Button wired to a registered command. Auto-fills label
and shortcut hint from the command, auto-disables when CanExecute returns
false, and routes clicks through the command registry.

## Usage

` + "```go" + `
// Register the command once (e.g. in OnInit).
w.RegisterCommand(gui.Command{
    ID:       "file.save",
    Label:    "Save",
    Icon:     gui.IconSave,
    Shortcut: gui.Shortcut{Key: gui.KeyS, Modifiers: gui.ModCtrl},
    Execute: func(_ *gui.Event, w *gui.Window) {
        // save logic
    },
})

// Use the button anywhere in a view function.
gui.CommandButton(w, "file.save", gui.ButtonCfg{ID: "btn-save"})
` + "```" + `

## Auto-Disabled via CanExecute

` + "```go" + `
w.RegisterCommand(gui.Command{
    ID:         "edit.delete",
    Label:      "Delete",
    CanExecute: func(w *gui.Window) bool { return hasSelection(w) },
    Execute:    func(_ *gui.Event, w *gui.Window) { deleteSelection(w) },
})

// Button greys out automatically when CanExecute returns false.
gui.CommandButton(w, "edit.delete", gui.ButtonCfg{ID: "btn-del"})
` + "```" + `

## Behavior

| Behavior          | Description                                        |
|-------------------|----------------------------------------------------|
| Auto-label        | Label + shortcut hint from Command if no Content   |
| Auto-disable      | Disabled when CanExecute returns false              |
| Click routing     | OnClick delegates to Command.Execute               |
| Custom content    | Supply ButtonCfg.Content to override auto-label    |
| Custom OnClick    | Supply ButtonCfg.OnClick to override command wiring |

## Key Properties

CommandButton accepts a standard ButtonCfg. The most relevant fields:

| Property    | Type         | Description                                |
|-------------|--------------|--------------------------------------------|
| ID          | string       | Unique identifier (required)               |
| Content     | []View       | Custom content (overrides auto-label)      |
| OnClick     | func(...)    | Custom handler (overrides command wiring)  |
| Disabled    | bool         | Force disable (also set by CanExecute)     |
| IDFocus     | uint32       | Tab-order focus ID (> 0 to enable)         |
| Sizing      | Sizing       | Combined axis sizing mode                  |

## Appearance

Inherits all ButtonCfg appearance properties (Color, Radius, Padding,
SizeBorder, etc.). See Button docs for the full list.

## Accessibility

| Property        | Type        | Description                            |
|-----------------|-------------|----------------------------------------|
| A11YRole        | AccessRole  | Accessible role override               |
| A11YState       | AccessState | Accessible state override              |
| A11YLabel       | string      | Accessible label                       |
| A11YDescription | string      | Accessible description                 |
`,

	"progress_bar": `Determinate and indeterminate progress indicators with
optional percentage text overlay and vertical orientation.

## Usage

` + "```go" + `
gui.ProgressBar(gui.ProgressBarCfg{
    ID:      "loading",
    Percent: 0.75,
    Sizing:  gui.FillFit,
})
` + "```" + `

## Indefinite

` + "```go" + `
gui.ProgressBar(gui.ProgressBarCfg{
    ID:         "sync",
    Indefinite: true,
    Sizing:     gui.FillFit,
})
` + "```" + `

## Key Properties

| Property   | Type    | Description                          |
|------------|---------|--------------------------------------|
| ID         | string  | Unique identifier (required)         |
| Percent    | float32 | Progress 0.0–1.0                     |
| Text       | string  | Custom overlay text                  |
| TextShow   | bool    | Show percentage text overlay         |
| Indefinite | bool    | Animated looping mode                |
| Vertical   | bool    | Vertical orientation                 |
| Disabled   | bool    | Disable interaction                  |
| Invisible  | bool    | Hide without removing from layout    |
| Sizing     | Sizing  | Combined axis sizing mode            |
| Width      | float32 | Fixed width                          |
| Height     | float32 | Fixed height                         |
| MinWidth   | float32 | Minimum width                        |
| MaxWidth   | float32 | Maximum width                        |
| MinHeight  | float32 | Minimum height                       |
| MaxHeight  | float32 | Maximum height                       |

## Appearance

| Property       | Type         | Description                      |
|----------------|--------------|----------------------------------|
| Color          | Color        | Track background color           |
| ColorBar       | Color        | Fill bar color                   |
| TextBackground | Color        | Background behind percentage text|
| TextPadding    | Opt[Padding] | Padding around percentage text   |
| TextStyle      | TextStyle    | Percentage text styling          |
| Radius         | float32      | Corner radius                    |

## Accessibility

| Property        | Type   | Description                      |
|-----------------|--------|----------------------------------|
| A11YLabel       | string | Accessible label                 |
| A11YDescription | string | Accessible description           |
`,

	"pulsar": `Animated blinking text indicator for loading states. Alternates
between two text strings synced to the window's input cursor blink.
Defaults to "..." / ".." if no text is provided.

## Usage

` + "```go" + `
gui.Pulsar(gui.PulsarCfg{ID: "p1"}, w)
` + "```" + `

## Custom Text

` + "```go" + `
gui.Pulsar(gui.PulsarCfg{
    ID:    "typing",
    Text1: "Typing...",
    Text2: "Typing..",
    Size:  gui.SomeF(14),
}, w)
` + "```" + `

## Key Properties

| Property | Type    | Description                          |
|----------|---------|--------------------------------------|
| ID       | string  | Unique identifier                    |
| Text1    | string  | Text shown when cursor is on         |
| Text2    | string  | Text shown when cursor is off        |
| Width    | float32 | Fixed width (auto-estimated if 0)    |

## Appearance

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| Color     | Color        | Text color                           |
| Size      | Opt[float32] | Font size                            |
| TextStyle | TextStyle    | Full text style override             |
`,

	"toast": `Non-blocking notifications with severity levels, auto-dismiss,
and optional action buttons. Toasts animate in/out and pause dismissal
on hover.

## Usage

` + "```go" + `
w.Toast(gui.ToastCfg{
    Title:    "Saved",
    Body:     "Document saved.",
    Severity: gui.ToastSuccess,
})
` + "```" + `

## With Action Button

` + "```go" + `
w.Toast(gui.ToastCfg{
    Title:       "File deleted",
    Body:        "1 file moved to trash.",
    Severity:    gui.ToastWarning,
    ActionLabel: "Undo",
    OnAction: func(w *gui.Window) {
        // undo delete
    },
})
` + "```" + `

## API

| Method              | Description                      |
|---------------------|----------------------------------|
| w.Toast(cfg)        | Show toast, returns uint64 ID    |
| w.ToastDismiss(id)  | Dismiss specific toast           |
| w.ToastDismissAll() | Dismiss all toasts               |

## Key Properties

| Property    | Type          | Description                          |
|-------------|---------------|--------------------------------------|
| Title       | string        | Toast heading                        |
| Body        | string        | Toast message body                   |
| Severity    | ToastSeverity | Visual style (color accent)          |
| Duration    | time.Duration | Auto-dismiss delay (0 = 3s default)  |
| ActionLabel | string        | Optional action button text          |

## Events

| Callback | Signature      | Fired when                           |
|----------|----------------|--------------------------------------|
| OnAction | func(*Window)  | Action button clicked                |

## Severity

| Constant     | Use case                             |
|--------------|--------------------------------------|
| ToastInfo    | Informational                        |
| ToastSuccess | Positive outcome                     |
| ToastWarning | Needs attention                      |
| ToastError   | Critical failure                     |
`,

	"badge": `Numeric and colored pill labels for counts and status indicators.
Dot mode renders a small circle; labeled mode renders text inside a
rounded pill.

## Usage

` + "```go" + `
gui.Badge(gui.BadgeCfg{Label: "5", Variant: gui.BadgeInfo})
` + "```" + `

## Dot Mode

` + "```go" + `
gui.Badge(gui.BadgeCfg{Dot: true, Variant: gui.BadgeSuccess})
` + "```" + `

## Capped Count

` + "```go" + `
gui.Badge(gui.BadgeCfg{Label: "150", Max: 99})
// Displays "99+"
` + "```" + `

## Key Properties

| Property  | Type         | Description                                |
|-----------|--------------|--------------------------------------------|
| Label     | string       | Badge text                                 |
| Variant   | BadgeVariant | Color variant preset                       |
| Max       | int          | Cap value; shows "max+" when exceeded      |
| Dot       | bool         | Show as a small dot instead of text        |

## Appearance

| Property  | Type         | Description                                |
|-----------|--------------|--------------------------------------------|
| Color     | Color        | Custom background color                    |
| DotSize   | Opt[float32] | Dot diameter (dot mode only)               |
| Padding   | Opt[Padding] | Inner padding                              |
| Radius    | Opt[float32] | Corner radius                              |
| TextStyle | TextStyle    | Label text styling                         |

## Variants

| Variant      | Use case                                   |
|--------------|--------------------------------------------|
| BadgeDefault | Custom color                               |
| BadgeInfo    | Informational                              |
| BadgeSuccess | Positive status                            |
| BadgeWarning | Needs attention                            |
| BadgeError   | Critical                                   |

## Accessibility

| Property        | Type   | Description                            |
|-----------------|--------|----------------------------------------|
| A11YLabel       | string | Accessible label                       |
| A11YDescription | string | Accessible description                 |
`,

	"skeleton": `Shimmer placeholder for content that is still loading.
Displays an animated gradient sweep over a solid base color,
signaling that real content will appear once data arrives.

## Usage

` + "```go" + `
gui.Skeleton(gui.SkeletonCfg{
    ID:     "sk-line",
    Sizing: gui.FillFixed,
    Height: 14,
})
` + "```" + `

## Circle Variant

` + "```go" + `
gui.Skeleton(gui.SkeletonCfg{
    ID:      "sk-avatar",
    Variant: gui.SkeletonCircle,
    Width:   48,
    Height:  48,
})
` + "```" + `

## Card Placeholder

` + "```go" + `
gui.Row(gui.ContainerCfg{
    Sizing:  gui.FillFit,
    Spacing: gui.SomeF(12),
    Content: []gui.View{
        gui.Skeleton(gui.SkeletonCfg{
            ID: "avatar", Variant: gui.SkeletonCircle,
            Width: 48, Height: 48,
        }),
        gui.Column(gui.ContainerCfg{
            Sizing:  gui.FillFit,
            Spacing: gui.SomeF(6),
            Content: []gui.View{
                gui.Skeleton(gui.SkeletonCfg{ID: "l1", Sizing: gui.FillFixed, Height: 14}),
                gui.Skeleton(gui.SkeletonCfg{ID: "l2", Width: 200, Height: 14}),
            },
        }),
    },
})
` + "```" + `

## Custom Colors

` + "```go" + `
gui.Skeleton(gui.SkeletonCfg{
    ID:             "sk-custom",
    Sizing:         gui.FillFixed,
    Height:         24,
    Color:          gui.RGB(60, 60, 80),
    ColorHighlight: gui.RGB(100, 100, 140),
})
` + "```" + `

## Key Properties

| Property  | Type            | Description                          |
|-----------|-----------------|--------------------------------------|
| ID        | string          | Unique identifier (drives animation) |
| Variant   | SkeletonVariant | Shape variant (rect or circle)       |
| Sizing    | Sizing          | Combined axis sizing mode            |
| Width     | float32         | Fixed width                          |
| Height    | float32         | Fixed height                         |
| MinWidth  | float32         | Minimum width                        |
| MaxWidth  | float32         | Maximum width                        |
| MinHeight | float32         | Minimum height                       |
| MaxHeight | float32         | Maximum height                       |
| Disabled  | bool            | Disable the shimmer animation        |
| Invisible | bool            | Hide without removing from layout    |

## Appearance

| Property       | Type         | Description                       |
|----------------|--------------|-----------------------------------|
| Color          | Color        | Base skeleton color               |
| ColorHighlight | Color        | Shimmer highlight color           |
| Radius         | Opt[float32] | Corner radius                     |

## Variants

| Variant        | Description                                |
|----------------|--------------------------------------------|
| SkeletonRect   | Rounded rectangle (default)                |
| SkeletonCircle | Circle, sized by Width and Height          |

## Accessibility

| Property        | Type   | Description                            |
|-----------------|--------|----------------------------------------|
| A11YLabel       | string | Accessible label (default "Loading")   |
| A11YDescription | string | Accessible description                 |
`,

	"theme_picker": `Palette icon that opens a floating dropdown of all
registered themes. Clicking a theme applies it immediately via
w.SetTheme. The dropdown supports keyboard navigation (arrow keys,
Enter, Escape).

## Usage

` + "```go" + `
gui.ThemePicker(gui.ThemePickerCfg{
    ID:          "my-theme-picker",
    FloatAnchor: gui.FloatBottomLeft,
    FloatTieOff: gui.FloatTopLeft,
    OnSelect: func(name string, _ *gui.Event, w *gui.Window) {
        fmt.Println("switched to", name)
    },
})
` + "```" + `

## Float Positioning

The dropdown is a floating panel. Control its anchor and tie-off
to place it relative to the icon:

` + "```go" + `
gui.ThemePicker(gui.ThemePickerCfg{
    ID:          "tp",
    FloatAnchor: gui.FloatTopRight,
    FloatTieOff: gui.FloatBottomRight,
})
` + "```" + `

## Key Properties

| Property     | Type                               | Description                          |
|--------------|------------------------------------|--------------------------------------|
| ID           | string                             | Unique identifier (required)         |
| IDFocus      | uint32                             | Tab-order focus ID (> 0 to enable)   |
| Sizing       | Sizing                             | Combined axis sizing mode            |
| OnSelect     | func(string, *Event, *Window)      | Called with theme name on selection   |
| FloatAnchor  | FloatAttach                        | Dropdown anchor point on parent      |
| FloatTieOff  | FloatAttach                        | Dropdown tie-off point on dropdown   |
| FloatOffsetX | float32                            | Horizontal offset from anchor        |
| FloatOffsetY | float32                            | Vertical offset from anchor          |

## Accessibility

| Property        | Type   | Description                            |
|-----------------|--------|----------------------------------------|
| A11YLabel       | string | Accessible label                       |
| A11YDescription | string | Accessible description                 |
`,

	// Input
	"input": `Single-line, password, and multiline text input with IME
composition, keyboard focus, masked input, and accessibility support.

## Usage

` + "```go" + `
gui.Input(gui.InputCfg{
    ID:          "name",
    IDFocus:     100,
    Sizing:      gui.FillFit,
    Text:        app.Name,
    Placeholder: "Enter name...",
    OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
        gui.State[App](w).Name = s
    },
})
` + "```" + `

## Password

` + "```go" + `
gui.Input(gui.InputCfg{
    ID:         "pw",
    IDFocus:    101,
    IsPassword: true,
    Text:       app.Password,
    OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
        gui.State[App](w).Password = s
    },
})
` + "```" + `

## Multiline

` + "```go" + `
gui.Input(gui.InputCfg{
    ID:     "notes",
    IDFocus: 102,
    Mode:   gui.InputMultiline,
    Height: 90,
    Text:   app.Notes,
    OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
        gui.State[App](w).Notes = s
    },
})
` + "```" + `

## Masked Input

` + "```go" + `
gui.Input(gui.InputCfg{
    ID:         "phone",
    MaskPreset: gui.MaskPhoneUS,
    Text:       app.Phone,
})
` + "```" + `

Presets: MaskPhoneUS, MaskCreditCard16, MaskCreditCardAmex,
MaskExpiryMMYY, MaskCVC. For custom masks, set Mask (pattern string)
and MaskTokens (custom token definitions).

## Key Properties

| Property         | Type            | Description                          |
|------------------|-----------------|--------------------------------------|
| Text             | string          | Current text value                   |
| Placeholder      | string          | Hint text shown when empty           |
| IsPassword       | bool            | Mask characters for password entry   |
| Mode             | InputMode       | InputSingleLine or InputMultiline    |
| MaskPreset       | InputMaskPreset | Built-in mask (phone, card, etc.)    |
| Mask             | string          | Custom mask pattern                  |
| MaskTokens       | []MaskTokenDef  | Custom token definitions for mask    |
| Disabled         | bool            | Disable interaction                  |
| Height           | float32         | Height (useful for multiline)        |
| MinWidth         | float32         | Minimum width                        |
| MaxWidth         | float32         | Maximum width                        |
| IDFocus          | uint32          | Tab-order focus ID (> 0 to enable)   |

## Appearance

| Property         | Type      | Description                          |
|------------------|-----------|--------------------------------------|
| Padding          | Opt[Padding] | Inner padding                     |
| Radius           | Opt[float32] | Corner radius                     |
| SizeBorder       | Opt[float32] | Border width                      |
| Color            | Color     | Background color                     |
| ColorHover       | Color     | Background on hover                  |
| ColorBorder      | Color     | Border color                         |
| ColorBorderFocus | Color     | Border color when focused            |
| TextStyle        | TextStyle | Text styling                         |
| PlaceholderStyle | TextStyle | Placeholder text styling             |

## Events

| Callback            | Signature                                          | Fired when                           |
|---------------------|----------------------------------------------------|--------------------------------------|
| OnTextChanged       | func(*Layout, string, *Window)                     | Text changes                         |
| OnTextCommit        | func(*Layout, string, InputCommitReason, *Window)  | Enter pressed or focus lost          |
| OnEnter             | func(*Layout, *Event, *Window)                     | Enter pressed (single-line)          |
| OnKeyDown           | func(*Layout, *Event, *Window)                     | Unhandled key event                  |
| OnBlur              | func(*Layout, *Window)                             | Focus lost                           |
| PreTextChange       | func(current, proposed string) (string, bool)      | Validate/transform before change     |
| PostCommitNormalize | func(text string, InputCommitReason) string        | Normalize text on commit             |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"numeric_input": `Locale-aware numeric input with optional step buttons,
currency/percent modes, and min/max validation.

## Usage

` + "```go" + `
gui.NumericInput(gui.NumericInputCfg{
    ID:          "qty",
    IDFocus:     200,
    Placeholder: "Enter number",
    Decimals:    2,
    Min:         gui.SomeD(0),
    Max:         gui.SomeD(999),
    OnValueCommit: func(_ *gui.Layout, v gui.Opt[float64], s string, w *gui.Window) {
        gui.State[App](w).Qty = v
    },
})
` + "```" + `

## Currency Mode

` + "```go" + `
gui.NumericInput(gui.NumericInputCfg{
    ID:   "price",
    Mode: gui.NumericCurrency,
    CurrencyCfg: gui.NumericCurrencyModeCfg{Symbol: "$"},
})
` + "```" + `

## Key Properties

| Property    | Type                   | Description                          |
|-------------|------------------------|--------------------------------------|
| Text        | string                 | Current text value                   |
| Value       | Opt[float64]           | Parsed numeric value                 |
| Placeholder | string                 | Hint text shown when empty           |
| Decimals    | int                    | Decimal places                       |
| Min         | Opt[float64]           | Minimum allowed value                |
| Max         | Opt[float64]           | Maximum allowed value                |
| Mode        | NumericInputMode       | Number, currency, or percent         |
| StepCfg     | NumericStepCfg         | Step button configuration            |
| CurrencyCfg | NumericCurrencyModeCfg | Currency mode settings               |
| PercentCfg  | NumericPercentModeCfg  | Percent mode settings                |
| Locale      | NumericLocaleCfg       | Locale formatting rules              |
| IDFocus     | uint32                 | Tab-order focus ID (> 0 to enable)   |
| Disabled    | bool                   | Disable interaction                  |
| Invisible   | bool                   | Hide without removing from layout    |
| Sizing      | Sizing                 | Combined axis sizing mode            |
| Width       | float32                | Fixed width                          |
| Height      | float32                | Fixed height                         |
| MinWidth    | float32                | Minimum width                        |
| MaxWidth    | float32                | Maximum width                        |
| MinHeight   | float32                | Minimum height                       |
| MaxHeight   | float32                | Maximum height                       |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| Radius           | Opt[float32] | Corner radius                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Background color                 |
| ColorHover       | Color        | Background on hover              |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| TextStyle        | TextStyle    | Text styling                     |
| PlaceholderStyle | TextStyle    | Placeholder text styling         |

## Events

| Callback      | Signature                                        | Fired when                   |
|---------------|--------------------------------------------------|------------------------------|
| OnTextChanged | func(*Layout, string, *Window)                   | Text changes                 |
| OnValueCommit | func(*Layout, Opt[float64], string, *Window)     | Value committed (blur/enter) |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"color_picker": `Interactive HSV color selection with SV area, hue slider,
alpha slider, hex input, and RGB/HSV channel inputs. Preserves hue
when saturation or value reaches zero.

## Usage

` + "```go" + `
gui.ColorPicker(gui.ColorPickerCfg{
    ID:    "cp",
    Color: app.Color,
    OnColorChange: func(c gui.Color, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Color = c
    },
})
` + "```" + `

## With HSV Channels

` + "```go" + `
gui.ColorPicker(gui.ColorPickerCfg{
    ID:      "cp-hsv",
    Color:   app.Color,
    ShowHSV: true,
    OnColorChange: func(c gui.Color, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Color = c
    },
})
` + "```" + `

## Key Properties

| Property | Type             | Description                        |
|----------|------------------|------------------------------------|
| Color    | Color            | Current color value                |
| ShowHSV  | bool             | Show H/S/V channel inputs          |
| IDFocus  | uint32           | Tab-order focus ID (> 0 to enable) |
| Sizing   | Sizing           | Combined axis sizing mode          |
| Width    | float32          | Fixed width                        |
| Height   | float32          | Fixed height                       |

## Appearance

| Property | Type             | Description                        |
|----------|------------------|------------------------------------|
| Style    | ColorPickerStyle | Full style override                |

ColorPickerStyle fields:

| Field            | Type    | Description                        |
|------------------|---------|------------------------------------|
| Color            | Color   | Background color                   |
| ColorHover       | Color   | Background on hover                |
| ColorBorder      | Color   | Border color                       |
| ColorBorderFocus | Color   | Border color when focused          |
| Padding          | Padding | Inner padding                      |
| SizeBorder       | float32 | Border width                       |
| Radius           | float32 | Corner radius                      |
| SVSize           | float32 | Saturation/value area size (px)    |
| SliderHeight     | float32 | Hue slider height (px)             |
| IndicatorSize    | float32 | Drag indicator diameter (px)       |

## Events

| Callback      | Signature                      | Fired when           |
|---------------|--------------------------------|----------------------|
| OnColorChange | func(Color, *Event, *Window)   | Color changed        |

## Accessibility

| Property        | Type   | Description                        |
|-----------------|--------|------------------------------------|
| A11YLabel       | string | Accessible label                   |
| A11YDescription | string | Accessible description             |

## Components

- **SV area** -- click/drag to set saturation and value
- **Hue slider** -- vertical rainbow slider for hue selection
- **Alpha slider** -- horizontal slider (0--255)
- **Hex input** -- editable hex color string
- **RGB inputs** -- Red, Green, Blue channel inputs (0--255)
- **HSV inputs** -- Hue (0--360), Sat (0--100), Val (0--100) (when ShowHSV is true)
`,

	"date_picker": `Calendar-style date selection with month/year navigation.
Supports single and multi-select, weekday/month/year filtering,
and locale-aware weekday headers.

## Usage

` + "```go" + `
gui.DatePicker(gui.DatePickerCfg{
    ID:    "dp",
    Dates: app.SelectedDates,
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).SelectedDates = dates
    },
})
` + "```" + `

## Multiple Selection

` + "```go" + `
gui.DatePicker(gui.DatePickerCfg{
    ID:             "dp-multi",
    Dates:          app.SelectedDates,
    SelectMultiple: true,
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).SelectedDates = dates
    },
})
` + "```" + `

## Weekday Filtering

` + "```go" + `
gui.DatePicker(gui.DatePickerCfg{
    ID: "dp-weekdays",
    AllowedWeekdays: []gui.DatePickerWeekdays{
        gui.DatePickerMonday, gui.DatePickerWednesday, gui.DatePickerFriday,
    },
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).SelectedDates = dates
    },
})
` + "```" + `

## Key Properties

| Property             | Type                 | Description                        |
|----------------------|----------------------|------------------------------------|
| Dates                | []time.Time          | Currently selected date(s)         |
| SelectMultiple       | bool                 | Allow multiple date selection      |
| MondayFirstDayOfWeek | bool                 | Start week on Monday               |
| ShowAdjacentMonths   | bool                 | Show prev/next month days          |
| HideTodayIndicator   | bool                 | Hide today border highlight        |
| WeekdaysLen          | DatePickerWeekdayLen | Header label length                |
| IDFocus              | uint32               | Tab-order focus ID (> 0 to enable) |
| Disabled             | bool                 | Disable interaction                |
| Invisible            | bool                 | Hide without removing from layout  |

## Filtering

| Property        | Type                 | Description                        |
|-----------------|----------------------|------------------------------------|
| AllowedWeekdays | []DatePickerWeekdays | Restrict to specific days          |
| AllowedMonths   | []DatePickerMonths   | Restrict to specific months        |
| AllowedYears    | []int                | Restrict to specific years         |
| AllowedDates    | []time.Time          | Restrict to specific dates         |

## Appearance

| Property         | Type         | Description                        |
|------------------|--------------|------------------------------------|
| Padding          | Opt[Padding] | Inner padding                      |
| SizeBorder       | Opt[float32] | Border width                       |
| CellSpacing      | Opt[float32] | Gap between day cells              |
| Radius           | Opt[float32] | Corner radius                      |
| RadiusBorder     | Opt[float32] | Outer border radius                |
| Color            | Color        | Background color                   |
| ColorHover       | Color        | Background on hover                |
| ColorFocus       | Color        | Background when focused            |
| ColorClick       | Color        | Background on click                |
| ColorBorder      | Color        | Border color                       |
| ColorBorderFocus | Color        | Border color when focused          |
| ColorSelect      | Color        | Selected date highlight            |
| TextStyle        | TextStyle    | Text styling                       |

## Events

| Callback | Signature                            | Fired when           |
|----------|--------------------------------------|----------------------|
| OnSelect | func([]time.Time, *Event, *Window)   | Date(s) selected     |

## Accessibility

| Property        | Type   | Description                        |
|-----------------|--------|------------------------------------|
| A11YLabel       | string | Accessible label                   |
| A11YDescription | string | Accessible description             |

## Weekday Label Lengths

| Constant           | Example |
|--------------------|---------|
| WeekdayOneLetter   | S       |
| WeekdayThreeLetter | Sun     |
| WeekdayFull        | Sunday  |

## Keyboard

- **Left/Right** -- navigate months
- Click month/year header to toggle roller picker
- In roller mode: **Up/Down** = month, **Shift+Up/Down** = year

## API

` + "`w.DatePickerReset(id)`" + ` clears picker state.
`,

	"date_picker_roller": `Rolling drum-style date selection. Supports mouse
scroll, click, and keyboard input. Each date component (day, month,
year) renders as a scrollable drum.

## Usage

` + "```go" + `
gui.DatePickerRoller(gui.DatePickerRollerCfg{
    ID:           "dpr",
    SelectedDate: time.Now(),
    DisplayMode:  gui.RollerMonthDayYear,
    VisibleItems: 5,
    LongMonths:   true,
    OnChange: func(t time.Time, w *gui.Window) {
        gui.State[App](w).Date = t
    },
})
` + "```" + `

## Year-Only Roller

` + "```go" + `
gui.DatePickerRoller(gui.DatePickerRollerCfg{
    ID:          "dpr-year",
    DisplayMode: gui.RollerYearOnly,
    MinYear:     2020,
    MaxYear:     2030,
    OnChange: func(t time.Time, w *gui.Window) {
        gui.State[App](w).Year = t.Year()
    },
})
` + "```" + `

## Display Modes

| Constant           | Format    | Description              |
|--------------------|-----------|--------------------------|
| RollerDayMonthYear | DD MMM YYYY | Day, month, year drums (default) |
| RollerMonthDayYear | MMM DD YYYY | Month, day, year drums   |
| RollerMonthYear    | MMM YYYY  | Month and year only      |
| RollerYearOnly     | YYYY      | Single year drum         |

## Key Properties

| Property     | Type                       | Description                        |
|--------------|----------------------------|------------------------------------|
| SelectedDate | time.Time                  | Currently selected date            |
| DisplayMode  | DatePickerRollerDisplayMode | Drum layout mode                  |
| VisibleItems | int                        | Visible rows per drum (must be odd)|
| ItemHeight   | float32                    | Row height in pixels               |
| LongMonths   | bool                       | "January" vs "Jan"                 |
| MinYear      | int                        | Earliest year (default 1900)       |
| MaxYear      | int                        | Latest year (default 2100)         |
| IDFocus      | uint32                     | Tab-order focus ID (> 0 to enable) |
| MinWidth     | float32                    | Minimum width                      |
| MaxWidth     | float32                    | Maximum width                      |

## Appearance

| Property    | Type         | Description                        |
|-------------|--------------|------------------------------------|
| Color       | Color        | Background color                   |
| ColorBorder | Color        | Border color                       |
| SizeBorder  | Opt[float32] | Border width                       |
| Radius      | Opt[float32] | Corner radius                      |
| Padding     | Opt[Padding] | Inner padding                      |
| TextStyle   | TextStyle    | Text styling                       |

## Events

| Callback | Signature                | Fired when           |
|----------|--------------------------|----------------------|
| OnChange | func(time.Time, *Window) | Date changed         |

## Accessibility

| Property        | Type   | Description                        |
|-----------------|--------|------------------------------------|
| A11YLabel       | string | Accessible label                   |
| A11YDescription | string | Accessible description             |

## Keyboard

- **Up/Down** -- change month
- **Shift+Up/Down** -- change year
- **Alt+Up/Down** -- change day
- **Escape** -- exit roller view
`,

	"input_date": `Text input with calendar popup for date entry. Combines a
text field with an inline date picker dropdown. Displays the selected
date formatted via the current locale.

## Usage

` + "```go" + `
gui.InputDate(gui.InputDateCfg{
    ID:          "id",
    IDFocus:     100,
    Date:        app.Date,
    Sizing:      gui.FillFit,
    Placeholder: "Select date...",
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Date = dates[0]
    },
})
` + "```" + `

## With Filtering

` + "```go" + `
gui.InputDate(gui.InputDateCfg{
    ID:   "id-weekday",
    Date: app.Date,
    AllowedWeekdays: []gui.DatePickerWeekdays{
        gui.DatePickerMonday, gui.DatePickerTuesday,
        gui.DatePickerWednesday, gui.DatePickerThursday,
        gui.DatePickerFriday,
    },
    OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Date = dates[0]
    },
})
` + "```" + `

## Key Properties

| Property             | Type                 | Description                        |
|----------------------|----------------------|------------------------------------|
| Date                 | time.Time            | Current date value                 |
| Placeholder          | string               | Hint text shown when empty         |
| SelectMultiple       | bool                 | Allow multiple date selection      |
| MondayFirstDayOfWeek | bool                 | Start week on Monday               |
| ShowAdjacentMonths   | bool                 | Show prev/next month days          |
| HideTodayIndicator   | bool                 | Hide today border highlight        |
| WeekdaysLen          | DatePickerWeekdayLen | Weekday header label length        |
| IDFocus              | uint32               | Tab-order focus ID (> 0 to enable) |
| Disabled             | bool                 | Disable interaction                |
| Invisible            | bool                 | Hide without removing from layout  |
| Sizing               | Sizing               | Combined axis sizing mode          |
| Width                | float32              | Fixed width                        |
| Height               | float32              | Fixed height                       |
| MinWidth             | float32              | Minimum width                      |
| MaxWidth             | float32              | Maximum width                      |

## Filtering

| Property        | Type                 | Description                        |
|-----------------|----------------------|------------------------------------|
| AllowedWeekdays | []DatePickerWeekdays | Restrict to specific days          |
| AllowedMonths   | []DatePickerMonths   | Restrict to specific months        |
| AllowedYears    | []int                | Restrict to specific years         |
| AllowedDates    | []time.Time          | Restrict to specific dates         |

## Appearance

| Property         | Type         | Description                        |
|------------------|--------------|------------------------------------|
| Padding          | Opt[Padding] | Inner padding                      |
| SizeBorder       | Opt[float32] | Border width                       |
| CellSpacing      | Opt[float32] | Gap between calendar day cells     |
| Radius           | Opt[float32] | Corner radius                      |
| RadiusBorder     | Opt[float32] | Outer border radius                |
| Color            | Color        | Background color                   |
| ColorHover       | Color        | Background on hover                |
| ColorFocus       | Color        | Background when focused            |
| ColorClick       | Color        | Background on click                |
| ColorBorder      | Color        | Border color                       |
| ColorBorderFocus | Color        | Border color when focused          |
| ColorSelect      | Color        | Selected date highlight            |
| TextStyle        | TextStyle    | Text styling                       |
| PlaceholderStyle | TextStyle    | Placeholder text styling           |

## Events

| Callback | Signature                            | Fired when           |
|----------|--------------------------------------|----------------------|
| OnSelect | func([]time.Time, *Event, *Window)   | Date(s) selected     |
| OnEnter  | func(*Layout, *Event, *Window)       | Enter pressed        |

## Accessibility

| Property        | Type   | Description                        |
|-----------------|--------|------------------------------------|
| A11YLabel       | string | Accessible label                   |
| A11YDescription | string | Accessible description             |
`,

	"forms": `Form container with built-in runtime validation,
submit/reset semantics, per-field state tracking (touched, dirty,
pending), configurable validation triggers, and stale field cleanup.

## Basic Usage

` + "```go" + `
gui.Form(gui.FormCfg{
    ID:     "my-form",
    Sizing: gui.FillFit,
    OnSubmit: func(e gui.FormSubmitEvent, w *gui.Window) {
        fmt.Println("values:", e.Values)
    },
    Content: []gui.View{ /* inputs, buttons, etc. */ },
})
` + "```" + `

## Field Registration

Register fields each frame so the form runtime tracks them.
Call FormRegisterFieldByID during view construction (form ID
is known), or FormRegisterField from event handlers (walks
the parent chain).

` + "```go" + `
gui.FormRegisterFieldByID(w, "my-form", gui.FormFieldAdapterCfg{
    FieldID:        "email",
    Value:          app.Email,
    SyncValidators: []gui.FormSyncValidator{validateEmail},
})
` + "```" + `

## Sync Validators

` + "```go" + `
func validateEmail(
    f gui.FormFieldSnapshot, _ gui.FormSnapshot,
) []gui.FormIssue {
    if !strings.Contains(f.Value, "@") {
        return []gui.FormIssue{{Msg: "must contain @"}}
    }
    return nil
}
` + "```" + `

## Async Validators

` + "```go" + `
func checkUnique(
    f gui.FormFieldSnapshot, _ gui.FormSnapshot,
    signal *gui.GridAbortSignal,
) []gui.FormIssue {
    // Check signal.IsAborted() periodically.
    resp := fetchAPI(f.Value)
    if resp.Taken {
        return []gui.FormIssue{{Msg: "already taken"}}
    }
    return nil
}
` + "```" + `

## Event Wiring

In input callbacks, trigger validation via FormOnFieldEvent:

` + "```go" + `
gui.Input(gui.InputCfg{
    OnTextChanged: func(l *gui.Layout, s string, w *gui.Window) {
        gui.State[App](w).Email = s
        gui.FormOnFieldEvent(w, l, emailCfg(s),
            gui.FormTriggerChange)
    },
    OnBlur: func(l *gui.Layout, w *gui.Window) {
        gui.FormOnFieldEvent(w, l, emailCfg(app.Email),
            gui.FormTriggerBlur)
    },
})
` + "```" + `

## Submit / Reset

` + "```go" + `
gui.FormRequestSubmit(w, "my-form")
gui.FormRequestReset(w, "my-form")
` + "```" + `

## Querying State

` + "```go" + `
summary := w.FormSummary("my-form")
fs, ok := w.FormFieldState("my-form", "email")
issues := w.FormFieldErrors("my-form", "email")
pending := w.FormPendingState("my-form")
` + "```" + `

## Key Properties

| Property           | Type             | Description                        |
|--------------------|------------------|------------------------------------|
| ID                 | string           | Required form identifier           |
| ValidateOn         | FormValidateOn   | When to trigger (default BlurSubmit)|
| NoSubmitOnEnter    | bool             | Disable enter-key submit           |
| AllowInvalidSubmit | bool             | Permit submit with errors          |
| AllowPendingSubmit | bool             | Permit submit while async pending  |
| OnSubmit           | func             | Called on successful submit         |
| OnReset            | func             | Called after reset                  |
| ErrorSlot          | func             | Custom per-field error view         |
| SummarySlot        | func             | Custom summary view                 |
| PendingSlot        | func             | Custom pending indicator view       |

## Validation Modes

| Mode           | Change | Blur | Submit |
|----------------|--------|------|--------|
| OnChange       | yes    | yes  | yes    |
| OnBlur         | no     | yes  | yes    |
| OnBlurSubmit   | no     | yes  | yes    |
| OnSubmit       | no     | no   | yes    |
`,

	// Selection
	"toggle": `Checkbox-style toggle with optional label. ` + "`Checkbox()`" + ` is
an alias for ` + "`Toggle()`" + `.

## Usage

` + "```go" + `
gui.Toggle(gui.ToggleCfg{
    ID:       "accept",
    IDFocus:  300,
    Label:    "Accept terms",
    Selected: app.Accepted,
    OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
        s := gui.State[App](w)
        s.Accepted = !s.Accepted
    },
})
` + "```" + `

## Custom Check Text

` + "```go" + `
gui.Toggle(gui.ToggleCfg{
    ID:           "star",
    TextSelect:   "★",
    TextUnselect: "☆",
    Selected:     app.Starred,
})
` + "```" + `

## Key Properties

| Property     | Type         | Description                          |
|--------------|--------------|--------------------------------------|
| Label        | string       | Label text beside the checkbox       |
| Selected     | bool         | Checked state                        |
| TextSelect   | string       | Text when selected (default "✓")     |
| TextUnselect | string       | Text when unselected                 |
| IDFocus      | uint32       | Tab-order focus ID (> 0 to enable)   |
| MinWidth     | float32      | Minimum width                        |
| Disabled     | bool         | Disable interaction                  |
| Invisible    | bool         | Hide without removing from layout    |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| Radius           | Opt[float32] | Corner radius                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Background color                 |
| ColorHover       | Color        | Background on hover              |
| ColorFocus       | Color        | Background when focused          |
| ColorClick       | Color        | Background on click              |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| ColorSelect      | Color        | Background when selected         |
| TextStyle        | TextStyle    | Check mark text styling          |
| TextStyleLabel   | TextStyle    | Label text styling               |

## Events

| Callback | Signature                        | Fired when       |
|----------|----------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)   | Toggle clicked   |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"switch": `Pill-shaped on/off toggle switch with animated thumb
and optional label.

## Usage

` + "```go" + `
gui.Switch(gui.SwitchCfg{
    ID:       "feature",
    IDFocus:  400,
    Label:    "Enable feature",
    Selected: app.Enabled,
    OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
        s := gui.State[App](w)
        s.Enabled = !s.Enabled
    },
})
` + "```" + `

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| Label     | string       | Label text beside the switch         |
| Selected  | bool         | On/off state                         |
| Width     | Opt[float32] | Track width                          |
| Height    | Opt[float32] | Track height                         |
| IDFocus   | uint32       | Tab-order focus ID (> 0 to enable)   |
| Disabled  | bool         | Disable interaction                  |
| Invisible | bool         | Hide without removing from layout    |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Track background color           |
| ColorHover       | Color        | Track on hover                   |
| ColorFocus       | Color        | Track when focused               |
| ColorClick       | Color        | Track on click                   |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| ColorSelect      | Color        | Thumb color when on              |
| ColorUnselect    | Color        | Thumb color when off             |
| TextStyle        | TextStyle    | Label text styling               |

## Events

| Callback | Signature                        | Fired when       |
|----------|----------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)   | Switch toggled   |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"radio": `Circular radio button for selecting one option. Typically
used inside a RadioButtonGroup, but can be used standalone.

## Usage

` + "```go" + `
gui.Radio(gui.RadioCfg{
    ID:       "opt-go",
    IDFocus:  500,
    Label:    "Go",
    Selected: app.Lang == "go",
    OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Lang = "go"
    },
})
` + "```" + `

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| Label     | string       | Label text beside the radio circle   |
| Selected  | bool         | Selection state                      |
| Size      | Opt[float32] | Circle diameter                      |
| IDFocus   | uint32       | Tab-order focus ID (> 0 to enable)   |
| Disabled  | bool         | Disable interaction                  |
| Invisible | bool         | Hide without removing from layout    |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Circle background                |
| ColorHover       | Color        | Circle on hover                  |
| ColorFocus       | Color        | Circle when focused              |
| ColorClick       | Color        | Circle on click                  |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| ColorSelect      | Color        | Fill color when selected         |
| ColorUnselect    | Color        | Fill color when unselected       |
| TextStyle        | TextStyle    | Label text styling               |

## Events

| Callback | Signature                        | Fired when       |
|----------|----------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)   | Radio clicked    |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"radio_group": `Grouped radio buttons in row or column layout with
optional group-box border and title.

## Usage

` + "```go" + `
gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
    Value:   app.Lang,
    IDFocus: 510,
    Title:   "Language",
    Options: []gui.RadioOption{
        gui.NewRadioOption("Go", "go"),
        gui.NewRadioOption("Rust", "rust"),
    },
    OnSelect: func(v string, w *gui.Window) {
        gui.State[App](w).Lang = v
    },
})
` + "```" + `

## Horizontal Layout

` + "```go" + `
gui.RadioButtonGroupRow(gui.RadioButtonGroupCfg{
    Value:   app.Size,
    Options: []gui.RadioOption{
        gui.NewRadioOption("S", "s"),
        gui.NewRadioOption("M", "m"),
        gui.NewRadioOption("L", "l"),
    },
    OnSelect: func(v string, w *gui.Window) {
        gui.State[App](w).Size = v
    },
})
` + "```" + `

## Stdlib Data Binding

Use ` + "`Items []string`" + ` when label equals value:

` + "```go" + `
gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
    Value:   "go",
    Items:   []string{"go", "rust", "zig"},
    OnSelect: func(v string, w *gui.Window) { ... },
})
` + "```" + `

When ` + "`Items`" + ` is set, ` + "`Options`" + ` is ignored.

## Key Properties

| Property  | Type            | Description                          |
|-----------|-----------------|--------------------------------------|
| Value     | string          | Currently selected value             |
| Items     | []string        | Simple string list (alt. to Options) |
| Options   | []RadioOption   | Available choices (Label + Value)    |
| Title     | string          | Group-box label                      |
| TitleBG   | Color           | Border-eraser background for title   |
| IDFocus   | uint32          | Tab-order focus ID for first radio   |
| MinWidth  | float32         | Minimum width                        |
| MinHeight | float32         | Minimum height                       |
| Sizing    | Sizing          | Combined axis sizing mode            |

## Appearance

| Property    | Type         | Description                          |
|-------------|--------------|--------------------------------------|
| Padding     | Opt[Padding] | Inner padding                        |
| Spacing     | Opt[float32] | Gap between radio buttons            |
| SizeBorder  | Opt[float32] | Group border width                   |
| ColorBorder | Color        | Group border color                   |

## Factories

| Function                      | Layout               |
|-------------------------------|----------------------|
| RadioButtonGroupColumn(cfg)   | Vertical (stacked)   |
| RadioButtonGroupRow(cfg)      | Horizontal (inline)  |

## Events

| Callback | Signature                | Fired when          |
|----------|--------------------------|---------------------|
| OnSelect | func(string, *Window)    | Selection changes   |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"select": `Dropdown selector with single or multi-select. Options
prefixed with "---" render as subheadings.

Select already accepts ` + "`[]string`" + ` directly via the
` + "`Options`" + ` field — the zero-configuration path.

## Usage

` + "```go" + `
gui.Select(gui.SelectCfg{
    ID:       "lang",
    IDFocus:  600,
    Selected: app.Selected,
    Options:  []string{"Go", "Rust", "Zig"},
    OnSelect: func(sel []string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Selected = sel
    },
})
` + "```" + `

## Multi-Select

` + "```go" + `
gui.Select(gui.SelectCfg{
    ID:             "tags",
    Placeholder:    "Choose tags...",
    SelectMultiple: true,
    Options:        []string{"alpha", "beta", "stable"},
    OnSelect: func(sel []string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Tags = sel
    },
})
` + "```" + `

## Key Properties

| Property       | Type     | Description                          |
|----------------|----------|--------------------------------------|
| Selected       | []string | Currently selected option(s)         |
| Options        | []string | Available choices ("---" = subhead)  |
| Placeholder    | string   | Hint text when empty                 |
| SelectMultiple | bool     | Allow multi-select                   |
| NoWrap         | bool     | Clip text in multi-select mode       |
| IDFocus        | uint32   | Tab-order focus ID (> 0 to enable)   |
| MinWidth       | float32  | Minimum width                        |
| MaxWidth       | float32  | Maximum width                        |
| FloatZIndex    | int      | Z-order for dropdown overlay         |
| Sizing         | Sizing   | Combined axis sizing mode            |
| Disabled       | bool     | Disable interaction                  |
| Invisible      | bool     | Hide without removing from layout    |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| Radius           | Opt[float32] | Corner radius                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Background color                 |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| ColorFocus       | Color        | Background when focused          |
| ColorSelect      | Color        | Selected item highlight          |
| TextStyle        | TextStyle    | Option text styling              |
| SubheadingStyle  | TextStyle    | Subheading text styling          |
| PlaceholderStyle | TextStyle    | Placeholder text styling         |

## Events

| Callback | Signature                            | Fired when          |
|----------|--------------------------------------|---------------------|
| OnSelect | func([]string, *Event, *Window)      | Selection changes   |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"listbox": `Scrollable list with single or multi-select, optional
subheadings, and drag-reorder support.

## Usage

` + "```go" + `
gui.ListBox(gui.ListBoxCfg{
    ID:          "lb",
    IDFocus:     700,
    Multiple:    true,
    Height:      200,
    SelectedIDs: app.Selected,
    Data: []gui.ListBoxOption{
        gui.NewListBoxSubheading("hdr", "Languages"),
        gui.NewListBoxOption("go", "Go", "go"),
        gui.NewListBoxOption("rs", "Rust", "rust"),
    },
    OnSelect: func(ids []string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Selected = ids
    },
})
` + "```" + `

## Reorderable List

` + "```go" + `
gui.ListBox(gui.ListBoxCfg{
    ID:          "reorder",
    Reorderable: true,
    Data:        items,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        reorderItems(movedID, beforeID, w)
    },
})
` + "```" + `

## Virtualization

Renders only visible rows when height is constrained. Requires
` + "`IDScroll > 0`" + ` and ` + "`Height`" + ` or ` + "`MaxHeight > 0`" + `.

` + "```go" + `
gui.ListBox(gui.ListBoxCfg{
    ID:        "virt-lb",
    IDScroll:  500,
    MaxHeight: 200,
    Data:      items,
})
` + "```" + `

## Stdlib Data Binding

Use ` + "`Items []string`" + ` instead of ` + "`Data`" + ` for simple lists:

` + "```go" + `
gui.ListBox(gui.ListBoxCfg{
    ID:    "simple",
    Items: []string{"Go", "Rust", "Zig"},
    OnSelect: func(ids []string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Selected = ids
    },
})
` + "```" + `

When ` + "`Items`" + ` is set, ` + "`Data`" + ` is ignored.

## Key Properties

| Property    | Type             | Description                          |
|-------------|------------------|--------------------------------------|
| SelectedIDs | []string         | Selected item IDs                    |
| Items       | []string         | Simple string list (alt. to Data)    |
| Data        | []ListBoxOption  | Items (ID, Name, Value, IsSubhead)   |
| Multiple    | bool             | Allow multi-select                   |
| Height      | float32          | Fixed height (enables virtualization)|
| MinWidth    | float32          | Minimum width                        |
| MaxWidth    | float32          | Maximum width                        |
| MinHeight   | float32          | Minimum height                       |
| MaxHeight   | float32          | Maximum height                       |
| IDScroll    | uint32           | Scroll ID (enables virtualization)   |
| IDFocus     | uint32           | Tab-order focus ID (> 0 to enable)   |
| Reorderable | bool             | Enable drag-reorder                  |
| Sizing      | Sizing           | Combined axis sizing mode            |
| Disabled    | bool             | Disable interaction                  |
| Invisible   | bool             | Hide without removing from layout    |

## Appearance

| Property        | Type         | Description                      |
|-----------------|--------------|----------------------------------|
| Padding         | Opt[Padding] | Inner padding                    |
| Radius          | Opt[float32] | Corner radius                    |
| SizeBorder      | Opt[float32] | Border width                     |
| Color           | Color        | Background color                 |
| ColorHover      | Color        | Item hover highlight             |
| ColorBorder     | Color        | Border color                     |
| ColorSelect     | Color        | Selected item highlight          |
| TextStyle       | TextStyle    | Item text styling                |
| SubheadingStyle | TextStyle    | Subheading text styling          |

## Events

| Callback  | Signature                                  | Fired when        |
|-----------|--------------------------------------------|-------------------|
| OnSelect  | func([]string, *Event, *Window)            | Selection changes  |
| OnReorder | func(movedID, beforeID string, *Window)    | Item reordered     |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"combobox": `Editable dropdown with type-ahead filtering. Typing
narrows the options list; selecting an option commits the value.

Combobox already accepts ` + "`[]string`" + ` directly via the
` + "`Options`" + ` field.

## Usage

` + "```go" + `
gui.Combobox(gui.ComboboxCfg{
    ID:      "cb",
    IDFocus: 800,
    Value:   app.Value,
    Options: []string{"Go", "Rust", "Zig"},
    OnSelect: func(v string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Value = v
    },
})
` + "```" + `

## With Placeholder

` + "```go" + `
gui.Combobox(gui.ComboboxCfg{
    ID:          "search",
    Placeholder: "Search languages...",
    Options:     languages,
    OnSelect: func(v string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Lang = v
    },
})
` + "```" + `

## Virtualization

The dropdown list virtualizes when ` + "`IDScroll > 0`" + ` and
` + "`MaxDropdownHeight > 0`" + `.

` + "```go" + `
gui.Combobox(gui.ComboboxCfg{
    IDScroll:          900,
    MaxDropdownHeight: 200,
    Options:           largeList,
})
` + "```" + `

## Key Properties

| Property          | Type     | Description                          |
|-------------------|----------|--------------------------------------|
| Value             | string   | Current selection                    |
| Placeholder       | string   | Hint text shown when empty           |
| Options           | []string | Searchable options                   |
| MaxDropdownHeight | float32  | Max dropdown pixel height            |
| IDFocus           | uint32   | Tab-order focus ID (> 0 to enable)   |
| IDScroll          | uint32   | Scroll ID (enables virtualization)   |
| MinWidth          | float32  | Minimum width                        |
| MaxWidth          | float32  | Maximum width                        |
| FloatZIndex       | int      | Z-order for dropdown overlay         |
| Sizing            | Sizing   | Combined axis sizing mode            |
| Disabled          | bool     | Disable interaction                  |

## Appearance

| Property         | Type         | Description                      |
|------------------|--------------|----------------------------------|
| Padding          | Opt[Padding] | Inner padding                    |
| Radius           | Opt[float32] | Corner radius                    |
| SizeBorder       | Opt[float32] | Border width                     |
| Color            | Color        | Background color                 |
| ColorBorder      | Color        | Border color                     |
| ColorBorderFocus | Color        | Border color when focused        |
| ColorFocus       | Color        | Background when focused          |
| ColorHighlight   | Color        | Highlighted option color         |
| ColorHover       | Color        | Option hover color               |
| TextStyle        | TextStyle    | Option text styling              |
| PlaceholderStyle | TextStyle    | Placeholder text styling         |

## Events

| Callback | Signature                          | Fired when          |
|----------|------------------------------------|---------------------|
| OnSelect | func(string, *Event, *Window)      | Option selected     |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"slider": `Draggable slider for selecting a value within a
numeric range. Supports horizontal and vertical orientations.

## Example

` + "```go" + `
gui.Slider(gui.SliderCfg{
    ID:      "vol",
    IDFocus: 900,
    Value:   app.Volume,
    Min:     0, Max: 100,
    OnChange: func(v float32, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Volume = v
    },
})
` + "```" + `

## Vertical Slider

` + "```go" + `
gui.Slider(gui.SliderCfg{
    ID:       "vert",
    Vertical: true,
    Value:    app.Level,
    Min:      0, Max: 10,
    Step:     0.5,
})
` + "```" + `

## Key Properties

| Property   | Type    | Description                          |
|------------|---------|--------------------------------------|
| Value      | float32 | Current value                        |
| Min        | float32 | Range minimum (default 0)            |
| Max        | float32 | Range maximum (default 100)          |
| Step       | float32 | Step increment                       |
| Vertical   | bool    | Vertical orientation                 |
| RoundValue | bool    | Round to nearest integer             |
| ThumbSize  | float32 | Thumb diameter                       |
| Size       | float32 | Track thickness                      |
| Width      | float32 | Fixed width                          |
| Height     | float32 | Fixed height                         |
| IDFocus    | uint32  | Tab-order focus ID (> 0 to enable)   |
| Sizing     | Sizing  | Combined axis sizing mode            |
| Disabled   | bool    | Disable interaction                  |
| Invisible  | bool    | Hide without removing from layout    |

## Appearance

| Property     | Type         | Description                      |
|--------------|--------------|----------------------------------|
| Padding      | Opt[Padding] | Inner padding                    |
| Radius       | Opt[float32] | Track corner radius              |
| RadiusBorder | Opt[float32] | Border corner radius             |
| SizeBorder   | Opt[float32] | Border width                     |
| Color        | Color        | Track background color           |
| ColorLeft    | Color        | Filled portion color             |
| ColorThumb   | Color        | Thumb color                      |
| ColorHover   | Color        | Thumb on hover                   |
| ColorFocus   | Color        | Thumb when focused               |
| ColorClick   | Color        | Thumb on click                   |
| ColorBorder  | Color        | Border color                     |

## Events

| Callback | Signature                          | Fired when       |
|----------|------------------------------------|------------------|
| OnChange | func(float32, *Event, *Window)     | Value changes    |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	// Data
	"table": `Sortable data table from string arrays with row selection,
alternating row colors, virtualized scrolling, and configurable borders.

## Usage

` + "```go" + `
cfg := gui.TableCfgFromData([][]string{
    {"Name", "Age"},   // header row
    {"Alice", "30"},
    {"Bob", "25"},
})
cfg.ID = "my-table"
w.Table(cfg)
` + "```" + `

## From CSV

` + "```go" + `
cfg, err := gui.TableCfgFromCSV("Name,Age\nAlice,30\nBob,25")
if err != nil { log.Fatal(err) }
cfg.ID = "csv-table"
w.Table(cfg)
` + "```" + `

## Custom Rows

` + "```go" + `
w.Table(gui.TableCfg{
    ID:          "custom",
    BorderStyle: gui.TableBorderAll,
    Data: []gui.TableRowCfg{
        {Cells: []gui.TableCellCfg{
            {Value: "Name", HeadCell: true},
            {Value: "Score", HeadCell: true},
        }},
        {Cells: []gui.TableCellCfg{
            {Value: "Alice"},
            {Value: "95", HAlign: gui.HAlignEndPtr()},
        }},
    },
})
` + "```" + `

## Virtualization

Renders only visible rows when scrolling is enabled. Requires
` + "`IDScroll > 0`" + ` and ` + "`Height`" + ` or ` + "`MaxHeight > 0`" + `.

` + "```go" + `
cfg.IDScroll  = 100
cfg.MaxHeight = 260
` + "```" + `

## Stdlib Data Binding

Set ` + "`RawData`" + ` for CSV-style string data. First row is the header:

` + "```go" + `
w.Table(gui.TableCfg{
    ID: "simple-table",
    RawData: [][]string{
        {"Name", "Age"},
        {"Alice", "30"},
        {"Bob",   "25"},
    },
})
` + "```" + `

When ` + "`RawData`" + ` is set, ` + "`Data`" + ` is ignored.

## Border Styles

| Constant             | Description                        |
|----------------------|------------------------------------|
| TableBorderNone      | No borders                         |
| TableBorderAll       | Full grid                          |
| TableBorderHorizontal| Horizontal lines between rows      |
| TableBorderHeaderOnly| Single line under header row       |

## Key Properties

| Property           | Type              | Description                          |
|--------------------|-------------------|--------------------------------------|
| RawData            | [][]string        | CSV-style data (alt. to Data)        |
| Data               | []TableRowCfg     | Row data with cells                  |
| ColumnAlignments   | []HorizontalAlign | Per-column alignment                 |
| ColumnWidthDefault | float32           | Default column width                 |
| ColumnWidthMin     | float32           | Minimum column width                 |
| AlignHead          | HorizontalAlign   | Header row alignment                 |
| MultiSelect        | bool              | Allow multi-row selection            |
| Selected           | map[int]bool      | Selected row indices                 |
| IDScroll           | uint32            | Scroll ID (enables virtualization)   |
| Scrollbar          | ScrollbarOverflow | Scrollbar overflow mode              |
| BorderStyle        | TableBorderStyle  | Cell border style                    |
| Sizing             | Sizing            | Combined axis sizing mode            |
| Width              | float32           | Fixed width                          |
| Height             | float32           | Fixed height                         |
| MinWidth           | float32           | Minimum width                        |
| MaxWidth           | float32           | Maximum width                        |
| MinHeight          | float32           | Minimum height                       |
| MaxHeight          | float32           | Maximum height                       |

## Appearance

| Property         | Type         | Description                          |
|------------------|--------------|--------------------------------------|
| ColorBorder      | Color        | Border/grid line color               |
| ColorSelect      | Color        | Selected row background              |
| ColorHover       | Color        | Hovered row background               |
| ColorRowAlt      | *Color       | Alternating row background           |
| CellPadding      | Opt[Padding] | Padding inside each cell             |
| TextStyle        | TextStyle    | Body cell text style                 |
| TextStyleHead    | TextStyle    | Header cell text style               |
| SizeBorder       | float32      | Border line width                    |
| SizeBorderHeader | float32      | Header border line width             |

## Events

| Callback | Signature                                    | Fired when             |
|----------|----------------------------------------------|------------------------|
| OnSelect | func(map[int]bool, int, *Event, *Window)     | Row selection changes  |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"data_grid": `Full-featured data grid with sorting, filtering, pagination,
virtualized scrolling, column chooser, grouping, aggregation, inline editing,
row selection, and CRUD toolbar support.

## Usage

` + "```go" + `
w.DataGrid(gui.DataGridCfg{
    ID:       "grid",
    PageSize: 10,
    Columns: []gui.GridColumnCfg{
        {ID: "name", Title: "Name", Sortable: true, Filterable: true},
        {ID: "age",  Title: "Age",  Sortable: true, Align: gui.HAlignEnd},
    },
    Rows: []gui.GridRow{
        {ID: "1", Cells: map[string]string{"name": "Alice", "age": "30"}},
        {ID: "2", Cells: map[string]string{"name": "Bob",   "age": "25"}},
    },
    ShowQuickFilter:  true,
    ShowColumnChooser: true,
})
` + "```" + `

## With DataSource

` + "```go" + `
source := gui.NewInMemoryDataSource(rows)
w.DataGrid(gui.DataGridCfg{
    ID:             "ds-grid",
    Columns:        cols,
    DataSource:     source,
    PaginationKind: gui.GridPaginationCursor,
    PageLimit:      50,
})
` + "```" + `

## Stdlib Data Binding

Use ` + "`RowsData []map[string]string`" + ` for key-value data.
When ` + "`Columns`" + ` is empty, columns are auto-generated from the
first row's keys:

` + "```go" + `
w.DataGrid(gui.DataGridCfg{
    ID: "simple-grid",
    RowsData: []map[string]string{
        {"name": "Alice", "age": "30"},
        {"name": "Bob",   "age": "25"},
    },
})
` + "```" + `

When ` + "`RowsData`" + ` is set, ` + "`Rows`" + ` is ignored.
` + "`DataSource`" + ` still wins over ` + "`RowsData`" + `.

## Virtualization

Automatically virtualizes when ` + "`Height`" + ` or ` + "`MaxHeight > 0`" + `. A scroll
ID is derived from the widget ID; setting ` + "`IDScroll`" + ` is optional.

` + "```go" + `
w.DataGrid(gui.DataGridCfg{
    ID:        "grid",
    MaxHeight: 300,
    // Virtualization is automatic.
})
` + "```" + `

## Key Properties

| Property               | Type               | Description                          |
|------------------------|--------------------|--------------------------------------|
| Columns                | []GridColumnCfg    | Column definitions                   |
| ColumnOrder            | []string           | Column display order by ID           |
| RowsData               | []map[string]string | Key-value row data (alt. to Rows)    |
| Rows                   | []GridRow          | Static row data                      |
| DataSource             | DataGridDataSource | Async data backend                   |
| PaginationKind         | GridPaginationKind | Cursor or offset pagination          |
| PageSize               | int                | Rows per page (static)               |
| PageLimit              | int                | Rows per page (data source)          |
| PageIndex              | int                | Current page index                   |
| Query                  | GridQueryState     | Active sorts, filters, search        |
| Selection              | GridSelection      | Current row selection state          |
| GroupBy                | []string           | Column IDs for row grouping          |
| Aggregates             | []GridAggregateCfg | Column aggregate definitions         |
| ShowQuickFilter        | bool               | Show search bar                      |
| ShowFilterRow          | bool               | Show per-column filters              |
| ShowColumnChooser      | bool               | Show column visibility picker        |
| ShowCRUDToolbar        | bool               | Show create/delete toolbar           |
| FreezeHeader           | bool               | Sticky header during scroll          |
| MultiSort              | *bool              | Allow multi-column sort              |
| MultiSelect            | *bool              | Allow multi-row selection            |
| RangeSelect            | *bool              | Allow shift-click range select       |
| ShowHeader             | *bool              | Show/hide header row                 |
| HiddenColumnIDs        | map[string]bool    | Columns hidden by ID                 |
| FrozenTopRowIDs        | []string           | Row IDs pinned to top                |
| DetailExpandedRowIDs   | map[string]bool    | Expanded detail row IDs              |
| QuickFilterPlaceholder | string             | Quick filter placeholder text        |
| QuickFilterDebounce    | time.Duration      | Quick filter debounce delay          |
| RowHeight              | float32            | Row height in pixels                 |
| HeaderHeight           | float32            | Header height in pixels              |
| IDFocus                | uint32             | Tab-order focus ID (> 0 to enable)   |
| IDScroll               | uint32             | Scroll ID (auto-generated if omitted)|
| Disabled               | bool               | Disable interaction                  |
| Invisible              | bool               | Hide without removing from layout    |
| Sizing                 | Sizing             | Combined axis sizing mode            |
| Width                  | float32            | Fixed width                          |
| Height                 | float32            | Fixed height                         |
| MinWidth               | float32            | Minimum width                        |
| MaxWidth               | float32            | Maximum width                        |
| MinHeight              | float32            | Minimum height                       |
| MaxHeight              | float32            | Maximum height                       |

## GridColumnCfg

| Property    | Type            | Description                          |
|-------------|-----------------|--------------------------------------|
| ID          | string          | Column identifier                    |
| Title       | string          | Header text                          |
| Width       | float32         | Column width                         |
| Sortable    | bool            | Enable sorting                       |
| Filterable  | bool            | Enable filtering                     |
| Editable    | bool            | Enable inline editing                |
| Resizable   | bool            | Enable drag-resize                   |
| Reorderable | bool            | Enable drag-reorder                  |
| Pin         | GridColumnPin   | Pin left or right                    |
| Align       | HorizontalAlign | Cell alignment                       |

## Appearance

| Property          | Type         | Description                          |
|-------------------|--------------|--------------------------------------|
| ColorBackground   | Color        | Grid background                      |
| ColorHeader       | Color        | Header background                    |
| ColorHeaderHover  | Color        | Header hover background              |
| ColorFilter       | Color        | Filter row background                |
| ColorQuickFilter  | Color        | Quick filter background              |
| ColorRowHover     | Color        | Row hover background                 |
| ColorRowAlt       | Color        | Alternating row background           |
| ColorRowSelected  | Color        | Selected row background              |
| ColorBorder       | Color        | Border/grid line color               |
| ColorResizeHandle | Color        | Column resize handle color           |
| ColorResizeActive | Color        | Active resize handle color           |
| PaddingCell       | Opt[Padding] | Cell padding                         |
| PaddingHeader     | Opt[Padding] | Header cell padding                  |
| PaddingFilter     | Padding      | Filter row padding                   |
| TextStyle         | TextStyle    | Body cell text style                 |
| TextStyleHeader   | TextStyle    | Header text style                    |
| TextStyleFilter   | TextStyle    | Filter text style                    |
| Radius            | float32      | Corner radius                        |
| SizeBorder        | float32      | Border width                         |
| Scrollbar         | ScrollbarOverflow | Scrollbar overflow mode          |

## Events

| Callback               | Signature                                              | Fired when                    |
|------------------------|--------------------------------------------------------|-------------------------------|
| OnQueryChange          | func(GridQueryState, *Event, *Window)                  | Sort/filter/search changes    |
| OnSelectionChange      | func(GridSelection, *Event, *Window)                   | Row selection changes         |
| OnColumnOrderChange    | func([]string, *Event, *Window)                        | Column reorder                |
| OnColumnPinChange      | func(string, GridColumnPin, *Event, *Window)           | Column pin changes            |
| OnHiddenColumnsChange  | func(map[string]bool, *Event, *Window)                 | Column visibility changes     |
| OnPageChange           | func(int, *Event, *Window)                             | Page navigation               |
| OnDetailExpandedChange | func(map[string]bool, *Event, *Window)                 | Detail row expand/collapse    |
| OnCellEdit             | func(GridCellEdit, *Event, *Window)                    | Inline cell edit committed    |
| OnRowsChange           | func([]GridRow, *Event, *Window)                       | Row data changes              |
| OnCellClick            | func(string, *Event, *Window)                          | Cell clicked                  |
| OnCellFormat           | func(GridRow, int, GridColumnCfg, string, *Window) GridCellFormat | Custom cell formatting |
| OnDetailRowView        | func(GridRow, *Window) View                            | Render detail row content     |
| OnCopyRows             | func([]GridRow, *Event, *Window) (string, bool)        | Copy selected rows            |
| OnRowActivate          | func(GridRow, *Event, *Window)                         | Row double-click/enter        |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"data_source": `Async data-source backed grid with CRUD operations, pagination,
abort handling, and ORM integration. Implement the ` + "`DataGridDataSource`" + `
interface or use the built-in ` + "`InMemoryDataSource`" + ` / ` + "`GridOrmDataSource`" + `.

## Usage

` + "```go" + `
source := gui.NewInMemoryDataSource([]gui.GridRow{
    {
        ID: "1",
        Cells: map[string]string{
            "name": "Alice", "team": "Core", "status": "Open",
        },
    },
})

w.DataGrid(gui.DataGridCfg{
    ID:              "ds-grid",
    Columns:         showcaseDataGridColumns(),
    DataSource:      source,
    PaginationKind:  gui.GridPaginationCursor,
    PageLimit:       50,
    ShowQuickFilter: true,
    ShowCRUDToolbar: true,
})
` + "```" + `

## ORM Data Source

` + "```go" + `
src, err := gui.NewGridOrmDataSource(gui.GridOrmDataSource{
    Columns: []gui.GridOrmColumnSpec{
        {ID: "name", DBField: "name", Sortable: true, Filterable: true},
    },
    FetchFn: func(spec gui.GridOrmQuerySpec, sig *gui.GridAbortSignal) (gui.GridOrmPage, error) {
        // Query database using spec.Sorts, spec.Filters, spec.Limit, spec.Offset
        return gui.GridOrmPage{Rows: rows, RowCount: total}, nil
    },
    CreateFn:  myCreateFn,
    UpdateFn:  myUpdateFn,
    DeleteFn:  myDeleteFn,
})
` + "```" + `

## DataGridDataSource Interface

` + "```go" + `
type DataGridDataSource interface {
    Capabilities() GridDataCapabilities
    FetchData(req GridDataRequest) (GridDataResult, error)
    MutateData(req GridMutationRequest) (GridMutationResult, error)
}
` + "```" + `

## GridDataCapabilities

| Field                    | Type | Description                          |
|--------------------------|------|--------------------------------------|
| SupportsCursorPagination | bool | Supports cursor-based pagination     |
| SupportsOffsetPagination | bool | Supports offset-based pagination     |
| SupportsNumberedPages    | bool | Supports numbered page navigation    |
| RowCountKnown            | bool | Total row count is available         |
| SupportsCreate           | bool | Supports row creation                |
| SupportsUpdate           | bool | Supports row updates                 |
| SupportsDelete           | bool | Supports single row deletion         |
| SupportsBatchDelete      | bool | Supports multi-row deletion          |

## InMemoryDataSource

| Field          | Type      | Description                          |
|----------------|-----------|--------------------------------------|
| Rows           | []GridRow | In-memory row data                   |
| DefaultLimit   | int       | Default page size (100)              |
| LatencyMs      | int       | Simulated latency in ms              |
| RowCountKnown  | bool      | Expose total row count               |
| SupportsCursor | bool      | Enable cursor pagination             |
| SupportsOffset | bool      | Enable offset pagination             |

## GridOrmDataSource

| Field          | Type              | Description                          |
|----------------|-------------------|--------------------------------------|
| Columns        | []GridOrmColumnSpec | Column specs with DB mapping       |
| FetchFn        | GridOrmFetchFn    | Fetch callback (required)            |
| CreateFn       | GridOrmCreateFn   | Create row callback                  |
| UpdateFn       | GridOrmUpdateFn   | Update row callback                  |
| DeleteFn       | GridOrmDeleteFn   | Delete row callback                  |
| DeleteManyFn   | GridOrmDeleteManyFn | Batch delete callback              |
| DefaultLimit   | int               | Default page size                    |
| SupportsOffset | bool              | Enable offset pagination             |
| RowCountKnown  | bool              | Expose total row count               |

Runtime stats available via ` + "`w.DataGridSourceStats(id)`" + `.
`,

	// Text
	"text": `Single-style text rendering with theme typography. Supports wrapping,
alignment, gradients, outlines, rotation, custom colors, and text selection
via IDFocus. Use for labels, headings, or larger blocks of text.

## Usage

` + "```go" + `
t := gui.CurrentTheme()

// Basic text
gui.Text(gui.TextCfg{Text: "Hello", TextStyle: t.N3})

// Wrapping text
gui.Text(gui.TextCfg{
    Text:      "Long paragraph that wraps to container width.",
    TextStyle: t.N4,
    Mode:      gui.TextModeWrap,
})

// Custom color
gui.Text(gui.TextCfg{
    Text: "Colored",
    TextStyle: gui.TextStyle{
        Color: gui.ColorFromString("#3b82f6"),
        Size:  t.N3.Size,
    },
})
` + "```" + `

## Theme Style Shortcuts

| Prefix | Meaning   | Sizes                  |
|--------|-----------|------------------------|
| N      | Normal    | N1–N6 (XLarge–Tiny)    |
| B      | Bold      | B1–B6                  |
| I      | Italic    | I1–I6                  |
| M      | Monospace | M1–M6                  |
| Icon   | Icon font | Icon1–Icon6            |

## Text Modes

| Mode                 | Behavior                       |
|----------------------|--------------------------------|
| TextModeSingleLine   | No wrapping (default)          |
| TextModeMultiline    | Honors ` + "`\\n`" + ` line breaks        |
| TextModeWrap         | Word-wraps to container width  |
| TextModeWrapKeepSpaces | Wraps preserving whitespace  |

## Key Properties

| Property          | Type      | Description                          |
|-------------------|-----------|--------------------------------------|
| Text              | string    | Content to display                   |
| TextStyle         | TextStyle | Font, size, color, decorations       |
| Mode              | TextMode  | Wrapping behavior                    |
| IDFocus           | uint32    | Tab-order focus ID (enables select)  |
| TabSize           | uint32    | Tab width in spaces                  |
| MinWidth          | float32   | Minimum text width                   |
| Clip              | bool      | Clip text to bounds                  |
| Opacity           | float32   | Text opacity (0.0–1.0)              |
| Hero              | bool      | Animate text transitions             |
| IsPassword        | bool      | Mask characters                      |
| PlaceholderActive | bool      | Render as placeholder style          |
| Sizing            | Sizing    | Override default sizing              |
| Disabled          | bool      | Disable interaction                  |
| Invisible         | bool      | Hide without removing from layout    |
| FocusSkip         | bool      | Skip in tab-order navigation         |

## TextStyle Fields

| Field           | Type                  | Description                    |
|-----------------|-----------------------|--------------------------------|
| Color           | Color                 | Text fill color                |
| Size            | float32               | Font size in points            |
| Align           | TextAlignment         | Left, center, or right         |
| Underline       | bool                  | Underline decoration           |
| Strikethrough   | bool                  | Strikethrough decoration       |
| LetterSpacing   | float32               | Extra space between characters |
| LineSpacing     | float32               | Extra space between lines      |
| Gradient        | *glyph.GradientConfig | Gradient text fill             |
| StrokeWidth     | float32               | Text outline width             |
| StrokeColor     | Color                 | Text outline color             |
| RotationRadians | float32               | Text rotation                  |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"rtf": `Mixed styles, links, abbreviations, footnotes, and decorations
within a single text block. Build a paragraph from ` + "`RichTextRun`" + `
slices rendered by ` + "`gui.RTF`" + `.

## Usage

` + "```go" + `
t := gui.CurrentTheme()
gui.RTF(gui.RtfCfg{
    RichText: gui.RichText{
        Runs: []gui.RichTextRun{
            gui.RichRun("Normal, ", t.N3),
            gui.RichRun("bold, ", t.B3),
            gui.RichRun("italic", t.I3),
        },
    },
})
` + "```" + `

## With Links and Abbreviations

` + "```go" + `
gui.RTF(gui.RtfCfg{
    RichText: gui.RichText{
        Runs: []gui.RichTextRun{
            gui.RichRun("Visit ", t.N3),
            gui.RichLink("Go docs", "https://go.dev", t.N3),
            gui.RichRun(". ", t.N3),
            gui.RichAbbr("HTML", "HyperText Markup Language", t.N3),
        },
    },
    Mode: gui.TextModeWrap,
})
` + "```" + `

## Run Helpers

| Helper                                     | Description                              |
|--------------------------------------------|------------------------------------------|
| ` + "`RichRun(text, style)`" + `           | Styled text run                          |
| ` + "`RichLink(text, url, style)`" + `     | Hyperlink (auto underline + theme color) |
| ` + "`RichAbbr(text, expansion, style)`" + ` | Abbreviation with tooltip              |
| ` + "`RichFootnote(id, content, style)`" + ` | Superscript footnote with tooltip      |
| ` + "`RichBr()`" + `                       | Line break                               |

## Text Decorations

Set ` + "`Underline`" + ` or ` + "`Strikethrough`" + ` on a ` + "`TextStyle`" + `:

` + "```go" + `
gui.RichRun("underlined", gui.TextStyle{
    Color: t.N3.Color, Size: t.N3.Size,
    Underline: true,
})
` + "```" + `

## Key Properties

| Property      | Type       | Description                          |
|---------------|------------|--------------------------------------|
| RichText      | RichText   | Runs of styled text                  |
| MinWidth      | float32    | Minimum block width                  |
| Mode          | TextMode   | Text wrapping mode                   |
| HangingIndent | float32    | Negative indent for wrapped lines    |
| Clip          | bool       | Clip text to bounds                  |
| BaseTextStyle | *TextStyle | Fallback style for runs              |
| IDFocus       | uint32     | Tab-order focus ID (enables select)  |
| Disabled      | bool       | Disable interaction                  |
| Invisible     | bool       | Hide without removing from layout    |
| FocusSkip     | bool       | Skip in tab-order navigation         |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"markdown": `Render markdown strings with syntax highlighting, tables, blockquotes,
math (LaTeX via CodeCogs), and mermaid diagrams (via Kroki). Uses the
built-in parser and renders via RTF views internally.

## Usage

` + "```go" + `
w.Markdown(gui.MarkdownCfg{
    Source: "# Hello\n**Bold** and *italic*",
    Style:  gui.DefaultMarkdownStyle(),
})
` + "```" + `

## Custom Style

` + "```go" + `
style := gui.DefaultMarkdownStyle()
style.LinkColor = gui.ColorFromString("#3b82f6")
style.CodeBlockBG = gui.RGBA(30, 30, 30, 255)
w.Markdown(gui.MarkdownCfg{
    Source: src,
    Style:  style,
})
` + "```" + `

## Supported Elements

- Headings (H1–H6), paragraphs, line breaks
- **Bold**, *italic*, ~~strikethrough~~, ` + "`code`" + `
- Ordered and unordered lists
- Tables with column alignment
- Fenced code blocks with syntax highlighting
- Blockquotes, horizontal rules
- Links and images
- Mermaid diagrams (` + "` ```mermaid `" + `)
- LaTeX math (requires ` + "`SetMarkdownExternalAPIsEnabled(true)`" + `)

## Key Properties

| Property            | Type           | Description                          |
|---------------------|----------------|--------------------------------------|
| Source              | string         | Markdown source string               |
| Style               | MarkdownStyle  | Typography and color configuration   |
| Mode                | Opt[TextMode]  | Text wrapping mode                   |
| IDFocus             | uint32         | Tab-order focus ID (enables select)  |
| MinWidth            | float32        | Minimum view width                   |
| MermaidWidth        | int            | Max mermaid diagram width (0 = 600)  |
| DisableExternalAPIs | bool           | Disable CodeCogs/Kroki APIs          |
| Disabled            | bool           | Disable interaction                  |
| Invisible           | bool           | Hide without removing from layout    |
| Clip                | bool           | Clip content to bounds               |
| FocusSkip           | bool           | Skip in tab-order navigation         |

## Appearance

| Property    | Type         | Description                          |
|-------------|--------------|--------------------------------------|
| Color       | Color        | Background color                     |
| ColorBorder | Color        | Border color                         |
| SizeBorder  | float32      | Border width                         |
| Radius      | float32      | Corner radius                        |
| Padding     | Opt[Padding] | Inner padding                        |

## MarkdownStyle

| Field            | Type             | Description                       |
|------------------|------------------|-----------------------------------|
| Text             | TextStyle        | Body text style                   |
| H1–H6           | TextStyle        | Heading styles                    |
| Bold             | TextStyle        | Bold text style                   |
| Italic           | TextStyle        | Italic text style                 |
| BoldItalic       | TextStyle        | Bold+italic text style            |
| Code             | TextStyle        | Inline code style                 |
| CodeBlockText    | TextStyle        | Code block text style             |
| CodeBlockBG      | Color            | Code block background             |
| CodeBlockPadding | Opt[Padding]     | Code block padding                |
| CodeBlockRadius  | float32          | Code block corner radius          |
| LinkColor        | Color            | Hyperlink color                   |
| HRColor          | Color            | Horizontal rule color             |
| BlockquoteBorder | Color            | Blockquote left border color      |
| BlockquoteBG     | Color            | Blockquote background             |
| BlockSpacing     | float32          | Spacing between blocks            |
| NestIndent       | float32          | Indent per nesting level          |
| H1Separator      | bool             | Show line under H1                |
| H2Separator      | bool             | Show line under H2                |
| TableBorderStyle | TableBorderStyle | Table border style                |
| TableBorderColor | Color            | Table border color                |
| TableHeadStyle   | TextStyle        | Table header text style           |
| TableCellStyle   | TextStyle        | Table cell text style             |
| HighlightBG      | Color            | Highlight background              |
| HardLineBreaks   | bool             | Treat newlines as hard breaks     |
| MermaidBG        | Color            | Mermaid diagram background        |
`,

	// Graphics
	"svg": `Scalable vector graphics from file or inline SVG string.
Faithful rendering of designer-authored static and animated SVG —
not a complete SVG 1.1/2 implementation. See ` + "`docs/svg-support.md`" + `
for the authoritative feature matrix.

## From File

` + "```go" + `
gui.Svg(gui.SvgCfg{
    FileName: "diagram.svg",
    Width:    200,
    Height:   150,
})
` + "```" + `

## Inline SVG

` + "```go" + `
gui.Svg(gui.SvgCfg{
    SvgData: "<svg viewBox=\"0 0 24 24\">...</svg>",
    Width:   48,
    Height:  48,
    Color:   gui.White,
})
` + "```" + `

Provide either ` + "`FileName`" + ` or ` + "`SvgData`" + `, not both. When width/height
are omitted, native SVG dimensions are used.

## Supported Features

**Shapes:** ` + "`<path>`" + `, ` + "`<rect>`" + ` (rx/ry), ` + "`<circle>`" + `,
` + "`<ellipse>`" + `, ` + "`<line>`" + `, ` + "`<polygon>`" + `,
` + "`<polyline>`" + `, ` + "`<g>`" + ` / ` + "`<a>`" + `, ` + "`<defs>`" + `,
` + "`<use>`" + ` / ` + "`<symbol>`" + ` (href + xlink:href).

**Text:** ` + "`<text>`" + `, ` + "`<tspan>`" + ` (positioned runs),
` + "`<textPath>`" + ` (text along path).

**Paint:** solid colors (named, ` + "`#rgb`" + `, ` + "`#rrggbb`" + `,
` + "`rgb()`" + `, ` + "`rgba()`" + `), ` + "`currentColor`" + `,
linear + radial gradients including ` + "`spreadMethod`" + ` =
` + "`pad`" + ` / ` + "`reflect`" + ` / ` + "`repeat`" + `, focal
` + "`fx`" + `/` + "`fy`" + `, ` + "`gradientUnits`" + ` (objectBoundingBox + userSpaceOnUse).

**Stroke:** width, dasharray + dashoffset, linecap, linejoin (incl. miter limit),
non-scaling-stroke flatness control.

**Transform & Clip:** ` + "`transform`" + ` (translate/rotate/scale/skew/matrix),
` + "`clipPath`" + `, single ` + "`feGaussianBlur`" + ` filter.

**Animation:** SMIL ` + "`<animate>`" + `, ` + "`<animateTransform>`" + `,
` + "`<animateMotion>`" + ` plus CSS ` + "`@keyframes`" + ` (` + "`NoAnimate`" + `
disables both).

**CSS:** inline ` + "`style=\"\"`" + `, ` + "`<style>`" + ` blocks, class +
attribute selectors, descendant + sibling combinators (` + "`>`" + `, ` + "`+`" + `,
` + "`~`" + `), pseudo-classes ` + "`:hover`" + ` / ` + "`:focus`" + ` /
` + "`:not()`" + `, custom properties (` + "`--name`" + ` + ` + "`var(--name, fallback)`" + `),
` + "`calc()`" + ` arithmetic in property values.

` + "```svg" + `
<svg viewBox="0 0 100 100" xmlns="http://www.w3.org/2000/svg">
  <style>
    svg { --accent: #06b6d4; }
    .dot { fill: #475569; r: 10; }
    .dot[data-state="on"] { fill: var(--accent); }
    .dot + .dot { r: calc(10 + 2); }
    .dot:hover { fill: #ec4899; }
  </style>
  <circle class="dot" cx="20" cy="50"/>
  <circle class="dot" cx="50" cy="50" data-state="on"/>
  <circle class="dot" cx="80" cy="50"/>
</svg>
` + "```" + `

**A11y:** ` + "`<title>`" + `, ` + "`<desc>`" + `, ` + "`aria-label`" + `,
` + "`aria-roledescription`" + `, ` + "`aria-hidden`" + ` (surfaced via
` + "`SvgParsed.A11y`" + ` and ` + "`SvgCfg.A11Y*`" + ` overrides).

**Not supported:** ` + "`<image>`" + `, ` + "`<foreignObject>`" + `,
` + "`<switch>`" + `, ` + "`<pattern>`" + `, ` + "`<filter>`" + ` graph,
` + "`gradientTransform`" + `. Unsupported elements render as the static
first frame or are ignored.

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| FileName  | string       | SVG file path                        |
| SvgData   | string       | Inline SVG string                    |
| Width     | float32      | Display width (0 = native)           |
| Height    | float32      | Display height (0 = native)          |
| Color     | Color        | Override fill (for monochrome icons) |
| NoAnimate | bool         | Disable SMIL + CSS animation         |
| Sizing    | Sizing       | Combined axis sizing mode            |
| Padding   | Opt[Padding] | Inner padding                        |

## Events

| Callback | Signature                          | Fired when       |
|----------|------------------------------------|------------------|
| OnClick  | func(*Layout, *Event, *Window)     | SVG clicked      |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label (overrides title)   |
| A11YDescription | string | Accessible description (overrides desc) |

` + "`<title>`" + `/` + "`<desc>`" + ` and ` + "`aria-*`" + ` attributes inside the SVG
populate ` + "`SvgParsed.A11y`" + ` and feed the platform a11y tree when no
` + "`A11YLabel`" + ` override is set.
`,

	"image": `Display raster image files (PNG, JPEG, GIF, BMP, WebP).
Supports local paths and remote HTTP/HTTPS URLs with automatic
download caching. Defaults to 100x100 when no size is specified.

## Usage

` + "```go" + `
gui.Image(gui.ImageCfg{
    Src:    "photo.png",
    Width:  200,
    Height: 150,
})
` + "```" + `

## Remote Image

` + "```go" + `
gui.Image(gui.ImageCfg{
    Src:    "https://example.com/logo.png",
    Width:  120,
    Height: 40,
    BgColor: gui.White,
})
` + "```" + `

Remote images are fetched asynchronously, cached locally, and
displayed on completion. A placeholder rectangle is shown while
downloading.

## Key Properties

| Property  | Type    | Description                          |
|-----------|---------|--------------------------------------|
| Src       | string  | File path or HTTP/HTTPS URL          |
| Width     | float32 | Display width (default 100)          |
| Height    | float32 | Display height (default 100)         |
| MinWidth  | float32 | Minimum width                        |
| MaxWidth  | float32 | Maximum width                        |
| MinHeight | float32 | Minimum height                       |
| MaxHeight | float32 | Maximum height                       |
| BgColor   | Color   | Opaque fill behind image             |
| Invisible | bool    | Hide without removing from layout    |

## Events

| Callback | Signature                          | Fired when         |
|----------|------------------------------------|--------------------|
| OnClick  | func(*Layout, *Event, *Window)     | Image clicked      |
| OnHover  | func(*Layout, *Event, *Window)     | Mouse enters image |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"gesture": `Touch gesture recognition system.

Recognizes tap, double-tap, long-press, pan, swipe, pinch, and rotate
from raw touch input. Single-touch automatically synthesizes mouse
events for backward compatibility.

## Gesture Types

| Type       | Touches | Trigger                                  |
|------------|---------|------------------------------------------|
| Tap        | 1       | Down + up < 300ms, < 10px movement       |
| DoubleTap  | 1       | Two taps within 300ms                    |
| LongPress  | 1       | Held > 500ms without movement            |
| Pan        | 1       | Down + move > 10px                       |
| Swipe      | 1       | Fast pan ending > 500 px/s               |
| Pinch      | 2       | Two-finger spread/squeeze                |
| Rotate     | 2       | Two-finger twist                         |

## Event Fields

| Field             | Type         | Description                         |
|-------------------|--------------|-------------------------------------|
| OnGesture         | callback     | Receives all gesture events         |
| e.GestureType     | GestureType  | Tap, DoubleTap, LongPress, Pan, .. |
| e.GesturePhase    | GesturePhase | Began, Changed, Ended, Cancelled   |
| e.GestureDX/DY    | float32      | Pan/swipe translation delta         |
| e.PinchScale      | float32      | Cumulative scale (1.0 = unchanged)  |
| e.GestureRotation | float32      | Cumulative rotation in radians      |
| e.VelocityX/Y     | float32      | Swipe velocity in px/s              |
| e.CentroidX/Y     | float32      | Center of active touches            |

Pan gestures automatically scroll containers with ` + "`IDScroll > 0`" + `.
Available on ` + "`ContainerCfg`" + ` and ` + "`DrawCanvasCfg`" + `.
Use Chrome DevTools touch emulation or a touchscreen to test.
`,

	"draw_canvas": `Procedural 2D drawing canvas with cached tessellation.
Draw shapes, lines, text, images, and arcs via the
` + "`OnDraw`" + ` callback. Output is tessellated into triangles
and cached by ` + "`Version`" + ` — only re-drawn when the version
changes. Optional ` + "`IDFocus`" + ` + ` + "`OnKeyDown`" + ` make the
canvas keyboard-focusable.

## Usage

` + "```go" + `
gui.DrawCanvas(gui.DrawCanvasCfg{
    ID:      "my-canvas",
    Version: 1,
    Width:   400,
    Height:  300,
    Color:   gui.RGBA(30, 30, 40, 255),
    Radius:  8,
    Padding: gui.Some(gui.Padding{Top: 20, Right: 20,
        Bottom: 20, Left: 20}),
    OnDraw: func(dc *gui.DrawContext) {
        dc.FilledRoundedRect(10, 10, 100, 60, 8, gui.White)
        dc.DashedLine(0, 80, 400, 80, gui.Gray, 1, 6, 4)
        dc.PolylineJoined(pts, gui.White, 2)
        dc.Text(10, 90, "label", style)
    },
})
` + "```" + `

## Drawing API

### Shapes

| Method           | Signature                                                          | Description                        |
|------------------|--------------------------------------------------------------------|------------------------------------|
| FilledRect       | (x, y, w, h float32, color Color)                                 | Filled rectangle                   |
| Rect             | (x, y, w, h float32, color Color, width float32)                  | Stroked rectangle                  |
| FilledRoundedRect| (x, y, w, h, radius float32, color Color)                         | Filled rectangle with rounded corners |
| RoundedRect      | (x, y, w, h, radius float32, color Color, width float32)          | Stroked rectangle with rounded corners |
| FilledPolygon    | (points []float32, color Color)                                    | Filled convex polygon              |
| FilledCircle     | (cx, cy, radius float32, color Color)                              | Filled circle                      |
| Circle           | (cx, cy, radius float32, color Color, width float32)               | Stroked circle                     |
| FilledArc        | (cx, cy, rx, ry, start, sweep float32, color Color)                | Filled elliptical arc              |
| Arc              | (cx, cy, rx, ry, start, sweep float32, color Color, width float32) | Stroked elliptical arc             |

### Lines

| Method          | Signature                                                                     | Description                             |
|-----------------|-------------------------------------------------------------------------------|-----------------------------------------|
| Line            | (x0, y0, x1, y1 float32, color Color, width float32)                         | Single solid line segment               |
| Polyline        | (points []float32, color Color, width float32)                                | Stroked open polyline (no joins)        |
| PolylineJoined  | (points []float32, color Color, width float32)                                | Polyline with miter joins at vertices   |
| DashedLine      | (x0, y0, x1, y1 float32, color Color, width, dashLen, gapLen float32)        | Dashed line segment                     |
| DashedPolyline  | (points []float32, color Color, width, dashLen, gapLen float32)               | Polyline with continuous dash pattern   |

### Text

| Method     | Signature                                       | Description                                 |
|------------|--------------------------------------------------|---------------------------------------------|
| Text       | (x, y float32, text string, style TextStyle)     | Draw text at position (top-left origin)     |
| TextWidth  | (text string, style TextStyle) float32           | Measure text width in given style           |
| FontHeight | (style TextStyle) float32                        | Line height for given style                 |

### Images

| Method | Signature                                                                           | Description                                         |
|--------|-------------------------------------------------------------------------------------|-----------------------------------------------------|
| Image  | (x, y, w, h float32, src string, bgOpacity Opt[float32], bgColor Color)             | Draw image inside the canvas; ` + "`src`" + ` matches ` + "`ImageCfg.Src`" + ` |

` + "`src`" + ` accepts the same forms as ` + "`ImageCfg.Src`" + `:

- Local filesystem path
- ` + "`http://`" + ` / ` + "`https://`" + ` URL (cached on disk)
- ` + "`data:`" + ` URL (base64 payload)

` + "`bgOpacity`" + ` is an ` + "`Opt[float32]`" + ` in [0, 1]; zero value = 1.0.
It modulates the background-color alpha only; it does not fade the
image texture itself. ` + "`bgColor`" + ` paints behind the image
(useful for PNGs with transparency); zero value = transparent.

Example:

` + "```go" + `
dc.Image(0, 0, 64, 64,
    "assets/tile.png",
    gui.SomeF(0.85), gui.Black)
` + "```" + `

## Keyboard Focus

Setting ` + "`IDFocus > 0`" + ` opts the canvas into tab order. The
paired ` + "`OnKeyDown`" + ` callback fires when the canvas is focused
and a key is pressed. Set ` + "`e.IsHandled = true`" + ` to stop
propagation. Bump ` + "`Version`" + ` to redraw after state changes.

` + "```go" + `
gui.DrawCanvas(gui.DrawCanvasCfg{
    ID:      "my-canvas",
    IDFocus: focusMyCanvas,
    Version: app.MyCanvasVersion,
    Width:   480, Height: 280,
    OnDraw: drawScene,
    OnKeyDown: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
        a := gui.State[App](w)
        switch e.KeyCode {
        case gui.KeyLeft:
            a.MarkerX -= 10
        case gui.KeyRight:
            a.MarkerX += 10
        default:
            return
        }
        a.MyCanvasVersion++
        e.IsHandled = true
    },
})
` + "```" + `

## Key Properties

| Property  | Type                           | Description                            |
|-----------|--------------------------------|----------------------------------------|
| ID        | string                         | Cache key (required)                   |
| Version   | uint64                         | Bump to invalidate cache               |
| Width     | float32                        | Canvas width                           |
| Height    | float32                        | Canvas height                          |
| Color     | Color                          | Background fill                        |
| Radius    | float32                        | Corner radius                          |
| Padding   | Opt[Padding]                   | Inner padding (shrinks draw area)      |
| Clip      | bool                           | Clip drawing to bounds                 |
| IDFocus   | uint32                         | Focus / tab order (0 = not focusable)  |
| OnDraw    | func(*DrawContext)             | Drawing callback                       |

## Events

| Callback      | Signature                          | Fired when                                 |
|---------------|------------------------------------|--------------------------------------------|
| OnClick       | func(*Layout, *Event, *Window)     | Canvas clicked                             |
| OnHover       | func(*Layout, *Event, *Window)     | Mouse enters canvas                        |
| OnMouseScroll | func(*Layout, *Event, *Window)     | Scroll wheel on canvas                     |
| OnKeyDown     | func(*Layout, *Event, *Window)     | Key pressed while canvas is focused        |

## Caching

Tessellation is cached per ` + "`ID`" + `. Bump ` + "`Version`" + ` when data
changes to trigger a re-draw. Same version = same triangles,
zero cost per frame. Images are cached alongside triangles, so
bump ` + "`Version`" + ` to pick up a new ` + "`src`" + ` within the same widget ID.

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |

Setting ` + "`IDFocus > 0`" + ` advertises the canvas as an interactive
element (button role) to assistive tech; non-focusable canvases
advertise as images. Provide a meaningful ` + "`A11YLabel`" + ` on
interactive canvases.
`,

	"gradient": `Linear and radial gradients with configurable direction
and color stops. Applied via the ` + "`Gradient`" + ` field on any container
or rectangle. ` + "`BorderGradient`" + ` applies a gradient to the border.

## Linear Gradient

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Gradient: &gui.GradientDef{
        Direction: gui.GradientToRight,
        Stops: []gui.GradientStop{
            {Pos: 0, Color: gui.ColorFromString("#3b82f6")},
            {Pos: 1, Color: gui.ColorFromString("#8b5cf6")},
        },
    },
})
` + "```" + `

## Radial Gradient

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Gradient: &gui.GradientDef{
        Type: gui.GradientRadial,
        Stops: []gui.GradientStop{
            {Pos: 0, Color: gui.White},
            {Pos: 1, Color: gui.ColorFromString("#3b82f6")},
        },
    },
})
` + "```" + `

## Border Gradient

` + "```go" + `
gui.Column(gui.ContainerCfg{
    SizeBorder: gui.SomeF(2),
    BorderGradient: &gui.GradientDef{
        Direction: gui.GradientToRight,
        Stops: []gui.GradientStop{
            {Pos: 0, Color: gui.ColorFromString("#3b82f6")},
            {Pos: 1, Color: gui.ColorFromString("#8b5cf6")},
        },
    },
})
` + "```" + `

## Custom Angle

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Gradient: &gui.GradientDef{
        HasAngle: true,
        Angle:    135,
        Stops: []gui.GradientStop{
            {Pos: 0, Color: gui.ColorFromString("#f97316")},
            {Pos: 1, Color: gui.ColorFromString("#ef4444")},
        },
    },
})
` + "```" + `

## GradientDef Properties

| Property  | Type              | Description                          |
|-----------|-------------------|--------------------------------------|
| Type      | GradientType      | GradientLinear (default) or GradientRadial |
| Direction | GradientDirection | Named direction constant              |
| Angle     | float32           | Explicit angle in degrees            |
| HasAngle  | bool              | True when Angle overrides Direction  |
| Stops     | []GradientStop    | Color stops (Pos 0.0-1.0)           |

## Gradient Types

| Type           | Description              |
|----------------|--------------------------|
| GradientLinear | Linear gradient (default) |
| GradientRadial | Radial gradient          |

## Directions

| Constant              | Angle |
|-----------------------|-------|
| GradientToTop         | 0°    |
| GradientToTopRight    | 45°   |
| GradientToRight       | 90°   |
| GradientToBottomRight | 135°  |
| GradientToBottom      | 180°  |
| GradientToBottomLeft  | 225°  |
| GradientToLeft        | 270°  |
| GradientToTopLeft     | 315°  |

## GradientStop

| Field | Type    | Description                |
|-------|---------|----------------------------|
| Color | Color   | Stop color                 |
| Pos   | float32 | Position along gradient (0.0-1.0) |
`,

	"blur": `Gaussian blur applied to a shape's fill. Set via the ` + "`BlurRadius`" + `
field on ` + "`ContainerCfg`" + `, ` + "`ButtonCfg`" + `, or ` + "`RectangleCfg`" + `.
Higher values produce softer, more diffused edges. The fill color must
have ` + "`A > 0`" + ` for the blur to be visible.

## Usage

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Width:      150,
    Height:     150,
    Sizing:     gui.FixedFixed,
    Radius:     gui.SomeF(75),
    Color:      gui.RGBA(0, 255, 0, 150),
    BlurRadius: 20,
})
` + "```" + `

## Soft Rectangle

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Width:      150,
    Height:     150,
    Sizing:     gui.FixedFixed,
    Radius:     gui.SomeF(20),
    Color:      gui.RGBA(255, 100, 100, 200),
    BlurRadius: 10,
})
` + "```" + `

## Heavy Glow

Large blur values create diffused glow effects.

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Width:      200,
    Height:     100,
    Sizing:     gui.FixedFixed,
    Radius:     gui.SomeF(10),
    Color:      gui.RGBA(60, 120, 255, 255),
    BlurRadius: 50,
})
` + "```" + `

## Widgets That Support BlurRadius

| Widget       | Field      | Type    |
|--------------|------------|---------|
| ContainerCfg | BlurRadius | float32 |
| ButtonCfg    | BlurRadius | float32 |
| RectangleCfg | BlurRadius | float32 |

## Notes

- ` + "`BlurRadius`" + ` is a plain ` + "`float32`" + `, not ` + "`Opt[float32]`" + `. Zero means no blur.
- Combine with ` + "`Radius`" + ` (corner rounding) to create orb or pill glows.
- Blur is rendered via a GPU shader pass; large radii have minimal
  extra cost on Metal/OpenGL backends.
- Distinct from ` + "`BoxShadow.BlurRadius`" + ` which blurs a drop shadow,
  not the shape fill itself.
`,

	"color_filter": `Post-processing color matrix transforms on container content.
Set ` + "`ColorFilter`" + ` on ` + "`ContainerCfg`" + ` to apply a 4×4 color
matrix to the container's rendered content. The container is captured
into an FBO, the matrix is applied via a GPU shader pass, and the
result is composited back. Works independently of or combined with
` + "`BlurRadius`" + `.

## Grayscale

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Width:       120,
    Sizing:      gui.FixedFit,
    Color:       gui.RGBA(200, 130, 100, 255),
    ColorFilter: gui.ColorFilterGrayscale(),
})
` + "```" + `

## Sepia

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Width:       120,
    Sizing:      gui.FixedFit,
    ColorFilter: gui.ColorFilterSepia(),
    Content:     []gui.View{ /* ... */ },
})
` + "```" + `

## Saturate

0 = grayscale, 1 = identity, >1 = oversaturated.

` + "```go" + `
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterSaturate(2.0),
    Content:     []gui.View{ /* ... */ },
})
` + "```" + `

## Brightness

1 = identity, <1 = dim, >1 = bright.

` + "```go" + `
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterBrightness(1.5),
    Content:     []gui.View{ /* ... */ },
})
` + "```" + `

## Contrast

1 = identity, 0 = all gray, >1 = higher contrast. Bias is injected
via the alpha column (correct for premultiplied-alpha FBO content).

` + "```go" + `
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterContrast(1.5),
    Content:     []gui.View{ /* ... */ },
})
` + "```" + `

## Hue Rotate

Rodrigues rotation around the (1,1,1) axis in RGB space.

` + "```go" + `
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterHueRotate(90),
    Content:     []gui.View{ /* ... */ },
})
` + "```" + `

## Invert

Negates RGB channels. Uses the alpha column to inject bias
(output.rgb = alpha − input.rgb).

` + "```go" + `
gui.Column(gui.ContainerCfg{
    ColorFilter: gui.ColorFilterInvert(),
    Content:     []gui.View{ /* ... */ },
})
` + "```" + `

## Combined with Blur

When both ` + "`BlurRadius`" + ` and ` + "`ColorFilter`" + ` are set, the FBO
pipeline runs gaussian blur first, then the color matrix pass,
then composites. The SDF blur (` + "`RenderBlur`" + `) is suppressed.

` + "```go" + `
gui.Column(gui.ContainerCfg{
    BlurRadius:  15,
    ColorFilter: gui.ColorFilterBrightness(1.3),
    Color:       gui.RGBA(0, 255, 128, 255),
    Radius:      gui.SomeF(50),
})
` + "```" + `

## Constructors

| Constructor                  | Description                              |
|------------------------------|------------------------------------------|
| ColorFilterIdentity()        | No-op (identity matrix)                  |
| ColorFilterGrayscale()       | Luminance-weighted grayscale             |
| ColorFilterSepia()           | Warm sepia tone                          |
| ColorFilterSaturate(amount)  | 0=gray, 1=identity, >1=oversaturated     |
| ColorFilterBrightness(amount)| Scale RGB channels                       |
| ColorFilterContrast(amount)  | Scale around 0.5 midpoint                |
| ColorFilterHueRotate(deg)    | Rotate hue in degrees                    |
| ColorFilterInvert()          | Negate RGB, keep alpha                   |

## Custom Matrix

For effects not covered by the built-in constructors, set
` + "`Matrix`" + ` directly. Column-major ` + "`[16]float32`" + `, applied as
` + "`clamp(M * pixel, 0, 1)`" + ` in the fragment shader.

` + "```go" + `
gui.Column(gui.ContainerCfg{
    ColorFilter: &gui.ColorFilter{
        Matrix: [16]float32{
            1, 0, 0, 0,  // column 0
            0, 1, 0, 0,  // column 1
            0, 0, 1, 0,  // column 2
            0, 0, 0, 1,  // column 3
        },
    },
})
` + "```" + `

## Notes

- Containers only — not available on individual widgets.
- No nesting. A single FBO pair is used; nested filter brackets
  are guarded against at the render level.
- Operates on premultiplied-alpha pixels from the FBO.
- Contrast and invert inject bias via the alpha column of the
  matrix, which is correct for premultiplied content.
- Rendered via Metal and OpenGL backends. SDL2 software backend
  ignores the color matrix.
`,

	"box_shadows": `Drop shadows on containers, buttons, rectangles, and
other elements. Set via the ` + "`Shadow`" + ` field (` + "`*BoxShadow`" + `) on any
` + "`ContainerCfg`" + `, ` + "`RectangleCfg`" + `, ` + "`ButtonCfg`" + `, or ` + "`SidebarCfg`" + `.

## Usage

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Shadow: &gui.BoxShadow{
        OffsetX:    4,
        OffsetY:    4,
        BlurRadius: 16,
        Color:      gui.RGBA(0, 0, 0, 80),
    },
})
` + "```" + `

## Elevated Card

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Radius: gui.SomeF(8),
    Shadow: &gui.BoxShadow{
        OffsetX:      0,
        OffsetY:      2,
        BlurRadius:   8,
        Color:        gui.RGBA(0, 0, 0, 60),
    },
    Content: []gui.View{ /* ... */ },
})
` + "```" + `

## BoxShadow Properties

| Property     | Type    | Description                              |
|--------------|---------|------------------------------------------|
| OffsetX      | float32 | Horizontal shadow offset                 |
| OffsetY      | float32 | Vertical shadow offset                   |
| BlurRadius   | float32 | Shadow blur amount                       |
| Color        | Color   | Shadow color (use RGBA for transparency) |

Positive OffsetX shifts right, positive OffsetY shifts down.
`,

	"rectangle": `Standalone visual shape — sharp, rounded, bordered, or pill.
Technically a container with no children, axis, or padding.
Supports gradients, shadows, shaders, and background blur.

## Usage

` + "```go" + `
gui.Rectangle(gui.RectangleCfg{
    Width:  100,
    Height: 60,
    Sizing: gui.FixedFixed,
    Color:  gui.ColorFromString("#3b82f6"),
    Radius: 8,
})
` + "```" + `

## Bordered

` + "```go" + `
gui.Rectangle(gui.RectangleCfg{
    Width:       80,
    Height:      80,
    Sizing:      gui.FixedFixed,
    ColorBorder: gui.White,
    SizeBorder:  2,
    Radius:      40,
})
` + "```" + `

## Key Properties

| Property       | Type         | Description                          |
|----------------|--------------|--------------------------------------|
| Color          | Color        | Fill color                           |
| ColorBorder    | Color        | Border color                         |
| SizeBorder     | float32      | Border thickness                     |
| Radius         | float32      | Corner radius                        |
| BlurRadius     | float32      | Background blur                      |
| Width          | float32      | Width                                |
| Height         | float32      | Height                               |
| MinWidth       | float32      | Minimum width                        |
| MinHeight      | float32      | Minimum height                       |
| MaxHeight      | float32      | Maximum height                       |
| Sizing         | Sizing       | Combined axis sizing mode            |
| Disabled       | bool         | Disable interaction                  |
| Invisible      | bool         | Hide without removing from layout    |

## Appearance

| Property       | Type         | Description                          |
|----------------|--------------|--------------------------------------|
| Gradient       | *GradientDef | Gradient fill                        |
| BorderGradient | *GradientDef | Gradient border                      |
| Shadow         | *BoxShadow   | Drop shadow                          |
| Shader         | *Shader      | Custom fragment shader               |
`,

	"icons": `256 icons from the Feather icon font. Each icon is a named
constant (e.g. ` + "`gui.IconCheck`" + `, ` + "`gui.IconFolder`" + `, ` + "`gui.IconSearch`" + `).
Render with ` + "`gui.Text`" + ` using one of the six theme Icon styles.
Icons are Unicode glyphs — no image files required.

## Usage

` + "```go" + `
// Single icon
gui.Text(gui.TextCfg{Text: gui.IconCheck, TextStyle: t.Icon4})

// Icon with custom color
gui.Text(gui.TextCfg{
    Text:      gui.IconAlertCircle,
    TextStyle: t.Icon3,
    Color:     gui.ColorRed,
})

// Icon inside a button
gui.Button(gui.ButtonCfg{
    ID: "save",
    Content: []gui.View{
        gui.Text(gui.TextCfg{Text: gui.IconSave, TextStyle: t.Icon4}),
        gui.Text(gui.TextCfg{Text: "Save"}),
    },
})
` + "```" + `

## Icon Styles

| Style   | Size   | Maps to        |
|---------|--------|----------------|
| t.Icon1 | XLarge | SizeTextXLarge |
| t.Icon2 | Large  | SizeTextLarge  |
| t.Icon3 | Medium | SizeTextMedium |
| t.Icon4 | Small  | SizeTextSmall  |
| t.Icon5 | XSmall | SizeTextXSmall |
| t.Icon6 | Tiny   | SizeTextTiny   |

## Programmatic Access

` + "`gui.IconLookup`" + ` is a ` + "`map[string]string`" + ` mapping snake_case names to
Unicode glyphs:

` + "```go" + `
for name, glyph := range gui.IconLookup {
    gui.Text(gui.TextCfg{Text: glyph, TextStyle: t.Icon4})
}
` + "```" + `

## Common Icons

| Constant        | Glyph | Usage                |
|-----------------|-------|----------------------|
| IconCheck       | check | Confirmation, success |
| IconX           | x     | Close, dismiss       |
| IconFolder      | folder | File browser         |
| IconSearch      | search | Search fields        |
| IconSave        | save  | Save actions         |
| IconTrash       | trash | Delete actions       |
| IconAlertCircle | alert | Warnings, errors     |
| IconEdit        | edit  | Edit actions         |
`,

	// Layout
	"row": `Horizontal container — children flow left to right. Uses
ContainerCfg for all sizing, alignment, scrolling, floating, borders,
and event handling.

## Usage

` + "```go" + `
gui.Row(gui.ContainerCfg{
    Spacing: gui.SomeF(8),
    Padding: gui.SomeP(4, 8, 4, 8),
    Sizing:  gui.FillFit,
    Content: []gui.View{child1, child2},
})
` + "```" + `

## Scrollable Row

` + "```go" + `
gui.Row(gui.ContainerCfg{
    IDScroll:   1,
    ScrollMode: gui.ScrollModeX,
    Sizing:     gui.FillFixed,
    Height:     200,
    Content:    items,
})
` + "```" + `

## Group Box (Titled Border)

` + "```go" + `
gui.Row(gui.ContainerCfg{
    Title:       "Options",
    TitleBG:     theme.ColorPanel,
    ColorBorder: theme.ColorBorder,
    Content:     items,
})
` + "```" + `

## Key Properties

| Property   | Type            | Description                          |
|------------|-----------------|--------------------------------------|
| Content    | []View          | Child views                          |
| Sizing     | Sizing          | Combined axis sizing mode            |
| Width      | float32         | Fixed width                          |
| Height     | float32         | Fixed height                         |
| MinWidth   | float32         | Minimum width                        |
| MaxWidth   | float32         | Maximum width                        |
| MinHeight  | float32         | Minimum height                       |
| MaxHeight  | float32         | Maximum height                       |
| Spacing    | Opt[float32]    | Gap between children                 |
| Padding    | Opt[Padding]    | Inner padding                        |
| HAlign     | HorizontalAlign | Horizontal content alignment         |
| VAlign     | VerticalAlign   | Vertical content alignment           |
| TextDir    | TextDirection   | Text/layout direction (LTR/RTL)      |
| IDFocus    | uint32          | Tab-order focus ID (> 0 to enable)   |
| IDScroll   | uint32          | Enable scrolling (> 0 to enable)     |
| ScrollMode | ScrollMode      | Scroll axis mode                     |
| Clip       | bool            | Clip children to bounds              |
| Disabled   | bool            | Disable interaction                  |
| Invisible  | bool            | Hide without removing from layout    |
| OverDraw   | bool            | Draw over siblings                   |
| Hero       | bool            | Hero animation participant           |

## Appearance

| Property       | Type           | Description                      |
|----------------|----------------|----------------------------------|
| Color          | Color          | Background color                 |
| ColorBorder    | Color          | Border color                     |
| SizeBorder     | Opt[float32]   | Border width                     |
| Radius         | Opt[float32]   | Corner radius                    |
| Opacity        | float32        | Opacity (0..1)                   |
| BlurRadius     | float32        | Background blur radius           |
| Shadow         | *BoxShadow     | Drop shadow                      |
| Gradient       | *GradientDef   | Background gradient              |
| BorderGradient | *GradientDef   | Border gradient                  |
| Shader         | *Shader        | Custom shader                    |
| Title          | string         | Group-box label in top border    |
| TitleBG        | Color          | Background behind title text     |

## Floating

| Property      | Type        | Description                        |
|---------------|-------------|------------------------------------|
| Float         | bool        | Float above siblings               |
| FloatAutoFlip | bool        | Auto-flip when clipped             |
| FloatAnchor   | FloatAttach | Anchor attachment point            |
| FloatTieOff   | FloatAttach | Tie-off attachment point           |
| FloatOffsetX  | float32     | Horizontal float offset            |
| FloatOffsetY  | float32     | Vertical float offset              |
| FloatZIndex   | int         | Z-order for floated elements       |

## Events

| Callback    | Signature                          | Fired when               |
|-------------|------------------------------------|--------------------------|
| OnClick     | func(*Layout, *Event, *Window)     | Left-click               |
| OnAnyClick  | func(*Layout, *Event, *Window)     | Any mouse button click   |
| OnChar      | func(*Layout, *Event, *Window)     | Character input          |
| OnKeyDown   | func(*Layout, *Event, *Window)     | Key pressed              |
| OnMouseMove | func(*Layout, *Event, *Window)     | Mouse movement           |
| OnMouseUp   | func(*Layout, *Event, *Window)     | Mouse button released    |
| OnHover     | func(*Layout, *Event, *Window)     | Mouse hover              |
| OnScroll    | func(*Layout, *Window)             | Scroll position changed  |
| OnIMECommit | func(*Layout, string, *Window)     | IME text committed       |
| AmendLayout | func(*Layout, *Window)             | Post-layout amendment    |

## Accessibility

| Property        | Type        | Description                      |
|-----------------|-------------|----------------------------------|
| A11YRole        | AccessRole  | Accessible role override         |
| A11YState       | AccessState | Accessible state override        |
| A11YLabel       | string      | Accessible label                 |
| A11YDescription | string      | Accessible description           |
`,

	"column": `Vertical container — children flow top to bottom. Uses
ContainerCfg for all sizing, alignment, scrolling, floating, borders,
and event handling. Prefer Column over Row when either direction works.

## Usage

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Spacing: gui.SomeF(8),
    Padding: gui.SomeP(4, 8, 4, 8),
    Sizing:  gui.FillFit,
    Content: []gui.View{child1, child2},
})
` + "```" + `

## Scrollable Column

` + "```go" + `
gui.Column(gui.ContainerCfg{
    IDScroll: 1,
    Sizing:   gui.FillFill,
    Content:  items,
})
` + "```" + `

## Group Box (Titled Border)

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Title:       "Settings",
    TitleBG:     theme.ColorPanel,
    ColorBorder: theme.ColorBorder,
    Content:     items,
})
` + "```" + `

## Key Properties

| Property   | Type            | Description                          |
|------------|-----------------|--------------------------------------|
| Content    | []View          | Child views                          |
| Sizing     | Sizing          | Combined axis sizing mode            |
| Width      | float32         | Fixed width                          |
| Height     | float32         | Fixed height                         |
| MinWidth   | float32         | Minimum width                        |
| MaxWidth   | float32         | Maximum width                        |
| MinHeight  | float32         | Minimum height                       |
| MaxHeight  | float32         | Maximum height                       |
| Spacing    | Opt[float32]    | Gap between children                 |
| Padding    | Opt[Padding]    | Inner padding                        |
| HAlign     | HorizontalAlign | Horizontal content alignment         |
| VAlign     | VerticalAlign   | Vertical content alignment           |
| TextDir    | TextDirection   | Text/layout direction (LTR/RTL)      |
| IDFocus    | uint32          | Tab-order focus ID (> 0 to enable)   |
| IDScroll   | uint32          | Enable scrolling (> 0 to enable)     |
| ScrollMode | ScrollMode      | Scroll axis mode                     |
| Clip       | bool            | Clip children to bounds              |
| Disabled   | bool            | Disable interaction                  |
| Invisible  | bool            | Hide without removing from layout    |
| OverDraw   | bool            | Draw over siblings                   |
| Hero       | bool            | Hero animation participant           |

## Appearance

| Property       | Type           | Description                      |
|----------------|----------------|----------------------------------|
| Color          | Color          | Background color                 |
| ColorBorder    | Color          | Border color                     |
| SizeBorder     | Opt[float32]   | Border width                     |
| Radius         | Opt[float32]   | Corner radius                    |
| Opacity        | float32        | Opacity (0..1)                   |
| BlurRadius     | float32        | Background blur radius           |
| Shadow         | *BoxShadow     | Drop shadow                      |
| Gradient       | *GradientDef   | Background gradient              |
| BorderGradient | *GradientDef   | Border gradient                  |
| Shader         | *Shader        | Custom shader                    |
| Title          | string         | Group-box label in top border    |
| TitleBG        | Color          | Background behind title text     |

## Floating

| Property      | Type        | Description                        |
|---------------|-------------|------------------------------------|
| Float         | bool        | Float above siblings               |
| FloatAutoFlip | bool        | Auto-flip when clipped             |
| FloatAnchor   | FloatAttach | Anchor attachment point            |
| FloatTieOff   | FloatAttach | Tie-off attachment point           |
| FloatOffsetX  | float32     | Horizontal float offset            |
| FloatOffsetY  | float32     | Vertical float offset              |
| FloatZIndex   | int         | Z-order for floated elements       |

## Events

| Callback    | Signature                          | Fired when               |
|-------------|------------------------------------|--------------------------|
| OnClick     | func(*Layout, *Event, *Window)     | Left-click               |
| OnAnyClick  | func(*Layout, *Event, *Window)     | Any mouse button click   |
| OnChar      | func(*Layout, *Event, *Window)     | Character input          |
| OnKeyDown   | func(*Layout, *Event, *Window)     | Key pressed              |
| OnMouseMove | func(*Layout, *Event, *Window)     | Mouse movement           |
| OnMouseUp   | func(*Layout, *Event, *Window)     | Mouse button released    |
| OnHover     | func(*Layout, *Event, *Window)     | Mouse hover              |
| OnScroll    | func(*Layout, *Window)             | Scroll position changed  |
| OnIMECommit | func(*Layout, string, *Window)     | IME text committed       |
| AmendLayout | func(*Layout, *Window)             | Post-layout amendment    |

## Accessibility

| Property        | Type        | Description                      |
|-----------------|-------------|----------------------------------|
| A11YRole        | AccessRole  | Accessible role override         |
| A11YState       | AccessState | Accessible state override        |
| A11YLabel       | string      | Accessible label                 |
| A11YDescription | string      | Accessible description           |
`,

	"wrap_panel": `Horizontal flow that wraps to the next line when full.
Items fill left to right, then break to the next row when the
container width is exceeded. Uses ContainerCfg with Wrap set
automatically by the factory function.

## Usage

` + "```go" + `
gui.Wrap(gui.ContainerCfg{
    Spacing: gui.SomeF(4),
    Sizing:  gui.FillFit,
    Content: items,
})
` + "```" + `

## Wrap with Alignment

` + "```go" + `
gui.Wrap(gui.ContainerCfg{
    Spacing: gui.SomeF(8),
    HAlign:  gui.HAlignCenter,
    VAlign:  gui.VAlignMiddle,
    Content: tags,
})
` + "```" + `

## Key Properties

| Property   | Type            | Description                          |
|------------|-----------------|--------------------------------------|
| Content    | []View          | Child views to wrap                  |
| Sizing     | Sizing          | Combined axis sizing mode            |
| Width      | float32         | Fixed width                          |
| Height     | float32         | Fixed height                         |
| MinWidth   | float32         | Minimum width                        |
| MaxWidth   | float32         | Maximum width                        |
| MinHeight  | float32         | Minimum height                       |
| MaxHeight  | float32         | Maximum height                       |
| Spacing    | Opt[float32]    | Gap between items (horizontal & row) |
| Padding    | Opt[Padding]    | Inner padding                        |
| HAlign     | HorizontalAlign | Horizontal alignment per row         |
| VAlign     | VerticalAlign   | Cross-axis alignment per row         |
| TextDir    | TextDirection   | Text/layout direction (LTR/RTL)      |
| Clip       | bool            | Clip children to bounds              |
| Disabled   | bool            | Disable interaction                  |
| Invisible  | bool            | Hide without removing from layout    |

## Appearance

| Property       | Type           | Description                      |
|----------------|----------------|----------------------------------|
| Color          | Color          | Background color                 |
| ColorBorder    | Color          | Border color                     |
| SizeBorder     | Opt[float32]   | Border width                     |
| Radius         | Opt[float32]   | Corner radius                    |
| Opacity        | float32        | Opacity (0..1)                   |
| Shadow         | *BoxShadow     | Drop shadow                      |
| Gradient       | *GradientDef   | Background gradient              |

## Events

| Callback    | Signature                          | Fired when               |
|-------------|------------------------------------|--------------------------|
| OnClick     | func(*Layout, *Event, *Window)     | Left-click               |
| OnHover     | func(*Layout, *Event, *Window)     | Mouse hover              |
| OnKeyDown   | func(*Layout, *Event, *Window)     | Key pressed              |
| OnMouseMove | func(*Layout, *Event, *Window)     | Mouse movement           |
| AmendLayout | func(*Layout, *Window)             | Post-layout amendment    |

## Accessibility

| Property        | Type        | Description                      |
|-----------------|-------------|----------------------------------|
| A11YRole        | AccessRole  | Accessible role override         |
| A11YLabel       | string      | Accessible label                 |
| A11YDescription | string      | Accessible description           |
`,

	"overflow_panel": `Toolbar that hides items that don't fit and shows them in a
dropdown menu. Useful for responsive toolbars and action bars. Items
that overflow are shown via a trigger button that opens a dropdown
Menu.

## Usage

` + "```go" + `
gui.OverflowPanel(w, gui.OverflowPanelCfg{
    ID:      "toolbar",
    IDFocus: 100,
    Items: []gui.OverflowItem{
        {ID: "cut",   View: cutBtn,   Text: "Cut"},
        {ID: "copy",  View: copyBtn,  Text: "Copy"},
        {ID: "paste", View: pasteBtn, Text: "Paste"},
    },
})
` + "```" + `

## Custom Trigger Button

` + "```go" + `
gui.OverflowPanel(w, gui.OverflowPanelCfg{
    ID:    "tb",
    Items: items,
    Trigger: []gui.View{
        gui.Text(gui.TextCfg{Text: "More..."}),
    },
})
` + "```" + `

## OverflowItem

| Property | Type                                  | Description                |
|----------|---------------------------------------|----------------------------|
| ID       | string                                | Item identifier            |
| View     | View                                  | Visible toolbar view       |
| Text     | string                                | Label in overflow dropdown |
| Action   | func(*MenuItemCfg, *Event, *Window)   | Overflow item action       |

## Key Properties

| Property     | Type         | Description                          |
|--------------|--------------|--------------------------------------|
| ID           | string       | Unique identifier                    |
| Items        | []OverflowItem | Ordered toolbar items              |
| Trigger      | []View       | Custom overflow button content       |
| Padding      | Opt[Padding] | Inner padding                        |
| Spacing      | float32      | Gap between items                    |
| IDFocus      | uint32       | Tab-order focus ID (> 0 to enable)   |
| Disabled     | bool         | Disable interaction                  |
| FloatAnchor  | FloatAttach  | Dropdown anchor point                |
| FloatTieOff  | FloatAttach  | Dropdown tie-off point               |
| FloatOffsetX | float32      | Dropdown horizontal offset           |
| FloatOffsetY | float32      | Dropdown vertical offset             |
| FloatZIndex  | int          | Dropdown z-order                     |
`,

	"expand_panel": `Collapsible section with a clickable header and expandable
body. Header shows an arrow indicator that reflects the open state.

## Usage

` + "```go" + `
gui.ExpandPanel(gui.ExpandPanelCfg{
    ID:   "ep",
    Open: app.Open,
    Head: gui.Text(gui.TextCfg{Text: "Details"}),
    Content: gui.Column(gui.ContainerCfg{
        Content: []gui.View{child1, child2},
    }),
    OnToggle: func(w *gui.Window) {
        gui.State[App](w).Open = !gui.State[App](w).Open
    },
})
` + "```" + `

## Key Properties

| Property  | Type         | Description                          |
|-----------|--------------|--------------------------------------|
| ID        | string       | Unique identifier                    |
| Head      | View         | Header view (always visible)         |
| Content   | View         | Collapsible body content             |
| Open      | bool         | Expanded state                       |
| Sizing    | Sizing       | Combined axis sizing mode            |
| MinWidth  | float32      | Minimum width                        |
| MaxWidth  | float32      | Maximum width                        |
| MinHeight | float32      | Minimum height                       |
| MaxHeight | float32      | Maximum height                       |

## Appearance

| Property    | Type         | Description                          |
|-------------|--------------|--------------------------------------|
| Color       | Color        | Background color                     |
| ColorHover  | Color        | Header background on hover           |
| ColorClick  | Color        | Header background on click           |
| ColorBorder | Color        | Border color                         |
| Padding     | Opt[Padding] | Inner padding                        |
| SizeBorder  | Opt[float32] | Border width                         |
| Radius      | Opt[float32] | Corner radius                        |

## Events

| Callback | Signature        | Fired when                           |
|----------|------------------|--------------------------------------|
| OnToggle | func(*Window)    | Header clicked or Space pressed      |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"sidebar": `Animated slide-out panel. Width animates between 0 and the
configured width using either a tween or spring animation. Called as a
method on Window.

## Usage

` + "```go" + `
w.Sidebar(gui.SidebarCfg{
    ID:      "sb",
    Open:    app.SidebarOpen,
    Width:   250,
    Content: []gui.View{navItems},
})
` + "```" + `

## Spring Animation

` + "```go" + `
w.Sidebar(gui.SidebarCfg{
    ID:            "sb",
    Open:          app.Open,
    TweenDuration: 0,
    Spring:        gui.SpringStiff,
    Content:       content,
})
` + "```" + `

## Key Properties

| Property      | Type          | Description                          |
|---------------|---------------|--------------------------------------|
| ID            | string        | Unique identifier                    |
| Open          | bool          | Sidebar visibility                   |
| Width         | float32       | Panel width (default 250)            |
| Content       | []View        | Sidebar content                      |
| Sizing        | Sizing        | Combined axis sizing (default FixedFill) |
| Clip          | bool          | Clip content to bounds               |
| Disabled      | bool          | Disable interaction                  |
| Invisible     | bool          | Hide without removing from layout    |

## Appearance

| Property | Type         | Description                          |
|----------|--------------|--------------------------------------|
| Color    | Color        | Background color                     |
| Shadow   | *BoxShadow   | Drop shadow                          |
| Radius   | float32      | Corner radius                        |
| Padding  | Opt[Padding] | Inner padding                        |

## Animation

| Property      | Type          | Description                          |
|---------------|---------------|--------------------------------------|
| Spring        | SpringCfg     | Spring animation config              |
| TweenDuration | time.Duration | Tween duration (default 300ms)       |
| TweenEasing   | EasingFn      | Easing function (default InOutCubic) |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"splitter": `Resizable two-pane split with a draggable divider handle.
Supports horizontal and vertical orientation, keyboard navigation,
pane collapse, and optional collapse buttons.

## Usage

` + "```go" + `
gui.Splitter(gui.SplitterCfg{
    ID:          "split",
    IDFocus:     100,
    Ratio:       gui.SomeF(0.3),
    Orientation: gui.SplitterHorizontal,
    First:  gui.SplitterPaneCfg{Content: []gui.View{left}},
    Second: gui.SplitterPaneCfg{Content: []gui.View{right}},
    OnChange: func(ratio float32, c gui.SplitterCollapsed,
        e *gui.Event, w *gui.Window) {
        s := gui.State[App](w)
        s.Ratio = ratio
        s.Collapsed = c
    },
})
` + "```" + `

## Collapsible Panes

` + "```go" + `
gui.Splitter(gui.SplitterCfg{
    ID:                  "split",
    ShowCollapseButtons: true,
    First: gui.SplitterPaneCfg{
        Collapsible:   true,
        CollapsedSize: 0,
        MinSize:       100,
        Content:       []gui.View{sidebar},
    },
    Second: gui.SplitterPaneCfg{Content: []gui.View{main}},
    OnChange: onChange,
})
` + "```" + `

## Key Properties

| Property            | Type                | Description                          |
|---------------------|---------------------|--------------------------------------|
| ID                  | string              | Unique identifier                    |
| IDFocus             | uint32              | Tab-order focus ID (> 0 to enable)   |
| Orientation         | SplitterOrientation | Horizontal or vertical split         |
| Sizing              | Sizing              | Combined axis sizing (default FillFill) |
| Ratio               | Opt[float32]        | Split position (0.0-1.0)            |
| Collapsed           | SplitterCollapsed   | Which pane is collapsed              |
| HandleSize          | Opt[float32]        | Drag handle thickness                |
| DragStep            | Opt[float32]        | Keyboard step size                   |
| DragStepLarge       | Opt[float32]        | Shift+arrow keyboard step            |
| ShowCollapseButtons | bool                | Show collapse/expand buttons         |
| Disabled            | bool                | Disable interaction                  |
| Invisible           | bool                | Hide without removing from layout    |

## SplitterPaneCfg (First / Second)

| Property      | Type    | Description                          |
|---------------|---------|--------------------------------------|
| MinSize       | float32 | Minimum pane size                    |
| MaxSize       | float32 | Maximum pane size                    |
| Collapsible   | bool    | Allow pane collapse                  |
| CollapsedSize | float32 | Size when collapsed                  |
| Content       | []View  | Pane content                         |

## Appearance

| Property          | Type    | Description                          |
|-------------------|---------|--------------------------------------|
| ColorHandle       | Color   | Handle background                    |
| ColorHandleHover  | Color   | Handle background on hover           |
| ColorHandleActive | Color   | Handle background while dragging     |
| ColorHandleBorder | Color   | Handle border color                  |
| ColorGrip         | Color   | Grip indicator color                 |
| ColorButton       | Color   | Collapse button background           |
| ColorButtonHover  | Color   | Collapse button hover                |
| ColorButtonActive | Color   | Collapse button active               |
| ColorButtonIcon   | Color   | Collapse button icon color           |
| SizeBorder        | Opt[float32] | Handle border width             |
| Radius            | Opt[float32] | Handle corner radius            |
| RadiusBorder      | Opt[float32] | Button/grip corner radius       |

## Events

| Callback | Signature                                              | Fired when                   |
|----------|--------------------------------------------------------|------------------------------|
| OnChange | func(float32, SplitterCollapsed, *Event, *Window)      | Ratio or collapse changes    |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |

## Serialization

SplitterState has JSON struct tags and text-marshaled enums,
so standard ` + "`encoding/json`" + ` works directly.

` + "```go" + `
// Save
state := gui.SplitterStateNormalize(gui.SplitterState{
    Ratio:     ratio,
    Collapsed: collapsed,
})
data, _ := json.Marshal(state)
// → {"ratio":0.3,"collapsed":"first"}

// Restore
var restored gui.SplitterState
json.Unmarshal(data, &restored)
` + "```" + `

Call SplitterStateNormalize before saving to clamp ratio to [0,1].
`,

	"dock_layout": `IDE-style docking layout with splits, tabs, and drag-and-drop
panel rearrangement. The layout is a user-owned binary tree of
DockNode values: internal nodes are splits, leaves are panel groups.

## Usage

` + "```go" + `
gui.DockLayout(gui.DockLayoutCfg{
    ID:     "dock",
    Root:   app.Root,
    Panels: panels,
    OnLayoutChange: func(root *gui.DockNode, w *gui.Window) {
        gui.State[App](w).Root = root
    },
    OnPanelSelect: func(groupID, panelID string, w *gui.Window) {
        a := gui.State[App](w)
        a.Root = gui.DockTreeSelectPanel(a.Root, groupID, panelID)
    },
    OnPanelClose: func(panelID string, w *gui.Window) {
        a := gui.State[App](w)
        a.Root = gui.DockTreeRemovePanel(a.Root, panelID)
    },
})
` + "```" + `

## Building the Tree

` + "```go" + `
root := gui.DockSplit("root", gui.DockSplitHorizontal, 0.2,
    gui.DockPanelGroup("left", []string{"explorer"}, "explorer"),
    gui.DockSplit("right", gui.DockSplitVertical, 0.75,
        gui.DockPanelGroup("top", []string{"editor", "preview"}, "editor"),
        gui.DockPanelGroup("bottom", []string{"console"}, "console"),
    ),
)
` + "```" + `

## DockLayoutCfg

| Property          | Type                          | Description                            |
|-------------------|-------------------------------|----------------------------------------|
| ID                | string                        | Unique identifier                      |
| Root              | *DockNode                     | Layout tree root                       |
| Panels            | []DockPanelDef                | Panel definitions                      |
| OnLayoutChange    | func(*DockNode, *Window)      | Fired on layout change                 |
| OnPanelSelect     | func(string, string, *Window) | Fired on tab select (groupID, panelID) |
| OnPanelClose      | func(string, *Window)         | Fired on panel close (panelID)         |
| Sizing            | Sizing                        | Combined axis sizing                   |
| ColorZonePreview  | Color                         | Drop zone highlight color              |
| ColorTab          | Color                         | Tab background                         |
| ColorTabActive    | Color                         | Active tab background                  |
| ColorTabHover     | Color                         | Tab hover background                   |
| ColorTabBar       | Color                         | Tab bar background                     |
| ColorTabSeparator | Color                         | Tab separator color                    |
| ColorContent      | Color                         | Content area background                |

## DockPanelDef

| Property | Type   | Description                 |
|----------|--------|-----------------------------|
| ID       | string | Unique panel identifier     |
| Label    | string | Tab label text              |
| Content  | []View | Panel content views         |
| Closable | bool   | Allow user to close panel   |

## Tree Operations

| Function                  | Description                         |
|---------------------------|-------------------------------------|
| DockTreeRemovePanel       | Remove panel; collapse empty splits |
| DockTreeAddTab            | Add panel as tab in existing group  |
| DockTreeSplitAt           | Split a group, insert panel at edge |
| DockTreeMovePanel         | Remove + insert (drag-and-drop)     |
| DockTreeSelectPanel       | Set active tab in a group           |
| DockTreeWrapRoot          | Dock panel at window edge           |
| DockTreeCollectPanelNodes | Collect all panel group nodes       |
| DockTreeFindGroupByPanel  | Find group containing a panel       |
| DockTreeFindGroupByID     | Find group by its ID                |

## Serialization

DockNode has JSON struct tags and text-marshaled enums, so standard
encoding/json works directly.

` + "```go" + `
// Save
data, _ := json.Marshal(app.Root)
os.WriteFile("layout.json", data, 0644)

// Restore
root := new(gui.DockNode)
data, _ = os.ReadFile("layout.json")
json.Unmarshal(data, root)
` + "```" + `

Example output:

` + "```json" + `
{
  "kind": "split",
  "id": "root",
  "dir": "horizontal",
  "ratio": 0.2,
  "first": {
    "kind": "panelGroup",
    "id": "left",
    "panelIDs": ["explorer"],
    "selectedID": "explorer"
  },
  "second": { ... }
}
` + "```" + `

## Events

| Callback       | Signature                         | Fired when              |
|----------------|-----------------------------------|-------------------------|
| OnLayoutChange | func(*DockNode, *Window)          | Drag-drop rearrangement |
| OnPanelSelect  | func(string, string, *Window)     | Tab selected            |
| OnPanelClose   | func(string, *Window)             | Panel closed            |
`,

	"scrollbar": `Custom scrollbar styling for scrollable containers. Applied
via ScrollbarCfgX/ScrollbarCfgY on ContainerCfg, or used directly.
Supports drag, gutter click, and auto-hide behavior.

## Usage (Container Override)

` + "```go" + `
gui.Column(gui.ContainerCfg{
    IDScroll:      myScrollID,
    ScrollbarCfgY: &gui.ScrollbarCfg{
        GapEdge:  4,
        Overflow: gui.ScrollbarOnHover,
    },
    Content: views,
})
` + "```" + `

## Hide Scrollbar

` + "```go" + `
gui.Column(gui.ContainerCfg{
    IDScroll:      myScrollID,
    ScrollbarCfgX: &gui.ScrollbarCfg{
        Overflow: gui.ScrollbarHidden,
    },
    Content: views,
})
` + "```" + `

## Key Properties

| Property        | Type               | Description                      |
|-----------------|--------------------|----------------------------------|
| ID              | string             | Unique identifier                |
| IDScroll        | uint32             | Scroll container to attach to    |
| Orientation     | ScrollbarOrientation | Horizontal or vertical         |
| Size            | float32            | Scrollbar thickness              |
| MinThumbSize    | float32            | Minimum thumb length             |
| Radius          | float32            | Track corner radius              |
| RadiusThumb     | float32            | Thumb corner radius              |
| GapEdge         | float32            | Gap from container edge          |
| GapEnd          | float32            | Gap from track ends              |
| Overflow        | ScrollbarOverflow  | Visibility mode                  |

## Appearance

| Property        | Type  | Description                          |
|-----------------|-------|--------------------------------------|
| ColorThumb      | Color | Thumb color                          |
| ColorBackground | Color | Track background color               |

## Overflow Modes

| Constant         | Behavior                               |
|------------------|----------------------------------------|
| ScrollbarAuto    | Show when content overflows (default)  |
| ScrollbarHidden  | Never show                             |
| ScrollbarVisible | Always show                            |
| ScrollbarOnHover | Show on mouse hover only               |
`,

	// Navigation
	"breadcrumb": `Trail navigation with clickable path segments and optional
content panels. Supports keyboard navigation (Left/Right/Home/End),
custom separators, and per-crumb content.

## Usage

` + "```go" + `
gui.Breadcrumb(gui.BreadcrumbCfg{
    ID:      "nav",
    IDFocus: 100,
    Items: []gui.BreadcrumbItemCfg{
        {ID: "home",     Label: "Home"},
        {ID: "settings", Label: "Settings"},
        {ID: "display",  Label: "Display"},
    },
    Selected: app.Page,
    OnSelect: func(id string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Page = id
    },
})
` + "```" + `

## With Content Panels

` + "```go" + `
gui.Breadcrumb(gui.BreadcrumbCfg{
    Items: []gui.BreadcrumbItemCfg{
        {ID: "home", Label: "Home", Content: []gui.View{homeView}},
        {ID: "docs", Label: "Docs", Content: []gui.View{docsView}},
    },
    Selected: app.Page,
    OnSelect: onSelect,
})
` + "```" + `

## BreadcrumbItemCfg

| Property | Type   | Description                          |
|----------|--------|--------------------------------------|
| ID       | string | Segment identifier                   |
| Label    | string | Display text                         |
| Content  | []View | Panel content for this segment       |
| Disabled | bool   | Disable this segment                 |

## Key Properties

| Property          | Type               | Description                      |
|-------------------|--------------------|----------------------------------|
| ID                | string             | Unique identifier                |
| Items             | []BreadcrumbItemCfg | Path segments                   |
| Selected          | string             | Active segment ID                |
| Separator         | string             | Separator character              |
| IDFocus           | uint32             | Tab-order focus ID               |
| Sizing            | Sizing             | Combined axis sizing             |
| Disabled          | bool               | Disable interaction              |
| Invisible         | bool               | Hide without removing            |

## Appearance

| Property           | Type         | Description                      |
|--------------------|--------------|----------------------------------|
| Color              | Color        | Outer background                 |
| ColorBorder        | Color        | Outer border color               |
| ColorTrail         | Color        | Trail row background             |
| ColorCrumb         | Color        | Crumb background                 |
| ColorCrumbHover    | Color        | Crumb hover background           |
| ColorCrumbClick    | Color        | Crumb click background           |
| ColorCrumbSelected | Color        | Selected crumb background        |
| ColorCrumbDisabled | Color        | Disabled crumb background        |
| ColorContent       | Color        | Content panel background         |
| ColorContentBorder | Color        | Content panel border             |
| Padding            | Opt[Padding] | Outer padding                    |
| PaddingTrail       | Opt[Padding] | Trail row padding                |
| PaddingCrumb       | Opt[Padding] | Individual crumb padding         |
| PaddingContent     | Opt[Padding] | Content panel padding            |
| Radius             | Opt[float32] | Outer corner radius              |
| RadiusCrumb        | Opt[float32] | Crumb corner radius              |
| RadiusContent      | Opt[float32] | Content panel corner radius      |
| Spacing            | Opt[float32] | Outer spacing                    |
| SpacingTrail       | Opt[float32] | Trail item spacing               |
| SizeBorder         | Opt[float32] | Outer border width               |
| SizeContentBorder  | Opt[float32] | Content panel border width       |
| TextStyle          | TextStyle    | Default crumb text style         |
| TextStyleSelected  | TextStyle    | Selected crumb text style        |
| TextStyleDisabled  | TextStyle    | Disabled crumb text style        |
| TextStyleSeparator | TextStyle    | Separator text style             |

## Events

| Callback | Signature                          | Fired when               |
|----------|------------------------------------|--------------------------|
| OnSelect | func(string, *Event, *Window)      | Segment clicked          |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"tab_control": `Tabbed content panels with keyboard navigation
(Left/Right/Home/End). Controlled component: Selected is owned by app
state and updated through OnSelect. Supports drag-reorder.

## Usage

` + "```go" + `
gui.TabControl(gui.TabControlCfg{
    ID:       "tabs",
    IDFocus:  100,
    Selected: app.Tab,
    Items: []gui.TabItemCfg{
        {ID: "t1", Label: "General",  Content: []gui.View{general}},
        {ID: "t2", Label: "Advanced", Content: []gui.View{advanced}},
    },
    OnSelect: func(id string, _ *gui.Event, w *gui.Window) {
        gui.State[App](w).Tab = id
    },
})
` + "```" + `

## Reorderable Tabs

` + "```go" + `
gui.TabControl(gui.TabControlCfg{
    ID:          "tabs",
    Reorderable: true,
    Items:       items,
    Selected:    app.Tab,
    OnSelect:    onSelect,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        // reorder items in app state
    },
})
` + "```" + `

## TabItemCfg

| Property | Type   | Description                          |
|----------|--------|--------------------------------------|
| ID       | string | Tab identifier                       |
| Label    | string | Tab header text                      |
| Content  | []View | Tab panel content                    |
| Disabled | bool   | Disable this tab                     |

## Key Properties

| Property    | Type          | Description                          |
|-------------|---------------|--------------------------------------|
| ID          | string        | Unique identifier                    |
| Items       | []TabItemCfg  | Tab definitions                      |
| Selected    | string        | Active tab ID                        |
| IDFocus     | uint32        | Tab-order focus ID (> 0 to enable)   |
| Sizing      | Sizing        | Combined axis sizing (default FillFill) |
| Reorderable | bool          | Enable drag-reorder of tabs          |
| Disabled    | bool          | Disable interaction                  |
| Invisible   | bool          | Hide without removing from layout    |
| Spacing       | Opt[float32] | Gap between header and content       |
| SpacingHeader | Opt[float32] | Gap between tab buttons              |

## Appearance

| Property           | Type         | Description                      |
|--------------------|--------------|----------------------------------|
| Color              | Color        | Outer background                 |
| ColorBorder        | Color        | Outer border color               |
| ColorHeader        | Color        | Header row background            |
| ColorHeaderBorder  | Color        | Header row border                |
| ColorContent       | Color        | Content panel background         |
| ColorContentBorder | Color        | Content panel border             |
| ColorTab           | Color        | Tab button background            |
| ColorTabHover      | Color        | Tab button hover                 |
| ColorTabFocus      | Color        | Tab button focus                 |
| ColorTabClick      | Color        | Tab button click                 |
| ColorTabSelected   | Color        | Selected tab background          |
| ColorTabDisabled   | Color        | Disabled tab background          |
| ColorTabBorder     | Color        | Tab button border                |
| ColorTabBorderFocus | Color       | Tab border when focused          |
| Padding            | Opt[Padding] | Outer padding                    |
| PaddingHeader      | Opt[Padding] | Header row padding               |
| PaddingContent     | Opt[Padding] | Content panel padding            |
| PaddingTab         | Opt[Padding] | Individual tab padding           |
| SizeBorder         | Opt[float32] | Outer border width               |
| SizeHeaderBorder   | Opt[float32] | Header border width              |
| SizeContentBorder  | Opt[float32] | Content border width             |
| SizeTabBorder      | Opt[float32] | Tab button border width          |
| Radius             | Opt[float32] | Outer corner radius              |
| RadiusHeader       | Opt[float32] | Header corner radius             |
| RadiusContent      | Opt[float32] | Content corner radius            |
| RadiusTab          | Opt[float32] | Tab button corner radius         |
| TextStyle          | TextStyle    | Default tab text style           |
| TextStyleSelected  | TextStyle    | Selected tab text style          |
| TextStyleDisabled  | TextStyle    | Disabled tab text style          |

## Events

| Callback  | Signature                                    | Fired when             |
|-----------|----------------------------------------------|------------------------|
| OnSelect  | func(string, *Event, *Window)                | Tab selection changes  |
| OnReorder | func(movedID, beforeID string, *Window)      | Tab reordered          |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	"menus": `Horizontal menubar and standalone vertical menu with keyboard
navigation (arrow keys, Enter/Space to select, Escape to close),
nested submenus, separators, and subtitles. Two factory functions:
Menubar (horizontal bar) and Menu (vertical standalone).

## Menubar Usage

` + "```go" + `
gui.Menubar(w, gui.MenubarCfg{
    ID:      "mb",
    IDFocus: 100,
    Items: []gui.MenuItemCfg{
        gui.MenuSubmenu("file", "File", []gui.MenuItemCfg{
            gui.MenuItemText("new", "New"),
            gui.MenuItemText("open", "Open"),
            gui.MenuSeparator(),
            gui.MenuItemText("quit", "Quit"),
        }),
    },
    Action: func(id string, _ *gui.Event, w *gui.Window) {
        // handle menu action
    },
})
` + "```" + `

## Standalone Menu

` + "```go" + `
gui.Menu(w, gui.MenubarCfg{
    ID:    "ctx",
    Float: true,
    Items: []gui.MenuItemCfg{
        gui.MenuItemText("cut", "Cut"),
        gui.MenuItemText("copy", "Copy"),
        gui.MenuSubtitle("Advanced"),
        gui.MenuItemText("paste", "Paste Special"),
    },
})
` + "```" + `

## MenuItemCfg

| Property   | Type                                | Description                |
|------------|-------------------------------------|----------------------------|
| ID         | string                              | Action identifier          |
| Text       | string                              | Display label              |
| Submenu    | []MenuItemCfg                       | Nested submenu items       |
| CustomView | View                                | Custom rendered content    |
| Separator  | bool                                | Render as separator line   |
| Padding    | Opt[Padding]                        | Item padding override      |
| Action     | func(*MenuItemCfg, *Event, *Window) | Item-level action callback |

Helper constructors: MenuItemText, MenuSeparator, MenuSubtitle,
MenuSubmenu.

## MenubarCfg Key Properties

| Property         | Type          | Description                      |
|------------------|---------------|----------------------------------|
| ID               | string        | Unique identifier                |
| IDFocus          | uint32        | Tab-order focus ID               |
| Items            | []MenuItemCfg | Top-level menu items             |
| Sizing           | Sizing        | Combined axis sizing             |
| Disabled         | bool          | Disable interaction              |
| Invisible        | bool          | Hide without removing            |

## MenubarCfg Appearance

| Property          | Type         | Description                      |
|-------------------|--------------|----------------------------------|
| Color             | Color        | Background color                 |
| ColorBorder       | Color        | Border color                     |
| ColorSelect       | Color        | Selected item highlight          |
| TextStyle         | TextStyle    | Item text style                  |
| TextStyleSubtitle | TextStyle    | Subtitle text style              |
| Padding           | Opt[Padding] | Outer padding                    |
| PaddingMenuItem   | Opt[Padding] | Menu item padding                |
| PaddingSubmenu    | Opt[Padding] | Submenu panel padding            |
| PaddingSubtitle   | Opt[Padding] | Subtitle item padding            |
| SizeBorder        | Opt[float32] | Border width                     |
| Radius            | Opt[float32] | Outer corner radius              |
| RadiusBorder      | Opt[float32] | Border corner radius             |
| RadiusSubmenu     | Opt[float32] | Submenu panel corner radius      |
| RadiusMenuItem    | Opt[float32] | Menu item corner radius          |
| Spacing           | Opt[float32] | Top-level item spacing           |
| SpacingSubmenu    | Opt[float32] | Submenu item spacing             |
| WidthSubmenuMin   | Opt[float32] | Minimum submenu width            |
| WidthSubmenuMax   | Opt[float32] | Maximum submenu width            |

## MenubarCfg Floating

| Property      | Type        | Description                        |
|---------------|-------------|------------------------------------|
| Float         | bool        | Float above siblings               |
| FloatAutoFlip | bool        | Auto-flip when clipped             |
| FloatAnchor   | FloatAttach | Anchor attachment point            |
| FloatTieOff   | FloatAttach | Tie-off attachment point           |
| FloatOffsetX  | float32     | Horizontal float offset            |
| FloatOffsetY  | float32     | Vertical float offset              |
| FloatZIndex   | int         | Z-order for floated elements       |

## Events

| Callback                | Signature                          | Fired when               |
|-------------------------|------------------------------------|--------------------------|
| Action (on MenubarCfg)  | func(string, *Event, *Window)      | Any item selected        |
| Action (on MenuItemCfg) | func(*MenuItemCfg, *Event, *Window)| Specific item selected   |
`,

	"command_palette": `Quick command search overlay with fuzzy filtering, keyboard
navigation, and grouped items. Shows a centered floating card with a
search input and scrollable results list.

## Usage

` + "```go" + `
gui.CommandPalette(gui.CommandPaletteCfg{
    ID:       "cmd",
    IDFocus:  focusPalette,
    IDScroll: scrollPalette,
    Items:    items,
    OnAction: func(id string, _ *gui.Event, w *gui.Window) {
        // handle action
    },
})

// Toggle with Ctrl+K or programmatically:
gui.CommandPaletteToggle("cmd", focusPalette, w)
` + "```" + `

## API

| Function                                     | Description              |
|----------------------------------------------|--------------------------|
| CommandPaletteShow(id, idFocus, idScroll, w) | Show and focus palette   |
| CommandPaletteDismiss(id, w)                 | Hide palette             |
| CommandPaletteToggle(id, idFocus, idScroll, w) | Toggle visibility      |
| CommandPaletteIsVisible(id, w) bool          | Check if visible         |

## Key Properties

| Property    | Type                 | Description                      |
|-------------|----------------------|----------------------------------|
| ID          | string               | Unique identifier                |
| Items       | []CommandPaletteItem | Available commands               |
| Placeholder | string               | Search input hint text           |
| Width       | float32              | Palette width                    |
| MaxHeight   | float32              | Maximum dropdown height          |
| IDFocus     | uint32               | Focus ID for input               |
| IDScroll    | uint32               | Scroll ID for results list       |
| FloatZIndex | int                  | Z-index for float layering       |

## Appearance

| Property       | Type         | Description                      |
|----------------|--------------|----------------------------------|
| Color          | Color        | Card background color            |
| ColorBorder    | Color        | Card border color                |
| ColorHighlight | Color        | Highlighted item color           |
| BackdropColor  | Color        | Semi-transparent backdrop        |
| SizeBorder     | Opt[float32] | Border width                     |
| Radius         | Opt[float32] | Corner radius                    |
| TextStyle      | TextStyle    | Item label text styling          |
| DetailStyle    | TextStyle    | Item detail text styling         |

## CommandPaletteItem

| Property | Type   | Description                          |
|----------|--------|--------------------------------------|
| ID       | string | Action identifier                    |
| Label    | string | Display text                         |
| Detail   | string | Secondary description                |
| Icon     | string | Icon glyph                           |
| Group    | string | Group heading                        |
| Disabled | bool   | Disable this item                    |

## Events

| Callback  | Signature                        | Fired when                   |
|-----------|----------------------------------|------------------------------|
| OnAction  | func(string, *Event, *Window)    | Command selected             |
| OnDismiss | func(*Window)                    | Palette dismissed            |
`,

	"context_menu": `Right-click context menu that opens at the cursor position.
Wraps any content view and intercepts right-click to show a floating
menu. Supports submenus, separators, subtitles, and keyboard navigation.

## Usage

` + "```go" + `
gui.ContextMenu(w, gui.ContextMenuCfg{
    ID:     "ctx",
    Sizing: gui.FillFit,
    Items: []gui.MenuItemCfg{
        {ID: "cut", Text: "Cut"},
        {ID: "copy", Text: "Copy"},
        {ID: "paste", Text: "Paste"},
        gui.MenuSeparator(),
        {ID: "delete", Text: "Delete"},
    },
    Action: func(id string, e *gui.Event, w *gui.Window) {
        // handle selected item
        e.IsHandled = true
    },
    Content: []gui.View{
        gui.Text(gui.TextCfg{Text: "Right-click here"}),
    },
})
` + "```" + `

## With Submenus

` + "```go" + `
gui.ContextMenu(w, gui.ContextMenuCfg{
    ID:     "ctx-fmt",
    Sizing: gui.FillFit,
    Items: []gui.MenuItemCfg{
        gui.MenuSubtitle("Edit"),
        {ID: "cut", Text: "Cut"},
        {ID: "copy", Text: "Copy"},
        gui.MenuSeparator(),
        gui.MenuSubmenu("format", "Format", []gui.MenuItemCfg{
            {ID: "bold", Text: "Bold"},
            {ID: "italic", Text: "Italic"},
        }),
    },
    Action: func(id string, e *gui.Event, w *gui.Window) {
        e.IsHandled = true
    },
    Content: []gui.View{...},
})
` + "```" + `

## Key Properties

| Property    | Type            | Description                         |
|-------------|-----------------|-------------------------------------|
| ID          | string          | Unique identifier                   |
| Items       | []MenuItemCfg   | Menu items to display               |
| Content     | []View          | Child views wrapped by context menu |
| IDFocus     | uint32          | Focus ID (auto-generated if 0)      |
| FloatZIndex | int             | Z-index for float layering          |
| Sizing      | Sizing          | Container sizing mode               |
| Width       | float32         | Container width                     |
| Height      | float32         | Container height                    |
| HAlign      | HorizontalAlign | Horizontal content alignment        |
| VAlign      | VerticalAlign   | Vertical content alignment          |
| Padding     | Opt[Padding]    | Inner padding                       |

## Appearance

| Property          | Type         | Description                      |
|-------------------|--------------|----------------------------------|
| Color             | Color        | Menu background color            |
| ColorBorder       | Color        | Menu border color                |
| ColorSelect       | Color        | Highlighted item color           |
| SizeBorder        | Opt[float32] | Border width                     |
| Radius            | Opt[float32] | Menu corner radius               |
| RadiusMenuItem    | Opt[float32] | Item corner radius               |
| TextStyle         | TextStyle    | Menu item text styling           |
| TextStyleSubtitle | TextStyle    | Subtitle text styling            |
| PaddingMenuItem   | Opt[Padding] | Menu item padding                |
| PaddingSubmenu    | Opt[Padding] | Submenu padding                  |
| SpacingSubmenu    | Opt[float32] | Submenu item spacing             |
| WidthSubmenuMin   | Opt[float32] | Minimum submenu width            |
| WidthSubmenuMax   | Opt[float32] | Maximum submenu width            |

## Events

| Callback   | Signature                          | Fired when                |
|------------|------------------------------------|---------------------------|
| Action     | func(string, *Event, *Window)      | Menu item selected        |
| OnAnyClick | func(*Layout, *Event, *Window)     | Any click before menu     |

## Menu Item Helpers

| Helper                                  | Description                     |
|-----------------------------------------|---------------------------------|
| MenuItemText(id, text)                  | Standard menu item              |
| MenuSeparator()                         | Horizontal divider              |
| MenuSubtitle(text)                      | Non-interactive section heading |
| MenuSubmenu(id, text, []MenuItemCfg)    | Nested submenu                  |

## Keyboard Navigation

| Key         | Action                              |
|-------------|-------------------------------------|
| Escape      | Close menu                          |
| Up / Down   | Move selection                      |
| Enter/Space | Activate selected item              |
| Right       | Open submenu                        |
| Left        | Close submenu / return to parent    |
`,

	// Overlays
	"dialog": `Modal dialog overlay with message, confirm, prompt, and custom
variants. Traps focus, dismisses on Escape, and supports Ctrl+C to
copy body text.

## Usage

` + "```go" + `
w.Dialog(gui.DialogCfg{
    Title:      "Confirm",
    Body:       "Delete this item?",
    DialogType: gui.DialogConfirm,
    OnOkYes: func(w *gui.Window) {
        // confirmed
    },
    OnCancelNo: func(w *gui.Window) {
        // cancelled
    },
})
` + "```" + `

## Prompt Dialog

` + "```go" + `
w.Dialog(gui.DialogCfg{
    Title:      "Rename",
    Body:       "Enter a new name:",
    DialogType: gui.DialogPrompt,
    Reply:      "Untitled",
    OnReply: func(text string, w *gui.Window) {
        gui.State[App](w).Name = text
    },
})
` + "```" + `

## API

| Method                   | Description                      |
|--------------------------|----------------------------------|
| w.Dialog(cfg)            | Show modal dialog                |
| w.DialogDismiss()        | Close current dialog             |
| w.DialogIsVisible() bool | Check if dialog is showing       |

## Dialog Types

| Type          | Buttons                          |
|---------------|----------------------------------|
| DialogMessage | OK                               |
| DialogConfirm | Yes / No                         |
| DialogPrompt  | Text input + OK / Cancel         |
| DialogCustom  | User-provided CustomContent      |

## Key Properties

| Property      | Type            | Description                      |
|---------------|-----------------|----------------------------------|
| Title         | string          | Dialog heading                   |
| Body          | string          | Message text                     |
| Reply         | string          | Pre-filled text (DialogPrompt)   |
| ID            | string          | Unique identifier                |
| DialogType    | DialogType      | Button configuration             |
| CustomContent | []View          | Custom views (DialogCustom)      |
| IDFocus       | uint32          | Initial focus target             |
| AlignButtons  | HorizontalAlign | Button alignment                 |
| Width         | float32         | Dialog width                     |
| Height        | float32         | Dialog height                    |
| MinWidth      | float32         | Minimum width                    |
| MinHeight     | float32         | Minimum height                   |
| MaxWidth      | float32         | Maximum width                    |
| MaxHeight     | float32         | Maximum height                   |

## Appearance

| Property       | Type         | Description                      |
|----------------|--------------|----------------------------------|
| Color          | Color        | Background color                 |
| ColorBorder    | Color        | Border color                     |
| Padding        | Opt[Padding] | Inner padding                    |
| SizeBorder     | Opt[float32] | Border width                     |
| Radius         | Opt[float32] | Corner radius                    |
| RadiusBorder   | Opt[float32] | Border corner radius             |
| TitleTextStyle | TextStyle    | Title text styling               |
| TextStyle      | TextStyle    | Body text styling                |

## Events

| Callback   | Signature                | Fired when                       |
|------------|--------------------------|----------------------------------|
| OnOkYes    | func(*Window)            | OK or Yes clicked                |
| OnCancelNo | func(*Window)            | Cancel, No, or Escape pressed    |
| OnReply    | func(string, *Window)    | Prompt submitted                 |
`,

	"tooltip": `Hover hints attached to any widget. Wrap any view with
` + "`WithTooltip`" + ` to show text on hover after a configurable delay.
Tooltips auto-flip when near screen edges.

## Usage

` + "```go" + `
gui.WithTooltip(w, gui.WithTooltipCfg{
    Text: "Helpful hint",
    Content: []gui.View{
        gui.Button(gui.ButtonCfg{...}),
    },
})
` + "```" + `

## Custom Positioning

` + "```go" + `
gui.WithTooltip(w, gui.WithTooltipCfg{
    Text:   "Right side tooltip",
    Anchor: gui.Some(gui.FloatMiddleRight),
    TieOff: gui.Some(gui.FloatMiddleLeft),
    Content: []gui.View{
        gui.Button(gui.ButtonCfg{...}),
    },
})
` + "```" + `

## WithTooltipCfg Properties

| Property | Type          | Description                          |
|----------|---------------|--------------------------------------|
| ID       | string        | Unique identifier                    |
| Text     | string        | Tooltip text content                 |
| Delay    | time.Duration | Hover delay before showing           |
| Anchor   | Opt[FloatAttach] | Tooltip anchor on trigger         |
| TieOff   | Opt[FloatAttach] | Tooltip attach point              |
| Content  | []View        | Wrapped target views                 |

## TooltipCfg Properties (Advanced)

| Property     | Type          | Description                      |
|--------------|---------------|----------------------------------|
| ID           | string        | Unique identifier                |
| Delay        | time.Duration | Hover delay before showing       |
| Content      | []View        | Custom tooltip content           |
| Anchor       | Opt[FloatAttach] | Float anchor point            |
| TieOff       | Opt[FloatAttach] | Float tie-off point           |
| OffsetX      | Opt[float32]     | Horizontal offset from anchor |
| OffsetY      | Opt[float32]     | Vertical offset from anchor   |
| FloatZIndex  | int           | Z-index for float layering       |

## Appearance

| Property     | Type         | Description                      |
|--------------|--------------|----------------------------------|
| Color        | Color        | Tooltip background color         |
| ColorBorder  | Color        | Border color                     |
| Padding      | Opt[Padding] | Inner padding                    |
| TextStyle    | TextStyle    | Tooltip text styling             |
| Radius       | Opt[float32] | Corner radius                    |
| SizeBorder   | Opt[float32] | Border width                     |
`,

	// Animations
	"animations": `Tween, spring, and keyframe animation types. All implement
the ` + "`Animation`" + ` interface and are managed via ` + "`w.AnimationAdd`" + `,
` + "`w.AnimationRemove`" + `, and ` + "`w.HasAnimation`" + `. The animation loop ticks
at ~60 fps (16ms). Each animation receives an ` + "`OnValue`" + ` callback
with the interpolated value and an optional ` + "`OnDone`" + ` callback.

## Tween

Interpolates from A to B over a fixed duration with easing.
Default: 300ms, EaseOutCubic.

` + "```go" + `
a := gui.NewTweenAnimation("slide", 0, 200,
    func(v float32, w *gui.Window) {
        gui.State[App](w).Offset = v
    })
a.Duration = 500 * time.Millisecond
a.Easing = gui.EaseInOutCubic
w.AnimationAdd(a)
` + "```" + `

## TweenAnimation Properties

| Property | Type          | Description                          |
|----------|---------------|--------------------------------------|
| AnimID   | string        | Unique animation identifier          |
| Duration | time.Duration | Animation length (default 300ms)     |
| Easing   | EasingFn      | Easing function (default EaseOutCubic) |
| From     | float32       | Start value                          |
| To       | float32       | End value                            |
| OnValue  | func(float32, *Window) | Called each tick with current value |
| OnDone   | func(*Window) | Called when animation completes       |

## Spring

Physics-based spring motion. Natural feel for interactive
elements — no fixed duration, settles based on physics.

` + "```go" + `
a := gui.NewSpringAnimation("bounce",
    func(v float32, w *gui.Window) {
        gui.State[App](w).Scale = v
    })
a.Config = gui.SpringBouncy
a.SpringTo(0, 1)
w.AnimationAdd(a)
` + "```" + `

## SpringAnimation Properties

| Property | Type          | Description                          |
|----------|---------------|--------------------------------------|
| AnimID   | string        | Unique animation identifier          |
| Config   | SpringCfg     | Spring physics parameters            |
| OnValue  | func(float32, *Window) | Called each tick              |
| OnDone   | func(*Window) | Called when spring comes to rest      |

## Spring Presets

| Preset        | Stiffness | Damping | Character              |
|---------------|-----------|---------|------------------------|
| SpringDefault | 100       | 10      | General purpose        |
| SpringGentle  | 50        | 8       | Soft, slow             |
| SpringBouncy  | 300       | 15      | Energetic, overshoots  |
| SpringStiff   | 500       | 30      | Fast, minimal bounce   |

## Keyframes

Multi-waypoint interpolation with per-segment easing.
Default: 500ms duration.

` + "```go" + `
a := gui.NewKeyframeAnimation("pulse",
    []gui.Keyframe{
        {At: 0.0, Value: 1.0, Easing: gui.EaseLinear},
        {At: 0.5, Value: 1.5, Easing: gui.EaseOutCubic},
        {At: 1.0, Value: 1.0, Easing: gui.EaseInCubic},
    },
    func(v float32, w *gui.Window) {
        gui.State[App](w).Scale = v
    })
a.Repeat = true
w.AnimationAdd(a)
` + "```" + `

## KeyframeAnimation Properties

| Property  | Type          | Description                          |
|-----------|---------------|--------------------------------------|
| AnimID    | string        | Unique animation identifier          |
| Duration  | time.Duration | Total animation length (default 500ms) |
| Keyframes | []Keyframe    | Waypoints with position and easing   |
| Repeat    | bool          | Loop continuously                    |
| OnValue   | func(float32, *Window) | Called each tick              |
| OnDone    | func(*Window) | Called when animation completes       |

## Keyframe

| Field  | Type     | Description                          |
|--------|----------|--------------------------------------|
| At     | float32  | Position 0.0-1.0                     |
| Value  | float32  | Value at this waypoint               |
| Easing | EasingFn | Easing TO this keyframe              |

## Easing Functions

| Function       | Character                            |
|----------------|--------------------------------------|
| EaseLinear     | Constant speed                       |
| EaseInQuad     | Slow start (quadratic)               |
| EaseOutQuad    | Slow end (quadratic)                 |
| EaseInOutQuad  | Slow start and end (quadratic)       |
| EaseInCubic    | Slow start (cubic)                   |
| EaseOutCubic   | Slow end (cubic, default tween)      |
| EaseInOutCubic | Slow start and end (cubic)           |
| EaseInBack     | Pulls back before accelerating       |
| EaseOutBack    | Overshoots then settles              |
| EaseOutElastic | Oscillates like a spring             |
| EaseOutBounce  | Bouncing ball                        |

## Window Animation API

| Method                | Description                          |
|-----------------------|--------------------------------------|
| w.AnimationAdd(a)     | Register or replace animation by ID  |
| w.AnimationRemove(id) | Stop and remove animation            |
| w.HasAnimation(id)    | Check if animation is active         |
`,

	// Theme
	"theme_gen": `Generate a complete theme from a ` + "`ThemeCfg`" + ` struct using
` + "`ThemeMaker`" + `. Provides full control over colors, sizing, padding,
radius, spacing, text sizes, and per-widget styles. Six built-in
themes are included as starting points.

## Usage

` + "```go" + `
cfg := gui.ThemeCfg{
    Name:            "ocean",
    ColorBackground: gui.ColorFromHSV(220, 0.15, 0.19),
    TextStyleDef:    gui.ThemeDark.Cfg.TextStyleDef,
    TitlebarDark:    true,
}
gui.SetTheme(gui.ThemeMaker(cfg))
` + "```" + `

## Built-in Themes

| Theme                | Description                          |
|----------------------|--------------------------------------|
| ThemeDark            | Dark with padding and radius         |
| ThemeDark.WithBorders(true)    | Dark with visible borders            |
| ThemeDark.WithPadding(false)   | Dark, compact                        |
| ThemeLight           | Light with padding and radius        |
| ThemeLight.WithBorders(true)   | Light with visible borders           |
| ThemeLight.WithPadding(false)  | Light, compact                       |

## ThemeCfg Color Properties

| Property         | Type  | Description                          |
|------------------|-------|--------------------------------------|
| ColorBackground  | Color | Window background                    |
| ColorPanel       | Color | Panel/card background                |
| ColorInterior    | Color | Input field interior                 |
| ColorHover       | Color | Hover highlight                      |
| ColorFocus       | Color | Focus ring color                     |
| ColorActive      | Color | Active/pressed state                 |
| ColorBorder      | Color | Default border color                 |
| ColorBorderFocus | Color | Border when focused                  |
| ColorSelect      | Color | Selection highlight                  |
| ColorSuccess     | Color | Success state color                  |
| ColorWarning     | Color | Warning state color                  |
| ColorError       | Color | Error state color                    |

## ThemeCfg Layout Properties

| Property      | Type    | Description                          |
|---------------|---------|--------------------------------------|
| Fill          | bool    | Fill widget backgrounds              |
| FillBorder    | bool    | Show widget borders                  |
| Padding       | Padding | Default padding                      |
| SizeBorder    | float32 | Default border width                 |
| Radius        | float32 | Default corner radius                |
| TitlebarDark  | bool    | Request dark OS titlebar             |

## ThemeCfg Size Scales

| Property       | Type    | Description                          |
|----------------|---------|--------------------------------------|
| PaddingSmall   | Padding | Small padding preset                 |
| PaddingMedium  | Padding | Medium padding preset                |
| PaddingLarge   | Padding | Large padding preset                 |
| RadiusSmall    | float32 | Small radius                         |
| RadiusMedium   | float32 | Medium radius                        |
| RadiusLarge    | float32 | Large radius                         |
| SpacingSmall   | float32 | Small spacing                        |
| SpacingMedium  | float32 | Medium spacing                       |
| SpacingLarge   | float32 | Large spacing                        |
| SizeTextTiny   | float32 | Tiny text size                       |
| SizeTextXSmall | float32 | XSmall text size                     |
| SizeTextSmall  | float32 | Small text size                      |
| SizeTextMedium | float32 | Medium text size                     |
| SizeTextLarge  | float32 | Large text size                      |
| SizeTextXLarge | float32 | XLarge text size                     |

## Color Strategies (Showcase)

| Strategy   | Description                          |
|------------|--------------------------------------|
| Mono       | Single-hue variations                |
| Complement | Opposite hue accent                  |
| Analogous  | Adjacent hue palette                 |
| Triadic    | Three evenly-spaced hues             |
| Warm       | Warm-shifted palette                 |
| Cool       | Cool-shifted palette                 |

Use the tint slider to control surface saturation. Dark mode
is derived automatically from the same seed color.
`,

	// Audio
	"audio": `Opt-in audio playback via SDL_mixer. Supports sound effects on
multiple mixing channels and one music track. Desktop only
(macOS, Windows, Linux).

## Setup

` + "```go" + `
import "github.com/go-gui-org/go-gui/gui/audio"

// Initialize once (e.g. in main or OnInit).
if err := audio.Init(); err != nil {
    log.Fatal(err)
}
defer audio.Quit()
` + "```" + `

## Sound Effects

` + "```go" + `
// Load from file.
click, _ := audio.LoadSound("click.wav")
defer click.Free()
click.PlayOnce()

// Load from embedded bytes.
snd, _ := audio.LoadSoundBytes(wavData)
snd.Play(-1, 0) // channel -1 = first free, 0 = no loop
` + "```" + `

## Music

` + "```go" + `
bgm, _ := audio.LoadMusic("theme.ogg")
defer bgm.Free()
bgm.Play(-1)        // -1 = loop forever
bgm.FadeIn(-1, 2000) // fade in over 2 s

audio.PauseMusic()
audio.ResumeMusic()
audio.FadeOutMusic(1000)
` + "```" + `

## Volume

` + "```go" + `
audio.SetMasterVolume(0.8) // sound effects 0.0–1.0
audio.SetMusicVolume(0.5)  // music channel 0.0–1.0

snd.SetVolume(0.3)         // per-sound volume
` + "```" + `

## audio.Cfg

| Field          | Type   | Default                      |
|----------------|--------|------------------------------|
| Frequency      | int    | 44100                        |
| Format         | uint16 | mix.DEFAULT_FORMAT           |
| OutputChannels | int    | 2 (stereo)                   |
| ChunkSize      | int    | 2048                         |
| MixChannels    | int    | 16                           |
| Formats        | int    | INIT_OGG ` + "|" + ` INIT_MP3 |

## Sound API

| Function              | Description                              |
|-----------------------|------------------------------------------|
| LoadSound(path)       | Load from file (WAV, OGG, FLAC, …)      |
| LoadSoundBytes(data)  | Load from byte slice                     |
| Play(channel, loops)  | Play on channel (-1 = auto)              |
| PlayOnce()            | Shorthand for Play(-1, 0)                |
| FadeIn(ch, loops, ms) | Play with fade-in                        |
| SetVolume(v)          | Per-sound volume 0.0–1.0                 |
| Free()                | Release resources                        |

## Music API

| Function           | Description                                 |
|--------------------|---------------------------------------------|
| LoadMusic(path)    | Load music file (OGG, MP3, FLAC, MOD, …)   |
| Play(loops)        | Play (0 = once, -1 = forever)               |
| FadeIn(loops, ms)  | Play with fade-in                           |
| Free()             | Release resources                           |

## Global Controls

| Function            | Description                                |
|---------------------|--------------------------------------------|
| SetMasterVolume(v)  | All sound channels 0.0–1.0                 |
| SetMusicVolume(v)   | Music channel 0.0–1.0                      |
| HaltChannel(ch)     | Stop channel (-1 = all)                    |
| HaltMusic()         | Stop music immediately                     |
| FadeOutMusic(ms)    | Fade out music                             |
| PauseMusic()        | Pause music                                |
| ResumeMusic()       | Resume music                               |

## Notes

- Only **one music track** plays at a time (SDL_mixer limitation)
- Do not Free a Sound while it is still playing
- Call Init from the **main goroutine** (same thread as SDL event loop)
- Requires SDL_mixer system library (` + "`brew install sdl2_mixer`" + ` on macOS)
`,

	// Notification
	"notification": `Send native OS notifications via the platform notification
center. Runs asynchronously; result is delivered via callback on the
main thread.

## Usage

` + "```go" + `
w.NativeNotification(gui.NativeNotificationCfg{
    Title: "App",
    Body:  "Task completed!",
    OnDone: func(r gui.NativeNotificationResult, w *gui.Window) {
        if r.Status == gui.NotificationOK {
            // delivered successfully
        }
    },
})
` + "```" + `

## Key Properties

| Property | Type   | Description                              |
|----------|--------|------------------------------------------|
| Title    | string | Notification title (required)            |
| Body     | string | Notification body text                   |

## Events

| Callback | Signature                                    | Fired when               |
|----------|----------------------------------------------|--------------------------|
| OnDone   | func(NativeNotificationResult, *Window)      | Notification delivered   |

## NativeNotificationResult

| Field        | Type                     | Description              |
|--------------|--------------------------|--------------------------|
| Status       | NativeNotificationStatus | Outcome status           |
| ErrorCode    | string                   | Platform error code      |
| ErrorMessage | string                   | Human-readable error     |

## Result Status

| Status             | Meaning                              |
|--------------------|--------------------------------------|
| NotificationOK     | Delivered successfully               |
| NotificationDenied | Permission denied by OS              |
| NotificationError  | Platform error                       |
`,

	// Shader
	"shader": `Apply custom fragment shaders (Metal + GLSL) to any container.
Write only the color-computation body — the framework wraps it with
struct definitions, SDF round-rect clipping, and pipeline caching
via ` + "`BuildGLSLFragment`" + `.

## Static Shader

` + "```go" + `
gui.Column(gui.ContainerCfg{
    Width: 200, Height: 200,
    Sizing: gui.FixedFixed,
    Radius: gui.SomeF(8),
    Shader: &gui.Shader{
        Metal: ` + "`" + `
            float2 st = in.uv * 0.5 + 0.5;
            float4 frag_color = float4(st.x, st.y, 0.5, 1.0);
        ` + "`" + `,
        GLSL: ` + "`" + `
            vec2 st = uv * 0.5 + 0.5;
            vec4 frag_color = vec4(st.x, st.y, 0.5, 1.0);
        ` + "`" + `,
    },
})
` + "```" + `

## Animated Shader

Pass time or other values via Params. Each float maps to
p0.x, p0.y, p0.z, p0.w, p1.x, ... (up to 16 floats).

` + "```go" + `
elapsed := float32(time.Since(startTime).Milliseconds()) / 1000.0

gui.Column(gui.ContainerCfg{
    Width: 200, Height: 200,
    Sizing: gui.FixedFixed,
    Radius: gui.SomeF(16),
    Shader: &gui.Shader{
        Metal: ` + "`" + `
            float t = in.p0.x;
            float2 st = in.uv * 0.5 + 0.5;
            float3 c = 0.5 + 0.5 * cos(t + st.xyx + float3(0,2,4));
            float4 frag_color = float4(c, 1.0);
        ` + "`" + `,
        GLSL: ` + "`" + `
            float t = p0.x;
            vec2 st = uv * 0.5 + 0.5;
            vec3 c = 0.5 + 0.5 * cos(t + st.xyx + vec3(0,2,4));
            vec4 frag_color = vec4(c, 1.0);
        ` + "`" + `,
        Params: []float32{elapsed},
    },
})
` + "```" + `

## Shader Properties

| Property | Type      | Description                          |
|----------|-----------|--------------------------------------|
| Metal    | string    | MSL fragment body                    |
| GLSL     | string    | GLSL fragment body (desktop GL 3.3 / WebGL2-compatible) |
| Params   | []float32 | Up to 16 custom floats               |

## Available Inputs

| Metal        | GLSL         | Type        | Description            |
|--------------|--------------|-------------|------------------------|
| in.uv        | uv           | float2/vec2 | -1..1 centered coords  |
| in.color     | color        | float4/vec4 | Vertex color           |
| in.p0..in.p3 | p0..p3       | float4/vec4 | Custom params          |
| in.position  | gl_FragCoord | float4/vec4 | Screen position        |

## Output

Declare a local ` + "`float4 frag_color`" + ` (Metal) or ` + "`vec4 frag_color`" + `
(GLSL). The framework applies SDF clipping automatically.

## Notes

- Must provide both Metal and GLSL bodies for cross-platform
- Pipeline is compiled once per unique source and cached
  (` + "`ShaderHash`" + ` computes the cache key)
- Shader fill takes priority over Gradient and solid Color
- Add a repeating animation to keep the frame loop hot
`,

	// Printing
	"printing": `Export the current window to PDF or send to the OS print
dialog. Supports paper sizes, margins, orientation, duplex,
color mode, page ranges, headers/footers, and DPI settings.

## Export PDF

` + "```go" + `
job := gui.NewPrintJob()
job.OutputPath = "/tmp/output.pdf"
job.Title = "My Document"
job.Paper = gui.PaperA4
job.Orientation = gui.PrintLandscape
r := w.ExportPrintJob(job)
if r.IsOk() {
    fmt.Println("Saved to", r.Path)
}
` + "```" + `

## Print via OS Dialog

` + "```go" + `
job := gui.NewPrintJob()
job.Title = "My Document"
r := w.RunPrintJob(job)
if r.Status == gui.PrintRunOK {
    // printed successfully
}
` + "```" + `

## PrintJob Properties

| Property    | Type                 | Description                          |
|-------------|----------------------|--------------------------------------|
| OutputPath  | string               | PDF output path (export only)        |
| Title       | string               | Document title                       |
| JobName     | string               | OS print job name                    |
| Paper       | PaperSize            | Paper size                           |
| Orientation | PrintOrientation     | Portrait or Landscape                |
| Margins     | PrintMargins         | Page margins in points (1/72 inch)   |
| Copies      | int                  | Number of copies (default 1)         |
| Duplex      | PrintDuplexMode      | Simplex / LongEdge / ShortEdge      |
| ColorMode   | PrintColorMode       | Default / Color / Grayscale          |
| ScaleMode   | PrintScaleMode       | FitToPage or ActualSize              |
| PageRanges  | []PrintPageRange     | Specific page ranges                 |
| Header      | PrintHeaderFooterCfg | Header text (left/center/right)      |
| Footer      | PrintHeaderFooterCfg | Footer text (left/center/right)      |
| Paginate    | bool                 | Enable pagination                    |
| RasterDPI   | int                  | Raster DPI (default 300)             |
| JPEGQuality | int                  | JPEG quality (default 85)            |

## Paper Sizes

| Constant    | Size           |
|-------------|----------------|
| PaperLetter | 8.5 x 11 in   |
| PaperLegal  | 8.5 x 14 in   |
| PaperA4     | 210 x 297 mm  |
| PaperA3     | 297 x 420 mm  |

## PrintMargins

| Field  | Type    | Description                |
|--------|---------|----------------------------|
| Top    | float32 | Top margin in points       |
| Right  | float32 | Right margin in points     |
| Bottom | float32 | Bottom margin in points    |
| Left   | float32 | Left margin in points      |

` + "`DefaultPrintMargins()`" + ` returns 36pt (0.5 inch) on all sides.

## PrintExportResult

| Field        | Type              | Description              |
|--------------|-------------------|--------------------------|
| Status       | PrintExportStatus | PrintExportOK or Error   |
| Path         | string            | Output file path         |
| ErrorCode    | string            | Error code if failed     |
| ErrorMessage | string            | Human-readable error     |

## PrintRunResult

| Field        | Type           | Description              |
|--------------|----------------|--------------------------|
| Status       | PrintRunStatus | OK / Cancel / Error      |
| ErrorCode    | string         | Error code if failed     |
| ErrorMessage | string         | Human-readable error     |
| PDFPath      | string         | Path to generated PDF    |
| Warnings     | []PrintWarning | Non-fatal issues         |
`,

	"tree": `Hierarchical expandable node display with virtualization,
lazy-loading, keyboard navigation, and drag-reorder support.

## Usage

` + "```go" + `
gui.Tree(gui.TreeCfg{
    ID:        "project-tree",
    IDFocus:   2001,
    IDScroll:  2002,
    MaxHeight: 240,
    OnSelect: func(nodeID string, _ *gui.Event, w *gui.Window) {
        gui.State[AppState](w).SelectedNode = nodeID
    },
    Nodes: []gui.TreeNodeCfg{
        {
            ID:   "src",
            Text: "src",
            Icon: gui.IconFolder,
            Nodes: []gui.TreeNodeCfg{
                {Text: "main.go"},
                {Text: "view_tree.go"},
            },
        },
    },
})
` + "```" + `

## Lazy Loading

` + "```go" + `
gui.Tree(gui.TreeCfg{
    ID: "lazy-tree",
    OnLazyLoad: func(treeID, nodeID string, w *gui.Window) {
        // Fetch children async, update state, call w.UpdateWindow()
    },
    Nodes: []gui.TreeNodeCfg{
        {ID: "remote", Text: "remote", Lazy: true},
    },
})
` + "```" + `

` + "`TreeNodeCfg.ID`" + ` defaults to ` + "`Text`" + ` when omitted. Node IDs must be unique within a tree.

## Virtualization

Renders only visible nodes (flattened) when height is constrained.
Requires ` + "`IDScroll > 0`" + ` and ` + "`Height`" + ` or ` + "`MaxHeight > 0`" + `.
The main usage example above shows the required configuration.

## Keyboard Navigation

` + "`Up`" + ` / ` + "`Down`" + ` / ` + "`Left`" + ` (collapse) / ` + "`Right`" + ` (expand) / ` + "`Home`" + ` / ` + "`End`" + ` / ` + "`Enter`" + ` / ` + "`Space`" + `

## Stdlib Data Binding

Use ` + "`ItemPaths []string`" + ` for flat path strings. Slash-separated
paths auto-expand into nested nodes:

` + "```go" + `
gui.Tree(gui.TreeCfg{
    ID:        "simple-tree",
    ItemPaths: []string{"src/main.go", "src/lib.go", "docs/readme.md"},
    OnSelect: func(nodeID string, _ *gui.Event, w *gui.Window) { ... },
})
` + "```" + `

When ` + "`ItemPaths`" + ` is set, ` + "`Nodes`" + ` is ignored.

## Key Properties

| Property    | Type            | Description                          |
|-------------|-----------------|--------------------------------------|
| ItemPaths   | []string        | Flat slash-separated paths (alt.)    |
| Nodes       | []TreeNodeCfg   | Root-level tree nodes                |
| Indent      | float32         | Indent per nesting level             |
| Spacing     | float32         | Vertical spacing between rows        |
| IDFocus     | uint32          | Tab-order focus ID (> 0 to enable)   |
| IDScroll    | uint32          | Scroll ID (enables virtualization)   |
| Reorderable | bool            | Enable drag-reorder of siblings      |
| Disabled    | bool            | Disable interaction                  |
| Invisible   | bool            | Hide without removing from layout    |
| Sizing      | Sizing          | Combined axis sizing mode            |
| Width       | float32         | Fixed width                          |
| Height      | float32         | Fixed height                         |
| MinWidth    | float32         | Minimum width                        |
| MaxWidth    | float32         | Maximum width                        |
| MinHeight   | float32         | Minimum height                       |
| MaxHeight   | float32         | Maximum height                       |

## TreeNodeCfg

| Property      | Type          | Description                          |
|---------------|---------------|--------------------------------------|
| ID            | string        | Node identifier (defaults to Text)   |
| Text          | string        | Display text                         |
| Icon          | string        | Icon string (e.g. IconFolder)        |
| Lazy          | bool          | Load children on expand              |
| Nodes         | []TreeNodeCfg | Child nodes                          |
| TextStyle     | TextStyle     | Text styling                         |
| TextStyleIcon | TextStyle     | Icon text styling                    |

## Appearance

| Property    | Type         | Description                          |
|-------------|--------------|--------------------------------------|
| Color       | Color        | Background color                     |
| ColorHover  | Color        | Hover background                     |
| ColorFocus  | Color        | Focused node background              |
| ColorBorder | Color        | Border color                         |
| Padding     | Opt[Padding] | Inner padding                        |
| SizeBorder  | Opt[float32] | Border width                         |
| Radius      | Opt[float32] | Corner radius                        |

## Events

| Callback   | Signature                                    | Fired when              |
|------------|----------------------------------------------|-------------------------|
| OnSelect   | func(string, *Event, *Window)                | Node selected           |
| OnLazyLoad | func(string, string, *Window)                | Lazy node expanded      |
| OnReorder  | func(movedID, beforeID string, w *Window)    | Node drag-reordered     |

## Accessibility

| Property        | Type   | Description                          |
|-----------------|--------|--------------------------------------|
| A11YLabel       | string | Accessible label                     |
| A11YDescription | string | Accessible description               |
`,

	// Drag Reorder
	"drag_reorder": `Drag items to reorder within lists, tabs, and tree
views. Keyboard shortcuts provide an accessible alternative.
Supports both vertical (ListBox, Tree) and horizontal (TabControl)
axes. Uses FLIP animation for smooth visual transitions.

## ListBox

` + "```go" + `
gui.ListBox(gui.ListBoxCfg{
    Reorderable: true,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        from, to := gui.ReorderIndices(ids, movedID, beforeID)
        if from >= 0 { sliceMove(items, from, to) }
    },
})
` + "```" + `

## TabControl

` + "```go" + `
gui.TabControl(gui.TabControlCfg{
    Reorderable: true,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        from, to := gui.ReorderIndices(tabIDs, movedID, beforeID)
        if from >= 0 { sliceMove(tabs, from, to) }
    },
})
` + "```" + `

## Tree

` + "```go" + `
gui.Tree(gui.TreeCfg{
    Reorderable: true,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        // Reorder scoped to siblings under the same parent
    },
})
` + "```" + `

## Behaviors

- 5px threshold before activation (prevents accidental drags)
- ` + "`Alt+Arrow`" + ` keyboard shortcut for accessible reordering
- ` + "`Escape`" + ` cancels an active drag
- Tree reordering is scoped to siblings under the same parent
- FLIP animation on index change and drop
- Auto-scroll near container edges during drag (40px zone)
- Mutation detection: drag cancels if backing list changes

## Drag Axes

| Constant                | Direction  | Used by            |
|-------------------------|------------|--------------------|
| DragReorderVertical     | Up/Down    | ListBox, Tree      |
| DragReorderHorizontal   | Left/Right | TabControl         |

## OnReorder Callback

| Parameter | Type   | Description                          |
|-----------|--------|--------------------------------------|
| movedID   | string | ID of the dragged item               |
| beforeID  | string | ID of the item to insert before      |

` + "`beforeID`" + ` is ` + "`\"\"`" + ` when dropping at the end of the list.

## Helper

` + "`gui.ReorderIndices(ids, movedID, beforeID)`" + ` computes
(fromIndex, toIndex) for use with slice reordering.
`,

	// Locale
	"locale": `Global locale system controlling number formatting, date
formatting, currency symbols, UI strings (OK, Cancel, etc.),
weekday/month names, text direction, and app-level translations.
Ten locales are registered by default; custom locales can be
added via ` + "`LocaleRegister`" + ` or loaded from JSON bundles.

## Set Locale

` + "```go" + `
// Set directly
gui.SetLocale(gui.LocaleDeDE)

// Set by ID on a window (also refreshes)
w.SetLocaleID("de-DE")

// Auto-detect from OS
gui.LocaleAutoDetect()
` + "```" + `

## Register Custom Locale

` + "```go" + `
gui.LocaleRegister(gui.Locale{
    ID:      "nl-NL",
    TextDir: gui.TextDirLTR,
    Number: gui.NumberFormat{
        DecimalSep: ',',
        GroupSep:   '.',
        GroupSizes: []int{3},
        MinusSign:  '-',
        PlusSign:   '+',
    },
    Date: gui.DateFormat{
        ShortDate:      "D-M-YYYY",
        LongDate:       "D MMMM YYYY",
        MonthYear:      "MMMM YYYY",
        FirstDayOfWeek: 1,
    },
    StrOK:     "OK",
    StrCancel: "Annuleren",
    StrYes:    "Ja",
    StrNo:     "Nee",
})
` + "```" + `

## Load from JSON

` + "```go" + `
locale, err := gui.LocaleLoad("locales/nl-NL.json")
if err == nil {
    gui.LocaleRegister(locale)
}
` + "```" + `

## Built-in Locales

| ID    | Language             | Text Dir |
|-------|----------------------|----------|
| en-US | English (US)         | LTR      |
| de-DE | German               | LTR      |
| fr-FR | French               | LTR      |
| es-ES | Spanish              | LTR      |
| pt-BR | Portuguese (Brazil)  | LTR      |
| ja-JP | Japanese             | LTR      |
| zh-CN | Chinese (Simplified) | LTR      |
| ko-KR | Korean               | LTR      |
| ar-SA | Arabic               | RTL      |
| he-IL | Hebrew               | RTL      |

## Locale Struct

| Field          | Type               | Description                          |
|----------------|--------------------|--------------------------------------|
| ID             | string             | Locale identifier (e.g. "en-US")     |
| TextDir        | TextDirection      | TextDirLTR or TextDirRTL             |
| Number         | NumberFormat        | Number formatting rules              |
| Date           | DateFormat          | Date formatting rules                |
| Currency       | CurrencyFormat      | Currency formatting rules            |
| Translations   | map[string]string   | App-level translation keys           |
| WeekdaysShort  | [7]string          | Sun..Sat short names                 |
| WeekdaysFull   | [7]string          | Sun..Sat full names                  |
| MonthsShort    | [12]string         | Jan..Dec short names                 |
| MonthsFull     | [12]string         | Jan..Dec full names                  |

## NumberFormat

| Field      | Type   | Description                          |
|------------|--------|--------------------------------------|
| DecimalSep | rune   | Decimal separator (default '.')      |
| GroupSep   | rune   | Thousands separator (default ',')    |
| GroupSizes | []int  | Digit grouping sizes (default [3])   |
| MinusSign  | rune   | Minus sign (default '-')             |
| PlusSign   | rune   | Plus sign (default '+')              |

## DateFormat

| Field          | Type   | Description                          |
|----------------|--------|--------------------------------------|
| ShortDate      | string | Short date pattern (e.g. "M/D/YYYY") |
| LongDate       | string | Long date pattern                    |
| MonthYear      | string | Month-year pattern                   |
| FirstDayOfWeek | uint8  | 0=Sunday, 1=Monday                   |
| Use24H         | bool   | Use 24-hour time format              |

## CurrencyFormat

| Field    | Type                 | Description                          |
|----------|----------------------|--------------------------------------|
| Symbol   | string               | Currency symbol (e.g. "$")           |
| Code     | string               | ISO code (e.g. "USD")                |
| Position | NumericAffixPosition | AffixPrefix or AffixSuffix           |
| Spacing  | bool                 | Space between symbol and number      |
| Decimals | int                  | Decimal places (default 2)           |

## UI Strings

| Field     | Default  | Description                          |
|-----------|----------|--------------------------------------|
| StrOK     | "OK"     | OK button label                      |
| StrCancel | "Cancel" | Cancel button label                  |
| StrYes    | "Yes"    | Yes button label                     |
| StrNo     | "No"     | No button label                      |
| StrSave   | "Save"   | Save action                          |
| StrDelete | "Delete" | Delete action                        |
| StrSearch | "Search" | Search action                        |

## What Changes

- Date picker day/month names and first-day-of-week
- Numeric input decimal/thousands separators
- Dialog button labels (OK, Cancel, Yes, No)
- RTL text direction (ar-SA, he-IL)
- Currency symbols and placement

## API

| Function                     | Description                          |
|------------------------------|--------------------------------------|
| SetLocale(l)                 | Set active global locale             |
| CurrentLocale()              | Get active locale                    |
| w.SetLocale(l)               | Set locale and refresh window        |
| w.SetLocaleID(id)            | Set locale by ID and refresh         |
| LocaleAutoDetect()           | Detect OS locale, set best match     |
| LocaleRegister(l)            | Add locale to registry               |
| LocaleGet(id)                | Look up locale by ID                 |
| LocaleRegisteredNames()      | List all registered locale IDs       |
| LocaleLoad(path)             | Load locale from JSON file           |
| LocaleParse(json)            | Parse locale from JSON string        |
| LocaleFormatDate(t, format)  | Format date with locale month names  |
| LocaleT(key)                 | Look up translation key              |
`,

	"rotated_box": `Rotate child content by quarter turns (90° increments).
Returns the child directly when turns == 0 (no rotation). Negative
values and values > 3 are normalized (mod 4).

## Usage

` + "```go" + `
gui.RotatedBox(gui.RotatedBoxCfg{
    QuarterTurns: 1,
    Content:      gui.Text(gui.TextCfg{Text: "Sideways"}),
})
` + "```" + `

## Vertical Label

` + "```go" + `
gui.RotatedBox(gui.RotatedBoxCfg{
    QuarterTurns: 3,
    Content: gui.Text(gui.TextCfg{
        Text:      "Y Axis",
        TextStyle: t.B3,
    }),
})
` + "```" + `

## Key Properties

| Property     | Type | Description                         |
|--------------|------|-------------------------------------|
| QuarterTurns | int  | Number of 90° clockwise turns (0–3) |
| Content      | View | Single child view to rotate         |

## Behavior

| Turns | Angle | Dimensions |
|-------|-------|------------|
| 0     | 0°    | unchanged  |
| 1     | 90°   | w ↔ h swap |
| 2     | 180°  | unchanged  |
| 3     | 270°  | w ↔ h swap |

Content is clipped to the rotated bounds. Sizing is always FitFit —
the rotated box measures to its child size (swapping width/height for
odd quarter turns).
`,

	// Multi-Window
	"multi_window": `Manage multiple OS windows from a single application. An
App instance acts as a registry for all windows, enabling cross-window
communication via Broadcast and QueueCommand.

## Setup

` + "```go" + `
app := gui.NewApp()
app.ExitMode = gui.ExitOnMainClose

main := gui.NewWindow(gui.WindowCfg{
    State: &MainState{},
    Title: "Main",
    OnInit: func(w *gui.Window) {
        w.UpdateView(mainView)
    },
})
backend.RunApp(app, main)
` + "```" + `

## Opening a Window at Runtime

` + "```go" + `
w.App().OpenWindow(gui.WindowCfg{
    State:  &ChildState{},
    Title:  "Child",
    Width:  300,
    Height: 200,
    OnInit: func(child *gui.Window) {
        child.UpdateView(childView)
    },
})
` + "```" + `

## Cross-Window Communication

` + "```go" + `
// Send a message from child to parent.
parent.QueueCommand(func(p *gui.Window) {
    gui.State[ParentState](p).Message = "hello"
    p.UpdateWindow()
})

// Broadcast to all windows.
w.App().Broadcast(func(other *gui.Window) {
    other.QueueCommand(func(o *gui.Window) {
        // update other window state
        o.UpdateWindow()
    })
})
` + "```" + `

## Key Types

| Type       | Description                                     |
|------------|-------------------------------------------------|
| App        | Window registry with exit-mode policy           |
| ExitMode   | ExitOnLastClose (default) or ExitOnMainClose    |
| WindowCfg  | Configuration for window creation               |

## App Methods

| Method      | Signature                        | Description                          |
|-------------|----------------------------------|--------------------------------------|
| Register    | (id uint32, w *Window)           | Associate platform ID with window    |
| Unregister  | (id uint32) bool                 | Remove window; returns exit signal   |
| Window      | (id uint32) *Window              | Lookup by platform ID                |
| Windows     | () []*Window                     | Snapshot of all windows in order      |
| OpenWindow  | (cfg WindowCfg)                  | Queue window creation (next frame)   |
| Broadcast   | (fn func(*Window))               | Call fn for every registered window  |

## Window Methods

| Method       | Signature              | Description                          |
|--------------|------------------------|--------------------------------------|
| App          | () *App                | Parent app (nil in single-window)    |
| PlatformID   | () uint32              | OS window ID (0 if unregistered)     |
| Close        | ()                     | Request close on next frame          |
| QueueCommand | (fn func(*Window))     | Thread-safe deferred callback        |

## Notes

- OpenWindow buffers up to 16 pending requests per frame.
- QueueCommand is safe to call from any goroutine.
- Close is atomic and safe to call from any goroutine.
- Each window has its own typed state slot via State[T](w).
`,

	"spinner": `Animated mathematical curve rendered as a particle trail.
Twenty-one named curve types are available, each with sensible
defaults. Override ParamA/B/D for custom tuning. Optional slow
rotation adds visual depth.

## Usage

` + "```go" + `
gui.Spinner(gui.SpinnerCfg{
    ID:        "loading",
    CurveType: gui.CurveRose,
    Size:      80,
}, w)
` + "```" + `

## With Color and Rotation

` + "```go" + `
gui.Spinner(gui.SpinnerCfg{
    ID:        "search",
    CurveType: gui.CurveHypotrochoid,
    Size:      100,
    Color:     gui.RGB(46, 160, 67),
    Rotate:    true,
}, w)
` + "```" + `

## Custom Parameters

` + "```go" + `
gui.Spinner(gui.SpinnerCfg{
    ID:          "custom",
    CurveType:   gui.CurveLissajous,
    Size:        120,
    ParamA:      gui.SomeF(5),
    ParamB:      gui.SomeF(6),
    ParamD:      gui.SomeF(1.57),
    StrokeWidth: 3,
    Speed:       1.5,
    TrailLength: 0.5,
    Particles:   80,
}, w)
` + "```" + `

## Key Properties

| Property    | Type         | Description                                     |
|-------------|--------------|-------------------------------------------------|
| ID          | string       | Unique identifier (drives animation)            |
| CurveType   | CurveType    | Named curve constant (21 available)             |
| Size        | float32      | Convenience square size (default 48)            |
| Width       | float32      | Explicit width (overrides Size)                 |
| Height      | float32      | Explicit height (overrides Size)                |
| Sizing      | Sizing       | Combined axis sizing mode                       |
| MinWidth    | float32      | Minimum width                                   |
| MaxWidth    | float32      | Maximum width                                   |
| MinHeight   | float32      | Minimum height                                  |
| MaxHeight   | float32      | Maximum height                                  |
| ParamA      | Opt[float32] | Curve-specific parameter A                      |
| ParamB      | Opt[float32] | Curve-specific parameter B                      |
| ParamD      | Opt[float32] | Curve-specific parameter D                      |

## Appearance

| Property    | Type    | Description                                        |
|-------------|---------|----------------------------------------------------|
| Color       | Color   | Particle and ghost path color (default ColorActive) |
| StrokeWidth | float32 | Base particle radius (default 2.5)                 |
| TrailLength | float32 | Fraction of curve visible as trail, 0-1 (def 0.35) |
| Particles   | int     | Number of dots in trail, 2-500 (default 60)        |

## Animation

| Property | Type    | Description                                       |
|----------|---------|---------------------------------------------------|
| Speed    | float32 | Animation speed multiplier (default 1)            |
| Rotate   | bool    | Slowly rotate entire curve (30s per revolution)   |

## Curve Types

| Constant              | Family       | Description                         |
|-----------------------|--------------|-------------------------------------|
| CurveOriginalThinking | Epitrochoid  | 7-petal ring (R=7, k=7, d=3)       |
| CurveThinkingFive     | Epitrochoid  | 5-petal ring                        |
| CurveThinkingNine     | Epitrochoid  | 9-petal ring                        |
| CurveRoseOrbit        | Rose Orbit   | Orbit with petal modulation         |
| CurveRose             | Rose         | 5-petal rose r=a cos(kθ)           |
| CurveRoseTwo          | Rose         | 2-petal rose                        |
| CurveRoseThree        | Rose         | 3-petal rose                        |
| CurveRoseFour         | Rose         | 4-petal rose                        |
| CurveLissajous        | Lissajous    | Crossed sine paths                  |
| CurveLemniscate       | Lemniscate   | Bernoulli infinity sign             |
| CurveHypotrochoid     | Hypotrochoid | Inner spirograph (R=8, r=3, d=5)   |
| CurveThreePetalSpiral | Hypotrochoid | 3-petal spirograph                  |
| CurveFourPetalSpiral  | Hypotrochoid | 4-petal spirograph                  |
| CurveFivePetalSpiral  | Hypotrochoid | 5-petal spirograph                  |
| CurveSixPetalSpiral   | Hypotrochoid | 6-petal spirograph                  |
| CurveButterfly        | Butterfly    | Butterfly curve with exponential    |
| CurveCardioid         | Cardioid     | Heart-shaped r=a(1-cosθ)           |
| CurveCardioidHeart    | Cardioid     | Upright heart r=a(1+cosθ)          |
| CurveHeartWave        | Heart Wave   | x^(2/3) envelope with sine ripples |
| CurveSpiral           | Spiral       | Archimedean spiral                  |
| CurveFourier          | Fourier      | Multi-harmonic sum                  |
`,
	"svg_spinner": `One hundred six built-in animated SVG spinners drawn from
the open-source svg-spinners collection plus extended SMIL- and
CSS-driven assets. Each kind is embedded as raw SVG and rendered
through the same pipeline as ` + "`gui.Svg`" + `, so assets recolor via
` + "`fill=\"currentColor\"`" + ` and scale cleanly at any size. Identify a
spinner by its ` + "`SvgSpinnerKind`" + ` constant; call ` + "`SvgSpinnerCount`" + `
or ` + "`SvgSpinnerName`" + ` to enumerate or label them.

## Usage

` + "```go" + `
gui.SvgSpinner(gui.SvgSpinnerCfg{
    ID:   "loading",
    Kind: gui.SvgSpinnerTailSpin,
})
` + "```" + `

## With Color

` + "```go" + `
gui.SvgSpinner(gui.SvgSpinnerCfg{
    ID:    "save",
    Kind:  gui.SvgSpinner3DotsBounce,
    Color: gui.RGB(46, 160, 67),
})
` + "```" + `

## Explicit Size

` + "```go" + `
gui.SvgSpinner(gui.SvgSpinnerCfg{
    ID:     "hero",
    Kind:   gui.SvgSpinner90Ring,
    Width:  96,
    Height: 96,
})
` + "```" + `

When both ` + "`Width`" + ` and ` + "`Height`" + ` are zero, the spinner
defaults to 48x48. Setting only one axis leaves the other at zero so
the containing layout or aspect ratio of the asset controls it.

## Enumerating Kinds

` + "```go" + `
for i := range gui.SvgSpinnerCount() {
    k := gui.SvgSpinnerKind(i)
    fmt.Println(gui.SvgSpinnerName(k)) // e.g. "90-ring"
}
` + "```" + `

## Key Properties

| Property  | Type           | Description                           |
|-----------|----------------|---------------------------------------|
| ID        | string         | Unique identifier (required)          |
| Kind      | SvgSpinnerKind | One of 106 built-in spinners          |
| Color     | Color          | Recolor monochrome assets             |
| Width     | float32        | Explicit width (default 48 if both 0) |
| Height    | float32        | Explicit height (default 48 if both 0)|
| Sizing    | Sizing         | Combined axis sizing mode             |
| Padding   | Opt[Padding]   | Outer padding                         |
| MinWidth  | float32        | Minimum width                         |
| MaxWidth  | float32        | Maximum width                         |
| MinHeight | float32        | Minimum height                        |
| MaxHeight | float32        | Maximum height                        |
| OnClick   | func           | Click handler (optional)              |

## Accessibility

| Property        | Type   | Description                                      |
|-----------------|--------|--------------------------------------------------|
| A11YLabel       | string | Short name announced by screen readers           |
| A11YDescription | string | Longer description (e.g. "loading, please wait") |

## Helpers

| Function          | Description                                   |
|-------------------|-----------------------------------------------|
| SvgSpinnerCount() | Returns the number of built-in kinds (106)    |
| SvgSpinnerName(k) | Returns the asset basename (e.g. "tail-spin") |

## Animation Pipeline

SMIL features:

- ` + "`<animate>`" + `, ` + "`<animateTransform>`" + ` (rotate, translate, scale,
  TRS sandwich), ` + "`<animateMotion>`" + ` with inline path or
  ` + "`<mpath>`" + ` reference plus ` + "`rotate=\"auto\"`" + `
- opacity, fill-opacity, stroke-opacity, per-role opacity
- primitive attribute animation (cx/cy/r/x/y/width/height/rx/ry)
- dash animations (stroke-dasharray, stroke-dashoffset)
- keyTimes, keySplines spline easing, calcMode=discrete
- syncbase begin chains, additive/accumulate="sum", from/to/by
- min/max dur clamps, restart="never"/"whenNotActive"
- ` + "`<set>`" + ` elements (freeze semantics by default)

CSS features:

- selector cascade (id, class, type), presentation-attribute fallback
- ` + "`@keyframes`" + ` with percentage stops, ` + "`animation`" + ` shorthand
- ` + "`animation-iteration-count`" + ` (including ` + "`infinite`" + `),
  ` + "`animation-timing-function`" + ` (` + "`steps()`" + `, cubic-bezier),
  delays, durations
- multi-animation per element, ` + "`@media`" + ` query gating
- ` + "`display:none`" + ` and mixed SMIL+CSS animation on the same shape

## Kinds: Rings & Circles

| Constant                     | Asset                 |
|------------------------------|-----------------------|
| SvgSpinner90Ring             | 90-ring               |
| SvgSpinner90RingWithBg       | 90-ring-with-bg       |
| SvgSpinner90RingWithGradient | 90-ring-with-gradient |
| SvgSpinner180Ring            | 180-ring              |
| SvgSpinner180RingWithBg      | 180-ring-with-bg      |
| SvgSpinner270Ring            | 270-ring              |
| SvgSpinner270RingWithBg      | 270-ring-with-bg      |
| SvgSpinnerCircles            | circles               |
| SvgSpinnerEclipse            | eclipse               |
| SvgSpinnerEclipseHalf        | eclipse-half          |
| SvgSpinnerLoader2            | loader2               |
| SvgSpinnerOval               | oval                  |
| SvgSpinnerRingResize         | ring-resize           |
| SvgSpinnerRings              | rings                 |
| SvgSpinnerSpinner            | spinner               |
| SvgSpinnerSpinnerDouble      | spinner-double        |
| SvgSpinnerSpinnerMultiple    | spinner-multiple      |
| SvgSpinnerSpinnerMultiple2   | spinner-multiple-2    |
| SvgSpinnerSpinningCircles    | spinning-circles      |
| SvgSpinnerTailSpin           | tail-spin             |

## Kinds: Dots

| Constant                    | Asset                |
|-----------------------------|----------------------|
| SvgSpinner12DotsScaleRotate | 12-dots-scale-rotate |
| SvgSpinner3DotsBounce       | 3-dots-bounce        |
| SvgSpinner3DotsFade         | 3-dots-fade          |
| SvgSpinner3DotsMove         | 3-dots-move          |
| SvgSpinner3DotsRotate       | 3-dots-rotate        |
| SvgSpinner3DotsScale        | 3-dots-scale         |
| SvgSpinner3DotsScaleMiddle  | 3-dots-scale-middle  |
| SvgSpinner4DotsGoeey        | 4-dots-goeey         |
| SvgSpinner4DotsRotate       | 4-dots-rotate        |
| SvgSpinner6DotsRotate       | 6-dots-rotate        |
| SvgSpinner6DotsScale        | 6-dots-scale         |
| SvgSpinner6DotsScaleMiddle  | 6-dots-scale-middle  |
| SvgSpinner8DotsRotate       | 8-dots-rotate        |
| SvgSpinnerBallTriangle      | ball-triangle        |
| SvgSpinnerDotRevolve        | dot-revolve          |

## Kinds: Bars

| Constant                  | Asset             |
|---------------------------|-------------------|
| SvgSpinnerBars            | bars              |
| SvgSpinnerBarsFade        | bars-fade         |
| SvgSpinnerBarsRotateFade  | bars-rotate-fade  |
| SvgSpinnerBarsScale       | bars-scale        |
| SvgSpinnerBarsScaleFade   | bars-scale-fade   |
| SvgSpinnerBarsScaleMiddle | bars-scale-middle |
| SvgSpinnerHorizontalBar   | horizontal-bar    |

## Kinds: Loaders

| Constant            | Asset       |
|---------------------|-------------|
| SvgSpinnerLoader1   | loader1     |
| SvgSpinnerLoader3   | loader3     |
| SvgSpinnerLoader4   | loader4     |
| SvgSpinnerLoader5   | loader5     |
| SvgSpinnerLoader6   | loader6     |
| SvgSpinnerLoader7   | loader7     |
| SvgSpinnerLoader8   | loader8     |
| SvgSpinnerLoader9   | loader9     |
| SvgSpinnerLoader10  | loader10    |
| SvgSpinnerLoaderWifi| loader-wifi |

## Kinds: Blocks

| Constant                 | Asset            |
|--------------------------|------------------|
| SvgSpinnerBlocksScale    | blocks-scale     |
| SvgSpinnerBlocksShuffle2 | blocks-shuffle-2 |
| SvgSpinnerBlocksShuffle3 | blocks-shuffle-3 |
| SvgSpinnerBlocksShuffle4 | blocks-shuffle-4 |
| SvgSpinnerBlocksShuffle5 | blocks-shuffle-5 |
| SvgSpinnerBlocksWave     | blocks-wave      |

## Kinds: Pulse

| Constant                     | Asset                |
|------------------------------|----------------------|
| SvgSpinnerGooeyBalls1        | gooey-balls-1        |
| SvgSpinnerGooeyBalls2        | gooey-balls-2        |
| SvgSpinnerHeartPulse         | heart-pulse          |
| SvgSpinnerHeartPulse2        | heart-pulse-2        |
| SvgSpinnerHeartPulse3        | heart-pulse-3        |
| SvgSpinnerPuff               | puff                 |
| SvgSpinnerPulse              | pulse                |
| SvgSpinnerPulse2             | pulse2               |
| SvgSpinnerPulse3             | pulse-3              |
| SvgSpinnerPulseMultiple      | pulse-multiple       |
| SvgSpinnerPulseRing          | pulse-ring           |
| SvgSpinnerPulseRings2        | pulse-rings-2        |
| SvgSpinnerPulseRings3        | pulse-rings-3        |
| SvgSpinnerPulseRingsMultiple | pulse-rings-multiple |

## Kinds: Cogs

| Constant        | Asset |
|-----------------|-------|
| SvgSpinnerCog01 | cog01 |
| SvgSpinnerCog02 | cog02 |
| SvgSpinnerCog03 | cog03 |
| SvgSpinnerCog04 | cog04 |
| SvgSpinnerCog05 | cog05 |
| SvgSpinnerCog06 | cog06 |
| SvgSpinnerCog07 | cog07 |
| SvgSpinnerCog08 | cog08 |
| SvgSpinnerCog09 | cog09 |
| SvgSpinnerCog10 | cog10 |
| SvgSpinnerCog11 | cog11 |
| SvgSpinnerCog12 | cog12 |
| SvgSpinnerCog13 | cog13 |
| SvgSpinnerCog14 | cog14 |
| SvgSpinnerCog15 | cog15 |
| SvgSpinnerCog16 | cog16 |
| SvgSpinnerCog17 | cog17 |
| SvgSpinnerCog18 | cog18 |
| SvgSpinnerCog19 | cog19 |
| SvgSpinnerCog20 | cog20 |
| SvgSpinnerCog21 | cog21 |
| SvgSpinnerCog22 | cog22 |
| SvgSpinnerCog23 | cog23 |
| SvgSpinnerCog24 | cog24 |

## Kinds: Miscellaneous

| Constant               | Asset         |
|------------------------|---------------|
| SvgSpinnerAudio        | audio         |
| SvgSpinnerBouncingBall | bouncing-ball |
| SvgSpinnerCircleFade   | circle-fade   |
| SvgSpinnerClock        | clock         |
| SvgSpinnerGrid         | grid          |
| SvgSpinnerHearts       | hearts        |
| SvgSpinnerTadpole      | tadpole       |
| SvgSpinnerWifi         | wifi          |
| SvgSpinnerWifiFade     | wifi-fade     |
| SvgSpinnerWindToy      | wind-toy      |

## Caveats

- Assets relying on SMIL or CSS features outside the supported subset
  may render as their static first frame.
- ` + "`<set>`" + ` defaults to freeze semantics (SMIL default is remove);
  authors who want remove must set ` + "`fill=\"remove\"`" + ` explicitly.
- ` + "`SvgSpinnerCount`" + ` and enum values are generated from the embedded
  asset directory, so numeric values shift when assets are added or
  removed — always reference kinds by their named constant.
`,
}
