// The process_monitor example is a small live task manager: a filterable
// process list with flat/tree views, sortable columns, and per-process
// CPU/RAM history charts. It is a functional port of the go-shirei
// process_monitor example, rebuilt on go-gui's immediate-mode pipeline and
// styled entirely from the standard theme tokens.
//
// Data is collected without any third-party dependency: on Unix by parsing
// `ps`, on Windows by parsing `tasklist`. See collect_unix.go /
// collect_windows.go. System memory totals come from sysmem_*.go.
package main

import "time"

// CPUPercentUnknown marks a process whose CPU could not be measured. It sorts
// below every real reading and renders as "--" rather than a fake 0.
const CPUPercentUnknown float64 = -1

// ProcInfo is one process as seen in a single sample. Fields that the OS
// refused to report are left zero with MetricsUnknown set, so the UI can show
// "--" instead of inventing data.
type ProcInfo struct {
	StartTime      time.Time
	Name           string
	Cmdline        string
	User           string
	State          string
	RSSBytes       uint64
	MemPercent     float64
	CPUPercent     float64 // CPUPercentUnknown when unreadable
	PID            int
	PPID           int
	Threads        int
	MetricsUnknown bool
}

// Snapshot is one full sweep of the process table plus system memory.
type Snapshot struct {
	Time             time.Time
	Processes        []ProcInfo
	TotalMemoryBytes uint64
	UsedMemoryBytes  uint64
}

// Collect takes one snapshot of the running processes. The platform-specific
// collectProcesses (collect_unix.go / collect_windows.go / collect_other.go)
// does the OS work; this wrapper fills in the memory totals and the derived
// per-process memory share.
func Collect() (*Snapshot, error) {
	procs, err := collectProcesses()
	if err != nil {
		return nil, err
	}

	total, used := systemMemory()
	snap := &Snapshot{
		Time:             time.Now(),
		Processes:        procs,
		TotalMemoryBytes: total,
		UsedMemoryBytes:  used,
	}

	// Derive each process's memory share now that the total is known.
	if total > 0 {
		for i := range snap.Processes {
			p := &snap.Processes[i]
			if !p.MetricsUnknown {
				p.MemPercent = float64(p.RSSBytes) / float64(total) * 100
			}
		}
	}
	return snap, nil
}
