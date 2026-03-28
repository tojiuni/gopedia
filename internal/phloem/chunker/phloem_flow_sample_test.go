package chunker

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopedia/internal/phloem/codesplitter"
	"gopedia/internal/phloem/toc"
	"gopedia/internal/phloem/types"
)

// TestPhloemFlowSampleHasCodeL2AndMultipleL3 validates doc/design/Rev2/references/phloem-flow.md:
// fenced code becomes L2 with section_id c*, and SplitToL3 yields multiple fragments (pytest calls this via go test).
func TestPhloemFlowSampleHasCodeL2AndMultipleL3(t *testing.T) {
	repoRoot := filepath.Join("..", "..", "..")
	mdPath := filepath.Join(repoRoot, "doc", "design", "Rev2", "references", "phloem-flow.md")
	b, err := os.ReadFile(mdPath)
	if err != nil {
		t.Skipf("sample file not found at %s: %v", mdPath, err)
	}
	content := string(b)
	roots, err := toc.MarkdownTOCParser{}.Parse(content)
	if err != nil {
		t.Fatalf("parse toc: %v", err)
	}
	chunks, err := ByHeadingChunker{}.Chunks(content, roots)
	if err != nil {
		t.Fatalf("chunks: %v", err)
	}
	var code *types.Chunk
	for i := range chunks {
		if chunks[i].SectionType == types.SectionTypeCode {
			code = &chunks[i]
			break
		}
	}
	if code == nil {
		t.Fatal("expected at least one SectionTypeCode chunk (mermaid fence) in phloem-flow.md")
	}
	if !strings.HasPrefix(code.SectionID, "c") {
		t.Fatalf("code chunk section_id should be c*, got %q", code.SectionID)
	}
	lang := ""
	if code.SourceMetadata != nil {
		if v, ok := code.SourceMetadata["language"].(string); ok {
			lang = v
		}
	}
	if lang != "mermaid" {
		t.Fatalf("expected mermaid language in metadata, got %q", lang)
	}
	parts := codesplitter.SplitToL3(code.Text, lang)
	if len(parts) < 3 {
		t.Fatalf("expected multiple L3 lines from mermaid block, got %d: %#v", len(parts), parts)
	}
}
