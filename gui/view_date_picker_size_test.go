package gui

import (
	"testing"
	"time"
)

// A calendar month spans four to six week rows, but the grid always
// emits six: a short month's trailing rows are spacers (or
// adjacent-month days). Those cells have to reserve the same border as
// an in-month cell, or the picker resizes as the user pages through
// the year. Covers 36 consecutive months in both adjacent-day modes
// and every WeekdaysLen so a wider cellSize does not reintroduce
// drift.
func TestDatePickerHeightStableAcrossMonths(t *testing.T) {
	for _, adj := range []bool{false, true} {
		for _, wdLen := range []DatePickerWeekdayLen{
			WeekdayOneLetter, WeekdayThreeLetter, WeekdayFull,
		} {
			var want float32
			for m := 1; m <= 12; m++ {
				for _, y := range []int{2026, 2027, 2028} {
					w := NewWindow(WindowCfg{State: new(int), Width: 600, Height: 800})
					w.viewGenerator = func(win *Window) View {
						return Column(ContainerCfg{Sizing: FillFill,
							Content: []View{DatePicker(DatePickerCfg{
								ID: "dp", ShowAdjacentMonths: adj,
								WeekdaysLen: wdLen,
								Dates: []time.Time{
									time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC),
								},
							})}})
					}
					n := time.Date(y, time.Month(m), 1, 12, 0, 0, 0, time.UTC)
					w.setVirtualNow(&n)
					w.refreshLayout = true
					w.FrameFn()
					l, ok := w.layout.FindByID("dp")
					if !ok {
						t.Fatal("no picker")
					}
					if want == 0 {
						want = l.Shape.Height
					}
					if l.Shape.Height != want {
						t.Errorf("adj=%v wdLen=%v %d-%02d height=%v want %v",
							adj, wdLen, y, m, l.Shape.Height, want)
					}
				}
			}
			t.Logf("adj=%v wdLen=%v height=%v", adj, wdLen, want)
		}
	}
}
