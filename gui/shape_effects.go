package gui

// shapeEffects holds optional visual effect fields.
type shapeEffects struct {
	Shadow         *BoxShadow
	Gradient       *GradientDef
	BorderGradient *GradientDef
	Shader         *Shader
	ColorFilter    *ColorFilter
	BlurRadius     float32

	// shaderBuiltins backs RenderCmd.shaderBuiltins for this shape's
	// shader. It is rewritten on every render pass, so a render-only
	// animation reuses it instead of allocating per pass.
	shaderBuiltins [16]float32

	// imageSurface is ImageCfg.Surface, kept here rather than on Shape
	// so the field does not grow every shape (TestShapeFitsSizeClass).
	imageSurface *PixelSurface
}
