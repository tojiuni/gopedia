package sink

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qdrant/go-client/qdrant"
	pb "gopedia/core/proto/gen/go"
	"gopedia/internal/phloem/embedder"
	"gopedia/internal/phloem/types"
)

// SinkWriter persists one document to Rhizome (PG, Qdrant) using chunks.
type SinkWriter interface {
	Write(ctx context.Context, msg *pb.RhizomeMessage, docID string, chunks []types.Chunk) error
}

// SinkConfig configures the sink. CollectionForDomain optionally overrides Qdrant collection per domain.
type SinkConfig struct {
	PGPool   *pgxpool.Pool
	Qdrant   *qdrant.Client
	Embedder embedder.Embedder
	// CollectionForDomain returns Qdrant collection name for a domain; if nil, default env QDRANT_COLLECTION is used.
	CollectionForDomain func(domain string) string
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
