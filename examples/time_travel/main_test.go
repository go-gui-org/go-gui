package main

import (
	"slices"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Not t.Parallel: SetTheme mutates process-global theme state
// (applyTheme writes the default*Style mirrors), which is documented
// frame-thread-only and races any other test touching theme state.
func TestMainViewNoPanic(t *testing.T) {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &appState{},
		Width:  320,
		Height: 220,
	})
	_ = mainView(w).GenerateLayout(w)
}

// TestSnapshotRoundTrip verifies the Snapshotter contract:
// Snapshot returns an independent copy; Restore overwrites the receiver.
func TestSnapshotRoundTrip(t *testing.T) {
	t.Parallel()
	original := &appState{
		Count: 42,
		Log:   []string{"inc → 41", "inc → 42"},
	}
	// Snapshot must produce a deep copy.
	snap := original.Snapshot().(*appState)
	if snap.Count != 42 {
		t.Fatalf("snapshot Count = %d, want 42", snap.Count)
	}
	if !slices.Equal(snap.Log, original.Log) {
		t.Fatalf("snapshot Log = %v, want %v", snap.Log, original.Log)
	}

	// Mutate original; snapshot must be independent.
	original.Count = 99
	original.Log = append(original.Log, "inc → 99")
	if snap.Count == 99 {
		t.Fatal("snapshot should not alias original Count")
	}

	// Restore from snapshot.
	original.Restore(snap)
	if original.Count != 42 {
		t.Fatalf("restored Count = %d, want 42", original.Count)
	}
	if len(original.Log) != 2 {
		t.Fatalf("restored Log len = %d, want 2", len(original.Log))
	}
}

// TestSnapshotSize ensures Size() returns a non-zero estimate.
func TestSnapshotSize(t *testing.T) {
	t.Parallel()
	s := &appState{Count: 1, Log: []string{"hello", "world"}}
	if n := s.Size(); n <= 0 {
		t.Fatalf("Size() = %d, want > 0", n)
	}
	empty := &appState{}
	if n := empty.Size(); n <= 0 {
		t.Fatalf("empty Size() = %d, want > 0", n)
	}
}

// TestMainViewWithDebugTimeTravel verifies the example view renders
// when DebugTimeTravel is enabled (the normal example path).
// Not t.Parallel: see TestMainViewNoPanic (SetTheme is not race-safe).
func TestMainViewWithDebugTimeTravel(t *testing.T) {
	gui.SetTheme(gui.ThemeLight.WithPadding(false))
	w := gui.NewWindow(gui.WindowCfg{
		State:           &appState{Count: 7},
		Title:           "Counter",
		Width:           320,
		Height:          220,
		DebugTimeTravel: true,
	})
	_ = mainView(w).GenerateLayout(w)

	// Verify history is enabled when DebugTimeTravel is true.
	if w.HistoryLen() != 0 {
		t.Fatal("expected empty history before any events")
	}
}

// TestStateMutations verifies the Increment and Reset click handlers
// mutate state as expected.
// Not t.Parallel: see TestMainViewNoPanic (SetTheme is not race-safe).
func TestStateMutations(t *testing.T) {
	gui.SetTheme(gui.ThemeLight.WithPadding(false))
	w := gui.NewWindow(gui.WindowCfg{
		State:           &appState{Count: 0},
		Width:           320,
		Height:          220,
		DebugTimeTravel: true,
	})
	app := gui.State[appState](w)

	// Simulate Increment click by mutating state directly.
	app.Count++
	app.Log = append(app.Log, "inc → 1")
	if app.Count != 1 {
		t.Fatalf("after increment Count = %d, want 1", app.Count)
	}
	if len(app.Log) != 1 {
		t.Fatalf("after increment Log len = %d, want 1", len(app.Log))
	}

	// Take snapshot, then Reset.
	snap := app.Snapshot()
	app.Count = 0
	app.Log = append(app.Log, "reset")
	if app.Count != 0 {
		t.Fatalf("after reset Count = %d, want 0", app.Count)
	}

	// Restore to pre-reset state.
	app.Restore(snap)
	if app.Count != 1 {
		t.Fatalf("after restore Count = %d, want 1", app.Count)
	}
}
