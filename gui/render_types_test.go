package gui

import "testing"

// renderCmdKindName returns a debug name for the given RenderKind.
func renderCmdKindName(k RenderKind) string {
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
	case RenderTermGrid:
		return "RenderTermGrid"
	default:
		return "Unknown"
	}
}

func TestRenderCmdKindNameExhaustive(t *testing.T) {
	tests := []struct {
		kind RenderKind
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
		{RenderTermGrid, "RenderTermGrid"},
		{RenderKind(255), "Unknown"},
	}
	for _, tt := range tests {
		got := renderCmdKindName(tt.kind)
		if got != tt.want {
			t.Errorf("renderCmdKindName(%d) = %q, want %q",
				tt.kind, got, tt.want)
		}
	}
}

func TestFindFilterBracketRangeFound(t *testing.T) {
	cmds := []RenderCmd{
		{Kind: RenderFilterBegin},
		{Kind: RenderRect},
		{Kind: RenderFilterEnd},
		{Kind: RenderRect},
	}
	r := findFilterBracketRange(cmds, 0)
	if !r.FoundEnd {
		t.Fatal("expected FoundEnd=true")
	}
	if r.StartIdx != 0 || r.EndIdx != 2 || r.NextIdx != 3 {
		t.Errorf("got start=%d end=%d next=%d, want 0/2/3",
			r.StartIdx, r.EndIdx, r.NextIdx)
	}
}

func TestFindFilterBracketRangeNotFound(t *testing.T) {
	cmds := []RenderCmd{
		{Kind: RenderFilterBegin},
		{Kind: RenderRect},
		{Kind: RenderRect},
	}
	r := findFilterBracketRange(cmds, 0)
	if r.FoundEnd {
		t.Fatal("expected FoundEnd=false")
	}
	if r.EndIdx != 3 || r.NextIdx != 3 {
		t.Errorf("got end=%d next=%d, want 3/3", r.EndIdx, r.NextIdx)
	}
}

func TestFindFilterBracketRangeMidSlice(t *testing.T) {
	cmds := []RenderCmd{
		{Kind: RenderRect},
		{Kind: RenderFilterBegin},
		{Kind: RenderRect},
		{Kind: RenderFilterEnd},
	}
	r := findFilterBracketRange(cmds, 1)
	if !r.FoundEnd || r.StartIdx != 1 || r.EndIdx != 3 || r.NextIdx != 4 {
		t.Errorf("got start=%d end=%d next=%d found=%v, want 1/3/4/true",
			r.StartIdx, r.EndIdx, r.NextIdx, r.FoundEnd)
	}
}

func TestFindFilterBracketRangeEmpty(t *testing.T) {
	r := findFilterBracketRange(nil, 0)
	if r.FoundEnd {
		t.Fatal("expected FoundEnd=false for empty slice")
	}
	if r.EndIdx != 0 || r.NextIdx != 0 {
		t.Errorf("got end=%d next=%d, want 0/0", r.EndIdx, r.NextIdx)
	}
}
