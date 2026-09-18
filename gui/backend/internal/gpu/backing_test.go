package gpu

import "testing"

func TestRetainBackingSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		needW, needH int
		curW, curH   int
		wantW, wantH int
	}{
		{"empty store takes need", 100, 80, 0, 0, 100, 80},
		{"covering store kept", 100, 80, 200, 200, 200, 200},
		{"exact store kept", 100, 80, 100, 80, 100, 80},
		{"grows short side only", 400, 100, 100, 100, 400, 100},
		{"huge store snaps back", 100, 100, 4000, 4000, 100, 100},
		// The shrink test is strict: exactly 8x waste is kept,
		// one pixel column more snaps back.
		{"8x waste kept", 100, 100, 800, 100, 800, 100},
		{"past 8x snaps back", 100, 100, 801, 100, 100, 100},
		{"negative store takes need", 50, 40, -1, 90, 50, 40},
		{"need clamped to one", 0, -5, 0, 0, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotW, gotH := RetainBackingSize(
				tt.needW, tt.needH, tt.curW, tt.curH)
			if gotW != tt.wantW || gotH != tt.wantH {
				t.Errorf("RetainBackingSize(%d, %d, %d, %d) "+
					"= (%d, %d), want (%d, %d)",
					tt.needW, tt.needH, tt.curW, tt.curH,
					gotW, gotH, tt.wantW, tt.wantH)
			}
		})
	}
}
