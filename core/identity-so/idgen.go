// Package identityso provides a 64-bit machine ID for Gopedia ingestion.
// One ID is generated per document and shared by all records (PostgreSQL, TypeDB, Qdrant).
package identityso

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// ID layout: 42-bit timestamp (ms), 10-bit worker ID, 12-bit sequence.
// Epoch: 2026-01-01 00:00:00 UTC.
const (
	epochMs     = 1735689600000 // 2026-01-01 00:00:00 UTC in ms
	workerBits  = 10
	seqBits     = 12
	workerMax   = 1<<workerBits - 1
	seqMax      = 1<<seqBits - 1
	workerShift = seqBits
	timeShift   = workerBits + seqBits
)

// Generator produces unique 64-bit IDs. Safe for concurrent use.
type Generator struct {
	mu        sync.Mutex
	workerID  int64
	sequence  int64
	lastMs    int64
}

// NewGenerator creates a generator. workerID is read from env GOPEDIA_IDENTITY_WORKER_ID
// or GOPEDIA_MARKDOWN_WORKER_ID (0 if unset). Clamped to [0, 1023].
func NewGenerator(workerID int64) *Generator {
	if workerID < 0 {
		workerID = 0
	}
	if workerID > workerMax {
		workerID = workerMax
	}
	return &Generator{workerID: workerID}
}

// GetMachineID returns a new 64-bit ID. Call once per document and reuse the result
// for all records (PG, TypeDB, Qdrant) of that document.
func (g *Generator) GetMachineID() int64 {
	g.mu.Lock()
	defer g.mu.Unlock()

	ms := time.Now().UTC().UnixMilli()
	if ms <= epochMs {
		ms = epochMs + 1
	}
	if ms == g.lastMs {
		g.sequence++
		if g.sequence > seqMax {
			// spin until next ms
			for ms <= g.lastMs {
				ms = time.Now().UTC().UnixMilli()
			}
			g.lastMs = ms
			g.sequence = 0
		}
	} else {
		g.lastMs = ms
		g.sequence = 0
	}

	// 42-bit time (ms since epoch), 10-bit worker, 12-bit seq
	t := (ms - epochMs) & ((1 << 42) - 1)
	return (t << timeShift) | (g.workerID << workerShift) | g.sequence
}

// WorkerIDFromEnv reads worker ID from GOPEDIA_IDENTITY_WORKER_ID or
// GOPEDIA_MARKDOWN_WORKER_ID. Returns 0 if unset or invalid.
func WorkerIDFromEnv() int64 {
	for _, key := range []string{"GOPEDIA_IDENTITY_WORKER_ID", "GOPEDIA_MARKDOWN_WORKER_ID"} {
		if v := os.Getenv(key); v != "" {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}
