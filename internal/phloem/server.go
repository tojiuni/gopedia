package phloem

import (
	"context"

	pb "gopedia/core/proto/gen/go"
)

// Server implements the Phloem gRPC service.
// It delegates to domain-specific pipelines registered in the registry.
type Server struct {
	pb.UnimplementedPhloemServer
}

// NewServer creates a Phloem gRPC server that routes requests by domain.
// Pipelines must be registered (e.g. in main) before serving.
func NewServer() *Server {
	return &Server{}
}

// domainFromRequest returns the pipeline domain: req.Domain, or source_metadata["domain"], or "wiki".
func domainFromRequest(req *pb.IngestRequest) string {
	if req != nil && req.Domain != "" {
		return req.Domain
	}
	if req != nil && req.SourceMetadata != nil {
		if d := req.SourceMetadata["domain"]; d != "" {
			return d
		}
	}
	return "wiki"
}

// IngestMarkdown receives markdown from Root and writes to Rhizome via the domain pipeline.
func (s *Server) IngestMarkdown(ctx context.Context, req *pb.IngestRequest) (*pb.IngestResponse, error) {
	domainKey := domainFromRequest(req)
	pipeline, ok := Get(domainKey)
	if !ok || pipeline == nil {
		return &pb.IngestResponse{Ok: false, ErrorMessage: "no pipeline registered for domain: " + domainKey}, nil
	}
	return pipeline.Process(ctx, req)
}
