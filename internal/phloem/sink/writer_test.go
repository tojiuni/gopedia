package sink

import (
	"testing"
)

func TestSplitSentencesEnglish(t *testing.T) {
	in := "# Title\n\nHello world. This is a test!\n\nLine two?\n"
	got := splitSentencesEnglish(stripMarkdownHeadings(in))
	if len(got) != 3 {
		t.Fatalf("expected 3 sentences, got %d: %#v", len(got), got)
	}
	if got[0] != "Hello world" {
		t.Fatalf("sentence[0]=%q", got[0])
	}
	if got[1] != "This is a test" {
		t.Fatalf("sentence[1]=%q", got[1])
	}
	if got[2] != "Line two" {
		t.Fatalf("sentence[2]=%q", got[2])
	}
}

