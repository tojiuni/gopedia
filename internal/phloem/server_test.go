package phloem

import (
	"context"
	"strconv"
	"testing"

	pb "gopedia/core/proto/gen/go"
)

// recordingSink records Write calls for tests. Implements SinkWriter.
type recordingSink struct {
	calls []writeCall
}

type writeCall struct {
	Msg     *pb.RhizomeMessage
	DocID   string
	FlatTOC []FlatTOCItem
}

func (r *recordingSink) Write(ctx context.Context, msg *pb.RhizomeMessage, docID string, flatTOC []FlatTOCItem) error {
	r.calls = append(r.calls, writeCall{Msg: msg, DocID: docID, FlatTOC: flatTOC})
	return nil
}

// TestIdentityMachineIDConsistency verifies that one IngestMarkdown call produces a single
// Write with the same machine_id in the response and in the sink (PG and Qdrant would both
// receive this same ID via msg.MachineId).
func TestIdentityMachineIDConsistency(t *testing.T) {
	rec := &recordingSink{}
	srv := NewServer(rec)

	ctx := context.Background()
	req := &pb.IngestRequest{
		Title:   "Test Doc",
		Content: "# A\n\nSection A body.\n\n## B\n\nSection B.",
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
	expectedDocID := strconv.FormatInt(resp.MachineId, 10)
	if call.DocID != expectedDocID {
		t.Errorf("sink DocID = %q, expected %q", call.DocID, expectedDocID)
	}
	// Same ID is used for PG row and every Qdrant point (L1 + L2) via msg.MachineId
	if call.Msg.Id != call.Msg.MachineId {
		t.Errorf("Msg.Id = %d, Msg.MachineId = %d (should match)", call.Msg.Id, call.Msg.MachineId)
	}
}
