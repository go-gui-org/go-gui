package gui

import "testing"

// goldenTableRowHeaderCases records the row header marker restyled
// (#1156; the default look is table_row_header in golden_cases_test.go).
// The case copies the setup from the issue: a large
// TextStyle sets the row height, and RowHeaderTextStyle keeps the
// marker small and centred in the row. Run by its own test because
// golden_cases_test.go is past the file-size limit.
func goldenTableRowHeaderCases() []goldenCase {
	data := func() []TableRowCfg {
		return []TableRowCfg{
			TR([]TableCellCfg{TH("ID"), TH("Name")}),
			TR([]TableCellCfg{TD("1"), TD("Alice")}),
			TR([]TableCellCfg{TD("2"), TD("Bob")}),
		}
	}
	return []goldenCase{
		{
			name: "table_row_header_marker_style",
			build: func(*Window) View {
				body := guiTheme.TextStyleDef
				body.Size *= 1.7
				small := guiTheme.TextStyleDef
				small.Size *= 0.75
				return Table(TableCfg{
					ID:                 "tbl",
					RowHeader:          true,
					RowHeaderMarker:    ">",
					RowHeaderTextStyle: small,
					TextStyle:          body,
					BorderStyle:        TableBorderAll,
					SizeBorder:         1,
					CurrentRow:         Some(2),
					Data:               data(),
				})
			},
		},
	}
}

func TestGoldenTableRowHeader(t *testing.T) {
	for _, c := range goldenTableRowHeaderCases() {
		t.Run(c.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{
				State:  new(int),
				Width:  goldenWidth,
				Height: goldenHeight,
			})
			w.SetView(wrapGoldenRoot(c.build))
			w.TestGolden(t, GoldenCfg{Dir: "testdata", Name: c.name})
		})
	}
}
