package main

import (
	"testing"
	"time"
)

func cpuOf(pt ProcessPoint) float64 { return pt.CPUPercent }

func TestResampleHistoryEmpty(t *testing.T) {
	t.Parallel()
	buckets := resampleHistory(nil, 5*time.Second, time.Second, cpuOf)
	if len(buckets) != 5 {
		t.Fatalf("bucket count = %d, want 5", len(buckets))
	}
	for i, b := range buckets {
		if b.HasData {
			t.Fatalf("bucket %d should have no data for empty history", i)
		}
	}
}

func TestResampleHistoryBucketsAndInterpolation(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0)
	hist := []ProcessPoint{
		{Time: base, CPUPercent: 0},
		{Time: base.Add(2 * time.Second), CPUPercent: 20},
		{Time: base.Add(4 * time.Second), CPUPercent: 40},
	}
	buckets := resampleHistory(hist, 5*time.Second, time.Second, cpuOf)
	if len(buckets) != 5 {
		t.Fatalf("bucket count = %d, want 5", len(buckets))
	}

	// Real samples land in slots 0, 2, 4; the gaps at 1 and 3 are interpolated.
	want := []struct {
		value  float64
		data   bool
		interp bool
	}{
		{0, true, false},
		{10, true, true},
		{20, true, false},
		{30, true, true},
		{40, true, false},
	}
	for i, w := range want {
		b := buckets[i]
		if b.HasData != w.data || b.Interpolated != w.interp || b.Value != w.value {
			t.Fatalf("bucket %d = %+v, want value=%v data=%v interp=%v",
				i, b, w.value, w.data, w.interp)
		}
	}
}

func TestResampleHistoryNoInterpolationBeforeFirstSample(t *testing.T) {
	t.Parallel()
	base := time.Unix(1_700_000_000, 0)
	// A single sample in the newest slot; older slots must stay empty.
	hist := []ProcessPoint{{Time: base.Add(4 * time.Second), CPUPercent: 50}}
	buckets := resampleHistory(hist, 5*time.Second, time.Second, cpuOf)
	for i := range 4 {
		if buckets[i].HasData {
			t.Fatalf("bucket %d should be empty before the first real sample", i)
		}
	}
	if !buckets[4].HasData || buckets[4].Value != 50 {
		t.Fatalf("newest bucket = %+v, want value=50 data=true", buckets[4])
	}
}

func TestRamHistoryScale(t *testing.T) {
	t.Parallel()
	const step = 500 * 1024 * 1024
	cases := []struct {
		maxRSS uint64
		want   float64
	}{
		{0, step},
		{300 * 1024 * 1024, step},
		{600 * 1024 * 1024, 2 * step},
		{step, step},
	}
	for _, c := range cases {
		hist := []ProcessPoint{{RSSBytes: c.maxRSS}}
		if got := ramHistoryScale(hist); got != c.want {
			t.Fatalf("ramHistoryScale(maxRSS=%d) = %v, want %v", c.maxRSS, got, c.want)
		}
	}
}
