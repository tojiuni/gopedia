package sink

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestExtractFirstMarkdownHeadingLine(t *testing.T) {
	in := "---\ntitle: X\n---\n\n## Section\n\nBody"
	got := extractFirstMarkdownHeadingLine(in)
	if got != "## Section" {
		t.Fatalf("expected ## Section, got %q", got)
	}
	if extractFirstMarkdownHeadingLine("no heading here") != "" {
		t.Fatal("expected empty")
	}
}

func TestExpandSemanticL3Fragments(t *testing.T) {
	sents := []string{"short"}
	if got := expandSemanticL3Fragments(sents, false); len(got) != 1 {
		t.Fatalf("no split when disabled: %#v", got)
	}
	long := "alpha, beta, gamma | delta"
	got := expandSemanticL3Fragments([]string{long}, true)
	if len(got) < 3 {
		t.Fatalf("expected clause splits, got %#v", got)
	}
}

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

func TestTruncateUTF8ByBytes(t *testing.T) {
	// 2 ASCII + 1 Korean rune (3 bytes) + 1 ASCII
	s := "ab한c"
	got := truncateUTF8ByBytes(s, 4)
	// Byte 4 cuts inside "한"; must drop partial rune → "ab"
	if got != "ab" {
		t.Fatalf("got %q want ab", got)
	}
	if !utf8.ValidString(got) {
		t.Fatal("not valid UTF-8")
	}
}

func TestSplitSentencesEnglishKeepsMarkdownLinkIntact(t *testing.T) {
	in := "일정은 [RoadMap/SKILL.md](RoadMap/SKILL.md) 참조합니다."
	got := splitSentencesEnglish(in)
	want := "[RoadMap/SKILL.md](RoadMap/SKILL.md)"
	ok := false
	for _, s := range got {
		if strings.Contains(s, want) {
			ok = true
			break
		}
	}
	if !ok {
		t.Fatalf("expected full markdown link in one sentence, got %#v", got)
	}
}

func TestSplitSentencesEnglishKeepsOrderedListMarkerIntact(t *testing.T) {
	in := "1. **Verify (발아)**: 최소 단위 Root(소스)."
	got := splitSentencesEnglish(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 sentence, got %d: %#v", len(got), got)
	}
}

func TestProjectIDFromMetadata(t *testing.T) {
	v, ok := projectIDFromMetadata(map[string]string{"project_id": "42"})
	if !ok || v != 42 {
		t.Fatalf("metadata project_id: got %d ok=%v", v, ok)
	}
	if v, ok := projectIDFromMetadata(nil); ok || v != 0 {
		t.Fatalf("nil meta: got %d ok=%v", v, ok)
	}
	if v, ok := projectIDFromMetadata(map[string]string{"project_id": "x"}); ok {
		t.Fatalf("invalid int should not ok, got %d", v)
	}
	t.Setenv("GOPEDIA_PROJECT_ID", "7")
	if got := projectIDForPayloadFromMetadata(nil); got != 7 {
		t.Fatalf("env fallback: got %d", got)
	}
	if got := projectIDForPayloadFromMetadata(map[string]string{"project_id": "3"}); got != 3 {
		t.Fatalf("metadata wins over env: got %d", got)
	}
}

