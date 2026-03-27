// Package integration provides Go integration tests for Gopedia service connectivity.
//
// OVERVIEW: TypeDB=typedb:1729, Qdrant=qdrant:6333/6334, PostgreSQL, Phloem gRPC.
// Connection failures cause test failure (no skip).
// Run from repo root: go test ./tests/integration/ -v
package integration

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "gopedia/core/proto/gen/go"
	"gopedia/core/ontology_so"
)

func dialReachable(network, addr string, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, network, addr)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func TestTypeDBReachable(t *testing.T) {
	host := getEnv("TYPEDB_HOST", "")
	if host == "" {
		t.Skip("TYPEDB_HOST not set")
	}
	port := getEnv("TYPEDB_PORT", "1729")
	addr := net.JoinHostPort(host, port)
	if !dialReachable("tcp", addr, 2*time.Second) {
		t.Fatalf("TypeDB not reachable at %s", addr)
	}
}

func TestQdrantConnect(t *testing.T) {
	host := getEnv("QDRANT_HOST", "")
	if host == "" {
		t.Skip("QDRANT_HOST not set")
	}
	port := 6334
	if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
		port = p
	}
	client, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		t.Fatalf("Qdrant connect failed: %v", err)
	}
	ctx := context.Background()
	_, err = client.CollectionExists(ctx, getEnv("QDRANT_COLLECTION", "gopedia_markdown"))
	if err != nil {
		t.Fatalf("Qdrant CollectionExists: %v", err)
	}
}

func TestPostgresConnect(t *testing.T) {
	host := getEnv("POSTGRES_HOST", "")
	user := getEnv("POSTGRES_USER", "")
	if host == "" || user == "" {
		t.Skip("POSTGRES_HOST or POSTGRES_USER not set")
	}
	port := getEnv("POSTGRES_PORT", "5432")
	pass := getEnv("POSTGRES_PASSWORD", "")
	db := getEnv("POSTGRES_DB", "gopedia")
	sslmode := getEnv("POSTGRES_SSLMODE", "disable")
	connStr := "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + db + "?sslmode=" + sslmode

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("Postgres connect failed: %v", err)
	}
	defer pool.Close()

	var exists bool
	err = pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'documents')",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("Postgres query: %v", err)
	}
	if !exists {
		t.Fatal("documents table not found (run core/ontology_so/postgres_ddl.sql)")
	}
	err = pool.QueryRow(ctx,
		"SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'projects')",
	).Scan(&exists)
	if err != nil {
		t.Fatalf("Postgres projects table query: %v", err)
	}
	if !exists {
		t.Fatal("projects table not found (run core/ontology_so/postgres_ddl.sql)")
	}
}

func TestPhloemGRPCReachable(t *testing.T) {
	addr := getEnv("GOPEDIA_PHLOEM_GRPC_ADDR", "localhost:50051")
	if addr == "" || addr == ":50051" {
		addr = "localhost:50051"
	}
	if !dialReachable("tcp", addr, 2*time.Second) {
		t.Fatalf("Phloem gRPC server not reachable at %s (start with: go run ./cmd/phloem)", addr)
	}
}

func TestPhloemIngestMarkdown(t *testing.T) {
	addr := getEnv("GOPEDIA_PHLOEM_GRPC_ADDR", "localhost:50051")
	if addr == "" || addr == ":50051" {
		addr = "localhost:50051"
	}
	if !dialReachable("tcp", addr, 2*time.Second) {
		t.Fatalf("Phloem gRPC server not reachable at %s", addr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewPhloemClient(conn)
	req := &pb.IngestRequest{
		Title:   "integration_test_doc",
		Content: "# Test\nMinimal content for Go integration test.",
	}
	resp, err := client.IngestMarkdown(ctx, req)
	if err != nil {
		t.Fatalf("IngestMarkdown: %v", err)
	}
	if resp == nil {
		t.Fatal("IngestMarkdown returned nil response")
	}
	_ = resp.GetMachineId()
	_ = resp.GetDocId()
}

func TestQdrantEnsureCollection(t *testing.T) {
	host := getEnv("QDRANT_HOST", "")
	if host == "" {
		t.Skip("QDRANT_HOST not set")
	}
	port := 6334
	if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
		port = p
	}
	client, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		t.Fatalf("Qdrant connect failed: %v", err)
	}
	ctx := context.Background()
	collection := getEnv("QDRANT_COLLECTION", "gopedia_markdown")
	err = ontologyso.EnsureQdrantCollection(ctx, client, collection, ontologyso.DefaultVectorSize)
	if err != nil {
		t.Fatalf("EnsureQdrantCollection: %v", err)
	}
}
