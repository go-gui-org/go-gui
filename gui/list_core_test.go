package gui

import "testing"

// --- listCoreFuzzyScore ---

func TestFuzzyScoreEmptyQuery(t *testing.T) {
	if listCoreFuzzyScore("anything", "") != 0 {
		t.Error("empty query should return 0")
	}
}

func TestFuzzyScoreEmptyCandidate(t *testing.T) {
	if listCoreFuzzyScore("", "q") != -1 {
		t.Error("empty candidate with non-empty query should return -1")
	}
}

func TestFuzzyScoreExactMatch(t *testing.T) {
	s := listCoreFuzzyScore("hello", "hello")
	if s != 0 {
		t.Errorf("exact match should score 0, got %d", s)
	}
}

func TestFuzzyScorePrefixMatch(t *testing.T) {
	s := listCoreFuzzyScore("hello", "hel")
	if s != 0 {
		t.Errorf("prefix match should score 0, got %d", s)
	}
}

func TestFuzzyScoreGap(t *testing.T) {
	// "hlo" in "hello": h(0) → l(3) gap=2, l(3) → o(4) gap=0
	s := listCoreFuzzyScore("hello", "hlo")
	if s != 2 {
		t.Errorf("'hlo' in 'hello' should score 2, got %d", s)
	}
}

func TestFuzzyScoreNoMatch(t *testing.T) {
	if listCoreFuzzyScore("hello", "xyz") != -1 {
		t.Error("non-matching query should return -1")
	}
}

func TestFuzzyScoreCaseInsensitive(t *testing.T) {
	s := listCoreFuzzyScore("Hello", "hel")
	if s < 0 {
		t.Error("case-insensitive match should succeed")
	}
}

func TestFuzzyScorePartialQueryMatch(t *testing.T) {
	// Query chars not all found.
	if listCoreFuzzyScore("ab", "abc") != -1 {
		t.Error("query longer than matchable chars should return -1")
	}
}

// --- listCoreNavigate ---

func TestNavigateEmptyList(t *testing.T) {
	if listCoreNavigate(KeyDown, 0) != listCoreNone {
		t.Error("empty list should return none")
	}
}

func TestNavigateKeys(t *testing.T) {
	cases := []struct {
		key  KeyCode
		want listCoreAction
	}{
		{KeyUp, listCoreMoveUp},
		{KeyDown, listCoreMoveDown},
		{KeyEnter, listCoreSelectItem},
		{KeyEscape, listCoreDismiss},
		{KeyHome, listCoreFirst},
		{KeyEnd, listCoreLast},
		{KeyTab, listCoreNone},
	}
	for _, tc := range cases {
		got := listCoreNavigate(tc.key, 10)
		if got != tc.want {
			t.Errorf("key %d: got %d, want %d", tc.key, got, tc.want)
		}
	}
}

// --- listCoreApplyNav ---

func TestApplyNavMoveUp(t *testing.T) {
	next, changed := listCoreApplyNav(listCoreMoveUp, 3, 10)
	if next != 2 || !changed {
		t.Errorf("move up from 3: got %d, changed=%v", next, changed)
	}
}

func TestApplyNavMoveUpAtZero(t *testing.T) {
	next, changed := listCoreApplyNav(listCoreMoveUp, 0, 10)
	if next != 0 || changed {
		t.Errorf("move up at 0: got %d, changed=%v", next, changed)
	}
}

func TestApplyNavMoveDown(t *testing.T) {
	next, changed := listCoreApplyNav(listCoreMoveDown, 3, 10)
	if next != 4 || !changed {
		t.Errorf("move down from 3: got %d, changed=%v", next, changed)
	}
}

func TestApplyNavMoveDownAtEnd(t *testing.T) {
	next, changed := listCoreApplyNav(listCoreMoveDown, 9, 10)
	if next != 9 || changed {
		t.Errorf("move down at end: got %d, changed=%v", next, changed)
	}
}

func TestApplyNavFirst(t *testing.T) {
	next, changed := listCoreApplyNav(listCoreFirst, 5, 10)
	if next != 0 || !changed {
		t.Errorf("first: got %d, changed=%v", next, changed)
	}
}

func TestApplyNavLast(t *testing.T) {
	next, changed := listCoreApplyNav(listCoreLast, 0, 10)
	if next != 9 || !changed {
		t.Errorf("last: got %d, changed=%v", next, changed)
	}
}

func TestApplyNavNone(t *testing.T) {
	next, changed := listCoreApplyNav(listCoreNone, 3, 10)
	if next != 3 || changed {
		t.Errorf("none: got %d, changed=%v", next, changed)
	}
}

// --- listCoreVisibleRange ---

func TestVisibleRangeEmptyList(t *testing.T) {
	first, last := listCoreVisibleRange(0, 20, 100, 0)
	if first != 0 || last != -1 {
		t.Errorf("empty list: got %d,%d", first, last)
	}
}

func TestVisibleRangeZeroRowHeight(t *testing.T) {
	first, last := listCoreVisibleRange(10, 0, 100, 0)
	if first != 0 || last != -1 {
		t.Errorf("zero row height: got %d,%d", first, last)
	}
}

func TestVisibleRangeNoScroll(t *testing.T) {
	first, last := listCoreVisibleRange(100, 20, 100, 0)
	// visibleRows = int(100/20)+1 = 6, buf=2
	// first=0, firstVisible=0, lastVisible=min(99, 0+6+2)=8
	if first != 0 {
		t.Errorf("first: got %d, want 0", first)
	}
	if last != 8 {
		t.Errorf("last: got %d, want 8", last)
	}
}

func TestVisibleRangeScrolled(t *testing.T) {
	// Scroll 60px into 100 items at 20px each.
	first, last := listCoreVisibleRange(100, 20, 100, -60)
	// absScroll=60, first=int(60/20)=3, firstVisible=max(0,3-2)=1
	// lastVisible=min(99, 3+6+2)=11
	if first != 1 {
		t.Errorf("first: got %d, want 1", first)
	}
	if last != 11 {
		t.Errorf("last: got %d, want 11", last)
	}
}

func TestVisibleRangeSingleItem(t *testing.T) {
	first, last := listCoreVisibleRange(1, 20, 100, 0)
	if first != 0 || last != 0 {
		t.Errorf("single item: got %d,%d", first, last)
	}
}

// --- listCoreFilter (additional) ---

func TestFilterNoMatches(t *testing.T) {
	items := []listCoreItem{
		{ID: "a", Label: "Alpha"},
		{ID: "b", Label: "Beta"},
	}
	result := listCoreFilter(items, "xyz")
	if len(result) != 0 {
		t.Errorf("no matches: got %d indices", len(result))
	}
}

func TestFilterRankedByScore(t *testing.T) {
	items := []listCoreItem{
		{ID: "a", Label: "xyzab"}, // 'ab' at positions 3,4 → gap 0
		{ID: "b", Label: "a___b"}, // 'ab' at positions 0,4 → gap 3
		{ID: "c", Label: "ab"},    // exact → gap 0
	}
	result := listCoreFilter(items, "ab")
	if len(result) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(result))
	}
	// Items with score 0 should come before score 3.
	// "ab" (idx 2, score 0) and "xyzab" (idx 0, score 0) before
	// "a___b" (idx 1, score 3).
	if result[len(result)-1] != 1 {
		t.Errorf("worst score should be last: got %v", result)
	}
}

// --- listBoxNextSelectedIDs ---

func TestNextSelectedSingle(t *testing.T) {
	result := listBoxNextSelectedIDs([]string{"a"}, "b", false)
	if len(result) != 1 || result[0] != "b" {
		t.Errorf("single select: got %v", result)
	}
}

func TestNextSelectedMultiAdd(t *testing.T) {
	result := listBoxNextSelectedIDs([]string{"a"}, "b", true)
	if len(result) != 2 {
		t.Errorf("multi add: got %v", result)
	}
}

func TestNextSelectedMultiRemove(t *testing.T) {
	result := listBoxNextSelectedIDs([]string{"a", "b"}, "a", true)
	if len(result) != 1 || result[0] != "b" {
		t.Errorf("multi remove: got %v", result)
	}
}

// --- listCoreSelectedSet ---

func TestSelectedSetSmall(t *testing.T) {
	// < 2 items → nil map (uses linear scan)
	m := listCoreSelectedSet([]string{"a"})
	if m != nil {
		t.Error("single item should return nil map")
	}
}

func TestSelectedSetLarge(t *testing.T) {
	m := listCoreSelectedSet([]string{"a", "b", "c"})
	if m == nil || len(m) != 3 {
		t.Errorf("expected map of 3, got %v", m)
	}
}

// --- Fuzz ---

func FuzzListCoreFuzzyScore(f *testing.F) {
	f.Add("hello", "hel")
	f.Add("", "a")
	f.Add("abc", "")
	f.Add("Hello World", "hw")
	f.Fuzz(func(t *testing.T, candidate, query string) {
		score := listCoreFuzzyScore(candidate, query)
		if len(query) == 0 && score != 0 {
			t.Error("empty query must return 0")
		}
		if score < -1 {
			t.Errorf("score should be >= -1, got %d", score)
		}
	})
}
