package gui

// Stack lays its content in layers, one over the other. Every child starts
// at the content origin; the first child is at the bottom and the last child
// is on top. The top child is drawn last and gets the pointer first.
//
// A Fit size is the largest child on each axis, plus padding. A Fill child
// takes the whole content box, which suits a background image or canvas.
// HAlign and VAlign place each child on both axes. To place one layer
// differently from the others, put it in a FillFill Column with its own
// alignment. Spacing has no effect.
//
// A Stack layer stays in the tree: it scrolls and clips with the Stack. Use
// a Float for content that must escape its container, such as a dropdown.
func Stack(cfg ContainerCfg) View {
	cfg.axis = axisStack
	return container(cfg)
}
