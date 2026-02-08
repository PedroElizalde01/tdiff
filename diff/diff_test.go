package diff

import "testing"

func TestParseUnified_PairsSimpleReplacement(t *testing.T) {
	input := "@@ -1 +1 @@\n-foo()\n+foo2()\n"
	rows, _ := ParseUnified(input)
	content := contentRows(rows)

	if len(content) != 1 {
		t.Fatalf("expected 1 content row, got %d", len(content))
	}
	if content[0].Old != "foo()" || content[0].New != "foo2()" {
		t.Fatalf("expected paired replacement, got old=%q new=%q", content[0].Old, content[0].New)
	}
	if content[0].OldNo == nil || content[0].NewNo == nil {
		t.Fatalf("expected both line numbers in paired replacement")
	}
}

func TestParseUnified_UnevenBlockKeepsMiddleDeletion(t *testing.T) {
	input := "@@ -1,3 +1,2 @@\n-A\n-B\n-C\n+A\n+C\n"
	rows, _ := ParseUnified(input)
	content := contentRows(rows)

	if len(content) != 3 {
		t.Fatalf("expected 3 content rows, got %d", len(content))
	}

	assertPair(t, content[0], "A", "A")
	assertDeletion(t, content[1], "B")
	assertPair(t, content[2], "C", "C")
}

func TestParseUnified_InsertOnly(t *testing.T) {
	input := "@@ -0,0 +1,2 @@\n+one\n+two\n"
	rows, _ := ParseUnified(input)
	content := contentRows(rows)

	if len(content) != 2 {
		t.Fatalf("expected 2 content rows, got %d", len(content))
	}

	assertAddition(t, content[0], "one")
	assertAddition(t, content[1], "two")
}

func TestParseUnified_DeleteOnly(t *testing.T) {
	input := "@@ -1,2 +0,0 @@\n-one\n-two\n"
	rows, _ := ParseUnified(input)
	content := contentRows(rows)

	if len(content) != 2 {
		t.Fatalf("expected 2 content rows, got %d", len(content))
	}

	assertDeletion(t, content[0], "one")
	assertDeletion(t, content[1], "two")
}

func TestParseUnified_RefactorLikePairsClosestLine(t *testing.T) {
	input := "@@ -1,2 +1,3 @@\n-count = normalize(oldCount)\n-legacyCleanup(tmp)\n+count = normalize(newCount)\n+count = normalize(newCount, true)\n+metrics.Inc()\n"
	rows, _ := ParseUnified(input)
	content := contentRows(rows)

	if len(content) != 4 {
		t.Fatalf("expected 4 content rows, got %d", len(content))
	}

	assertPair(t, content[0], "count = normalize(oldCount)", "count = normalize(newCount)")
	assertDeletion(t, content[1], "legacyCleanup(tmp)")
	assertAddition(t, content[2], "count = normalize(newCount, true)")
	assertAddition(t, content[3], "metrics.Inc()")
}

func contentRows(rows []Row) []Row {
	out := make([]Row, 0, len(rows))
	for _, row := range rows {
		if row.Kind == Meta || row.Kind == Hunk {
			continue
		}
		out = append(out, row)
	}
	return out
}

func assertPair(t *testing.T, row Row, oldText, newText string) {
	t.Helper()
	if row.Old != oldText || row.New != newText {
		t.Fatalf("expected pair old=%q new=%q, got old=%q new=%q", oldText, newText, row.Old, row.New)
	}
	if row.OldNo == nil || row.NewNo == nil {
		t.Fatalf("expected both line numbers for pair row")
	}
}

func assertDeletion(t *testing.T, row Row, oldText string) {
	t.Helper()
	if row.Old != oldText || row.New != "" {
		t.Fatalf("expected deletion old=%q, got old=%q new=%q", oldText, row.Old, row.New)
	}
	if row.OldNo == nil || row.NewNo != nil {
		t.Fatalf("expected deletion line numbers old!=nil new=nil")
	}
	if row.Kind != Del {
		t.Fatalf("expected Del kind, got %v", row.Kind)
	}
}

func assertAddition(t *testing.T, row Row, newText string) {
	t.Helper()
	if row.New != newText || row.Old != "" {
		t.Fatalf("expected addition new=%q, got old=%q new=%q", newText, row.Old, row.New)
	}
	if row.NewNo == nil || row.OldNo != nil {
		t.Fatalf("expected addition line numbers new!=nil old=nil")
	}
	if row.Kind != Add {
		t.Fatalf("expected Add kind, got %v", row.Kind)
	}
}
