//go:build windows

package main

import (
	"os"
	"testing"
	"time"
)

func TestProcessStartTimeCurrentProcess(t *testing.T) {
	t.Parallel()
	got := processStartTime(os.Getpid())
	if got.IsZero() {
		t.Fatal("expected a start time for the current process")
	}
	age := time.Since(got)
	if age < 0 || age > 24*time.Hour {
		t.Fatalf("start time %v (age %v) is implausible", got, age)
	}
}

func TestProcessStartTimeBogusPID(t *testing.T) {
	t.Parallel()
	if got := processStartTime(1 << 30); !got.IsZero() {
		t.Fatalf("expected zero time for a bogus PID, got %v", got)
	}
}
