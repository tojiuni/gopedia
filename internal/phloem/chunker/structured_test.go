package chunker

import (
	"strings"
	"testing"

	"gopedia/internal/phloem/types"
)

func TestExpandStructuredOrderedList(t *testing.T) {
	parent := types.Chunk{
		SectionID: "s0",
		Path:      "Doc > Sec",
		Text:      "## Sec\n\n1. First item here.\n2. Second item.\n",
		Level:     types.LevelL2,
		Version:   1,
	}
	var ctr blockCounters
	out := expandStructuredBlocks(parent, &ctr)
	// Consecutive ordered items are now grouped into a single chunk.
	if len(out) != 2 {
		t.Fatalf("want 2 chunks (heading + 1 grouped ordered), got %d: %#v", len(out), out)
	}
	if out[0].SectionID != "s0" || out[0].SectionType != types.SectionTypeHeading {
		t.Fatalf("chunk0: %#v", out[0])
	}
	if !strings.Contains(out[0].Text, "## Sec") {
		t.Fatalf("heading chunk should keep title, got %q", out[0].Text)
	}
	if out[1].SectionID != "o1" || out[1].ParentSectionID != "s0" || out[1].SectionType != types.SectionTypeOrdered {
		t.Fatalf("chunk1: %#v", out[1])
	}
	if !strings.Contains(out[1].Text, "First item") || !strings.Contains(out[1].Text, "Second item") {
		t.Fatalf("grouped chunk should contain all items, got %q", out[1].Text)
	}
}

func TestExpandStructuredGFMTable(t *testing.T) {
	md := "## T\n\n| a | b |\n|---|---|\n| 1 | 2 |\n"
	parent := types.Chunk{SectionID: "s1", Path: "P", Text: md, Level: types.LevelL2}
	var ctr blockCounters
	out := expandStructuredBlocks(parent, &ctr)
	if len(out) != 2 {
		t.Fatalf("want 2 chunks, got %d", len(out))
	}
	if out[1].SectionID != "t1" || out[1].SectionType != types.SectionTypeTable {
		t.Fatalf("table chunk: %#v", out[1])
	}
	hdr := out[1].SourceMetadata["headers"]
	hs, ok := hdr.([]string)
	if !ok || len(hs) < 2 {
		t.Fatalf("headers meta: %#v", hdr)
	}
}
