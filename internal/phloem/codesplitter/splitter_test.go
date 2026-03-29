package codesplitter

import (
	"testing"
)

func TestSplitToL3_StripsFencesAndSplitsLines(t *testing.T) {
	in := "```mermaid\nsequenceDiagram\n    A->>B: hi\n```"
	got := SplitToL3(in, "mermaid")
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d: %#v", len(got), got)
	}
	if got[0] != "sequenceDiagram" {
		t.Fatalf("got[0]=%q", got[0])
	}
	if got[1] != "    A->>B: hi" {
		t.Fatalf("got[1]=%q", got[1])
	}
}

func TestSplitToL3_Empty(t *testing.T) {
	if SplitToL3("", "go") != nil {
		t.Fatal("expected nil")
	}
	if SplitToL3("   \n  \n", "go") != nil {
		t.Fatal("expected nil")
	}
}

func TestSplitToL3_NoFences(t *testing.T) {
	got := SplitToL3("line1\n\nline2", "")
	if len(got) != 2 || got[0] != "line1" || got[1] != "line2" {
		t.Fatalf("got %#v", got)
	}
}
