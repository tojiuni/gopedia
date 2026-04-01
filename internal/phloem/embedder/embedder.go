package embedder

import "context"

// Embedder produces vectors for text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	// VectorSize returns the dimension of embeddings produced by this embedder.
	VectorSize() int
}
