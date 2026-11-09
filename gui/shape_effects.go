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

	// groupTitle is a titled container's group-box label, a text shape
	// the container draws itself (renderGroupBoxTitle), and titleBG is
	// ContainerCfg.TitleBG behind it. Here rather than on Shape for the
	// same reason as imageSurface (#1238). titleBand is half the
	// label's first line, the band above the border (groupTitleBand);
	// a label with more lines is taller than twice it.
	groupTitle *Shape
	titleBG    Color
	titleBand  float32
}
