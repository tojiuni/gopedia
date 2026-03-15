package sink

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qdrant/go-client/qdrant"
	pb "gopedia/core/proto/gen/go"
	"gopedia/internal/phloem/embedder"
	"gopedia/internal/phloem/types"
)

// DefaultSink writes to PostgreSQL and Qdrant (L1 doc summary + L2 per chunk).
type DefaultSink struct {
	pg     *pgxpool.Pool
	qdrant *qdrant.Client
	embed  embedder.Embedder
	cfg    SinkConfig
}

// NewDefaultSink creates a sink. Any of PG/Qdrant/Embedder can be nil; those steps are skipped.
func NewDefaultSink(cfg SinkConfig) *DefaultSink {
	return &DefaultSink{
		pg:     cfg.PGPool,
		qdrant: cfg.Qdrant,
		embed:  cfg.Embedder,
		cfg:    cfg,
	}
}

// Write persists one document: PG doc row, then Qdrant L1 + L2 points from chunks.
func (s *DefaultSink) Write(ctx context.Context, msg *pb.RhizomeMessage, docID string, chunks []types.Chunk) error {
	if s.pg != nil {
		metaJSON, _ := json.Marshal(msg.SourceMetadata)
		_, err := s.pg.Exec(ctx,
			`INSERT INTO documents (id, machine_id, title, source_metadata, created_at)
			 VALUES ($1, $2, $3, $4, now())
			 ON CONFLICT (id) DO UPDATE SET title = $3, source_metadata = $4`,
			msg.MachineId, msg.MachineId, msg.Title, metaJSON,
		)
		if err != nil {
			return fmt.Errorf("postgres insert: %w", err)
		}
	}

	collection := getEnv("QDRANT_COLLECTION", "gopedia_markdown")
	if s.cfg.CollectionForDomain != nil {
		if d := msg.SourceMetadata["domain"]; d != "" {
			collection = s.cfg.CollectionForDomain(d)
		}
	}

	if s.qdrant != nil && s.embed != nil {
		l1Text := msg.Title
		if len(msg.Content) > 500 {
			l1Text += "\n" + msg.Content[:500]
		} else {
			l1Text += "\n" + msg.Content
		}
		vec, err := s.embed.Embed(ctx, l1Text)
		if err != nil {
			slog.Warn("embed L1 failed, skipping Qdrant L1", "err", err)
		} else {
			if err := s.upsertPoint(ctx, collection, docID+"_l1", vec, docID, msg.MachineId, "", ""); err != nil {
				return fmt.Errorf("qdrant L1: %w", err)
			}
		}
		for _, c := range chunks {
			vec, err := s.embed.Embed(ctx, c.Text)
			if err != nil {
				slog.Warn("embed section failed", "section", c.SectionID, "err", err)
				continue
			}
			if err := s.upsertPoint(ctx, collection, c.SectionID, vec, docID, msg.MachineId, c.Path, c.SectionID); err != nil {
				return fmt.Errorf("qdrant L2 %s: %w", c.SectionID, err)
			}
		}
	}
	return nil
}

func (s *DefaultSink) upsertPoint(ctx context.Context, collection, pointID string, vector []float32, docID string, machineID int64, tocPath, sectionID string) error {
	payload := map[string]interface{}{
		"doc_id":     docID,
		"machine_id": machineID,
		"toc_path":   tocPath,
		"section_id": sectionID,
	}
	points := []*qdrant.PointStruct{{
		Id:      qdrant.NewID(pointID),
		Vectors: qdrant.NewVectors(vector...),
		Payload: payloadMapToPayload(payload),
	}}
	_, err := s.qdrant.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collection,
		Points:         points,
	})
	return err
}

func payloadMapToPayload(m map[string]interface{}) map[string]*qdrant.Value {
	out := make(map[string]*qdrant.Value)
	for k, v := range m {
		switch x := v.(type) {
		case string:
			out[k] = &qdrant.Value{Kind: &qdrant.Value_StringValue{StringValue: x}}
		case int64:
			out[k] = &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: x}}
		case int:
			out[k] = &qdrant.Value{Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(x)}}
		}
	}
	return out
}
