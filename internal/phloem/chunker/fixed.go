package chunker

import (
	"fmt"
	"strings"

	"gopedia/internal/phloem/types"
)

// ByFixedSizeChunker splits content into fixed-size chunks (e.g. for PDF by token/char).
type ByFixedSizeChunker struct {
	MaxChars int // max characters per chunk; 0 = default 500
}

// Chunks ignores roots and splits content by size. Each chunk gets a generated SectionID and Path.
func (c ByFixedSizeChunker) Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error) {
	max := c.MaxChars
	if max <= 0 {
		max = 500
	}
	var out []types.Chunk
	runes := []rune(strings.TrimSpace(content))
	for i := 0; i < len(runes); i += max {
		end := i + max
		if end > len(runes) {
			end = len(runes)
		}
		text := string(runes[i:end])
		idx := len(out)
		out = append(out, types.Chunk{
			SectionID: fmtChunkID(idx),
			Path:      "",
			Text:      text,
		})
	}
	if len(out) == 0 && content != "" {
		out = append(out, types.Chunk{SectionID: "s0", Path: "", Text: content})
	}
	return out, nil
}

func fmtChunkID(i int) string {
	return fmt.Sprintf("s%d", i)
}
