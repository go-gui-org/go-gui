package gui

import "testing"

// renderCmdKindName returns a debug name for the given RenderKind.
func renderCmdKindName(k renderKind) string {
	switch k {
	case RenderNone:
		return "RenderNone"
	case RenderClip:
		return "RenderClip"
	case RenderRect:
		return "RenderRect"
	case RenderStrokeRect:
		return "RenderStrokeRect"
	case RenderCircle:
		return "RenderCircle"
	case RenderImage:
		return "RenderImage"
	case RenderText:
		return "RenderText"
	case RenderLine:
		return "RenderLine"
	case RenderShadow:
		return "RenderShadow"
	case RenderBlur:
		return "RenderBlur"
	case RenderGradient:
		return "RenderGradient"
	case RenderGradientBorder:
		return "RenderGradientBorder"
	case RenderSvg:
		return "RenderSvg"
	case RenderLayout:
		return "RenderLayout"
	case RenderLayoutTransformed:
		return "RenderLayoutTransformed"
	case RenderLayoutPlaced:
		return "RenderLayoutPlaced"
	case RenderFilterBegin:
		return "RenderFilterBegin"
	case RenderFilterEnd:
		return "RenderFilterEnd"
	case RenderFilterComposite:
		return "RenderFilterComposite"
	case RenderCustomShader:
		return "RenderCustomShader"
	case RenderTextPath:
		return "RenderTextPath"
	case RenderRTF:
		return "RenderRTF"
	case RenderRotateBegin:
		return "RenderRotateBegin"
	case RenderRotateEnd:
		return "RenderRotateEnd"
	case RenderStencilBegin:
		return "RenderStencilBegin"
	case RenderStencilEnd:
		return "RenderStencilEnd"
	default:
		return "Unknown"
	}
}

func TestRenderCmdKindNameExhaustive(t *testing.T) {
	tests := []struct {
		kind renderKind
		want string
	}{
		{RenderNone, "RenderNone"},
		{RenderClip, "RenderClip"},
		{RenderRect, "RenderRect"},
		{RenderStrokeRect, "RenderStrokeRect"},
		{RenderCircle, "RenderCircle"},
		{RenderImage, "RenderImage"},
		{RenderText, "RenderText"},
		{RenderLine, "RenderLine"},
		{RenderShadow, "RenderShadow"},
		{RenderBlur, "RenderBlur"},
		{RenderGradient, "RenderGradient"},
		{RenderGradientBorder, "RenderGradientBorder"},
		{RenderSvg, "RenderSvg"},
		{RenderLayout, "RenderLayout"},
		{RenderLayoutTransformed, "RenderLayoutTransformed"},
		{RenderLayoutPlaced, "RenderLayoutPlaced"},
		{RenderFilterBegin, "RenderFilterBegin"},
		{RenderFilterEnd, "RenderFilterEnd"},
		{RenderFilterComposite, "RenderFilterComposite"},
		{RenderCustomShader, "RenderCustomShader"},
		{RenderTextPath, "RenderTextPath"},
		{RenderRTF, "RenderRTF"},
		{RenderRotateBegin, "RenderRotateBegin"},
		{RenderRotateEnd, "RenderRotateEnd"},
		{RenderStencilBegin, "RenderStencilBegin"},
		{RenderStencilEnd, "RenderStencilEnd"},
		{renderKind(255), "Unknown"},
	}
	for _, tt := range tests {
		got := renderCmdKindName(tt.kind)
		if got != tt.want {
			t.Errorf("renderCmdKindName(%d) = %q, want %q",
				tt.kind, got, tt.want)
		}
	}
}
