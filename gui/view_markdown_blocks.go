package gui

// view_markdown_blocks.go contains block renderers for the Markdown view.

import (
	"fmt"
	"strconv"
	"time"

	"github.com/go-gui-org/go-gui/gui/markdown"
)

// renderMdMath renders a display math block.
func renderMdMath(
	block MarkdownBlock, cfg MarkdownCfg, w *Window,
) View {
	codeFallback := Column(ContainerCfg{
		Color:      cfg.Style.CodeBlockBG,
		Padding:    cfg.Style.CodeBlockPadding,
		Radius:     Some(cfg.Style.CodeBlockRadius),
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
		case DiagramLoading:
			return codeFallback
		case DiagramReady:
			return Image(ImageCfg{
				Src:    entry.PNGPath,
				Width:  entry.Width,
				Height: entry.Height,
			})
		case DiagramError:
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
				State:     DiagramLoading,
				RequestID: reqID,
			})
		fetchMathAsync(w, block.MathLatex, diagramHash,
			reqID, cfg.Style.MathDPIDisplay,
			cfg.Style.Text.Color, cfg.MathFetcher)
	}
	return codeFallback
}

// renderMdMermaid renders a mermaid diagram block.
func renderMdMermaid(
	block MarkdownBlock, cfg MarkdownCfg, w *Window,
) View {
	source := richTextPlain(block.Content)
	codeFallback := Column(ContainerCfg{
		Color:      cfg.Style.CodeBlockBG,
		Padding:    cfg.Style.CodeBlockPadding,
		Radius:     Some(cfg.Style.CodeBlockRadius),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content: []View{
			RTF(RtfCfg{
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
		case DiagramLoading:
			return Text(TextCfg{
				Text:      "Loading diagram...",
				TextStyle: cfg.Style.Text,
			})
		case DiagramReady:
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
				Src:     entry.PNGPath,
				Width:   imgW,
				Height:  imgH,
				BgColor: White,
			})
		case DiagramError:
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
				State:     DiagramLoading,
				RequestID: reqID,
			})
		fetchMermaidAsync(w, source, diagramHash, reqID,
			cfg.MermaidFetcher)
	}
	return codeFallback
}

// mdCopyButton builds a floating copy-to-clipboard button
// with a 2-second check-mark animation.
func mdCopyButton(
	animID string, w *Window,
	onClick func(*Layout, *Event, *Window),
) View {
	copied := w.hasAnimationLocked(animID)

	iconStyle := guiTheme.Icon5
	iconStyle.Color = Gray

	var btnContent []View
	if copied {
		checkStyle := iconStyle
		checkStyle.Color = Color{80, 200, 80, 255, true}
		btnContent = []View{
			Text(TextCfg{Text: IconCheck, TextStyle: checkStyle}),
		}
	} else {
		btnContent = []View{
			Text(TextCfg{Text: IconFile, TextStyle: iconStyle}),
		}
	}

	return Button(ButtonCfg{
		Float:        true,
		FloatAnchor:  FloatTopRight,
		FloatTieOff:  FloatTopRight,
		FloatOffsetX: -4,
		FloatOffsetY: 4,
		Radius:       SomeF(4),
		Color:        ColorTransparent,
		SizeBorder:   SomeF(0),
		Padding:      SomeP(2, 4, 2, 4),
		Content:      btnContent,
		OnClick:      onClick,
	})
}

// renderMdCode renders a fenced code block with a copy-to-clipboard button.
func renderMdCode(
	block MarkdownBlock, cfg MarkdownCfg, w *Window, blockIdx int,
) View {
	animID := "md_cp_" + strconv.Itoa(blockIdx)
	copyBtn := mdCopyButton(animID, w,
		func(_ *Layout, e *Event, w *Window) {
			plain := richTextPlain(block.Content)
			w.SetClipboard(plain)
			w.AnimationAdd(&Animate{
				AnimID:   animID,
				Delay:    2 * time.Second,
				Callback: func(*Animate, *Window) {},
			})
			e.IsHandled = true
		})

	return Column(ContainerCfg{
		Color:      cfg.Style.CodeBlockBG,
		Padding:    cfg.Style.CodeBlockPadding,
		Radius:     Some(cfg.Style.CodeBlockRadius),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Clip:       true,
		Content: []View{
			RTF(RtfCfg{
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
		Spacing:    Some(cfg.Style.BlockSpacing / 2),
		Content:    listItems,
	})
}

func mdRenderMathBlock(
	block MarkdownBlock, cfg MarkdownCfg, w *Window,
) View {
	return Column(ContainerCfg{
		Sizing:     FillFit,
		HAlign:     HAlignCenter,
		SizeBorder: NoBorder,
		Content: []View{
			renderMdMath(block, cfg, w),
		},
	})
}

func mdRenderCodeBlock(
	block MarkdownBlock, cfg MarkdownCfg, w *Window, idx int,
) View {
	if block.CodeLanguage == "mermaid" {
		return Column(ContainerCfg{
			Sizing:     FillFit,
			HAlign:     HAlignCenter,
			SizeBorder: NoBorder,
			Content: []View{
				renderMdMermaid(block, cfg, w),
			},
		})
	}
	return renderMdCode(block, cfg, w, idx)
}

func mdRenderTable(
	block MarkdownBlock, cfg MarkdownCfg, w *Window, idx int,
) View {
	if block.TableData == nil {
		return nil
	}
	return Column(ContainerCfg{
		Sizing:  FillFit,
		Padding: NoPadding,
		Clip:    true,
		Content: []View{
			w.Table(TableCfg{
				ID:               cfg.ID + ".table." + strconv.Itoa(idx),
				BorderStyle:      cfg.Style.TableBorderStyle,
				ColorBorder:      cfg.Style.TableBorderColor,
				SizeBorder:       cfg.Style.TableBorderSize,
				TextStyleHead:    cfg.Style.TableHeadStyle,
				TextStyle:        cfg.Style.TableCellStyle,
				CellPadding:      cfg.Style.TableCellPadding,
				ColorRowAlt:      cfg.Style.TableRowAlt,
				ColumnAlignments: block.TableData.Alignments,
				Data:             buildMarkdownTableData(*block.TableData, cfg.Style),
			}),
		},
	})
}

func mdRenderHR(cfg MarkdownCfg) View {
	return Rectangle(RectangleCfg{
		Sizing: FillFixed,
		Height: 1,
		Color:  cfg.Style.HRColor,
	})
}

func applyMdCtx(cfg *RtfCfg, ctx *mdBlockCtx) {
	if ctx != nil {
		cfg.markdownID = ctx.ID
		cfg.markdownBlockStart = ctx.Start
	}
}

func mdRenderBlockquote(
	block MarkdownBlock, cfg MarkdownCfg, mode TextMode,
	ctx *mdBlockCtx,
) View {
	leftMargin := float32(
		block.BlockquoteDepth-1) * cfg.Style.NestIndent
	rtfCfg := RtfCfg{
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.BaseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	return Row(ContainerCfg{
		Sizing:     FillFit,
		Padding:    SomeP(0, 0, 0, leftMargin),
		SizeBorder: NoBorder,
		Content: []View{
			Rectangle(RectangleCfg{
				Sizing: FixedFill,
				Width:  3,
				Color:  cfg.Style.BlockquoteBorder,
			}),
			Column(ContainerCfg{
				Color:      cfg.Style.BlockquoteBG,
				Sizing:     FillFit,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Content:    []View{RTF(rtfCfg)},
			}),
		},
	})
}

func mdRenderImage(block MarkdownBlock) View {
	return Image(ImageCfg{
		Src:    block.ImageSrc,
		Width:  block.ImageWidth,
		Height: block.ImageHeight,
	})
}

// mdRenderHeading returns 1 or 2 views: an optional H1 spacer
// plus the heading container.
func mdRenderHeading(
	block MarkdownBlock, cfg MarkdownCfg, mode TextMode,
	ctx *mdBlockCtx,
) []View {
	var views []View
	if block.HeaderLevel == 1 {
		views = append(views, Rectangle(RectangleCfg{
			Sizing: FillFixed,
			Height: 3,
		}))
	}
	rtfCfg := RtfCfg{
		ID:            block.AnchorSlug,
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.BaseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	headingContent := []View{RTF(rtfCfg)}
	if (block.HeaderLevel == 1 && cfg.Style.H1Separator) ||
		(block.HeaderLevel == 2 && cfg.Style.H2Separator) {
		headingContent = append(headingContent,
			Rectangle(RectangleCfg{
				Sizing: FillFixed,
				Height: 1,
				Color:  cfg.Style.HRColor,
			}))
	}
	views = append(views, Column(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		A11YRole:   AccessRoleHeading,
		A11Y:       &AccessInfo{},
		Content:    headingContent,
	}))
	return views
}

func mdRenderDefTerm(block MarkdownBlock, mode TextMode, ctx *mdBlockCtx) View {
	rtfCfg := RtfCfg{
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.BaseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	return RTF(rtfCfg)
}

func mdRenderDefValue(
	block MarkdownBlock, cfg MarkdownCfg, mode TextMode, ctx *mdBlockCtx,
) View {
	rtfCfg := RtfCfg{
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.BaseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	return Row(ContainerCfg{
		Sizing:  FillFit,
		Padding: SomeP(0, 0, 0, cfg.Style.NestIndent),
		Content: []View{RTF(rtfCfg)},
	})
}

func mdRenderListItem(
	block MarkdownBlock, cfg MarkdownCfg, mode TextMode,
	ctx *mdBlockCtx,
) View {
	indentW := float32(block.ListIndent) *
		cfg.Style.NestIndent
	prefixW := float32(len(block.ListPrefix)) *
		cfg.Style.PrefixCharWidth
	if block.ListPrefix == "• " {
		prefixW /= 2
	} else if block.ListIndent > 0 {
		indentW += 4
	}
	rtfCfg := RtfCfg{
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.BaseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	return Row(ContainerCfg{
		Sizing:     FillFit,
		Padding:    SomeP(0, 0, 0, indentW),
		SizeBorder: NoBorder,
		Content: []View{
			Column(ContainerCfg{
				Sizing:     FixedFit,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Width:      prefixW,
				Content: []View{
					Text(TextCfg{
						Text:      block.ListPrefix,
						TextStyle: cfg.Style.Text,
					}),
				},
			}),
			Column(ContainerCfg{
				Sizing:     FillFit,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Content:    []View{RTF(rtfCfg)},
			}),
		},
	})
}

func mdRenderParagraph(
	block MarkdownBlock, cfg MarkdownCfg, mode TextMode,
	ctx *mdBlockCtx,
) View {
	rtfCfg := RtfCfg{
		ID:            cfg.ID,
		Clip:          cfg.Clip,
		FocusSkip:     cfg.FocusSkip,
		Disabled:      cfg.Disabled,
		MinWidth:      cfg.MinWidth,
		Mode:          mode,
		RichText:      block.Content,
		BaseTextStyle: &block.BaseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
	if ctx == nil {
		rtfCfg.IDFocus = cfg.IDFocus
	}
	return RTF(rtfCfg)
}
