Drag items to reorder within lists, tabs, and tree
views. Keyboard shortcuts provide an accessible alternative.
Supports both vertical (ListBox, Tree) and horizontal (TabControl)
axes. Uses FLIP animation for smooth visual transitions.

## ListBox

```go
gui.ListBox(gui.ListBoxCfg{
    Reorderable: true,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        from, to := gui.ReorderIndices(ids, movedID, beforeID)
        if from >= 0 { sliceMove(items, from, to) }
    },
})
```

## TabControl

```go
gui.TabControl(gui.TabControlCfg{
    Reorderable: true,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        from, to := gui.ReorderIndices(tabIDs, movedID, beforeID)
        if from >= 0 { sliceMove(tabs, from, to) }
    },
})
```

## Tree

```go
gui.Tree(gui.TreeCfg{
    Reorderable: true,
    OnReorder: func(movedID, beforeID string, w *gui.Window) {
        // Reorder scoped to siblings under the same parent
    },
})
```

## Behaviors

- 5px threshold before activation (prevents accidental drags)
- `Alt+Arrow` keyboard shortcut for accessible reordering
- `Escape` cancels an active drag
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

`beforeID` is `\"\"` when dropping at the end of the list.

## Helper

`gui.ReorderIndices(ids, movedID, beforeID)` computes
(fromIndex, toIndex) for use with slice reordering.
