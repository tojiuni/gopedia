// Docker network integration tests: DOCKER_NETWORK_EXTERNAL 검증 및
// neunexus / traefik-net 내부 서비스명(typedb, qdrant, postgres, phloem-flow) 연결 검사.
// Connection failures cause test failure (no skip).
package integration

import (
	"context"
	"net"
	"os"
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

const (
	internalTypeDBHost     = "typedb"
	internalQdrantHost     = "qdrant"
	internalPostgresHost   = "postgres"
	internalPhloemAddr     = "phloem-flow:50051"
	internalTypeDBPort     = "1729"
	internalQdrantGRPCPort = 6334
	internalPostgresPort   = "5432"
)

func TestDockerNetworkExternalValid(t *testing.T) {
	v := os.Getenv("DOCKER_NETWORK_EXTERNAL")
	if v == "" {
		t.Skip("DOCKER_NETWORK_EXTERNAL not set")
	}
	switch v {
	case "neunexus", "traefik-net":
	default:
		t.Fatalf("DOCKER_NETWORK_EXTERNAL must be neunexus or traefik-net, got %q", v)
	}
}

func withInternalEnv(overrides map[string]string, fn func()) {
	prev := make(map[string]string)
	for k, v := range overrides {
		prev[k] = os.Getenv(k)
		os.Setenv(k, v)
	}
	defer func() {
		for k, p := range prev {
			if p != "" {
				os.Setenv(k, p)
			} else {
				os.Unsetenv(k)
			}
		}
	}()
	fn()
}

func TestConnectivityOverNeunexus(t *testing.T) {
	if os.Getenv("DOCKER_NETWORK_EXTERNAL") != "neunexus" {
		t.Skip("run with DOCKER_NETWORK_EXTERNAL=neunexus (e.g. inside container on neunexus)")
	}
	overrides := map[string]string{
		"TYPEDB_HOST":              internalTypeDBHost,
		"TYPEDB_PORT":              internalTypeDBPort,
		"QDRANT_HOST":              internalQdrantHost,
		"QDRANT_GRPC_PORT":         strconv.Itoa(internalQdrantGRPCPort),
		"POSTGRES_HOST":            internalPostgresHost,
		"POSTGRES_PORT":            internalPostgresPort,
		"GOPEDIA_PHLOEM_GRPC_ADDR": internalPhloemAddr,
	}
	withInternalEnv(overrides, func() {
		runConnectivityChecks(t)
	})
}

func TestConnectivityOverTraefikNet(t *testing.T) {
	if os.Getenv("DOCKER_NETWORK_EXTERNAL") != "traefik-net" {
		t.Skip("run with DOCKER_NETWORK_EXTERNAL=traefik-net (e.g. inside container on traefik-net)")
	}
	overrides := map[string]string{
		"TYPEDB_HOST":              internalTypeDBHost,
		"TYPEDB_PORT":              internalTypeDBPort,
		"QDRANT_HOST":              internalQdrantHost,
		"QDRANT_GRPC_PORT":         strconv.Itoa(internalQdrantGRPCPort),
		"POSTGRES_HOST":            internalPostgresHost,
		"POSTGRES_PORT":            internalPostgresPort,
		"GOPEDIA_PHLOEM_GRPC_ADDR": internalPhloemAddr,
	}
	withInternalEnv(overrides, func() {
		runConnectivityChecks(t)
	})
}

func runConnectivityChecks(t *testing.T) {
	t.Helper()
	ctx := context.Background()

	if host := getEnv("TYPEDB_HOST", ""); host != "" {
		addr := net.JoinHostPort(host, getEnv("TYPEDB_PORT", "1729"))
		if !dialReachable("tcp", addr, 3*time.Second) {
			t.Errorf("TypeDB not reachable at %s", addr)
		}
	}

	if host := getEnv("QDRANT_HOST", ""); host != "" {
		port := 6334
		if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
			port = p
		}
		client, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
		if err != nil {
			t.Errorf("Qdrant connect: %v", err)
		} else {
			_, err = client.CollectionExists(ctx, getEnv("QDRANT_COLLECTION", "gopedia_markdown"))
			if err != nil {
				t.Errorf("Qdrant CollectionExists: %v", err)
			}
		}
	}

	if host, user := getEnv("POSTGRES_HOST", ""), getEnv("POSTGRES_USER", ""); host != "" && user != "" {
		port := getEnv("POSTGRES_PORT", "5432")
		pass := getEnv("POSTGRES_PASSWORD", "")
		db := getEnv("POSTGRES_DB", "gopedia")
		sslmode := getEnv("POSTGRES_SSLMODE", "disable")
		connStr := "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + db + "?sslmode=" + sslmode
		pool, err := pgxpool.New(ctx, connStr)
		if err != nil {
			t.Errorf("Postgres connect: %v", err)
		} else {
			defer pool.Close()
			var exists bool
			_ = pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'documents')").Scan(&exists)
			if !exists {
				t.Error("documents table not found (run core/ontology_so/postgres_ddl.sql)")
			}
		}
	}

	addr := getEnv("GOPEDIA_PHLOEM_GRPC_ADDR", "")
	if addr == "" || addr == ":50051" {
		addr = "localhost:50051"
	}
	if !dialReachable("tcp", addr, 3*time.Second) {
		t.Errorf("Phloem not reachable at %s", addr)
	} else {
		conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Errorf("Phloem gRPC dial: %v", err)
		} else {
			defer conn.Close()
			client := pb.NewPhloemClient(conn)
			cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			resp, err := client.IngestMarkdown(cctx, &pb.IngestRequest{Title: "docker_net_test", Content: "# Test"})
			if err != nil {
				t.Errorf("Phloem IngestMarkdown: %v", err)
			} else if resp == nil {
				t.Error("Phloem IngestMarkdown returned nil")
			}
		}
	}

	if host := getEnv("QDRANT_HOST", ""); host != "" {
		port := 6334
		if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
			port = p
		}
		client, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
		if err == nil {
			err = ontologyso.EnsureQdrantCollection(ctx, client, getEnv("QDRANT_COLLECTION", "gopedia_markdown"), ontologyso.DefaultVectorSize)
			if err != nil {
				t.Errorf("EnsureQdrantCollection: %v", err)
			}
		}
	}
}
