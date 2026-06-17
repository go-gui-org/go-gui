package gui

// RotatedBoxCfg configures a RotatedBox view.
type RotatedBoxCfg struct {
	Content      View // single child
	QuarterTurns int  // 1=90° CW, 2=180°, 3=270° CW
}

// RotatedBox rotates its child content by quarter turns.
// Returns the child directly when turns == 0 (no rotation).
func RotatedBox(cfg RotatedBoxCfg) View {
	turns := ((cfg.QuarterTurns % 4) + 4) % 4
	if turns == 0 || cfg.Content == nil {
		if cfg.Content != nil {
			return cfg.Content
		}
		return &rotatedBoxView{turns: 0}
	}
	return &rotatedBoxView{
		turns:   uint8(turns),
		content: cfg.Content,
	}
}

type rotatedBoxView struct {
	content View
	turns   uint8
}

func (v *rotatedBoxView) Content() []View {
	if v.content == nil {
		return nil
	}
	return []View{v.content}
}

func (v *rotatedBoxView) GenerateLayout(w *Window) Layout {
	return Layout{
		Shape: w.allocShape(Shape{
			shapeType:    shapeRectangle,
			Axis:         AxisTopToBottom,
			QuarterTurns: v.turns,
			Clip:         true,
			Sizing:       FitFit,
			Opacity:      1.0,
		}),
	}
}
