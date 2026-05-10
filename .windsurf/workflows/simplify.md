---
description: Simplify complex code by identifying refactoring opportunities
---

# Code Simplification Workflow

This workflow helps identify and implement simplifications in the go-gui codebase to improve maintainability and readability.

## When to Use

- When code becomes complex or hard to understand
- When functions are getting too long
- When there are nested conditionals that could be simplified
- When duplicate code patterns emerge
- When API design could be more intuitive

## Steps

### 1. Analyze Current Changes

Focus on simplifying the code you're currently working on:

```bash
# Check what files have uncommitted changes
git status --porcelain

# Review the specific changes you've made
git diff

# For the current file, look for complexity issues:
# - Long functions (>50 lines)
# - Deeply nested conditionals (>3 levels)
# - Repeated code patterns
# - Magic numbers that should be constants
```

### 2. Identify Refactoring Opportunities

Look for common patterns:

- **Extract Function**: Long functions (>50 lines)
- **Extract Method**: Repeated code blocks
- **Simplify Conditional**: Nested if/else chains
- **Replace Magic Numbers**: Named constants
- **Reduce Parameters**: Too many function arguments
- **Consolidate Duplicate**: Similar code patterns

### 3. Apply Simplifications

Use these refactoring techniques:

#### Extract Function
```go
// Before
func complexFunction(w *Window) {
    // ... 30+ lines of setup logic
    // ... main logic
    // ... 20+ lines of cleanup
}

// After  
func complexFunction(w *Window) {
    setup(w)
    mainLogic(w)
    cleanup(w)
}
```

#### Simplify Conditionals
```go
// Before
if x != nil && x.Type == Button && x.Enabled && x.Visible {
    // handle button
}

// After
if isButtonVisibleAndEnabled(x) {
    // handle button
}
```

#### Extract Constants
```go
// Before
button.Height = 32
button.Padding = 8

// After
const (
    ButtonHeight = 32
    ButtonPadding = 8
)
button.Height = ButtonHeight
button.Padding = ButtonPadding
```

### 4. Verify Simplifications

Ensure changes don't break functionality:

```bash
# Run all tests
go test ./...

# Check linting
golangci-lint run ./gui/...

# Build verification
go build ./...
```

### 5. Update Documentation

Update any affected documentation or examples.

## Best Practices

- **One change at a time**: Don't refactor multiple things simultaneously
- **Preserve behavior**: Simplifications should not change observable behavior
- **Add tests**: If simplifying untested code, add tests first
- **Use descriptive names**: Extracted functions should clearly state their purpose
- **Keep functions small**: Aim for functions under 30-40 lines
- **Reduce nesting**: Avoid deeply nested conditionals (>3 levels)

## Common Simplification Patterns in go-gui

### Widget Factory Simplification
```go
// Complex factory with many parameters
func ComplexButton(text string, onClick func(*Layout, *Event, *Window), 
                  width, height float32, enabled bool, style ButtonStyle, 
                  icon string, tooltip string) *Layout {
    // ... complex logic
}

// Simplified with config struct
func Button(cfg ButtonCfg) *Layout {
    // ... clean logic
}
```

### Layout Simplification
```go
// Before: nested container configs
func buildUI() *Layout {
    return gui.Column(gui.ContainerCfg{
        Content: []gui.View{
            gui.Row(gui.ContainerCfg{
                Content: []gui.View{
                    gui.Text(gui.TextCfg{Text: "Label"}),
                    gui.Input(gui.InputCfg{Text: "Value"}),
                },
            }),
        },
    })
}

// After: extract layout functions
func buildUI() *Layout {
    return gui.Column(gui.ContainerCfg{
        Content: []gui.View{
            buildLabelInputRow(),
        },
    })
}

func buildLabelInputRow() gui.View {
    return gui.Row(gui.ContainerCfg{
        Content: []gui.View{
            gui.Text(gui.TextCfg{Text: "Label"}),
            gui.Input(gui.InputCfg{Text: "Value"}),
        },
    })
}
```

## Tools

- **gocyclo**: Cyclomatic complexity analysis
- **golangci-lint**: Comprehensive linting
- **go test**: Test verification
- **grep**: Pattern searching
- **git diff**: Review changes before commit