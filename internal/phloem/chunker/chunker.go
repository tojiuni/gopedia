package chunker

import "gopedia/internal/phloem/types"

// Chunker turns content and optional structure (TOC) into chunks for embedding/sink.
//
// Long-term direction: prefer AST or byte-offset–based parsing so chunks are zero-overlap and
// no semantic data is lost, while still normalizing away only *spurious* whitespace/newlines for
// storage and viewer restore (see heading.go, fixed.go).
type Chunker interface {
	Chunks(content string, roots []types.TOCNode) ([]types.Chunk, error)
}
