package gui

// goldenHTMLCases records Window.HTML (#1158). Kept out of
// golden_cases_test.go, which is past the file-size limit.
func goldenHTMLCases() []goldenCase {
	return []goldenCase{
		{
			// One tag of most kinds the subset supports, through the
			// Markdown renderer: heading, inline styles, a link, a
			// list, a quote and a rule. "*today*" and "#1" must record
			// as literal text.
			name: "html_subset",
			build: func(w *Window) View {
				return w.HTML(MarkdownCfg{
					ID: "html",
					Source: `<h2>Spring sale</h2>` +
						`<p>Save <b>50%</b> on <i>shoes</i> *today* #1 ` +
						`<a href="https://example.test">details</a></p>` +
						`<ul><li>Free <u>shipping</u></li><li>Easy returns</li></ul>` +
						`<blockquote>Best deal of the year</blockquote><hr>`,
				})
			},
		},
	}
}
