package chunker

import "gopedia/internal/phloem/types"

// Chunker turns content and optional structure (TOC) into chunks for embedding/sink.
type Chunker interface {
	Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error)
}
