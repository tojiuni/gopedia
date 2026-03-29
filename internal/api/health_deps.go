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
)

func checkPostgres() DepStatus {
	t0 := time.Now()
	cs := pgConnString()
	if cs == "" {
		return DepStatus{Status: "skipped", Error: "postgres not configured"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cs)
	if err != nil {
		return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), Error: err.Error()}
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), Error: err.Error()}
	}
	return DepStatus{Status: "ok", LatencyMs: elapsedMs(t0)}
}

func checkQdrant() DepStatus {
	t0 := time.Now()
	host := getEnv("QDRANT_HOST", "")
	if host == "" {
		return DepStatus{Status: "skipped", Error: "QDRANT_HOST not set"}
	}
	port := 6334
	if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
		port = p
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), Error: err.Error()}
	}
	defer client.Close()
	coll := getEnv("QDRANT_COLLECTION", "gopedia_markdown")
	_, err = client.GetCollectionInfo(ctx, coll)
	if err != nil {
		return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), Error: err.Error()}
	}
	return DepStatus{Status: "ok", LatencyMs: elapsedMs(t0)}
}

func checkTypeDB() DepStatus {
	t0 := time.Now()
	host := getEnv("TYPEDB_HOST", "")
	if host == "" {
		return DepStatus{Status: "skipped", Error: "TYPEDB_HOST not set"}
	}
	port := getEnv("TYPEDB_PORT", "1729")
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), Error: err.Error()}
	}
	_ = conn.Close()
	return DepStatus{Status: "ok", LatencyMs: elapsedMs(t0)}
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
		return DepStatus{Status: "error", LatencyMs: elapsedMs(t0), Error: err.Error()}
	}
	_ = conn.Close()
	return DepStatus{Status: "ok", LatencyMs: elapsedMs(t0)}
}

func elapsedMs(t0 time.Time) int64 {
	return time.Since(t0).Milliseconds()
}
