package phloem

import (
	"context"

	pb "gopedia/core/proto/gen/go"
)

// Pipeline runs domain-specific ingestion: parse structure → chunk → sink.
type Pipeline interface {
	Process(ctx context.Context, req *pb.IngestRequest) (*pb.IngestResponse, error)
}
