package gui

// view_markdown_blocks.go contains block renderers for the Markdown view.

import (
	"fmt"
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
		Radius:     Some(cfg.Style.codeBlockRadius),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content: []View{
			Text(TextCfg{
				Text:      block.MathLatex,
				TextStyle: cfg.Style.Code,
			}),
		},
	})

	if cfg.disableExternalAPIs || !markdownExternalAPIsEnabled {
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
			cfg.Style.Text.Color, cfg.mathFetcher)
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
		Radius:     Some(cfg.Style.codeBlockRadius),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content: []View{
			RTF(RTFCfg{
				RichText: block.Content,
				Mode:     TextModeSingleLine,
			}),
		},
	})

	if cfg.disableExternalAPIs || !markdownExternalAPIsEnabled {
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
			mw := float32(cfg.mermaidWidth)
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
			cfg.mermaidFetcher)
	}
	return codeFallback
}

// mdCopyButton builds a floating copy-to-clipboard button
// with a 2-second check-mark animation.
func mdCopyButton(
	animID string, w *Window,
	onClick func(EventCtx),
) View {
	copied := w.hasAnimationLocked(animID)

	iconStyle := guiTheme.icon5
	iconStyle.Color = Gray

	var btnContent []View
	if copied {
		checkStyle := iconStyle
		checkStyle.Color = Color{80, 200, 80, 255, true}
		btnContent = []View{
			Text(TextCfg{Text: iconCheck, TextStyle: checkStyle}),
		}
	} else {
		btnContent = []View{
			Text(TextCfg{Text: iconFile, TextStyle: iconStyle}),
		}
	}

	return Button(ButtonCfg{
		// animID already identifies this code block uniquely (it keys
		// the check-mark animation), so it namespaces the button too.
		ID:           ScopeID(animID, "copy"),
		Float:        true,
		FloatAnchor:  FloatTopRight,
		FloatTieOff:  FloatTopRight,
		FloatOffsetX: -4,
		FloatOffsetY: 4,
		Radius:       SomeF(4),
		Color:        ColorTransparent,
		SizeBorder:   SomeF(0),
		Padding:      NewPadding(2, 4, 2, 4),
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
		Radius:     Some(cfg.Style.codeBlockRadius),
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
		Spacing:    Some(cfg.Style.blockSpacing / 2),
		Content:    listItems,
	})
}

func mdRenderMathBlock(
	block markdownBlock, cfg MarkdownCfg, w *Window,
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
	block markdownBlock, cfg MarkdownCfg, w *Window, idx int,
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
	block markdownBlock, cfg MarkdownCfg, w *Window, idx int,
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
				ID:               ScopeIDN(cfg.ID, "table", idx),
				BorderStyle:      cfg.Style.TableBorderStyle,
				ColorBorder:      cfg.Style.tableBorderColor,
				SizeBorder:       cfg.Style.tableBorderSize,
				TextStyleHead:    cfg.Style.tableHeadStyle,
				TextStyle:        cfg.Style.tableCellStyle,
				cellPadding:      cfg.Style.tableCellPadding,
				ColorRowAlt:      cfg.Style.tableRowAlt,
				columnAlignments: block.TableData.Alignments,
				Data:             buildMarkdownTableData(*block.TableData, cfg.Style),
			}),
		},
	})
}

func mdRenderHR(cfg MarkdownCfg) View {
	return Rectangle(RectangleCfg{
		Sizing: FillFixed,
		Height: 1,
		Color:  cfg.Style.hRColor,
	})
}

func applyMdCtx(cfg *RTFCfg, ctx *mdBlockCtx) {
	if ctx != nil {
		cfg.markdownID = ctx.ID
		cfg.markdownBlockStart = ctx.Start
	}
}

func mdRenderBlockquote(
	block markdownBlock, cfg MarkdownCfg, mode textMode,
	ctx *mdBlockCtx,
) View {
	leftMargin := float32(
		block.BlockquoteDepth-1) * cfg.Style.nestIndent
	rtfCfg := RTFCfg{
		RichText:      block.Content,
		Mode:          mode,
		BaseTextStyle: &block.baseStyle,
	}
	applyMdCtx(&rtfCfg, ctx)
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
				Content:    []View{RTF(rtfCfg)},
			}),
		},
	})
}

func mdRenderImage(block markdownBlock) View {
	return Image(ImageCfg{
		Src:    block.ImageSrc,
		Width:  block.ImageWidth,
		Height: block.ImageHeight,
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
			Rectangle(RectangleCfg{
				Sizing: FillFixed,
				Height: 1,
				Color:  cfg.Style.hRColor,
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
	return Row(ContainerCfg{
		Sizing:  FillFit,
		Padding: NewPadding(0, 0, 0, cfg.Style.nestIndent),
		Content: []View{RTF(rtfCfg)},
	})
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
		prefixW = float32(len(block.ListPrefix)) *
			cfg.Style.prefixCharWidth
		if block.ListPrefix == "• " {
			prefixW /= 2
		} else if block.ListIndent > 0 {
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
	return Row(ContainerCfg{
		Sizing:     FillFit,
		Padding:    NewPadding(0, 0, 0, indentW),
		SizeBorder: NoBorder,
		Content: []View{
			Column(ContainerCfg{
				Sizing:     FixedFit,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Width:      prefixW,
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
	})
}

// mdTaskCheckbox renders a fixed-size box for a GFM task-list item.
// Drawing a real box (rather than relying on Unicode ballot-box glyphs
// like ☑/☐) keeps checked/unchecked states pixel-identical regardless
// of platform font/glyph-fallback differences.
func mdTaskCheckbox(checked bool, boxSize float32, cfg MarkdownCfg) View {
	boxColor := ColorTransparent
	var content []View
	if checked {
		boxColor = cfg.Style.linkColor
		checkStyle := guiTheme.icon5
		checkStyle.Size = boxSize * 1.1
		checkStyle.Color = White
		content = []View{
			Text(TextCfg{Text: iconCheck, TextStyle: checkStyle}),
		}
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
		SizeBorder:  Some(float32(1)),
		Radius:      Some(float32(2)),
		Padding:     NoPadding,
		HAlign:      HAlignCenter,
		VAlign:      VAlignMiddle,
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
