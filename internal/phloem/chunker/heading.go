package chunker

import (
	"gopedia/internal/phloem/toc"
	"gopedia/internal/phloem/types"
)

// ByHeadingChunker produces one chunk per TOC section (heading text only).
type ByHeadingChunker struct{}

// Chunks flattens the TOC and returns one Chunk per node (Text = node text).
func (ByHeadingChunker) Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error) {
	flat := toc.FlattenTOC(roots)
	out := make([]types.Chunk, len(flat))
	for i := range flat {
		out[i] = types.Chunk{
			SectionID: flat[i].SectionID,
			Path:      flat[i].Path,
			Text:      flat[i].Node.Text,
		}
	}
	return out, nil
}
