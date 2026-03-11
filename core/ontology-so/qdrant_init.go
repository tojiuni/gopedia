// Package ontologyso provides TypeDB and Qdrant schema initialization for Gopedia 0.0.1.
// TypeDB schema is in typedb_schema.typeql; run typedb_init.py to apply it.
// Qdrant collection is created by EnsureQdrantCollection.
package ontologyso

import (
	"context"
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

// DefaultVectorSize is the OpenAI embedding dimension used for L1/L2.
const DefaultVectorSize = 1536

// EnsureQdrantCollection creates the collection if it does not exist.
// Payload fields used: doc_id, machine_id, toc_path, section_id (indexed by Phloem sink).
// If client is nil, returns nil without error.
func EnsureQdrantCollection(ctx context.Context, client *qdrant.Client, collectionName string, vectorSize uint64) error {
	if client == nil {
		return nil
	}
	if vectorSize == 0 {
		vectorSize = DefaultVectorSize
	}

	exists, err := client.CollectionExists(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("check collection exists: %w", err)
	}
	if exists {
		return nil
	}

	err = client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: collectionName,
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     vectorSize,
			Distance: qdrant.Distance_Cosine,
		}),
	})
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	return nil
}
