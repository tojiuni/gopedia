package chunker

import (
	"fmt"

	"gopedia/internal/phloem/types"
)

// BySymbolChunker produces one chunk per code symbol (function, class, etc.).
// Long-term: tree-sitter or Go AST byte offsets for zero-overlap chunks; strip only spurious whitespace.
// TODO: implement AST-based slicing when CodeTOCParser provides symbol ranges.
type BySymbolChunker struct{}

// Chunks returns a placeholder: one chunk per root node. Replace with symbol-based splitting.
func (BySymbolChunker) Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error) {
	if len(roots) == 0 {
		return []types.Chunk{{SectionID: "s0", Path: "", Text: content}}, nil
	}
	out := make([]types.Chunk, 0, len(roots))
	for i, n := range roots {
		out = append(out, types.Chunk{
			SectionID: fmt.Sprintf("s%d", i),
			Path:      n.Text,
			Text:      n.Text,
		})
	}
	return out, nil
}
