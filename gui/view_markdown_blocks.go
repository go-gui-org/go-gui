package gui

// view_markdown_blocks.go contains block renderers for the Markdown view.

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui/markdown"
)

// renderMdMath renders a display math block.
func renderMdMath(
	block markdownBlock, cfg MarkdownCfg, w *Window,
) View {
	codeFallback := Column(ContainerCfg{
		Color:      cfg.Style.CodeBlockBG,
		Padding:    cfg.Style.codeBlockPadding,
		Radius:     RadiusPx(cfg.Style.codeBlockRadius),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content: []View{
			Text(TextCfg{
				Text:      block.MathLatex,
				TextStyle: cfg.Style.Code,
			}),
		},
	})

	if cfg.DisableExternalAPIs || !markdownExternalAPIsEnabled {
		return codeFallback
	}

	diagramHash := diagramCacheHash(
		fmt.Sprintf("display_%d", markdown.MathHash(block.MathLatex)))

	cache := ensureDiagramCache(w)
	if entry, ok := cache.Get(diagramHash); ok {
		switch entry.State {
		case diagramLoading:
			return codeFallback
		case diagramReady:
			return Image(ImageCfg{
				Src:    entry.pNGPath,
				Width:  entry.Width,
				Height: entry.Height,
			})
		case diagramError:
			return markdownDiagramErrorView(
				entry.Error, cfg.Style.Code,
			)
		}
	}

	// Start async fetch.
	if cache.LoadingCount() <
		maxConcurrentDiagramFetches {
		reqID := nextDiagramRequestID(w)
		w.viewState.diagramCache.Set(diagramHash,
			DiagramCacheEntry{
				State:     diagramLoading,
				RequestID: reqID,
			})
		fetchMathAsync(w, block.MathLatex, diagramHash,
			reqID, cfg.Style.mathDPIDisplay,
			cfg.Style.Text.Color, cfg.MathFetcher)
	}
	return codeFallback
}

// renderMdMermaid renders a mermaid diagram block.
func renderMdMermaid(
	block markdownBlock, cfg MarkdownCfg, w *Window,
) View {
	source := richTextPlain(block.Content)
	codeFallback := Column(ContainerCfg{
		Color:      cfg.Style.CodeBlockBG,
		Padding:    cfg.Style.codeBlockPadding,
		Radius:     RadiusPx(cfg.Style.codeBlockRadius),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content: []View{
			RTF(RTFCfg{
				RichText: block.Content,
				Mode:     TextModeSingleLine,
			}),
		},
	})

	if cfg.DisableExternalAPIs || !markdownExternalAPIsEnabled {
		return codeFallback
	}

	diagramHash := diagramCacheHash(source)

	cache := ensureDiagramCache(w)
	if entry, ok := cache.Get(diagramHash); ok {
		switch entry.State {
		case diagramLoading:
			return Text(TextCfg{
				Text:      "Loading diagram...",
				TextStyle: cfg.Style.Text,
			})
		case diagramReady:
			imgW, imgH := entry.Width, entry.Height
			mw := float32(cfg.MermaidWidth)
			if mw <= 0 {
				mw = 600
			}
			if imgW > mw {
				imgH *= mw / imgW
				imgW = mw
			}
			return Image(ImageCfg{
				Src:     entry.pNGPath,
				Width:   imgW,
				Height:  imgH,
				BgColor: White,
			})
		case diagramError:
			return markdownDiagramErrorView(
				entry.Error, cfg.Style.Code,
			)
		}
	}

	if cache.LoadingCount() <
		maxConcurrentDiagramFetches {
		reqID := nextDiagramRequestID(w)
		cache.Set(diagramHash,
			DiagramCacheEntry{
				State:     diagramLoading,
				RequestID: reqID,
			})
		fetchMermaidAsync(w, source, diagramHash, reqID,
			cfg.MermaidFetcher)
	}
	return codeFallback
}

// mdCopyButtonInset is how far the floating copy button sits in from
// the top-right corner it is tied to. mdCopyButtonPadX is its side
// padding around the icon.
const (
	mdCopyButtonInset float32 = 4
	mdCopyButtonPadX  float32 = 4
)

// mdCopyButtonReserve is the width the floating copy button covers,
// counted from the right edge it is tied to, plus a gap so text does
// not touch it. The icon glyph is about as wide as its font size.
func mdCopyButtonReserve() float32 {
	return mdCopyButtonInset + 2*mdCopyButtonPadX +
		guiTheme.TextStyleIconXSmall.Size + gapTight
}

// mdCopyButton builds a floating copy-to-clipboard button
// with a 2-second check-mark animation.
func mdCopyButton(
	animID string, w *Window,
	onClick func(EventCtx),
) View {
	// HasAnimation, not hasAnimationLocked: this runs in a view
	// function, which holds w.mu but not w.animMu, and the animation
	// loop writes w.animations from its own goroutine under animMu.
	copied := w.HasAnimation(animID)

	iconStyle := guiTheme.TextStyleIconXSmall
	iconStyle.Color = Gray

	var btnContent []View
	if copied {
		checkStyle := iconStyle
		checkStyle.Color = RGBA(80, 200, 80, 255)
		btnContent = []View{
			Text(TextCfg{Text: IconCheck, TextStyle: checkStyle}),
		}
	} else {
		btnContent = []View{
			Text(TextCfg{Text: IconFile, TextStyle: iconStyle}),
		}
	}

	return Button(ButtonCfg{
		// animID already identifies this code block uniquely (it keys
		// the check-mark animation), so it namespaces the button too.
		ID:           ScopeID(animID, "copy"),
		Float:        true,
		FloatAnchor:  FloatTopRight,
		FloatTieOff:  FloatTopRight,
		FloatOffsetX: -mdCopyButtonInset,
		FloatOffsetY: mdCopyButtonInset,
		Radius:       RadiusSmall,
		Color:        ColorTransparent,
		SizeBorder:   NoBorder,
		Padding:      NewPadding(2, mdCopyButtonPadX, 2, mdCopyButtonPadX), // ergonomics-audit:spacing — icon-sized floating button
		Content:      btnContent,
		OnClick:      onClick,
	})
}

// renderMdCode renders a fenced code block with a copy-to-clipboard button.
func renderMdCode(
	block markdownBlock, cfg MarkdownCfg, w *Window, blockIdx int,
) View {
	// Scoped to the document: the block index alone repeats across
	// Markdown views, so two documents in one window would give their
	// nth code block the same copy-button ID and animation key.
	animID := ScopeIDN(cfg.ID, "code", blockIdx)
	copyBtn := mdCopyButton(animID, w,
		func(ctx EventCtx) {
			plain := richTextPlain(block.Content)
			ctx.Window.SetClipboard(plain)
			ctx.Window.AnimationAdd(&Animate{
				AnimID:   animID,
				Delay:    2 * time.Second,
				Callback: func(*Animate, *Window) {},
			})
			ctx.Consume()
		})

	return Column(ContainerCfg{
		Color:      cfg.Style.CodeBlockBG,
		Padding:    cfg.Style.codeBlockPadding,
		Radius:     RadiusPx(cfg.Style.codeBlockRadius),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Clip:       true,
		Content: []View{
			RTF(RTFCfg{
				RichText: block.Content,
				Mode:     TextModeSingleLine,
			}),
			copyBtn,
		},
	})
}

func mdFlushListItems(
	listItems []View, cfg MarkdownCfg,
) View {
	return Column(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Spacing:    SpacingPx(cfg.Style.blockSpacing / 2),
		Content:    listItems,
	})
}

func mdRenderMathBlock(
	block markdownBlock, cfg MarkdownCfg, w *Window,
) View {
	return mdWrapQuote(block, Column(ContainerCfg{
		Sizing:     FillFit,
		HAlign:     HAlignCenter,
		SizeBorder: NoBorder,
		Content: []View{
			renderMdMath(block, cfg, w),
		},
	}), cfg)
}

func mdRenderCodeBlock(
	block markdownBlock, cfg MarkdownCfg, w *Window, idx int,
) View {
	if block.CodeLanguage == "mermaid" {
		return mdWrapQuote(block, Column(ContainerCfg{
			Sizing:     FillFit,
			HAlign:     HAlignCenter,
			SizeBorder: NoBorder,
			Content: []View{
				renderMdMermaid(block, cfg, w),
			},
		}), cfg)
	}
	return mdWrapQuote(block, renderMdCode(block, cfg, w, idx), cfg)
}

func mdRenderTable(
	block markdownBlock, cfg MarkdownCfg, w *Window, idx int,
) View {
	if block.TableData == nil {
		return nil
	}
	return mdWrapQuote(block, Column(ContainerCfg{
		Sizing:  FillFit,
		Padding: NoPadding,
		Clip:    true,
		Content: []View{
			w.Table(TableCfg{
				ID:               ScopeIDN(cfg.ID, "table", idx),
				BorderStyle:      cfg.Style.TableBorderStyle,
				Colors:           ColorSet{Border: cfg.Style.tableBorderColor},
				SizeBorder:       cfg.Style.tableBorderSize,
				TextStyleHead:    cfg.Style.tableHeadStyle,
				TextStyle:        cfg.Style.tableCellStyle,
				CellPadding:      cfg.Style.tableCellPadding,
				ColorRowAlt:      cfg.Style.tableRowAlt,
				ColumnAlignments: block.TableData.Alignments,
				Data:             buildMarkdownTableData(*block.TableData, cfg.Style),
			}),
		},
	}), cfg)
}

func mdRenderHR(cfg MarkdownCfg) View {
	return Separator(SeparatorCfg{
		Color: cfg.Style.hRColor,
	})
}

func applyMdCtx(cfg *RTFCfg, ctx *mdBlockCtx) {
	if ctx != nil {
		cfg.markdownID = ctx.ID
		cfg.markdownSel = ctx.Sel
		cfg.markdownBlockStart = ctx.Start
	}
}

// mdWrapQuote draws quote chrome around an already-rendered view
// when block carries quote depth from a stamped composite (a quoted
// list, fence, heading, table or image), and returns v untouched
// otherwise. Every kind shares this wrapper, so the bar and the
// background cannot drift between kinds.
func mdWrapQuote(block markdownBlock, v View, cfg MarkdownCfg) View {
	if v == nil || block.BlockquoteDepth <= 0 {
		return v
	}
	leftMargin := float32(
		block.BlockquoteDepth-1) * cfg.Style.nestIndent
	return Row(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NewPadding(0, 0, 0, leftMargin),
		SizeBorder: NoBorder,
		Content: []View{
			Rectangle(RectangleCfg{
				Sizing: FixedFill,
				Width:  3,
				Color:  cfg.Style.blockquoteBorder,
			}),
			Column(ContainerCfg{
				Color:      cfg.Style.blockquoteBG,
				Sizing:     FillFit,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Content:    []View{v},
			}),
		},
	})
}

func mdRenderBlockquote(
	block markdownBlock, cfg MarkdownCfg, mode textMode,
	ctx *mdBlockCtx,
) View {
	rtfCfg := RTFCfg{
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.baseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	return mdWrapQuote(block, RTF(rtfCfg), cfg)
}

func mdRenderImage(block markdownBlock, cfg MarkdownCfg) View {
	if !mdImageAllowed(block.ImageSrc, cfg) {
		return mdWrapQuote(block, mdBlockedImage(block), cfg)
	}
	return mdWrapQuote(block, Image(ImageCfg{
		Src:    mdLowerHTTPScheme(block.ImageSrc),
		Width:  block.ImageWidth,
		Height: block.ImageHeight,
	}), cfg)
}

// mdImageAllowed applies the MarkdownCfg image policy (#1105). Data
// URLs and mem: sources do no I/O, so they always pass. An http(s)
// src needs AllowRemoteImages; anything else is a path on disk and
// needs AllowLocalImages. An empty src (the parser drops unsafe
// ones) passes, so Image shows its usual missing-source text.
func mdImageAllowed(src string, cfg MarkdownCfg) bool {
	switch {
	case src == "", isDataURL(src), isMemImage(src):
		return true
	case mdIsHTTPURL(src):
		return cfg.AllowRemoteImages
	default:
		return cfg.AllowLocalImages
	}
}

// mdIsHTTPURL is isHTTPURL without regard to case. The markdown parser
// and the web backend both accept "HTTPS://", so a case-sensitive test
// would file such a src under local paths, and AllowLocalImages alone
// would then let the browser fetch it (#1105).
func mdIsHTTPURL(src string) bool {
	return mdHasPrefixFold(src, "http://") ||
		mdHasPrefixFold(src, "https://")
}

// mdLowerHTTPScheme lowers an http(s) scheme written in another case.
// Image matches the scheme case-sensitively (isHTTPURL), so it would
// take "HTTPS://host/x.png" for a path on disk and stat it, though the
// policy passed it as remote (#1105). Only a mixed-case src allocates.
func mdLowerHTTPScheme(src string) string {
	if !mdIsHTTPURL(src) || isHTTPURL(src) {
		return src
	}
	i := strings.IndexByte(src, ':')
	return strings.ToLower(src[:i]) + src[i:]
}

// mdHasPrefixFold is strings.HasPrefix under ASCII case folding,
// without the allocation strings.ToLower would make.
func mdHasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) &&
		strings.EqualFold(s[:len(prefix)], prefix)
}

// mdBlockedImage stands in for an image the policy refused: a picture
// icon and the alt text, in the theme's secondary text role. It does
// not touch the src, so it starts no download and opens no file.
func mdBlockedImage(block markdownBlock) View {
	iconStyle := guiTheme.TextStyleIconXSmall
	iconStyle.Color = guiTheme.TextStyleSecondary.Color
	alt := block.ImageAlt
	if alt == "" {
		alt = "image"
	}
	return Row(ContainerCfg{
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		VAlign:     VAlignMiddle,
		Content: []View{
			Text(TextCfg{Text: IconPicture, TextStyle: iconStyle}),
			Text(TextCfg{
				Text:      alt,
				TextStyle: guiTheme.TextStyleSecondary,
			}),
		},
	})
}

// mdRenderHeading returns 1 or 2 views: an optional H1 spacer
// plus the heading container.
func mdRenderHeading(
	block markdownBlock, cfg MarkdownCfg, mode textMode,
	ctx *mdBlockCtx,
) []View {
	var views []View
	if block.HeaderLevel == 1 {
		views = append(views, Rectangle(RectangleCfg{
			Sizing: FillFixed,
			Height: 3,
		}))
	}
	rtfCfg := RTFCfg{
		// Scoped to the document: the slug is derived from heading text,
		// so two Markdown views showing the same heading would otherwise
		// claim one ID. Two identical headings in ONE document still
		// collide, but the duplicate audit now reports that rather than
		// letting it pass silently.
		ID:            ScopeID(cfg.ID, "h", block.AnchorSlug),
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.baseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	headingContent := []View{RTF(rtfCfg)}
	if (block.HeaderLevel == 1 && cfg.Style.h1Separator) ||
		(block.HeaderLevel == 2 && cfg.Style.h2Separator) {
		headingContent = append(headingContent,
			Separator(SeparatorCfg{
				Color: cfg.Style.hRColor,
			}))
	}
	views = append(views, Column(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		A11YRole:   AccessRoleHeading,
		a11Y:       &accessInfo{},
		Content:    headingContent,
	}))
	for i := range views {
		views[i] = mdWrapQuote(block, views[i], cfg)
	}
	return views
}

func mdRenderDefTerm(block markdownBlock, mode textMode, ctx *mdBlockCtx) View {
	rtfCfg := RTFCfg{
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.baseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	return RTF(rtfCfg)
}

func mdRenderDefValue(
	block markdownBlock, cfg MarkdownCfg, mode textMode, ctx *mdBlockCtx,
) View {
	rtfCfg := RTFCfg{
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.baseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	return mdWrapQuote(block, Row(ContainerCfg{
		Sizing:  FillFit,
		Padding: NewPadding(0, 0, 0, cfg.Style.nestIndent),
		Content: []View{RTF(rtfCfg)},
	}), cfg)
}

func mdRenderListItem(
	block markdownBlock, cfg MarkdownCfg, mode textMode,
	ctx *mdBlockCtx,
) View {
	indentW := float32(block.ListIndent) *
		cfg.Style.nestIndent

	var prefixW float32
	var prefixView View
	if block.IsTaskItem {
		boxSize := cfg.Style.Text.Size * 0.8
		prefixW = boxSize + 6
		prefixView = mdTaskCheckbox(block.TaskChecked, boxSize, cfg)
		if block.ListIndent > 0 {
			indentW += 4
		}
	} else {
		// Runes, not bytes: prefixCharWidth is a width per character,
		// and "• " is 4 bytes for 2 characters. The byte count used to
		// be corrected by halving it for that one prefix, which landed
		// on the right number for the bullet and the wrong one for
		// every other multi-byte marker.
		prefixW = float32(utf8RuneCount(block.ListPrefix)) *
			cfg.Style.prefixCharWidth
		if block.ListPrefix != "• " && block.ListIndent > 0 {
			indentW += 4
		}
		prefixView = Text(TextCfg{
			Text:      block.ListPrefix,
			TextStyle: cfg.Style.Text,
		})
	}

	rtfCfg := RTFCfg{
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.baseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	return mdWrapQuote(block, Row(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NewPadding(0, 0, 0, indentW),
		SizeBorder: NoBorder,
		Content: []View{
			// The marker column is a floor, not a cap. prefixW is
			// computed from a nominal character width, so a measurer
			// whose glyphs are wider makes the marker outgrow a Fixed
			// box and spill over the text beside it. Fit sizes the
			// column to the marker it actually holds, and MinWidth
			// keeps markers of different lengths aligned down the list.
			Column(ContainerCfg{
				Sizing:     FitFit,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				MinWidth:   prefixW,
				VAlign:     VAlignMiddle,
				Content:    []View{prefixView},
			}),
			Column(ContainerCfg{
				Sizing:     FillFit,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Content:    []View{RTF(rtfCfg)},
			}),
		},
	}), cfg)
}

// mdTaskCheckbox renders a fixed-size box for a GFM task-list item.
// Drawing a real box (rather than relying on Unicode ballot-box glyphs
// like ☑/☐) keeps checked/unchecked states pixel-identical regardless
// of platform font/glyph-fallback differences.
func mdTaskCheckbox(checked bool, boxSize float32, cfg MarkdownCfg) View {
	boxColor := ColorTransparent
	var content []View
	var amend func(EventCtx)
	if checked {
		// A fill under a white check, so the fill color. linkColor is
		// tuned to read as text on the page (issue #863).
		boxColor = guiTheme.ColorSelect
		checkStyle := guiTheme.TextStyleIconXSmall
		checkStyle.Size = boxSize * 1.1
		checkStyle.Color = White
		content = []View{
			Text(TextCfg{Text: IconCheck, TextStyle: checkStyle}),
		}
		// Same defect as Toggle's check: the icon font's advance box is
		// taller and wider than the mark, so flow centring leaves it high
		// and off to one side inside this small fixed box.
		amend = centerGlyphOnInk(IconCheck, checkStyle)
	}

	return Column(ContainerCfg{
		Sizing: FixedFixed,
		Width:  boxSize,
		Height: boxSize,
		Color:  boxColor,
		// No dedicated checkbox-border style field exists yet; HRColor
		// is reused as the neutral, low-emphasis border color other
		// MarkdownStyle divider/border colors already use.
		ColorBorder: cfg.Style.hRColor,
		// Fixed px, not BorderThin: the outline is the checkbox, so
		// it must survive WithBorders(false) (#867).
		SizeBorder:  BorderPx(sizeBorderDef),
		Radius:      RadiusSmall,
		Padding:     NoPadding,
		HAlign:      HAlignCenter,
		VAlign:      VAlignMiddle,
		AmendLayout: amend,
		Content:     content,
	})
}

func mdRenderParagraph(
	block markdownBlock, cfg MarkdownCfg, mode textMode,
	ctx *mdBlockCtx,
) View {
	rtfCfg := RTFCfg{
		Clip:          cfg.Clip,
		FocusSkip:     cfg.FocusSkip,
		Disabled:      cfg.Disabled,
		MinWidth:      cfg.MinWidth,
		Mode:          mode,
		RichText:      block.Content,
		BaseTextStyle: &block.baseStyle,
	}
	// No ID and no Focusable: a paragraph is one block of a document,
	// not the widget. The document's container claims cfg.ID and is
	// the focus target; the blocks are tied to it through
	// TC.MarkdownID (applyMdCtx), which is what selection keys on.
	// Giving every block cfg.ID collapsed N paragraphs onto one
	// identity.
	applyMdCtx(&rtfCfg, ctx)
	return RTF(rtfCfg)
}
