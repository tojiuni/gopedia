package domain

import (
	"context"
	"fmt"

	pb "gopedia/core/proto/gen/go"
	identityso "gopedia/core/identity_so"
	"gopedia/internal/phloem"
	"gopedia/internal/phloem/chunker"
	"gopedia/internal/phloem/sink"
	"gopedia/internal/phloem/toc"
)

// CodePipeline ingests source code: code structure + symbol chunker + sink.
type CodePipeline struct {
	Parser toc.TOCParser
	Chunk  chunker.Chunker
	Sink   sink.SinkWriter
	IDGen  *identityso.Generator
}

// Process implements phloem.Pipeline.
func (p *CodePipeline) Process(ctx context.Context, req *pb.IngestRequest) (*pb.IngestResponse, error) {
	machineID := p.IDGen.GetMachineID()

	roots, err := p.Parser.Parse(req.Content)
	if err != nil {
		return &pb.IngestResponse{Ok: false, ErrorMessage: err.Error()}, nil
	}

	tocJSON, _ := toc.TOCToJSON(roots)

	chunks, err := p.Chunk.Chunks(req.Content, roots)
	if err != nil {
		return &pb.IngestResponse{Ok: false, ErrorMessage: err.Error()}, nil
	}

	msg := &pb.RhizomeMessage{
		Id:             machineID,
		Title:          req.Title,
		Content:        req.Content,
		Toc:            tocJSON,
		SourceMetadata: req.SourceMetadata,
		MachineId:      machineID,
	}

	docID, err := p.Sink.Write(ctx, msg, chunks)
	if err != nil {
		return &pb.IngestResponse{
			MachineId:    machineID,
			DocId:        "",
			Ok:           false,
			ErrorMessage: fmt.Sprintf("sink: %v", err),
		}, nil
	}

	return &pb.IngestResponse{MachineId: machineID, DocId: docID, Ok: true}, nil
}

// NewCodePipeline builds a code pipeline with the given components.
func NewCodePipeline(parser toc.TOCParser, ch chunker.Chunker, sw sink.SinkWriter, idGen *identityso.Generator) phloem.Pipeline {
	return &CodePipeline{
		Parser: parser,
		Chunk:  ch,
		Sink:   sw,
		IDGen:  idGen,
	}
}
