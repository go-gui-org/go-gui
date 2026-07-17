package main

import (
	"testing"
	"time"
)

func snapAt(t time.Time, procs ...ProcInfo) *Snapshot {
	return &Snapshot{Time: t, Processes: procs}
}

func TestStoreUpdateAddsAndCounts(t *testing.T) {
	t.Parallel()
	s := NewProcessStore()
	t0 := time.Unix(1_700_000_000, 0)
	s.Update(snapAt(t0,
		ProcInfo{PID: 1, Name: "init"},
		ProcInfo{PID: 2, Name: "app"},
	), nil)

	if got := len(s.ByKey); got != 2 {
		t.Fatalf("ByKey len = %d, want 2", got)
	}
	if got := s.ActiveCount(); got != 2 {
		t.Fatalf("ActiveCount = %d, want 2", got)
	}
}

func TestStoreStablePointerAcrossUpdates(t *testing.T) {
	t.Parallel()
	s := NewProcessStore()
	t0 := time.Unix(1_700_000_000, 0)
	s.Update(snapAt(t0, ProcInfo{PID: 7, Name: "svc", RSSBytes: 100}), nil)
	first := s.ByKey[ProcessKey{PID: 7}]

	s.Update(snapAt(t0.Add(time.Second), ProcInfo{PID: 7, Name: "svc", RSSBytes: 200}), nil)
	second := s.ByKey[ProcessKey{PID: 7}]

	if first != second {
		t.Fatal("expected the same *Process across updates for a reused PID")
	}
	if second.RSSBytes != 200 {
		t.Fatalf("RSSBytes = %d, want 200 (latest sample)", second.RSSBytes)
	}
	if len(second.History) != 2 {
		t.Fatalf("history len = %d, want 2", len(second.History))
	}
}

func TestStoreMarksAndEvictsStopped(t *testing.T) {
	t.Parallel()
	s := NewProcessStore()
	t0 := time.Unix(1_700_000_000, 0)
	s.Update(snapAt(t0, ProcInfo{PID: 9, Name: "gone"}), nil)

	// Process disappears: first missing sample marks it stopped, not evicted.
	s.Update(snapAt(t0.Add(time.Second)), nil)
	p := s.ByKey[ProcessKey{PID: 9}]
	if p == nil {
		t.Fatal("stopped process evicted too early")
	}
	if p.Running() {
		t.Fatal("process should be marked stopped")
	}

	// After the keep window elapses, it is evicted.
	s.Update(snapAt(t0.Add(keepStoppedFor+2*time.Second)), nil)
	if _, ok := s.ByKey[ProcessKey{PID: 9}]; ok {
		t.Fatal("stopped process should have been evicted after keepStoppedFor")
	}
}

func TestStoreKeepsSelectedStopped(t *testing.T) {
	t.Parallel()
	s := NewProcessStore()
	t0 := time.Unix(1_700_000_000, 0)
	s.Update(snapAt(t0, ProcInfo{PID: 9, Name: "gone"}), nil)
	selected := s.ByKey[ProcessKey{PID: 9}]

	s.Update(snapAt(t0.Add(time.Second)), selected)
	s.Update(snapAt(t0.Add(keepStoppedFor+2*time.Second)), selected)

	if _, ok := s.ByKey[ProcessKey{PID: 9}]; !ok {
		t.Fatal("selected stopped process must not be evicted")
	}
}

func TestHistoryRingBufferCap(t *testing.T) {
	t.Parallel()
	p := &Process{}
	base := time.Unix(1_700_000_000, 0)
	for i := range maxHistoryPoints + 60 {
		p.appendHistory(base.Add(time.Duration(i)*time.Second), float64(i), uint64(i))
	}
	if len(p.History) != maxHistoryPoints {
		t.Fatalf("history len = %d, want %d", len(p.History), maxHistoryPoints)
	}
	// The oldest points should have been dropped; the last point is the newest.
	last := p.History[len(p.History)-1]
	if last.CPUPercent != float64(maxHistoryPoints+59) {
		t.Fatalf("newest CPU = %v, want %v", last.CPUPercent, float64(maxHistoryPoints+59))
	}
}
