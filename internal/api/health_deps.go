package api

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gopedia/internal/platform/env"
)

func checkPostgres() DepStatus {
	t0 := time.Now()
	cs := env.PostgresConnString()
	if cs != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, cs)
		if err != nil {
			return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), CheckLevel: "full", Error: err.Error()}
		}
		defer pool.Close()
		if err := pool.Ping(ctx); err != nil {
			return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), CheckLevel: "full", Error: err.Error()}
		}
		return DepStatus{Status: "ok", LatencyMs: elapsedMs(t0), CheckLevel: "full"}
	}
	host := getEnv("POSTGRES_HOST", "127.0.0.1")
	port := getEnv("POSTGRES_PORT", "5432")
	return checkTCP(t0, host, port, "tcp")
}

func checkQdrant() DepStatus {
	t0 := time.Now()
	host := env.DialQdrantHost()
	if host != "" {
		port := 6334
		if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
			port = p
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
		if err != nil {
			return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), CheckLevel: "full", Error: err.Error()}
		}
		defer client.Close()
		coll := getEnv("QDRANT_COLLECTION", "gopedia_markdown")
		_, err = client.GetCollectionInfo(ctx, coll)
		if err != nil {
			return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), CheckLevel: "full", Error: err.Error()}
		}
		return DepStatus{Status: "ok", LatencyMs: elapsedMs(t0), CheckLevel: "full"}
	}
	// Best-effort fallback for environments without explicit .env.
	// Try gRPC first, then HTTP port.
	if st := checkTCP(t0, "127.0.0.1", "6334", "tcp"); st.Status == "ok" {
		return st
	}
	return checkTCP(t0, "127.0.0.1", "6333", "tcp")
}

func checkTypeDB() DepStatus {
	t0 := time.Now()
	host := env.DialTypeDBHost()
	if host == "" {
		host = "127.0.0.1"
	}
	port := getEnv("TYPEDB_PORT", "1729")
	return checkTCP(t0, host, port, "tcp")
}

func checkPhloemGRPC() DepStatus {
	t0 := time.Now()
	addr := getEnv("GOPEDIA_PHLOEM_GRPC_ADDR", ":50051")
	if addr == "" {
		return DepStatus{Status: "skipped", Error: "GOPEDIA_PHLOEM_GRPC_ADDR not set"}
	}
	if len(addr) > 0 && addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), CheckLevel: "grpc", Error: err.Error()}
	}
	_ = conn.Close()
	return DepStatus{Status: "ok", LatencyMs: elapsedMs(t0), CheckLevel: "grpc"}
}

func elapsedMs(t0 time.Time) int64 {
	return time.Since(t0).Milliseconds()
}

func checkTCP(t0 time.Time, host, port, level string) DepStatus {
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), CheckLevel: level, Error: err.Error()}
	}
	_ = conn.Close()
	return DepStatus{Status: "ok", LatencyMs: elapsedMs(t0), CheckLevel: level}
}
