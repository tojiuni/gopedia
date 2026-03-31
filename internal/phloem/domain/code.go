package domain

import (
	"context"
	"fmt"
	"strings"

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

	// Detect language from source path (title) and configure the parser.
	if cp, ok := p.Parser.(*toc.CodeTOCParser); ok {
		lang := req.SourceMetadata["language"]
		if lang == "" {
			lang = detectLangFromTitle(req.Title)
		}
		cp.Lang = lang
	}

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

// detectLangFromTitle infers the programming language from a file path/title.
func detectLangFromTitle(title string) string {
	lower := strings.ToLower(title)
	switch {
	case strings.HasSuffix(lower, ".go"):
		return "go"
	case strings.HasSuffix(lower, ".py"):
		return "python"
	case strings.HasSuffix(lower, ".ts"), strings.HasSuffix(lower, ".tsx"):
		return "typescript"
	case strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".jsx"):
		return "typescript" // tree-sitter-typescript covers JS
	default:
		return "python" // safe default
	}
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
