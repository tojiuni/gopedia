package integration

import (
	"context"
	"testing"
)

// TestInitAllFromEnv runs InitAllFromEnv (Postgres DDL + Qdrant collection) when env is set.
// Use to ensure documents table and Qdrant collection exist before other integration tests.
func TestInitAllFromEnv(t *testing.T) {
	if getEnv("POSTGRES_HOST", "") == "" && getEnv("QDRANT_HOST", "") == "" {
		t.Skip("POSTGRES_HOST and QDRANT_HOST not set")
	}
	repoRoot := getEnv("GOPEDIA_REPO_ROOT", ".")
	if repoRoot == "" {
		repoRoot = "."
	}
	ctx := context.Background()
	err := InitAllFromEnv(ctx, repoRoot)
	if err != nil {
		t.Fatalf("InitAllFromEnv: %v", err)
	}
}
