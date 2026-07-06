package gui

import (
	"math"
	"testing"
)

func TestTermGridPanicsOnZeroCols(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on zero Cols")
		}
	}()
	TermGrid(TermGridCfg{Cols: 0, Rows: 1, CellW: 8, CellH: 16})
}

func TestTermGridPanicsOnZeroRows(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on zero Rows")
		}
	}()
	TermGrid(TermGridCfg{Cols: 1, Rows: 0, CellW: 8, CellH: 16})
}

func TestTermGridPanicsOnZeroCellW(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on zero CellW")
		}
	}()
	TermGrid(TermGridCfg{Cols: 1, Rows: 1, CellW: 0, CellH: 16})
}

func TestTermGridPanicsOnZeroCellH(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on zero CellH")
		}
	}()
	TermGrid(TermGridCfg{Cols: 1, Rows: 1, CellW: 8, CellH: 0})
}

func TestTermGridPanicsOnNaNCellW(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on NaN CellW")
		}
	}()
	TermGrid(TermGridCfg{
		Cols: 1, Rows: 1, CellW: float32(math.NaN()), CellH: 16,
	})
}

func TestTermGridPanicsOnNaNCellH(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on NaN CellH")
		}
	}()
	TermGrid(TermGridCfg{
		Cols: 1, Rows: 1, CellW: 8, CellH: float32(math.NaN()),
	})
}

func TestTermGridPanicsOnInfCellW(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on Inf CellW")
		}
	}()
	TermGrid(TermGridCfg{
		Cols: 1, Rows: 1, CellW: float32(math.Inf(1)), CellH: 16,
	})
}

func TestTermGridPanicsOnInfCellH(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on Inf CellH")
		}
	}()
	TermGrid(TermGridCfg{
		Cols: 1, Rows: 1, CellW: 8, CellH: float32(math.Inf(1)),
	})
}

func TestTermGridPanicsOnInsufficientCells(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on insufficient Cells")
		}
	}()
	TermGrid(TermGridCfg{
		Cols: 4, Rows: 2, CellW: 8, CellH: 16,
		Cells: make([]TermCell, 7), // need 8
	})
}

func TestTermGridLayoutWiresEventHandlers(t *testing.T) {
	w := makeWindowWithScratch()
	keyDownFired := false
	clickFired := false
	scrollFired := false
	v := TermGrid(TermGridCfg{
		Cols: 4, Rows: 2, CellW: 8, CellH: 16,
		Cells: termCells(4, 2),
		OnKeyDown: func(_ *Layout, _ *Event, _ *Window) {
			keyDownFired = true
		},
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			clickFired = true
		},
		OnMouseScroll: func(_ *Layout, _ *Event, _ *Window) {
			scrollFired = true
		},
	})
	l := v.GenerateLayout(w)
	s := l.Shape
	if s.events == nil {
		t.Fatal("events is nil, expected event handlers")
	}
	if s.events.OnKeyDown == nil {
		t.Error("OnKeyDown not wired")
	}
	if s.events.OnClick == nil {
		t.Error("OnClick not wired")
	}
	if s.events.OnMouseScroll == nil {
		t.Error("OnMouseScroll not wired")
	}
	s.events.OnKeyDown(nil, nil, nil)
	s.events.OnClick(nil, nil, nil)
	s.events.OnMouseScroll(nil, nil, nil)
	if !keyDownFired {
		t.Error("OnKeyDown callback did not fire")
	}
	if !clickFired {
		t.Error("OnClick callback did not fire")
	}
	if !scrollFired {
		t.Error("OnMouseScroll callback did not fire")
	}
}

func TestTermGridLayoutNoEventsWhenNoCallbacks(t *testing.T) {
	w := makeWindowWithScratch()
	v := TermGrid(TermGridCfg{
		Cols: 4, Rows: 2, CellW: 8, CellH: 16,
		Cells: termCells(4, 2),
	})
	l := v.GenerateLayout(w)
	if l.Shape.events != nil {
		t.Error("expected nil events when no callbacks set")
	}
}

func TestTermGridLayoutIDFocusSetsA11YRole(t *testing.T) {
	w := makeWindowWithScratch()
	v := TermGrid(TermGridCfg{
		Cols: 4, Rows: 2, CellW: 8, CellH: 16,
		Cells:   termCells(4, 2),
		IDFocus: 1,
	})
	l := v.GenerateLayout(w)
	if l.Shape.A11YRole != AccessRoleTextArea {
		t.Errorf("A11YRole = %v, want AccessRoleTextArea", l.Shape.A11YRole)
	}
}

func TestTermGridLayoutDefaultA11YRoleIsImage(t *testing.T) {
	w := makeWindowWithScratch()
	v := TermGrid(TermGridCfg{
		Cols: 4, Rows: 2, CellW: 8, CellH: 16,
		Cells: termCells(4, 2),
	})
	l := v.GenerateLayout(w)
	if l.Shape.A11YRole != AccessRoleImage {
		t.Errorf("A11YRole = %v, want AccessRoleImage", l.Shape.A11YRole)
	}
}

func TestTermGridLayoutCustomSizing(t *testing.T) {
	w := makeWindowWithScratch()
	v := TermGrid(TermGridCfg{
		Cols: 4, Rows: 2, CellW: 8, CellH: 16,
		Cells:  termCells(4, 2),
		Sizing: FillFill,
	})
	l := v.GenerateLayout(w)
	if l.Shape.Sizing != FillFill {
		t.Errorf("Sizing = %v, want FillFill", l.Shape.Sizing)
	}
}

func TestTermGridLayoutPassesTextStyle(t *testing.T) {
	w := makeWindowWithScratch()
	ts := TextStyle{Size: 14}
	v := TermGrid(TermGridCfg{
		Cols: 4, Rows: 2,
		CellW: 8, CellH: 16,
		Cells:     termCells(4, 2),
		TextStyle: ts,
	})
	l := v.GenerateLayout(w)
	if l.Shape.tg == nil {
		t.Fatal("shape.tg is nil")
	}
	if l.Shape.tg.Style.Size != 14 {
		t.Errorf("TermGridData.Style.Size = %v, want 14",
			l.Shape.tg.Style.Size)
	}
}
