package chunker

import (
	"strings"
	"testing"
)

func TestByFixedSizeChunkerSemanticBoundaries(t *testing.T) {
	content := strings.Repeat("word ", 200) + ". " + strings.Repeat("next ", 200)
	chunks, err := (ByFixedSizeChunker{MaxChars: 120}).Chunks(content, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if !c.SemanticL3Split {
			t.Fatalf("chunk should set SemanticL3Split: %#v", c)
		}
	}
}
