package phloem_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	identityso "gopedia/core/identity_so"
	pb "gopedia/core/proto/gen/go"
	"gopedia/internal/phloem"
	"gopedia/internal/phloem/chunker"
	"gopedia/internal/phloem/domain"
	"gopedia/internal/phloem/toc"
	"gopedia/internal/phloem/types"
)

// recordingChunkSink records Write calls for tests. Implements sink.SinkWriter.
type recordingChunkSink struct {
	calls []chunkWriteCall
}

type chunkWriteCall struct {
	Msg    *pb.RhizomeMessage
	DocID  string
	Chunks []types.Chunk
}

func (r *recordingChunkSink) Write(ctx context.Context, msg *pb.RhizomeMessage, chunks []types.Chunk) (string, error) {
	docID := ""
	if msg != nil {
		docID = "test-doc-uuid"
	}
	r.calls = append(r.calls, chunkWriteCall{Msg: msg, DocID: docID, Chunks: chunks})
	return docID, nil
}

// TestIdentityMachineIDConsistency verifies that one IngestMarkdown call (via domain pipeline)
// produces a single Write with the same machine_id in the response and in the sink.
func TestIdentityMachineIDConsistency(t *testing.T) {
	rec := &recordingChunkSink{}
	idGen := identityso.NewGenerator(1)
	phloem.Register(domain.Wiki, domain.NewWikiPipeline(
		toc.MarkdownTOCParser{},
		chunker.ByHeadingChunker{},
		rec,
		idGen,
	))
	srv := phloem.NewServer(nil)

	ctx := context.Background()
	req := &pb.IngestRequest{
		Title:   "Test Doc",
		Content: "# A\n\nSection A body.\n\n## B\n\nSection B.",
		Domain:  "wiki",
	}

	resp, err := srv.IngestMarkdown(ctx, req)
	if err != nil {
		t.Fatalf("IngestMarkdown: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("IngestMarkdown not ok: %s", resp.ErrorMessage)
	}

	if n := len(rec.calls); n != 1 {
		t.Fatalf("expected 1 Write call, got %d", n)
	}
	call := rec.calls[0]
	if call.Msg == nil {
		t.Fatal("Write call has nil Msg")
	}
	if call.Msg.MachineId != resp.MachineId {
		t.Errorf("sink Msg.MachineId = %d, response MachineId = %d", call.Msg.MachineId, resp.MachineId)
	}
	if call.DocID != "test-doc-uuid" {
		t.Errorf("sink DocID = %q, expected test-doc-uuid", call.DocID)
	}
	if call.Msg.Id != call.Msg.MachineId {
		t.Errorf("Msg.Id = %d, Msg.MachineId = %d (should match)", call.Msg.Id, call.Msg.MachineId)
	}
}

// TestIngestMarkdownDomainRouting verifies that requests are routed by domain and unknown domain returns error.
func TestIngestMarkdownDomainRouting(t *testing.T) {
	rec := &recordingChunkSink{}
	idGen := identityso.NewGenerator(2)
	phloem.Register(domain.Wiki, domain.NewWikiPipeline(
		toc.MarkdownTOCParser{},
		chunker.ByHeadingChunker{},
		rec,
		idGen,
	))
	srv := phloem.NewServer(nil)
	ctx := context.Background()

	// Explicit domain "wiki" uses registered pipeline.
	req := &pb.IngestRequest{Title: "T", Content: "# H", Domain: "wiki"}
	resp, err := srv.IngestMarkdown(ctx, req)
	if err != nil {
		t.Fatalf("IngestMarkdown: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("expected ok for domain wiki: %s", resp.ErrorMessage)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("expected 1 sink call, got %d", len(rec.calls))
	}

	// Unknown domain has no pipeline: expect not ok.
	reqUnknown := &pb.IngestRequest{Title: "T", Content: "# H", Domain: "unknown-domain-xyz"}
	resp2, _ := srv.IngestMarkdown(ctx, reqUnknown)
	if resp2.Ok {
		t.Fatal("expected not ok for unknown domain")
	}
	if resp2.ErrorMessage == "" {
		t.Error("expected error message for unknown domain")
	}
}

// TestWikiDomainPipelineWithSampleMD verifies wiki domain routing with tests/fixtures/sample.md content.
func TestWikiDomainPipelineWithSampleMD(t *testing.T) {
	samplePath := filepath.Join("..", "..", "tests", "fixtures", "sample.md")
	content, err := os.ReadFile(samplePath)
	if err != nil {
		t.Skipf("fixture not found (run from repo root or internal/phloem): %v", err)
	}

	rec := &recordingChunkSink{}
	idGen := identityso.NewGenerator(3)
	phloem.Register(domain.Wiki, domain.NewWikiPipeline(
		toc.MarkdownTOCParser{},
		chunker.ByHeadingChunker{},
		rec,
		idGen,
	))
	srv := phloem.NewServer(nil)
	ctx := context.Background()

	req := &pb.IngestRequest{
		Title:   "Sample for Transpiration E2E",
		Content: string(content),
		Domain:  "wiki",
	}
	resp, err := srv.IngestMarkdown(ctx, req)
	if err != nil {
		t.Fatalf("IngestMarkdown: %v", err)
	}
	if !resp.Ok {
		t.Fatalf("IngestMarkdown not ok: %s", resp.ErrorMessage)
	}

	if n := len(rec.calls); n != 1 {
		t.Fatalf("expected 1 Write call, got %d", n)
	}
	call := rec.calls[0]
	if call.Msg == nil {
		t.Fatal("Write call has nil Msg")
	}
	if call.Msg.Title != req.Title {
		t.Errorf("sink Title = %q, want %q", call.Msg.Title, req.Title)
	}
	// sample.md has # Introduction, ## Goals, ## Implementation -> 3 TOC chunks
	if want := 3; len(call.Chunks) != want {
		t.Errorf("expected %d chunks from sample.md TOC, got %d", want, len(call.Chunks))
	}
	// Sanity: first chunk should include the Introduction section body (not just the heading title).
	if len(call.Chunks) > 0 {
		if !strings.Contains(call.Chunks[0].Text, "# Introduction") {
			t.Errorf("first chunk Text missing Introduction heading line: %q", call.Chunks[0].Text)
		}
	}
}
