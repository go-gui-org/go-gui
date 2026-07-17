package main

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	maxHistoryPoints = 240
	keepStoppedFor   = 60 * time.Second
)

// ProcessKey identifies a process across samples. Including StartTime guards
// against PID reuse; our collectors do not populate StartTime yet, so in
// practice the key is PID-only. Documented limitation (see README).
type ProcessKey struct {
	StartTime time.Time
	PID       int
}

// ProcessPoint is one history sample for the charts.
type ProcessPoint struct {
	Time       time.Time
	CPUPercent float64
	RSSBytes   uint64
}

// Process is a stable, store-owned view of a process: the latest sample plus
// rolling history and tree-view scratch. Table rows bind to *Process pointers
// so selection and history survive across refreshes.
type Process struct {
	ProcInfo
	Key       ProcessKey
	LastSeen  time.Time
	StoppedAt time.Time
	History   []ProcessPoint

	// Collapsed hides this process's children in tree view.
	Collapsed bool

	// Tree scratch, recomputed each time visible rows are built.
	TreeDepth      int
	TreeChildCount int
}

// Running reports whether the process was present in the latest sample.
func (p *Process) Running() bool { return p.StoppedAt.IsZero() }

// appendHistory pushes one point, capping the ring buffer at maxHistoryPoints.
func (p *Process) appendHistory(t time.Time, cpu float64, rss uint64) {
	p.History = append(p.History, ProcessPoint{Time: t, CPUPercent: cpu, RSSBytes: rss})
	if len(p.History) > maxHistoryPoints {
		copy(p.History, p.History[len(p.History)-maxHistoryPoints:])
		p.History = p.History[:maxHistoryPoints]
	}
}

// ProcessStore keeps a stable *Process per identity and ages out processes
// that have exited.
type ProcessStore struct {
	ByKey map[ProcessKey]*Process
}

// NewProcessStore returns an empty store.
func NewProcessStore() *ProcessStore {
	return &ProcessStore{ByKey: make(map[ProcessKey]*Process)}
}

func processKey(info ProcInfo) ProcessKey {
	return ProcessKey{PID: info.PID, StartTime: info.StartTime}
}

// Update folds a snapshot into the store: refresh live processes, append
// history, and mark then eventually evict processes that have disappeared.
// selected is never evicted so its detail/charts stay visible.
func (s *ProcessStore) Update(snap *Snapshot, selected *Process) {
	if snap == nil {
		return
	}
	if s.ByKey == nil {
		s.ByKey = make(map[ProcessKey]*Process)
	}

	seen := make(map[ProcessKey]bool, len(snap.Processes))
	for _, info := range snap.Processes {
		key := processKey(info)
		p := s.ByKey[key]
		if p == nil {
			p = &Process{Key: key}
			s.ByKey[key] = p
		}
		p.ProcInfo = info
		p.LastSeen = snap.Time
		p.StoppedAt = time.Time{}
		// Unknown CPU charts as a flat 0 line; the row label carries the "--".
		p.appendHistory(snap.Time, max(info.CPUPercent, 0), info.RSSBytes)
		seen[key] = true
	}

	for key, p := range s.ByKey {
		if seen[key] {
			continue
		}
		if p.StoppedAt.IsZero() {
			p.StoppedAt = snap.Time
			p.State = "x"
			p.CPUPercent = 0
			p.appendHistory(snap.Time, 0, p.RSSBytes)
		}
		if p != selected && snap.Time.Sub(p.StoppedAt) > keepStoppedFor {
			delete(s.ByKey, key)
		}
	}
}

// Processes returns a snapshot slice of every tracked process (random order).
func (s *ProcessStore) Processes() []*Process {
	if s == nil {
		return nil
	}
	out := make([]*Process, 0, len(s.ByKey))
	for _, p := range s.ByKey {
		out = append(out, p)
	}
	return out
}

// ActiveCount returns the number of currently-running processes.
func (s *ProcessStore) ActiveCount() int {
	if s == nil {
		return 0
	}
	var n int
	for _, p := range s.ByKey {
		if p.Running() {
			n++
		}
	}
	return n
}

// visibleRows applies the filter and ordering (or tree flattening) to produce
// the ordered rows the table displays.
func visibleRows(procs []*Process, filter string, less func(a, b *Process) bool, desc, tree bool) []*Process {
	needle := strings.ToLower(strings.TrimSpace(filter))
	if tree {
		return treeRows(procs, needle, less, desc)
	}
	rows := make([]*Process, 0, len(procs))
	for _, p := range procs {
		if needle == "" || processMatches(p, needle) {
			rows = append(rows, p)
		}
	}
	orderProcesses(rows, less, desc)
	return rows
}

// orderProcesses sorts in place by the column comparator, honoring direction.
// A nil comparator keeps the given order.
func orderProcesses(procs []*Process, less func(a, b *Process) bool, desc bool) {
	if less == nil {
		return
	}
	sort.SliceStable(procs, func(i, j int) bool {
		if desc {
			return less(procs[j], procs[i])
		}
		return less(procs[i], procs[j])
	})
}

// treeRows arranges processes as a PPID forest flattened depth-first. Sibling
// order follows the active sort. Collapsed subtrees are hidden. When a filter
// is active only matching processes and their ancestors are shown, and
// collapse state is ignored so matches stay visible.
func treeRows(procs []*Process, needle string, less func(a, b *Process) bool, desc bool) []*Process {
	byPID := make(map[int]*Process, len(procs))
	for _, p := range procs {
		byPID[p.PID] = p
	}

	parentOf := func(p *Process) *Process {
		parent, ok := byPID[p.PPID]
		if !ok || parent == p {
			return nil
		}
		return parent
	}

	children := make(map[int][]*Process)
	var roots []*Process
	for _, p := range procs {
		if parent := parentOf(p); parent != nil {
			children[parent.PID] = append(children[parent.PID], p)
		} else {
			roots = append(roots, p)
		}
	}

	// Filtering: include matches plus their ancestor chain.
	var visible map[*Process]bool
	if needle != "" {
		visible = make(map[*Process]bool)
		for _, p := range procs {
			if !processMatches(p, needle) {
				continue
			}
			for cur := p; cur != nil && !visible[cur]; cur = parentOf(cur) {
				visible[cur] = true
			}
		}
	}

	shownChildren := func(p *Process) []*Process {
		kids := children[p.PID]
		if visible == nil {
			return kids
		}
		var out []*Process
		for _, c := range kids {
			if visible[c] {
				out = append(out, c)
			}
		}
		return out
	}

	orderProcesses(roots, less, desc)

	var rows []*Process
	seen := make(map[*Process]bool)
	var walk func(p *Process, depth int)
	walk = func(p *Process, depth int) {
		if seen[p] {
			return // cycle guard
		}
		seen[p] = true

		kids := shownChildren(p)
		orderProcesses(kids, less, desc)
		p.TreeDepth = depth
		p.TreeChildCount = len(kids)
		rows = append(rows, p)

		expanded := !p.Collapsed || visible != nil
		if expanded {
			for _, c := range kids {
				walk(c, depth+1)
			}
		}
	}
	for _, r := range roots {
		if visible != nil && !visible[r] {
			continue
		}
		walk(r, 0)
	}
	return rows
}

// processMatches reports whether the process matches the lowercased needle in
// name, command line, user, or PID.
func processMatches(p *Process, needle string) bool {
	return strings.Contains(strings.ToLower(p.Name), needle) ||
		strings.Contains(strings.ToLower(p.Cmdline), needle) ||
		strings.Contains(strings.ToLower(p.User), needle) ||
		strings.Contains(strconv.Itoa(p.PID), needle)
}
