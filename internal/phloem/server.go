package phloem

import (
	"context"
	"fmt"
	"strconv"

	pb "gopedia/core/proto/gen/go"
	identityso "gopedia/core/identity_so"
)

// Server implements the Phloem gRPC service.
type Server struct {
	pb.UnimplementedPhloemServer
	idGen *identityso.Generator
	sink  SinkWriter
}

// NewServer creates a Phloem gRPC server.
func NewServer(sink SinkWriter) *Server {
	workerID := identityso.WorkerIDFromEnv()
	return &Server{
		idGen: identityso.NewGenerator(workerID),
		sink:  sink,
	}
}

// IngestMarkdown receives markdown from Root and writes to Rhizome (PG, TypeDB, Qdrant).
func (s *Server) IngestMarkdown(ctx context.Context, req *pb.IngestRequest) (*pb.IngestResponse, error) {
	machineID := s.idGen.GetMachineID()
	docID := strconv.FormatInt(machineID, 10)

	tocRoots := ParseTOC(req.Content)
	tocJSON, err := TOCToJSON(tocRoots)
	if err != nil {
		return &pb.IngestResponse{Ok: false, ErrorMessage: err.Error()}, nil
	}
	flatTOC := FlattenTOC(tocRoots)

	msg := &pb.RhizomeMessage{
		Id:             machineID,
		Title:          req.Title,
		Content:        req.Content,
		Toc:            tocJSON,
		SourceMetadata: req.SourceMetadata,
		MachineId:      machineID,
	}

	if err := s.sink.Write(ctx, msg, docID, flatTOC); err != nil {
		return &pb.IngestResponse{
			MachineId:     machineID,
			DocId:         docID,
			Ok:            false,
			ErrorMessage:  fmt.Sprintf("sink: %v", err),
		}, nil
	}

	return &pb.IngestResponse{
		MachineId: machineID,
		DocId:     docID,
		Ok:        true,
	}, nil
}
