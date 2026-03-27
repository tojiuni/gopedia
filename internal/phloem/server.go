package phloem

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "gopedia/core/proto/gen/go"
)

// Server implements the Phloem gRPC service.
// It delegates to domain-specific pipelines registered in the registry.
type Server struct {
	pb.UnimplementedPhloemServer
	pg *pgxpool.Pool
}

// NewServer creates a Phloem gRPC server that routes requests by domain.
// Pipelines must be registered (e.g. in main) before serving.
// pg may be nil; RegisterProject will fail until Postgres is configured.
func NewServer(pg *pgxpool.Pool) *Server {
	return &Server{pg: pg}
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

// RegisterProject inserts or updates a row in projects by root_path and returns its id.
func (s *Server) RegisterProject(ctx context.Context, req *pb.RegisterProjectRequest) (*pb.RegisterProjectResponse, error) {
	if s.pg == nil {
		return nil, status.Error(codes.FailedPrecondition, "postgres not configured")
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	root := strings.TrimSpace(req.GetRootPath())
	if root == "" {
		return nil, status.Error(codes.InvalidArgument, "root_path is required")
	}
	root = filepath.Clean(root)

	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = filepath.Base(root)
	}

	meta := req.GetMetadata()
	if meta == nil {
		meta = map[string]string{}
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "metadata: %v", err)
	}

	machineID := req.GetMachineId()
	if machineID == 0 {
		machineID = ProjectMachineID(root)
	}
	if machineID == 0 {
		return nil, status.Error(codes.InvalidArgument, "could not derive machine_id for root_path")
	}

	var id int64
	var storedMID int64
	err = s.pg.QueryRow(ctx, `
		INSERT INTO projects (machine_id, name, root_path, source_metadata)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (root_path) DO UPDATE SET
			name = EXCLUDED.name,
			source_metadata = EXCLUDED.source_metadata,
			modified_at = now()
		RETURNING id, machine_id`,
		machineID, name, root, metaJSON,
	).Scan(&id, &storedMID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.Internal, "register project: no id returned")
		}
		return nil, status.Errorf(codes.Internal, "register project: %v", err)
	}
	if req.GetMachineId() != 0 && storedMID != req.GetMachineId() {
		return nil, status.Errorf(
			codes.FailedPrecondition,
			"root_path already registered with machine_id %d (request had %d)",
			storedMID, req.GetMachineId(),
		)
	}
	return &pb.RegisterProjectResponse{ProjectId: id, MachineId: storedMID}, nil
}
