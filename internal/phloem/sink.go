package phloem

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qdrant/go-client/qdrant"
	pb "gopedia/core/proto/gen/go"
)

// SinkWriter is the interface used to persist one document to Rhizome (PG, Qdrant).
// *Sink implements it; tests can use a recording implementation.
type SinkWriter interface {
	Write(ctx context.Context, msg *pb.RhizomeMessage, docID string, flatTOC []FlatTOCItem) error
}

// Sink writes to PostgreSQL, TypeDB, and Qdrant (L1 → L2 order).
type Sink struct {
	pg     *pgxpool.Pool
	qdrant *qdrant.Client
	embed  *Embedder
	// TypeDB: optional; if nil, TypeDB insert is skipped (run typedb_insert.py separately or add Go driver).
}

// SinkConfig configures the sink.
type SinkConfig struct {
	PGPool    *pgxpool.Pool
	Qdrant    *qdrant.Client
	Embedder  *Embedder
}

// NewSink creates a sink. Any of PG/Qdrant/Embedder can be nil; those steps are skipped.
func NewSink(c SinkConfig) *Sink {
	return &Sink{
		pg:     c.PGPool,
		qdrant: c.Qdrant,
		embed:  c.Embedder,
	}
}

// Write persists one document to Rhizome (PG doc row, then Qdrant L1 + L2 points).
// docID is a string id for this document (e.g. fmt.Sprintf("%d", machineID)).
func (s *Sink) Write(ctx context.Context, msg *pb.RhizomeMessage, docID string, flatTOC []FlatTOCItem) error {
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

	if s.qdrant != nil && s.embed != nil {
		collection := getEnv("QDRANT_COLLECTION", "gopedia_markdown")
		// L1: one point for document summary (title + first 500 chars of content)
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
			err = s.upsertPoint(ctx, collection, docID+"_l1", vec, docID, msg.MachineId, "", "")
			if err != nil {
				return fmt.Errorf("qdrant L1: %w", err)
			}
		}
		// L2: one point per section
		for _, item := range flatTOC {
			sectionText := item.Node.Text
			vec, err := s.embed.Embed(ctx, sectionText)
			if err != nil {
				slog.Warn("embed section failed", "section", item.SectionID, "err", err)
				continue
			}
			err = s.upsertPoint(ctx, collection, item.SectionID, vec, docID, msg.MachineId, item.Path, item.SectionID)
			if err != nil {
				return fmt.Errorf("qdrant L2 %s: %w", item.SectionID, err)
			}
		}
	}
	return nil
}

func (s *Sink) upsertPoint(ctx context.Context, collection, pointID string, vector []float32, docID string, machineID int64, tocPath, sectionID string) error {
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

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
