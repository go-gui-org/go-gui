package main

import (
	"testing"
)

// proc builds a *Process with the identity/tree fields the row logic needs.
func proc(pid, ppid int, name string) *Process {
	return &Process{ProcInfo: ProcInfo{PID: pid, PPID: ppid, Name: name}}
}

func byPID(a, b *Process) bool { return a.PID < b.PID }

// names extracts the display order for easy assertions.
func names(rows []*Process) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Name
	}
	return out
}

func TestTreeRowsHierarchy(t *testing.T) {
	t.Parallel()
	procs := []*Process{
		proc(1, 0, "init"),
		proc(10, 1, "shell"),
		proc(11, 10, "editor"),
		proc(12, 1, "daemon"),
	}
	rows := treeRows(procs, "", byPID, false)

	got := names(rows)
	want := []string{"init", "shell", "editor", "daemon"}
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rows = %v, want %v", got, want)
		}
	}

	depth := map[string]int{}
	kids := map[string]int{}
	for _, r := range rows {
		depth[r.Name] = r.TreeDepth
		kids[r.Name] = r.TreeChildCount
	}
	if depth["init"] != 0 || depth["shell"] != 1 || depth["editor"] != 2 || depth["daemon"] != 1 {
		t.Fatalf("unexpected depths: %v", depth)
	}
	if kids["init"] != 2 || kids["shell"] != 1 || kids["editor"] != 0 {
		t.Fatalf("unexpected child counts: %v", kids)
	}
}

func TestTreeRowsCollapseHidesSubtree(t *testing.T) {
	t.Parallel()
	shell := proc(10, 1, "shell")
	shell.Collapsed = true
	procs := []*Process{
		proc(1, 0, "init"),
		shell,
		proc(11, 10, "editor"), // hidden under collapsed shell
	}
	rows := treeRows(procs, "", byPID, false)
	for _, r := range rows {
		if r.Name == "editor" {
			t.Fatal("editor should be hidden under a collapsed parent")
		}
	}
}

func TestTreeRowsFilterKeepsAncestorsIgnoringCollapse(t *testing.T) {
	t.Parallel()
	shell := proc(10, 1, "shell")
	shell.Collapsed = true // must be ignored while filtering
	procs := []*Process{
		proc(1, 0, "init"),
		shell,
		proc(11, 10, "editor"),
		proc(12, 1, "unrelated"),
	}
	rows := treeRows(procs, "editor", byPID, false)

	got := names(rows)
	want := map[string]bool{"init": true, "shell": true, "editor": true}
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want ancestors of the match only", got)
	}
	for _, n := range got {
		if !want[n] {
			t.Fatalf("unexpected row %q in %v", n, got)
		}
	}
}

func TestVisibleRowsFlatFilterAndSort(t *testing.T) {
	t.Parallel()
	procs := []*Process{
		proc(3, 0, "cee"),
		proc(1, 0, "aye"),
		proc(2, 0, "bee"),
	}
	rows := visibleRows(procs, "", colDefs[colName].less, false, false)
	got := names(rows)
	want := []string{"aye", "bee", "cee"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sorted rows = %v, want %v", got, want)
		}
	}

	filtered := visibleRows(procs, "bee", nil, false, false)
	if len(filtered) != 1 || filtered[0].Name != "bee" {
		t.Fatalf("filtered rows = %v, want [bee]", names(filtered))
	}
}
