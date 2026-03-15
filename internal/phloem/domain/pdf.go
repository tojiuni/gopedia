package domain

import (
	"context"
	"fmt"
	"strconv"

	pb "gopedia/core/proto/gen/go"
	identityso "gopedia/core/identity_so"
	"gopedia/internal/phloem"
	"gopedia/internal/phloem/chunker"
	"gopedia/internal/phloem/sink"
	"gopedia/internal/phloem/toc"
)

// PDFPipeline ingests PDF/OCR output: page/block structure + fixed-size chunker + sink.
type PDFPipeline struct {
	Parser toc.TOCParser
	Chunk  chunker.Chunker
	Sink   sink.SinkWriter
	IDGen  *identityso.Generator
}

// Process implements phloem.Pipeline.
func (p *PDFPipeline) Process(ctx context.Context, req *pb.IngestRequest) (*pb.IngestResponse, error) {
	machineID := p.IDGen.GetMachineID()
	docID := strconv.FormatInt(machineID, 10)

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

	if err := p.Sink.Write(ctx, msg, docID, chunks); err != nil {
		return &pb.IngestResponse{
			MachineId:    machineID,
			DocId:       docID,
			Ok:          false,
			ErrorMessage: fmt.Sprintf("sink: %v", err),
		}, nil
	}

	return &pb.IngestResponse{MachineId: machineID, DocId: docID, Ok: true}, nil
}

// NewPDFPipeline builds a PDF pipeline with the given components.
func NewPDFPipeline(parser toc.TOCParser, ch chunker.Chunker, sw sink.SinkWriter, idGen *identityso.Generator) phloem.Pipeline {
	return &PDFPipeline{
		Parser: parser,
		Chunk:  ch,
		Sink:   sw,
		IDGen:  idGen,
	}
}
